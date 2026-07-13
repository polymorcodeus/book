package cmd

import (
	"fmt"
	"net/url"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/catalog"
	"github.com/polymorcodeus/book/internal/model"
	"github.com/polymorcodeus/book/internal/web"
)

func mark(bs *book.BookShelves, config *book.Config) error {
	_, err := tea.NewProgram(markRootScreen(bs, &book.Mark{}, "get", config)).Run()
	return err
}

func marks(bs *book.BookShelves, shelfName string, collectionName string, format string, config *book.Config) error {
	// Non-interactive path: all required flags provided
	if shelfName != "" && collectionName != "" && !config.Interactive {
		shelf := bs.Shelf(shelfName)
		if shelf == nil || book.StructIsEmpty(shelf) {
			return fmt.Errorf("shelf %q not found", shelfName)
		}
		collection := shelf.Collection(collectionName)
		if collection == nil || book.StructIsEmpty(collection) {
			return fmt.Errorf("collection %q not found in shelf %q", collectionName, shelfName)
		}

		return book.PrintCatalog(collection, format)
	}
	_, err := tea.NewProgram(markRootScreen(bs, &book.Mark{}, "list", config)).Run()
	return err
}

func editMark(bs *book.BookShelves, config *book.Config) error {
	_, err := tea.NewProgram(markRootScreen(bs, &book.Mark{}, "edit", config)).Run()
	return err
}

func addMark(bs *book.BookShelves, URL string, tags string, shelfName string, collectionName string, title string, config *book.Config) error {
	if _, err := url.ParseRequestURI(URL); err != nil {
		return err
	}
	id := book.GenerateID(URL)

	// Ensure URL hash not in bookshelves
	if err := bs.VerifyUniqueURL(id); err != nil {
		return err
	}

	mark := book.Mark{
		Id:   id,
		Url:  URL,
		Tags: strings.Split(tags, ","),
	}

	// Use provided title or fetch from URL
	if title != "" {
		mark.Name = title
	} else {
		fetchedTitle, err := web.LoadWebsite(mark.Url)
		if err != nil {
			return err
		}
		if fetchedTitle == "" {
			mark.Name = "couldn't fetch page title"
		} else {
			mark.Name = fetchedTitle
		}
	}

	// Non-interactive path: all required flags provided
	if shelfName != "" && collectionName != "" {
		shelf := bs.Shelf(shelfName)
		if shelf == nil || book.StructIsEmpty(shelf) {
			return fmt.Errorf("shelf %q not found", shelfName)
		}
		collection := shelf.Collection(collectionName)
		if collection == nil || book.StructIsEmpty(collection) {
			return fmt.Errorf("collection %q not found in shelf %q", collectionName, shelfName)
		}
		mark.Shelf = shelf
		mark.Collection = collection
		collection.AddMark(&mark)
		if err := catalog.UpdateShelfFile(shelf); err != nil {
			return err
		}
		return nil
	}

	_, err := tea.NewProgram(markRootScreen(bs, &mark, "add", config)).Run()
	return err
}

func removeMark(bs *book.BookShelves, config *book.Config) error {
	_, err := tea.NewProgram(markRootScreen(bs, &book.Mark{}, "delete", config)).Run()
	return err
}

func markRootScreen(bs *book.BookShelves, mark *book.Mark, action string, config *book.Config) model.RootScreen {
	if book.StructIsEmpty(mark) {
		screen := model.GetMarkForm(bs, &book.Mark{}, config, action)
		return model.RootScreen{Model: &screen}
	}
	screen := model.GetMarkForm(bs, mark, config, action)
	return model.RootScreen{Model: &screen}
}
