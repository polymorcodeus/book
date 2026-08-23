package catalog

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/polymorcodeus/book/internal/book"
)

// MigrateReport summarizes the result of a MigrateShelfDir run.
type MigrateReport struct {
	Upgraded  int
	AlreadyV2 int
	Backups   []string
}

// MigrateOptions controls migration behavior.
type MigrateOptions struct {
	// DryRun reports what would change without writing files.
	DryRun bool
}

// MigrateShelfDir scans dir for v1 shelf TOML files and upgrades them to v2.
// If dir is inside a git worktree, the tree must be clean and no .bak files are
// written (rollback is a git operation). Otherwise a one-time .bak copy is kept
// for each upgraded file.
func MigrateShelfDir(dir string, opts MigrateOptions) (*MigrateReport, error) {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("shelf directory does not exist: %s", dir)
		}
		return nil, fmt.Errorf("stat shelf directory: %w", err)
	}

	useGit, err := isGitRepo(dir)
	if err != nil {
		return nil, err
	}
	if useGit {
		clean, err := gitIsClean(dir)
		if err != nil {
			return nil, err
		}
		if !clean {
			return nil, fmt.Errorf("shelf directory has uncommitted changes; commit or stash before migrating")
		}
	}

	globDir := fmt.Sprintf("%s/*.toml", dir)
	files, err := filepath.Glob(globDir)
	if err != nil {
		return nil, fmt.Errorf("glob shelf files: %w", err)
	}

	report := &MigrateReport{}
	for _, file := range files {
		upgraded, backup, err := migrateShelfFile(file, useGit, opts)
		if err != nil {
			return report, fmt.Errorf("migrate %s: %w", file, err)
		}
		if upgraded {
			report.Upgraded++
			if backup != "" {
				report.Backups = append(report.Backups, backup)
			}
		} else {
			report.AlreadyV2++
		}
	}

	return report, nil
}

// migrateShelfFile upgrades a single shelf file if it is v1. It returns whether
// the file was upgraded, the path to any backup written, and an error.
func migrateShelfFile(file string, useGit bool, opts MigrateOptions) (bool, string, error) {
	var shelf book.Shelf
	if _, err := toml.DecodeFile(file, &shelf); err != nil {
		return false, "", fmt.Errorf("decode: %w", err)
	}

	if shelf.IsV2() {
		return false, "", nil
	}

	if opts.DryRun {
		return true, "", nil
	}

	if !useGit {
		backup := file + ".bak"
		if _, err := os.Stat(backup); err == nil {
			return false, "", fmt.Errorf("backup already exists: %s", backup)
		}
		if err := copyFile(file, backup); err != nil {
			return false, "", fmt.Errorf("backup: %w", err)
		}
	}

	MigrateShelf(&shelf)
	shelf.FilePath = file

	if err := CreateTOML(&shelf); err != nil {
		return false, "", fmt.Errorf("write: %w", err)
	}

	backup := ""
	if !useGit {
		backup = file + ".bak"
	}
	return true, backup, nil
}

// MigrateShelf upgrades a decoded shelf to v2 in-place. IDs are assigned using
// deterministic seeds; timestamps are left empty so two machines migrating the
// same v1 file produce identical bytes.
func MigrateShelf(s *book.Shelf) {
	v2 := 2
	s.SchemaVersion = &v2
	if s.ID == "" {
		s.ID = book.GenerateShelfID(s.Name)
	}
	for _, c := range s.Collections {
		if c.ID == "" {
			c.ID = book.GenerateCollectionID(s.Name, c.Name)
		}
	}
}

// isGitRepo reports whether dir is inside a git worktree.
func isGitRepo(dir string) (bool, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	if err != nil {
		// git not installed or dir is not a repo
		return false, nil
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

// gitIsClean reports whether the git worktree containing dir has no uncommitted changes.
func gitIsClean(dir string) (bool, error) {
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return len(strings.TrimSpace(string(out))) == 0, nil
}

// copyFile copies src to dst using buffered I/O.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
