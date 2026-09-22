package cmd

import (
	"fmt"

	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/catalog"
	"github.com/polymorcodeus/book/internal/model"
	"github.com/polymorcodeus/book/internal/theme"
)

func collections(cache *indexCache, bs *book.BookShelves, shelfName string, format string, config *theme.UIConfig) error {
	// Non-interactive path: all required flags provided
	if shelfName != "" && !config.Interactive {
		idx, err := cache.sync(config.Config)
		if err != nil {
			return err
		}
		names, err := idx.CollectionNames(shelfName)
		if err != nil {
			return err
		}
		if format == "" {
			for _, name := range names {
				fmt.Println(name)
			}
			return nil
		}
		return printCatalog(names, format)
	}

	return runProgram(collectionRootScreen(bs, "list", config))
}

func addCollection(bs *book.BookShelves, shelfName, collectionName, description string, config *theme.UIConfig) error {
	if err := requireFlags("shelf", shelfName, "name", collectionName); err != nil {
		if !config.Interactive {
			return err
		}
		return runProgram(collectionRootScreen(bs, "add", config))
	}

	shelf, ok := bs.Shelf(shelfName)
	if !ok {
		return fmt.Errorf("shelf %q not found", shelfName)
	}

	collection, err := book.NewCollection(shelf, collectionName, description)
	if err != nil {
		return err
	}
	shelf.AddCollection(collection)
	shelf.Touch()
	if err := catalog.UpdateShelfFile(shelf); err != nil {
		return err
	}
	return nil
}

func removeCollection(bs *book.BookShelves, shelfName, collectionName string, confirmed bool) error {
	if err := requireFlags("shelf", shelfName, "name", collectionName); err != nil {
		return err
	}

	if !confirmed {
		return fmt.Errorf("remove collection requires --confirm")
	}

	shelf, ok := bs.Shelf(shelfName)
	if !ok {
		return fmt.Errorf("shelf %q not found", shelfName)
	}

	if err := shelf.RemoveCollection(collectionName); err != nil {
		return err
	}
	shelf.Touch()
	return catalog.UpdateShelfFile(shelf)
}

func collectionRootScreen(bs *book.BookShelves, action string, config *theme.UIConfig) model.RootScreen {
	return model.RootScreen{
		Model: model.GetCollectionForm(bs, config, action),
	}
}
