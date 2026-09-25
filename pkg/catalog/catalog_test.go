package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/polymorcodeus/book/pkg/book"
)

// TestLoadShelvesPreservesOnDiskPath ensures a shelf file whose filename no
// longer matches its shelf_name still records the real path, so a later save
// updates that file in place instead of writing a derived duplicate.
func TestLoadShelvesPreservesOnDiskPath(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{ShelfRoot: dir, CatalogFormat: "toml"}

	renamed := filepath.Join(dir, "renamed.toml")
	if err := os.WriteFile(renamed, []byte("shelf_name = \"original\"\n"), 0644); err != nil {
		t.Fatalf("write shelf: %v", err)
	}

	var shelves book.BookShelves
	if err := LoadShelves(&shelves, paths); err != nil {
		t.Fatalf("load shelves: %v", err)
	}

	shelf, ok := shelves.Shelf("original")
	if !ok {
		t.Fatal("shelf not found")
	}
	if shelf.FilePath != renamed {
		t.Fatalf("FilePath = %q, want %q", shelf.FilePath, renamed)
	}

	if err := UpdateShelfFile(shelf); err != nil {
		t.Fatalf("update shelf file: %v", err)
	}
	derived := filepath.Join(dir, "original.toml")
	if _, err := os.Stat(derived); !os.IsNotExist(err) {
		t.Fatalf("save created duplicate derived file %q", derived)
	}
}
