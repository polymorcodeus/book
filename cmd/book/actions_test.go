package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/polymorcodeus/book/internal/theme"
	"github.com/polymorcodeus/book/pkg/book"
	"github.com/polymorcodeus/book/pkg/catalog"
	"github.com/polymorcodeus/book/pkg/web"
)

func testConfig(t *testing.T) *theme.UIConfig {
	t.Helper()
	tmp := t.TempDir()
	return &theme.UIConfig{
		Config: &book.Config{
			CatalogFormat: "toml",
			ShelfRoot:     tmp,
			ConfigFile:    filepath.Join(tmp, "config"),
		},
	}
}

func loadShelves(t *testing.T, config *theme.UIConfig) *book.BookShelves {
	t.Helper()
	var bs book.BookShelves
	if err := catalog.LoadShelves(&bs, catalog.PathsFromConfig(config.Config)); err != nil {
		t.Fatalf("load shelves: %v", err)
	}
	return &bs
}

func testShelf(t *testing.T, bs *book.BookShelves, name string) *book.Shelf {
	t.Helper()
	shelf, ok := bs.Shelf(name)
	if !ok {
		t.Fatalf("shelf %q not found", name)
	}
	return shelf
}

func seedShelf(t *testing.T, config *theme.UIConfig, name, collection string) *book.BookShelves {
	t.Helper()
	bs := &book.BookShelves{}
	if err := addShelf(bs, name, "test shelf", config); err != nil {
		t.Fatalf("seed shelf: %v", err)
	}
	if collection != "" {
		shelf := testShelf(t, bs, name)
		col, err := book.NewCollection(shelf, collection, "")
		if err != nil {
			t.Fatalf("seed collection: %v", err)
		}
		shelf.AddCollection(col)
		if err := catalog.UpdateShelfFile(shelf); err != nil {
			t.Fatalf("persist seeded collection: %v", err)
		}
	}
	return loadShelves(t, config)
}

func TestAddShelf(t *testing.T) {
	config := testConfig(t)
	bs := &book.BookShelves{}

	if err := addShelf(bs, "dev", "software bookmarks", config); err != nil {
		t.Fatalf("addShelf error: %v", err)
	}

	if len(*bs) != 1 {
		t.Fatalf("got %d shelves, want 1", len(*bs))
	}
	shelf := testShelf(t, bs, "dev")
	if shelf.Name != "dev" {
		t.Errorf("Name = %q, want dev", shelf.Name)
	}
	if shelf.FilePath == "" {
		t.Error("FilePath not set")
	}

	// File should exist on disk.
	if _, err := os.Stat(shelf.FilePath); err != nil {
		t.Errorf("shelf file not created: %v", err)
	}

	// Duplicate name should fail.
	if err := addShelf(bs, "dev", "", config); err == nil {
		t.Error("expected error for duplicate shelf name")
	}
}

func TestRemoveShelf(t *testing.T) {
	config := testConfig(t)
	bs := seedShelf(t, config, "dev", "")

	// Missing confirm should not remove anything.
	if err := removeShelf(bs, "dev", false); err == nil {
		t.Fatal("expected error without --confirm")
	}
	if _, err := os.Stat(testShelf(t, bs, "dev").FilePath); err != nil {
		t.Fatal("shelf file removed before confirm")
	}

	if err := removeShelf(bs, "dev", true); err != nil {
		t.Fatalf("removeShelf error: %v", err)
	}
	if len(*bs) != 0 {
		t.Errorf("got %d shelves, want 0", len(*bs))
	}
	if _, err := os.Stat(filepath.Join(config.ShelfRoot, "dev.toml")); !os.IsNotExist(err) {
		t.Error("shelf file still exists after remove")
	}

	// Removing missing shelf should error.
	if err := removeShelf(bs, "missing", true); err == nil {
		t.Error("expected error for missing shelf")
	}
}

func TestAddCollection(t *testing.T) {
	config := testConfig(t)
	bs := seedShelf(t, config, "dev", "")

	if err := addCollection(bs, "dev", "docs", "documentation", config); err != nil {
		t.Fatalf("addCollection error: %v", err)
	}

	shelf := testShelf(t, bs, "dev")
	if shelf.Collection("docs") == nil {
		t.Fatal("collection not found in memory")
	}

	reloaded := loadShelves(t, config)
	if testShelf(t, reloaded, "dev").Collection("docs") == nil {
		t.Fatal("collection not persisted")
	}

	// Missing shelf should error.
	if err := addCollection(bs, "missing", "docs", "", config); err == nil {
		t.Error("expected error for missing shelf")
	}
}

