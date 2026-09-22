// Package catalog handles loading of shelf files and creating/writing of toml
// and json files
package catalog

import (
	"fmt"
	"path/filepath"

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
