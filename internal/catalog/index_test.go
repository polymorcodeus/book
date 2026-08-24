package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/polymorcodeus/book/internal/book"
)

func testConfig(t *testing.T) *book.Config {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))

	cfg := &book.Config{
		ShelfRoot:     filepath.Join(dir, "shelf.d"),
		CatalogFormat: "toml",
		ConfigFile:    filepath.Join(dir, "config"),
	}
	if err := os.MkdirAll(cfg.ShelfRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func writeShelfFile(t *testing.T, cfg *book.Config, s *book.Shelf) {
	t.Helper()
	s.AddFileDetail(cfg)
	if err := CreateTOML(s); err != nil {
		t.Fatalf("write shelf file: %v", err)
	}
}

func sampleShelf() *book.Shelf {
	v2 := 2
	return &book.Shelf{
		SchemaVersion: &v2,
		ID:            book.GenerateShelfID("work"),
		Name:          "work",
		Description:   "work stuff",
		Collections: map[string]*book.Collection{
			"golang": {
				ID:          book.GenerateCollectionID("work", "golang"),
				Name:        "golang",
				Description: "go links",
				Marks: []*book.Mark{
					{ID: book.GenerateID("https://go.dev"), Name: "The Go Programming Language", URL: "https://go.dev", Tags: []string{"lang", "official"}},
					{ID: book.GenerateID("https://pkg.go.dev"), Name: "Golang patterns", URL: "https://pkg.go.dev", Tags: []string{"docs"}},
				},
			},
		},
	}
}

func TestOpenIndexCreatesSchema(t *testing.T) {
	cfg := testConfig(t)
	ix, err := OpenIndex(cfg)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = ix.Close() }()

	for _, table := range []string{"shelves", "collections", "marks", "tags", "file_meta", "marks_fts"} {
		var name string
		err := ix.db.QueryRow(`SELECT name FROM sqlite_master WHERE type IN ('table','virtual') AND name = ?`, table).Scan(&name)
		if err != nil {
			t.Errorf("table %q missing: %v", table, err)
		}
	}
}

func TestUpsertAndRead(t *testing.T) {
	cfg := testConfig(t)
	ix, err := OpenIndex(cfg)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = ix.Close() }()

	s := sampleShelf()
	writeShelfFile(t, cfg, s)
	if err := ix.UpsertShelf(s); err != nil {
		t.Fatalf("UpsertShelf: %v", err)
	}

	names, err := ix.ShelfNames()
	if err != nil {
		t.Fatalf("ShelfNames: %v", err)
	}
	if len(names) != 1 || names[0] != "work" {
		t.Fatalf("ShelfNames = %v, want [work]", names)
	}

	cols, err := ix.CollectionNames("work")
	if err != nil {
		t.Fatalf("CollectionNames: %v", err)
	}
	if len(cols) != 1 || cols[0] != "golang" {
		t.Fatalf("CollectionNames = %v, want [golang]", cols)
	}

	col, err := ix.Collection("work", "golang")
	if err != nil {
		t.Fatalf("Collection: %v", err)
	}
	if len(col.Marks) != 2 {
		t.Fatalf("Collection marks = %d, want 2", len(col.Marks))
	}
	if got := col.Marks[0].Tags; len(got) != 2 || got[0] != "lang" || got[1] != "official" {
		t.Fatalf("mark tags = %v, want [lang official]", got)
	}
}

func TestCollectionNotFound(t *testing.T) {
	cfg := testConfig(t)
	ix, err := OpenIndex(cfg)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = ix.Close() }()

	if _, err := ix.CollectionNames("missing"); err == nil {
		t.Fatal("CollectionNames(missing) = nil error, want not-found error")
	}
	if _, err := ix.Collection("work", "nope"); err == nil {
		t.Fatal("Collection(work, nope) = nil error, want not-found error")
	}
}

func TestRebuildFromDisk(t *testing.T) {
	cfg := testConfig(t)
	writeShelfFile(t, cfg, sampleShelf())

	ix, err := OpenIndex(cfg)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = ix.Close() }()

	report, err := ix.Rebuild(cfg)
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if report.Indexed != 1 {
		t.Fatalf("Rebuild indexed = %d, want 1", report.Indexed)
	}

	names, err := ix.ShelfNames()
	if err != nil {
		t.Fatalf("ShelfNames: %v", err)
	}
	if len(names) != 1 || names[0] != "work" {
		t.Fatalf("ShelfNames = %v, want [work]", names)
	}
}

func TestSyncIncrementalAndPrune(t *testing.T) {
	cfg := testConfig(t)
	writeShelfFile(t, cfg, sampleShelf())

	ix, err := OpenIndex(cfg)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = ix.Close() }()

	if _, err := ix.Rebuild(cfg); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	// No changes: everything unchanged, nothing reindexed.
	report, err := ix.Sync(cfg)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if report.Reindexed != 0 || report.Unchanged != 1 || report.Removed != 0 {
		t.Fatalf("Sync = %+v, want unchanged only", report)
	}

	// Add a mark and rewrite: should reindex exactly one file.
	s := sampleShelf()
	s.Collections["golang"].Marks = append(s.Collections["golang"].Marks,
		&book.Mark{ID: book.GenerateID("https://example.com"), Name: "Example", URL: "https://example.com", Tags: []string{"misc"}})
	writeShelfFile(t, cfg, s)

	report, err = ix.Sync(cfg)
	if err != nil {
		t.Fatalf("Sync after change: %v", err)
	}
	if report.Reindexed != 1 {
		t.Fatalf("Sync reindexed = %d, want 1", report.Reindexed)
	}

	col, err := ix.Collection("work", "golang")
	if err != nil {
		t.Fatalf("Collection: %v", err)
	}
	if len(col.Marks) != 3 {
		t.Fatalf("Collection marks = %d, want 3", len(col.Marks))
	}

	// Remove the file: should prune exactly one shelf.
	if err := os.Remove(s.FilePath); err != nil {
		t.Fatal(err)
	}
	report, err = ix.Sync(cfg)
	if err != nil {
		t.Fatalf("Sync after remove: %v", err)
	}
	if report.Removed != 1 {
		t.Fatalf("Sync removed = %d, want 1", report.Removed)
	}
	names, err := ix.ShelfNames()
	if err != nil {
		t.Fatalf("ShelfNames: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("ShelfNames = %v, want empty", names)
	}
}

