package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/catalog"
	"github.com/polymorcodeus/book/internal/model"
	"github.com/polymorcodeus/book/internal/web"
)

func mark(bs *book.BookShelves, config *book.Config) error {
	return runProgram(markRootScreen(bs, &book.Mark{}, "get", config))
}

func marks(bs *book.BookShelves, shelfName string, collectionName string, format string, config *book.Config) error {
	// Non-interactive path: all required flags provided
	if shelfName != "" && collectionName != "" && !config.Interactive {
		idx, err := syncIndex(config)
		if err != nil {
			return err
		}
		collection, err := idx.Collection(shelfName, collectionName)
		if err != nil {
			return err
		}
		return book.PrintCatalog(collection, format)
	}
	return runProgram(markRootScreen(bs, &book.Mark{}, "list", config))
}

func editMark(bs *book.BookShelves, config *book.Config) error {
	return runProgram(markRootScreen(bs, &book.Mark{}, "edit", config))
}

func searchMarks(query string, tags string, shelfName string, collectionName string, format string, config *book.Config) error {
	clauses, err := book.ParseTagFilter(tags)
	if err != nil {
		return err
	}

	idx, err := syncIndex(config)
	if err != nil {
		return err
	}

	results, err := idx.Search(query, shelfName, collectionName, clauses)
	if err != nil {
		return err
	}

	switch format {
	case "json":
		return book.PrintCatalog(results, format)
	case "toml":
		// TOML requires a top-level map or struct, so wrap the slice.
		wrapped := struct {
			Marks []catalog.SearchResult `toml:"marks"`
		}{results}
		return book.PrintCatalog(wrapped, format)
	default:
		for _, r := range results {
			fmt.Printf("%s %s\n", r.Title, r.URL)
		}
		return nil
	}
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
		ID:   id,
		URL:  URL,
		Tags: strings.Split(tags, ","),
	}

	// Use provided title or fetch from URL
	if title != "" {
		mark.Name = title
	} else {
		fetchedTitle, err := web.LoadWebsite(mark.URL)
		if err != nil {
			if errors.Is(err, web.ErrTitleUnavailable) {
				// Non-interactive path can't prompt for a title.
				if shelfName != "" && collectionName != "" {
					return fmt.Errorf("couldn't fetch title for %s; provide --title", mark.URL)
				}
				// Interactive path: leave the title empty so the user is
				// forced to enter it manually in the edit form.
			} else {
				return err
			}
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
		now := book.NowTimestamp()
		mark.CreatedAt = now
		mark.UpdatedAt = now
		if collection.UpdatedAt != "" {
			collection.UpdatedAt = now
		}
		if shelf.IsV2() {
			shelf.UpdatedAt = now
		}
		collection.AddMark(&mark)
		if err := catalog.UpdateShelfFile(shelf); err != nil {
			return err
		}
		return nil
	}

	return runProgram(markRootScreen(bs, &mark, "add", config))
}

func removeMark(bs *book.BookShelves, config *book.Config) error {
	return runProgram(markRootScreen(bs, &book.Mark{}, "delete", config))
}

func markRootScreen(bs *book.BookShelves, mark *book.Mark, action string, config *book.Config) model.RootScreen {
	if book.StructIsEmpty(mark) {
		screen := model.GetMarkForm(bs, &book.Mark{}, config, action)
		return model.RootScreen{Model: &screen}
	}
	screen := model.GetMarkForm(bs, mark, config, action)
	return model.RootScreen{Model: &screen}
}
