package catalog

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/polymorcodeus/book/internal/book"
)

const v1Fixture = `shelf_name = "work"

[Collections]
  [Collections.dev]
    collection_name = "dev"

    [[Collections.dev.marks]]
      title = "one"
      url = "https://a"
`

const v2Fixture = `schema_version = 2
shelf_id = "12345678"
shelf_name = "work"

[Collections]
  [Collections.dev]
    collection_id = "87654321"
    collection_name = "dev"
`

func TestV1ShelfFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "work.toml"), []byte(v1Fixture), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "home.toml"), []byte(v2Fixture), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := V1ShelfFiles(dir, "toml")
	if err != nil {
		t.Fatalf("V1ShelfFiles: %v", err)
	}
	want := []string{filepath.Join(dir, "work.toml")}
	if !slices.Equal(got, want) {
		t.Errorf("V1ShelfFiles() = %v, want %v", got, want)
	}
}

func TestV1ShelfFilesNone(t *testing.T) {
	dir := t.TempDir()
	got, err := V1ShelfFiles(dir, "toml")
	if err != nil {
		t.Fatalf("V1ShelfFiles: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("V1ShelfFiles() = %v, want none", got)
	}
}

func TestStrayDebris(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.toml.tmp", "b.toml.bak", "c.toml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := StrayDebris(dir, "toml")
	if err != nil {
		t.Fatalf("StrayDebris: %v", err)
	}
	want := []string{
		filepath.Join(dir, "a.toml.tmp"),
		filepath.Join(dir, "b.toml.bak"),
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("StrayDebris() = %v, want %v", got, want)
	}
}

func TestIndexStaleFiles(t *testing.T) {
	cfg := testConfig(t)
	ix, err := OpenIndex(cfg)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = ix.Close() }()

	s := sampleShelf()
	writeShelfFile(t, cfg, s)

	// Not yet indexed: the file should be reported stale.
	stale, err := ix.StaleFiles(cfg)
	if err != nil {
		t.Fatalf("StaleFiles before sync: %v", err)
	}
	if !slices.Equal(stale, []string{s.FilePath}) {
		t.Fatalf("StaleFiles before sync = %v, want %v", stale, []string{s.FilePath})
	}

	// After a sync the index is current.
	if _, err := ix.Sync(cfg); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	stale, err = ix.StaleFiles(cfg)
	if err != nil {
		t.Fatalf("StaleFiles after sync: %v", err)
	}
	if len(stale) != 0 {
		t.Fatalf("StaleFiles after sync = %v, want empty", stale)
	}

	// Modifying the file marks it stale again.
	s.Collections["golang"].Marks = append(s.Collections["golang"].Marks,
		&book.Mark{ID: book.GenerateID("https://example.com"), Name: "New", URL: "https://example.com"})
	writeShelfFile(t, cfg, s)
	stale, err = ix.StaleFiles(cfg)
	if err != nil {
		t.Fatalf("StaleFiles after modify: %v", err)
	}
	if !slices.Equal(stale, []string{s.FilePath}) {
		t.Fatalf("StaleFiles after modify = %v, want %v", stale, []string{s.FilePath})
	}

	// Removing the file is also reported as stale.
	if err := os.Remove(s.FilePath); err != nil {
		t.Fatal(err)
	}
	stale, err = ix.StaleFiles(cfg)
	if err != nil {
		t.Fatalf("StaleFiles after remove: %v", err)
	}
	if !slices.Equal(stale, []string{s.FilePath}) {
		t.Fatalf("StaleFiles after remove = %v, want %v", stale, []string{s.FilePath})
	}
}
