package catalog

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/polymorcodeus/book/internal/book"
)

// fixtureMigratedTOML is the expected v2 output after migrating fixtureTOML.
// Migrated rows carry deterministic IDs but leave timestamps empty.
const fixtureMigratedTOML = `schema_version = 2
shelf_id = "0eb3e36b"
shelf_name = "archive"
shelf_desc = "where books go to die!"

[Collections]
  [Collections.powash311]
    collection_id = "6d264600"
    collection_name = "powash311"
    collection_desc = "pow pow powashell"

    [[Collections.powash311.marks]]
      catalog_id = "21f96eef"
      title = "counteractive/o365beat: Elastic Beat for fetching and shipping Office 365 audit events"
      url = "https://github.com/counteractive/o365beat"
      tags = ["powershell", "windows"]

    [[Collections.powash311.marks]]
      catalog_id = "e1c3808c"
      title = "Introduction to Testing Your PowerShell Code with Pester - Simple Talk"
      url = "https://www.red-gate.com/simple-talk/sysadmin/powershell/introduction-to-testing-your-powershell-code-with-pester/"
      tags = ["powershell", "windows"]

  [Collections.swyfty]
    collection_id = "3d8b97ac"
    collection_name = "swyfty"
    collection_desc = "getting schhwifty"

    [[Collections.swyfty.marks]]
      catalog_id = "fde869ba"
      title = "Make an API call - Box Developer Documentation"
      url = "https://developer.box.com/guides/mobile/ios/quick-start/make-api-call/"
      tags = ["swift", "ios", "api"]

    [[Collections.swyfty.marks]]
      catalog_id = "9af83e1a"
      title = "Implement an API client in Swift using Generics, Codable and Combine | by Marina Sauca | Mac O’Clock | Medium"
      url = "https://medium.com/macoclock/swift-generic-api-854afdb9315e"
      tags = ["swift", "ios", "api"]
`

func TestMigrateShelfDirGolden(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "archive.toml")
	if err := os.WriteFile(path, []byte(fixtureTOML), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	report, err := MigrateShelfDir(dir, MigrateOptions{})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if report.Upgraded != 1 {
		t.Errorf("Upgraded = %d, want 1", report.Upgraded)
	}
	if report.AlreadyV2 != 0 {
		t.Errorf("AlreadyV2 = %d, want 0", report.AlreadyV2)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated file: %v", err)
	}

	want := normalizeTOMLWhitespace([]byte(fixtureMigratedTOML))
	gotNorm := normalizeTOMLWhitespace(got)
	if !bytes.Equal(gotNorm, want) {
		t.Logf("want (%d bytes):\n%s", len(want), want)
		t.Logf("got  (%d bytes):\n%s", len(gotNorm), gotNorm)
		t.Errorf("migrated output mismatch")
	}
}

func TestMigrateShelfDirIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "archive.toml")
	if err := os.WriteFile(path, []byte(fixtureTOML), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	first, err := MigrateShelfDir(dir, MigrateOptions{})
	if err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if first.Upgraded != 1 {
		t.Errorf("first Upgraded = %d, want 1", first.Upgraded)
	}

	afterFirst, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after first: %v", err)
	}

	second, err := MigrateShelfDir(dir, MigrateOptions{})
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if second.Upgraded != 0 {
		t.Errorf("second Upgraded = %d, want 0", second.Upgraded)
	}
	if second.AlreadyV2 != 1 {
		t.Errorf("second AlreadyV2 = %d, want 1", second.AlreadyV2)
	}

	afterSecond, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after second: %v", err)
	}
	if !bytes.Equal(afterFirst, afterSecond) {
		t.Errorf("second migrate changed the file")
	}
}

func TestMigrateShelfDirBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "archive.toml")
	if err := os.WriteFile(path, []byte(fixtureTOML), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	report, err := MigrateShelfDir(dir, MigrateOptions{})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(report.Backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(report.Backups))
	}

	backup := path + ".bak"
	if report.Backups[0] != backup {
		t.Errorf("backup path = %q, want %q", report.Backups[0], backup)
	}

	got, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !bytes.Equal(got, []byte(fixtureTOML)) {
		t.Errorf("backup does not match original")
	}
}

func TestMigrateShelfDirGitClean(t *testing.T) {
	if !gitAvailable(t) {
		t.Skip("git not available")
	}

	dir := initGitRepo(t)
	path := filepath.Join(dir, "archive.toml")
	if err := os.WriteFile(path, []byte(fixtureTOML), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	gitCommit(t, dir, "initial v1 shelf")

	report, err := MigrateShelfDir(dir, MigrateOptions{})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if report.Upgraded != 1 {
		t.Errorf("Upgraded = %d, want 1", report.Upgraded)
	}
	if len(report.Backups) != 0 {
		t.Errorf("expected no backups in git repo, got %v", report.Backups)
	}

	backup := path + ".bak"
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Errorf("backup %s should not exist in git repo", backup)
	}
}

func TestMigrateShelfDirGitDirty(t *testing.T) {
	if !gitAvailable(t) {
		t.Skip("git not available")
	}

	dir := initGitRepo(t)
	path := filepath.Join(dir, "archive.toml")
	if err := os.WriteFile(path, []byte(fixtureTOML), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	gitCommit(t, dir, "initial v1 shelf")

	// Make the tree dirty.
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	_, err := MigrateShelfDir(dir, MigrateOptions{})
	if err == nil {
		t.Fatalf("expected error for dirty git tree")
	}
	if !strings.Contains(err.Error(), "uncommitted") {
		t.Errorf("error message does not mention uncommitted changes: %v", err)
	}
}

func TestMigrateShelf(t *testing.T) {
	shelf := book.Shelf{
		Name: "archive",
		Collections: map[string]*book.Collection{
			"powash311": {Name: "powash311"},
		},
	}

	MigrateShelf(&shelf)

	if shelf.SchemaVersion == nil || *shelf.SchemaVersion != 2 {
		t.Errorf("SchemaVersion = %v, want 2", shelf.SchemaVersion)
	}
	if shelf.ID != book.GenerateShelfID("archive") {
		t.Errorf("ID = %q, want %q", shelf.ID, book.GenerateShelfID("archive"))
	}
	if shelf.CreatedAt != "" {
		t.Errorf("CreatedAt should be empty for migrated shelf, got %q", shelf.CreatedAt)
	}

	col := shelf.Collections["powash311"]
	wantColID := book.GenerateCollectionID("archive", "powash311")
	if col.ID != wantColID {
		t.Errorf("collection ID = %q, want %q", col.ID, wantColID)
	}
	if col.CreatedAt != "" {
		t.Errorf("collection CreatedAt should be empty, got %q", col.CreatedAt)
	}
}

func TestRoundTripIdempotencyMigrated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "archive.toml")
	if err := os.WriteFile(path, []byte(fixtureTOML), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if _, err := MigrateShelfDir(dir, MigrateOptions{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var shelf book.Shelf
	if _, err := toml.DecodeFile(path, &shelf); err != nil {
		t.Fatalf("decode migrated file: %v", err)
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(shelf); err != nil {
		t.Fatalf("encode migrated shelf: %v", err)
	}

	want := normalizeTOMLWhitespace([]byte(fixtureMigratedTOML))
	got := normalizeTOMLWhitespace(buf.Bytes())
	if !bytes.Equal(got, want) {
		t.Logf("want (%d bytes):\n%s", len(want), want)
		t.Logf("got  (%d bytes):\n%s", len(got), got)
		t.Errorf("migrated round-trip mismatch")
	}
}

func gitAvailable(t *testing.T) bool {
	t.Helper()
	_, err := exec.LookPath("git")
	return err == nil
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	return dir
}

func gitCommit(t *testing.T, dir, message string) {
	t.Helper()
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", message)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