func TestRemoveCollection(t *testing.T) {
	config := testConfig(t)
	bs := seedShelf(t, config, "dev", "docs")

	if err := removeCollection(bs, "dev", "docs", true); err != nil {
		t.Fatalf("removeCollection error: %v", err)
	}

	if testShelf(t, bs, "dev").Collection("docs") != nil {
		t.Error("collection still in memory")
	}

	reloaded := loadShelves(t, config)
	if testShelf(t, reloaded, "dev").Collection("docs") != nil {
		t.Error("collection not removed from disk")
	}

	// Missing confirm should error.
	if err := removeCollection(bs, "dev", "other", false); err == nil {
		t.Error("expected error without --confirm")
	}
}

func TestAddMark(t *testing.T) {
	config := testConfig(t)
	bs := seedShelf(t, config, "dev", "docs")

	if err := addMark(context.Background(), bs, "https://example.com", "go,cli", "dev", "docs", "Example", config); err != nil {
		t.Fatalf("addMark error: %v", err)
	}

	reloaded := loadShelves(t, config)
	collection := testShelf(t, reloaded, "dev").Collection("docs")
	if len(collection.Marks) != 1 {
		t.Fatalf("got %d marks, want 1", len(collection.Marks))
	}
	mark := collection.Marks[0]
	if mark.Title != "Example" {
		t.Errorf("Title = %q, want Example", mark.Title)
	}
	if !strings.EqualFold(strings.Join(mark.Tags, ","), "go,cli") {
		t.Errorf("Tags = %v, want [go cli]", mark.Tags)
	}

	// Duplicate URL should fail.
	if err := addMark(context.Background(), bs, "https://example.com", "", "dev", "docs", "", config); err == nil {
		t.Error("expected error for duplicate URL")
	}
}

func TestGetMark(t *testing.T) {
	config := testConfig(t)
	bs := seedShelf(t, config, "dev", "docs")
	if err := addMark(context.Background(), bs, "https://example.com", "go", "dev", "docs", "Example", config); err != nil {
		t.Fatalf("addMark error: %v", err)
	}
	bs = loadShelves(t, config)
	mark := testShelf(t, bs, "dev").Collection("docs").Marks[0]

	// By ID.
	if err := getMark(bs, mark.ID, "", "", config); err != nil {
		t.Fatalf("getMark by id error: %v", err)
	}

	// By URL.
	if err := getMark(bs, "", mark.URL, "", config); err != nil {
		t.Fatalf("getMark by url error: %v", err)
	}

	// Missing both should error in non-interactive mode.
	if err := getMark(bs, "", "", "", &theme.UIConfig{Config: &book.Config{Interactive: false}}); err == nil {
		t.Error("expected error when id and url are empty")
	}

	// Missing mark should error.
	if err := getMark(bs, "badid", "", "", config); err == nil {
		t.Error("expected error for missing mark")
	}
}

func TestEditMark(t *testing.T) {
	config := testConfig(t)
	bs := seedShelf(t, config, "dev", "docs")
	if err := addMark(context.Background(), bs, "https://example.com", "go", "dev", "docs", "Example", config); err != nil {
		t.Fatalf("addMark error: %v", err)
	}
	bs = loadShelves(t, config)
	mark := testShelf(t, bs, "dev").Collection("docs").Marks[0]

	if err := editMark(bs, mark.ID, "Updated", "go,cli", "", config); err != nil {
		t.Fatalf("editMark error: %v", err)
	}

	reloaded := loadShelves(t, config)
	updated := testShelf(t, reloaded, "dev").Collection("docs").Marks[0]
	if updated.Title != "Updated" {
		t.Errorf("Title = %q, want Updated", updated.Title)
	}
	if !strings.EqualFold(strings.Join(updated.Tags, ","), "go,cli") {
		t.Errorf("Tags = %v, want [go cli]", updated.Tags)
	}
}

func TestEditMarkURLCollision(t *testing.T) {
	config := testConfig(t)
	bs := seedShelf(t, config, "dev", "docs")
	if err := addMark(context.Background(), bs, "https://one.example.com", "", "dev", "docs", "One", config); err != nil {
		t.Fatalf("addMark one error: %v", err)
	}
	if err := addMark(context.Background(), bs, "https://two.example.com", "", "dev", "docs", "Two", config); err != nil {
		t.Fatalf("addMark two error: %v", err)
	}
	bs = loadShelves(t, config)
	marks := testShelf(t, bs, "dev").Collection("docs").Marks
	oneID := marks[0].ID

	// Changing mark one to mark two's URL should fail before mutating.
	if err := editMark(bs, oneID, "", "", "https://two.example.com", config); err == nil {
		t.Fatal("expected error for URL collision")
	}

	reloaded := loadShelves(t, config)
	one := reloaded.FindMarkByID(oneID)
	if one.URL != "https://one.example.com" {
		t.Errorf("mark mutated before collision check: URL = %q", one.URL)
	}
	if one.ID != oneID {
		t.Errorf("mark ID mutated before collision check: ID = %q", one.ID)
	}
}