func TestSyncIgnoresContentUnchangedTouch(t *testing.T) {
	cfg := testConfig(t)
	writeShelfFile(t, cfg, sampleShelf())

	ix, err := OpenIndex(cfg)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = ix.Close() }()

	if _, err := ix.Rebuild(cfg); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	// Rewrite identical content: mtime changes but hash does not.
	writeShelfFile(t, cfg, sampleShelf())

	report, err := ix.Sync(cfg)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if report.Reindexed != 0 {
		t.Fatalf("Sync reindexed = %d, want 0 (hash unchanged)", report.Reindexed)
	}
}

func TestSearch(t *testing.T) {
	cfg := testConfig(t)
	writeShelfFile(t, cfg, sampleShelf())

	ix, err := OpenIndex(cfg)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = ix.Close() }()

	if _, err := ix.Rebuild(cfg); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	results, err := ix.Search("golang", "", "", nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search(golang) = %d results, want 1", len(results))
	}
	if results[0].Title != "Golang patterns" || results[0].Shelf != "work" || results[0].Collection != "golang" {
		t.Fatalf("Search result = %+v", results[0])
	}

	// URL token search should also match.
	results, err = ix.Search("pkg", "", "", nil)
	if err != nil {
		t.Fatalf("Search(pkg): %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search(pkg) = %d results, want 1", len(results))
	}
}

func TestSearchTagFilter(t *testing.T) {
	cfg := testConfig(t)
	writeShelfFile(t, cfg, sampleShelf())

	ix, err := OpenIndex(cfg)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = ix.Close() }()

	if _, err := ix.Rebuild(cfg); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	// "go" matches both marks; OR filter (lang OR docs) keeps both.
	results, err := ix.Search("go", "", "", [][]string{{"lang", "docs"}})
	if err != nil {
		t.Fatalf("Search OR: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Search OR = %d results, want 2", len(results))
	}

	// Single tag filter narrows to the mark carrying "lang".
	results, err = ix.Search("go", "", "", [][]string{{"lang"}})
	if err != nil {
		t.Fatalf("Search single tag: %v", err)
	}
	if len(results) != 1 || results[0].Title != "The Go Programming Language" {
		t.Fatalf("Search single tag = %+v, want mark with lang", results)
	}

	// AND filter (lang AND official) matches only mark 1.
	results, err = ix.Search("go", "", "", [][]string{{"lang"}, {"official"}})
	if err != nil {
		t.Fatalf("Search AND: %v", err)
	}
	if len(results) != 1 || results[0].Title != "The Go Programming Language" {
		t.Fatalf("Search AND = %+v, want mark with lang+official", results)
	}

	// AND filter with no overlap matches nothing.
	results, err = ix.Search("go", "", "", [][]string{{"lang"}, {"docs"}})
	if err != nil {
		t.Fatalf("Search AND empty: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Search AND empty = %d results, want 0", len(results))
	}
}

func TestSearchTagsOnly(t *testing.T) {
	cfg := testConfig(t)
	writeShelfFile(t, cfg, sampleShelf())

	ix, err := OpenIndex(cfg)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = ix.Close() }()

	if _, err := ix.Rebuild(cfg); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	// Empty query with a tag filter returns every mark carrying that tag.
	results, err := ix.Search("", "", "", [][]string{{"docs"}})
	if err != nil {
		t.Fatalf("Search tags-only: %v", err)
	}
	if len(results) != 1 || results[0].Title != "Golang patterns" {
		t.Fatalf("Search tags-only = %+v, want mark with docs", results)
	}

	// Empty query with a shelf filter returns every mark in that shelf.
	results, err = ix.Search("", "work", "", nil)
	if err != nil {
		t.Fatalf("Search shelf-only: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Search shelf-only = %d results, want 2", len(results))
	}
}

func TestSearchExcludesSoftDeleted(t *testing.T) {
	cfg := testConfig(t)

	v2 := 2
	s := &book.Shelf{
		SchemaVersion: &v2,
		ID:            book.GenerateShelfID("work"),
		Name:          "work",
		Collections: map[string]*book.Collection{
			"golang": {
				ID:   book.GenerateCollectionID("work", "golang"),
				Name: "golang",
				Marks: []*book.Mark{
					{ID: book.GenerateID("https://go.dev"), Name: "The Go Programming Language", URL: "https://go.dev", Tags: []string{"lang"}},
					{ID: book.GenerateID("https://pkg.go.dev"), Name: "Golang patterns", URL: "https://pkg.go.dev", Tags: []string{"docs"}, DeletedAt: book.NowTimestamp()},
				},
			},
		},
	}
	writeShelfFile(t, cfg, s)

	ix, err := OpenIndex(cfg)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer func() { _ = ix.Close() }()

	if _, err := ix.Rebuild(cfg); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	results, err := ix.Search("go", "", "", nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 || results[0].Title != "The Go Programming Language" {
		t.Fatalf("Search excluded soft-deleted = %+v, want only the non-deleted mark", results)
	}
}
