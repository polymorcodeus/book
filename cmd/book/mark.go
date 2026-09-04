package cmd

import (
	"errors"
	"fmt"

	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/catalog"
	"github.com/polymorcodeus/book/internal/model"
	"github.com/polymorcodeus/book/internal/web"
)

func getMark(bs *book.BookShelves, id, url, format string, config *book.Config) error {
	if id == "" && url == "" {
		if !config.Interactive {
			return fmt.Errorf("missing required flag: --id or --url")
		}
		return runProgram(markRootScreen(bs, &book.Mark{}, "get", config))
	}

	var target *book.Mark
	if id != "" {
		target = bs.FindMarkByID(id)
	} else {
		target = bs.FindMarkByURL(url)
	}
	if target == nil {
		if id != "" {
			return fmt.Errorf("mark with id %q not found", id)
		}
		return fmt.Errorf("mark with url %q not found", url)
	}

	if format == "" {
		fmt.Println(target.FullDetail())
		return nil
	}
	return book.PrintCatalog(target, format)
}

func marks(bs *book.BookShelves, shelfName string, collectionName string, format string, trash bool, config *book.Config) error {
	// Trash listing reads the derived index and is always non-interactive.
	if trash {
		idx, err := syncIndex(config)
		if err != nil {
			return err
		}
		deleted, err := idx.DeletedMarks(shelfName, collectionName)
		if err != nil {
			return err
		}
		switch format {
		case "toml":
			wrapped := struct {
				Marks []catalog.SearchResult `toml:"marks"`
			}{deleted}
			return book.PrintCatalog(wrapped, format)
		case "json":
			return book.PrintCatalog(deleted, format)
		default:
			for _, r := range deleted {
				fmt.Printf("%s %s\n", r.Title, r.URL)
			}
			return nil
		}
	}

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
		if format == "" {
			for _, m := range collection.Marks {
				if m.IsDeleted() {
					continue
				}
				fmt.Printf("%s %s\n", m.Name, m.URL)
			}
			return nil
		}
		return book.PrintCatalog(collection, format)
	}
	return runProgram(markRootScreen(bs, &book.Mark{}, "list", config))
}

func editMark(bs *book.BookShelves, id, title, tags, url string, config *book.Config) error {
	if err := requireFlag("id", id); err != nil {
		if !config.Interactive {
			return err
		}
		return runProgram(markRootScreen(bs, &book.Mark{}, "edit", config))
	}

	target := bs.FindMarkByID(id)
	if target == nil {
		return fmt.Errorf("mark with id %q not found", id)
	}

	if title == "" && tags == "" && url == "" {
		return fmt.Errorf("no edits provided; pass --title, --tags, or --url")
	}

	if url != "" {
		if err := book.ValidateURL(url); err != nil {
			return err
		}
		if err := bs.VerifyUniqueURL(book.GenerateID(url), target); err != nil {
			return err
		}
	}

	if err := target.UpdateMark(title, url, book.SplitTags(tags)); err != nil {
		return err
	}
	target.Touch()
	return catalog.UpdateShelfFile(target.Shelf)
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
	mark, err := book.NewMarkFromInput(URL, book.SplitTags(tags))
	if err != nil {
		return err
	}

	// Ensure URL hash not in bookshelves
	if err := bs.VerifyUniqueURL(mark.ID, nil); err != nil {
		return err
	}

	// Use provided title or fetch from URL
	fetched := book.TitleFetchResult{}
	if title == "" {
		fetchedTitle, err := web.LoadWebsite(mark.URL)
		if err != nil {
			if !errors.Is(err, web.ErrTitleUnavailable) {
				return err
			}
			fetched.Unavailable = true
		} else {
			fetched.Title = fetchedTitle
		}
	}
	mark.Name, err = book.ResolveMarkTitle(title, mark.URL, fetched, config.Interactive)
	if err != nil {
		return err
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
		mark.RecordAdd()
		collection.AddMark(&mark)
		if err := catalog.UpdateShelfFile(shelf); err != nil {
			return err
		}
		return nil
	}

	return runProgram(markRootScreen(bs, &mark, "add", config))
}

func removeMark(bs *book.BookShelves, id string, confirmed bool, config *book.Config) error {
	if err := requireFlag("id", id); err != nil {
		if !config.Interactive {
			return err
		}
		return runProgram(markRootScreen(bs, &book.Mark{}, "delete", config))
	}

	if !confirmed {
		return fmt.Errorf("remove mark requires --confirm")
	}

	target := bs.FindMarkByID(id)
	if target == nil {
		return fmt.Errorf("mark with id %q not found", id)
	}

	target.RecordDelete()
	return catalog.UpdateShelfFile(target.Shelf)
}

func restoreMark(bs *book.BookShelves, id string, shelfName string, collectionName string, url string) error {
	// --id is the preferred path: IDs are globally unique, so no shelf or
	// collection scoping is needed.
	if id != "" {
		target := bs.SoftDeletedByID(id)
		if target == nil {
			return fmt.Errorf("no trashed mark with id %q", id)
		}
		return clearSoftDelete(target)
	}

	if shelfName == "" || collectionName == "" || url == "" {
		return fmt.Errorf("restore requires --id, or --shelf/--collection/--url")
	}

	shelf := bs.Shelf(shelfName)
	if book.StructIsEmpty(shelf) {
		return fmt.Errorf("shelf %q not found", shelfName)
	}
	collection := shelf.Collection(collectionName)
	if book.StructIsEmpty(collection) {
		return fmt.Errorf("collection %q not found in shelf %q", collectionName, shelfName)
	}

	var target *book.Mark
	for _, m := range collection.Marks {
		if m.URL == url && m.IsDeleted() {
			target = m
			break
		}
	}
	if target == nil {
		return fmt.Errorf("no trashed mark with url %q in %q/%q", url, shelfName, collectionName)
	}
	return clearSoftDelete(target)
}

// clearSoftDelete restores a soft-deleted mark in place and persists its shelf.
func clearSoftDelete(target *book.Mark) error {
	target.DeletedAt = ""
	target.Touch()
	return catalog.UpdateShelfFile(target.Shelf)
}

func markRootScreen(bs *book.BookShelves, mark *book.Mark, action string, config *book.Config) model.RootScreen {
	if book.StructIsEmpty(mark) {
		screen := model.GetMarkForm(bs, &book.Mark{}, config, action)
		return model.RootScreen{Model: &screen}
	}
	screen := model.GetMarkForm(bs, mark, config, action)
	return model.RootScreen{Model: &screen}
}
