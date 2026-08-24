package cmd

import (
	tea "charm.land/bubbletea/v2"
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
		return book.PrintCatalog(names, format)
	}

	_, err := tea.NewProgram(shelfRootScreen(bs, "list", config)).Run()
	return err
}

func addShelf(bs *book.BookShelves, config *book.Config) error {
	_, err := tea.NewProgram(shelfRootScreen(bs, "add", config)).Run()
	return err
}

func shelfRootScreen(bs *book.BookShelves, action string, config *book.Config) model.RootScreen {
	return model.RootScreen{
		Model: model.GetShelfForm(bs, config, action),
	}
}
