package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/BurntSushi/toml"

	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/theme"
)

func VerifyExists(filename string) (bool, error) {
	_, err := os.Stat(filename)
	if err == nil {
		return true, nil // File or directory exists
	}

	if errors.Is(err, os.ErrNotExist) {
		return false, nil // File does exist
	}
	return false, err // remaining errors
}

func CreateTOML(t book.TOMLFile) (err error) {
	path := t.FileDetail()

	// Resolve symlinks so we write to the real target, not replace the link.
	writePath, err := resolveWritePath(path)
	if err != nil {
		return err
	}
	tmpPath := writePath + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	// Cleanup runs only if we failed before the final rename.
	// os.Remove error is explicitly ignored because this is best-effort.
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if err = toml.NewEncoder(f).Encode(t); err != nil {
		return err
	}

	return os.Rename(tmpPath, writePath)
}

func UpdateShelfFile(s *book.Shelf) error {
	return CreateTOML(s)
}

func EnsureConfig(c *book.Config) error {
	exists, err := VerifyExists(c.ConfigFile)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if !c.Autoconfirm {
		return fmt.Errorf("%s", fmt.Sprintf("set --confirm to create config file %s", c.ConfigFile))
	}

	fileCfg := &book.FileConfig{
		CatalogFormat: c.CatalogFormat,
		ShelfRoot:     c.ShelfRoot,
		Autoconfirm:   nil,
		Interactive:   nil,
		ThemeFile:     c.ThemeFile,
		ConfigFile:    c.ConfigFile,
		TemplateFile:  c.TemplateFile,
	}

	return CreateTOML(fileCfg)
}

// PrintConfigSources prints rendered configuration when config file is present
func PrintConfigSources(config *book.Config) error {
	var fileCfg book.FileConfig
	if _, err := toml.DecodeFile(config.ConfigFile, &fileCfg); err != nil {
		return err
	}

	heading := config.Theme.Style("highlight").Render("book config file exists - rendered configuration")
	fmt.Println(heading)
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "FIELD\tFILE VALUE\tEFFECTIVE")
	_, _ = fmt.Fprintln(w, "-----\t----------\t---------")

	// bool helper
	printBool := func(name string, fileVal *bool, effVal bool) {
		var f string
		if fileVal == nil {
			f = "<unset>"
		} else {
			f = fmt.Sprintf("%t", *fileVal)
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%t\n", name, f, effVal)
	}

	printBool("autoconfirm", fileCfg.Autoconfirm, config.Autoconfirm)
	printBool("interactive", fileCfg.Interactive, config.Interactive)

	// string helper
	printString := func(name, fileVal, effVal string) {
		if fileVal == "" {
			fileVal = "<unset>"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", name, fileVal, effVal)
	}

	printString("catalog_format", fileCfg.CatalogFormat, config.CatalogFormat)
	printString("shelf_directory", fileCfg.ShelfRoot, config.ShelfRoot)
	printString("theme_file", fileCfg.ThemeFile, config.ThemeFile)
	printString("template_file", fileCfg.TemplateFile, config.TemplateFile)

	_ = w.Flush()
	return nil
}

// resolveWritePath returns the real filesystem path that should be written to.
// If path is a symlink, it follows the link and also resolves any directory
// symlinks in the parent directories of the target.
func resolveWritePath(path string) (string, error) {
	// Anchor to an absolute path first. If the path is relative,
	// filepath.Dir will return "." and relative symlink targets
	// will be resolved against the wrong base.
	if !filepath.IsAbs(path) {
		var err error
		path, err = filepath.Abs(path)
		if err != nil {
			return "", err
		}
	}

	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, nil
		}
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return path, nil
	}

	// Follow the symlink chain for the final path component.
	const maxHops = 16
	current := path
	for range maxHops {
		target, err := os.Readlink(current)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(current), target)
		}
		current = filepath.Clean(target)

		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				break
			}
			return "", err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			break
		}
	}

	// Resolve all symlinks in the path, including directory symlinks
	// like personal.lnk in the middle of the path.
	realPath, err := filepath.EvalSymlinks(current)
	if err == nil {
		return realPath, nil
	}

	// If the final component doesn't exist (dangling symlink),
	// resolve the parent directory and append the basename.
	dir := filepath.Dir(current)
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		// Can't resolve parent either. Return the cleaned path;
		// the kernel will follow directory symlinks at runtime.
		return current, nil
	}
	return filepath.Join(realDir, filepath.Base(current)), nil
}

// DumpDefaultTheme serialises the built-in defaults as indented JSON.
func DumpDefaults(config *book.Config, dump string) (err error) {
	var jsonData []byte
	var (
		tmpPath,
		writePath string
	)

	switch dump {
	case "theme":
		writePath, err = resolveWritePath(config.ThemeFile) // = not :=
		if err != nil {
			return err
		}
		tmpPath = writePath + ".tmp"

		if jsonData, err = json.MarshalIndent(theme.DefaultThemeConfig(), "", "  "); err != nil {
			return err
		}
	case "template":
		writePath, err = resolveWritePath(config.TemplateFile) // = not :=
		if err != nil {
			return err
		}
		tmpPath = writePath + ".tmp"

		if jsonData, err = json.MarshalIndent(book.DefaultViewTemplates, "", "  "); err != nil {
			return err
		}
	}

	if !config.Autoconfirm {
		return fmt.Errorf("set --confirm to create %s file %s", dump, tmpPath)
	}

	if exists, _ := VerifyExists(writePath); exists {
		return os.ErrExist
	}

	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err = f.Write(jsonData); err != nil {
		return err
	}

	return os.Rename(tmpPath, writePath)
}
