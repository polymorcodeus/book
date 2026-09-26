package book_test

import (
	"fmt"
	"os"

	"github.com/polymorcodeus/book/pkg/book"
	"github.com/polymorcodeus/book/pkg/catalog"
)

// Example demonstrates the library round trip an external consumer performs:
// build a shelf with a collection and a mark, persist it as TOML, and load it
// back from disk.
func Example() {
	dir, err := os.MkdirTemp("", "book-example")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	paths := catalog.Paths{ShelfRoot: dir, CatalogFormat: "toml"}

	shelf, err := book.NewShelf("work", "work stuff")
	if err != nil {
		panic(err)
	}
	collection, err := book.NewCollection(shelf, "golang", "go links")
	if err != nil {
		panic(err)
	}
	mark, err := book.NewMarkFromInput("https://go.dev", book.SplitTags("lang, official"))
	if err != nil {
		panic(err)
	}
	mark.Title = "The Go Programming Language"
	mark.Shelf = shelf
	mark.Collection = collection
	mark.RecordAdd()
	collection.AddMark(&mark)
	shelf.AddCollection(collection)

	shelf.FilePath = catalog.ShelfPath(shelf.Name, paths)
	if err := catalog.UpdateShelfFile(shelf); err != nil {
		panic(err)
	}

	var shelves book.BookShelves
	if err := catalog.LoadShelves(&shelves, paths); err != nil {
		panic(err)
	}

	loaded, ok := shelves.Shelf("work")
	if !ok {
		panic("shelf not found")
	}
	got := loaded.Collection("golang").Mark("The Go Programming Language")
	fmt.Println(loaded.Name)
	fmt.Println(got.Title)
	fmt.Println(got.URL)
	fmt.Println(got.Tags)

	// Output:
	// work
	// The Go Programming Language
	// https://go.dev
	// [lang official]
}
