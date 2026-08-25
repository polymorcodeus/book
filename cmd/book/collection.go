package cmd

import (
	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/model"
)

func collections(bs *book.BookShelves, shelfName string, format string, config *book.Config) error {
	// Non-interactive path: all required flags provided
	if shelfName != "" && !config.Interactive {
		idx, err := syncIndex(config)
		if err != nil {
			return err
		}
		names, err := idx.CollectionNames(shelfName)
		if err != nil {
			return err
		}
		return book.PrintCatalog(names, format)
	}

	return runProgram(collectionRootScreen(bs, "list", config))
}

func addCollection(bs *book.BookShelves, config *book.Config) error {
	return runProgram(collectionRootScreen(bs, "add", config))
}

func collectionRootScreen(bs *book.BookShelves, action string, config *book.Config) model.RootScreen {
	return model.RootScreen{
		Model: model.GetCollectionForm(bs, config, action),
	}
}
