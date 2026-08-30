package cmd

import (
	"fmt"

	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/model"
)

func shelves(bs *book.BookShelves, format string, config *book.Config) error {
	if !config.Interactive {
		idx, err := syncIndex(config)
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

func addShelf(bs *book.BookShelves, config *book.Config) error {
	return runProgram(shelfRootScreen(bs, "add", config))
}

func shelfRootScreen(bs *book.BookShelves, action string, config *book.Config) model.RootScreen {
	return model.RootScreen{
		Model: model.GetShelfForm(bs, config, action),
	}
}
