package cmd

import (
	tea "charm.land/bubbletea/v2"
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

	_, err := tea.NewProgram(collectionRootScreen(bs, "list", config)).Run()
	return err
}

func addCollection(bs *book.BookShelves, config *book.Config) error {
	_, err := tea.NewProgram(collectionRootScreen(bs, "add", config)).Run()
	return err
}

func collectionRootScreen(bs *book.BookShelves, action string, config *book.Config) model.RootScreen {
	return model.RootScreen{
		Model: model.GetCollectionForm(bs, config, action),
	}
}
