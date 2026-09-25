// Package catalog persists book shelves as TOML files on disk and maintains a
// derived SQLite index over them for fast reads, search, and staleness
// checks. The TOML files are the source of truth; the index is disposable and
// can be rebuilt from them at any time.
package catalog

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/polymorcodeus/book/pkg/book"
)

// Paths locates the on-disk catalog data: the directory of shelf files and
// their format. It is the only storage-location input the package's read and
// index entry points require.
type Paths struct {
	// ShelfRoot is the directory holding one file per shelf.
	ShelfRoot string
	// CatalogFormat is the file extension of shelf files ("toml").
	CatalogFormat string
	// ConfigFile is an optional path to exclude from shelf globs, for when a
	// config file lives inside ShelfRoot. Empty disables the exclusion.
	ConfigFile string
}

// PathsFromConfig derives storage Paths from application configuration.
func PathsFromConfig(c *book.Config) Paths {
	return Paths{
		ShelfRoot:     c.ShelfRoot,
		CatalogFormat: c.CatalogFormat,
		ConfigFile:    c.ConfigFile,
	}
}

// ShelfPath returns the on-disk file path for a shelf with the given name.
// Spaces in the name become underscores.
func ShelfPath(name string, paths Paths) string {
	formattedName := strings.ReplaceAll(strings.TrimSpace(name), " ", "_")
	fileName := fmt.Sprintf("%s.%s", formattedName, paths.CatalogFormat)
	return filepath.Join(paths.ShelfRoot, fileName)
}

// LoadShelves reads all shelf TOML files from disk into the given BookShelves.
func LoadShelves(bs *book.BookShelves, paths Paths) error {
	globDir := fmt.Sprintf("%s/*.%s", paths.ShelfRoot, paths.CatalogFormat)
	files, err := filepath.Glob(globDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if paths.ConfigFile != "" && filepath.Base(file) == filepath.Base(paths.ConfigFile) {
			continue
		}
		var shelf book.Shelf
		if _, err := toml.DecodeFile(file, &shelf); err != nil {
			return err
		}
		shelf.FilePath = ShelfPath(shelf.Name, paths)
		bs.AddShelf(shelf)
	}

	bs.LoadParents()
	return nil
}