func TestRemoveMark(t *testing.T) {
	config := testConfig(t)
	bs := seedShelf(t, config, "dev", "docs")
	if err := addMark(context.Background(), bs, "https://example.com", "", "dev", "docs", "Example", config); err != nil {
		t.Fatalf("addMark error: %v", err)
	}
	bs = loadShelves(t, config)
	mark := testShelf(t, bs, "dev").Collection("docs").Marks[0]

	if err := removeMark(bs, mark.ID, true, config); err != nil {
		t.Fatalf("removeMark error: %v", err)
	}

	reloaded := loadShelves(t, config)
	removed := reloaded.FindMarkByID(mark.ID)
	if removed == nil || !removed.IsDeleted() {
		t.Error("mark was not soft-deleted")
	}

	// Missing confirm should error.
	if err := removeMark(bs, "any", false, config); err == nil {
		t.Error("expected error without --confirm")
	}
}

func TestRequireFlag(t *testing.T) {
	if err := requireFlag("name", "value"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := requireFlag("name", ""); err == nil {
		t.Error("expected error for empty value")
	} else if !strings.Contains(err.Error(), "--name") {
		t.Errorf("error message missing flag name: %v", err)
	}
}

func TestRequireFlags(t *testing.T) {
	if err := requireFlags("shelf", "dev", "name", "docs"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := requireFlags("shelf", "", "name", ""); err == nil {
		t.Fatal("expected error for missing flags")
	} else {
		msg := err.Error()
		if !strings.Contains(msg, "--shelf") || !strings.Contains(msg, "--name") {
			t.Errorf("error message missing flags: %v", err)
		}
	}
}

func TestWebsiteError(t *testing.T) {
	notFound := websiteError(fmt.Errorf("fetch title: %w: %s", web.ErrNotFound, "https://example.com"), "https://example.com")
	if !strings.Contains(notFound.Error(), "4oh4") {
		t.Errorf("websiteError(ErrNotFound) = %q, want the 4oh4 quip", notFound)
	}
	if !strings.Contains(notFound.Error(), "https://example.com") {
		t.Errorf("websiteError(ErrNotFound) = %q, want the url", notFound)
	}

	other := websiteError(errors.New("connection refused"), "https://example.com")
	if strings.Contains(other.Error(), "4oh4") {
		t.Errorf("websiteError(other) = %q, want a plain wrapped error", other)
	}
	if !strings.Contains(other.Error(), "load website:") {
		t.Errorf("websiteError(other) = %q, want the load website context", other)
	}
}

func TestUniqueURLError(t *testing.T) {
	trashed := &book.Mark{ID: "abc12345", Title: "gone", URL: "https://example.com/trashed", DeletedAt: book.NowTimestamp()}
	active := &book.Mark{ID: "def67890", Title: "here", URL: "https://example.com/active"}
	bs := book.BookShelves{{
		Name: "dev",
		Collections: map[string]*book.Collection{
			"docs": {Name: "docs", Marks: []*book.Mark{trashed, active}},
		},
	}}

	trashedErr := bs.VerifyUniqueURL("abc12345", nil)
	if trashedErr == nil {
		t.Fatal("VerifyUniqueURL(trashed) expected error")
	}
	got := uniqueURLError(&bs, "abc12345", trashedErr)
	if !strings.Contains(got.Error(), "book mark restore --id abc12345") {
		t.Errorf("uniqueURLError(trashed) = %q, want the restore command with id", got)
	}
	if !errors.Is(got, book.ErrURLTrashed) {
		t.Errorf("uniqueURLError(trashed) lost the ErrURLTrashed chain: %v", got)
	}

	dupErr := bs.VerifyUniqueURL("def67890", nil)
	if dupErr == nil {
		t.Fatal("VerifyUniqueURL(active) expected error")
	}
	got = uniqueURLError(&bs, "def67890", dupErr)
	if strings.Contains(got.Error(), "restore") {
		t.Errorf("uniqueURLError(active) = %q, want no restore hint", got)
	}
	if !errors.Is(got, book.ErrDuplicateURL) {
		t.Errorf("uniqueURLError(active) lost the ErrDuplicateURL chain: %v", got)
	}
}

func TestTitleError(t *testing.T) {
	required := titleError(fmt.Errorf("%w for %s", book.ErrTitleRequired, "https://example.com"))
	if !strings.Contains(required.Error(), "--title") {
		t.Errorf("titleError(ErrTitleRequired) = %q, want the --title hint", required)
	}
	if !errors.Is(required, book.ErrTitleRequired) {
		t.Errorf("titleError(ErrTitleRequired) lost the chain: %v", required)
	}

	other := errors.New("boom")
	if got := titleError(other); got.Error() != "boom" {
		t.Errorf("titleError(other) = %q, want the error unchanged", got)
	}
}
