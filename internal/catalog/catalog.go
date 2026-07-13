// Package catalog handles loading of shelf files and creating/writing of toml
// and json files
package catalog

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"charm.land/huh/v2/spinner"
	"github.com/BurntSushi/toml"

	"github.com/polymorcodeus/book/internal/book"
)

// LoadShelves reads all shelf TOML files from disk into the given BookShelves.
func LoadShelves(bs *book.BookShelves, config *book.Config) error {
	globDir := fmt.Sprintf("%s/*.%s", config.ShelfRoot, config.CatalogFormat)
	files, err := filepath.Glob(globDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if filepath.Base(file) == filepath.Base(config.ConfigFile) {
			continue
		}
		var shelf book.Shelf
		if _, err := toml.DecodeFile(file, &shelf); err != nil {
			return err
		}
		shelf.AddFileDetail(config)
		bs.AddShelf(shelf)
	}

	bs.LoadParents()
	return nil
}

// LoadCatalog loads shelves with an optional spinner when running interactively.
func LoadCatalog(bs *book.BookShelves, config *book.Config, interactive bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if !interactive {
		return LoadShelves(bs, config)
	}

	return spinner.New().
		Context(ctx).
		ActionWithErr(func(context.Context) error {
			time.Sleep(1 * time.Second)
			return LoadShelves(bs, config)
		}).
		Title("Loading your bookshelves ...").
		Run()
}
