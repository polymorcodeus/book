package cmd

import (
	"fmt"
	"os"

	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/catalog"
	"github.com/polymorcodeus/book/internal/model"
)

func shelves(cache *indexCache, bs *book.BookShelves, format string, config *book.Config) error {
	if !config.Interactive {
		idx, err := cache.sync(config)
		if err != nil {
			return err
		}
		names, err := idx.ShelfNames()
		if err != nil {
			return err
		}
		if format == "" {
			for _, name := range names {
				fmt.Println(name)
			}
			return nil
		}
		return book.PrintCatalog(names, format)
	}

	return runProgram(shelfRootScreen(bs, "list", config))
}

func addShelf(bs *book.BookShelves, name, description string, config *book.Config) error {
	if err := requireFlag("name", name); err != nil {
		if !config.Interactive {
			return err
		}
		return runProgram(shelfRootScreen(bs, "add", config))
	}

	if err := bs.ValidateNewShelfName(name); err != nil {
		return err
	}

	shelf, err := book.NewShelf(name, description)
	if err != nil {
		return err
	}
	shelf.AddFileDetail(config)
	if err := catalog.UpdateShelfFile(shelf); err != nil {
		return err
	}
	bs.AddShelf(*shelf)
	return nil
}

func removeShelf(bs *book.BookShelves, name string, confirmed bool) error {
	if err := requireFlag("name", name); err != nil {
		return err
	}

	if !confirmed {
		return fmt.Errorf("remove shelf requires --confirm")
	}

	removed, err := bs.RemoveShelf(name)
	if err != nil {
		return err
	}

	if err := os.Remove(removed.FilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete shelf file: %w", err)
	}
	return nil
}

func shelfRootScreen(bs *book.BookShelves, action string, config *book.Config) model.RootScreen {
	return model.RootScreen{
		Model: model.GetShelfForm(bs, config, action),
	}
}
