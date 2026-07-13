package cmd

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/model"
)

func collections(bs *book.BookShelves, shelfName string, format string, config *book.Config) error {
	var err error

	// Non-interactive path: all required flags provided
	if shelfName != "" && !config.Interactive {
		shelf := bs.Shelf(shelfName)
		if shelf == nil || book.StructIsEmpty(shelf) {
			return fmt.Errorf("shelf %q not found", shelfName)
		}
		return book.PrintCatalog(shelf.CollectionsNames(), format)
	} else {
		_, err = tea.NewProgram(collectionRootScreen(bs, "list", config)).Run()
	}

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
