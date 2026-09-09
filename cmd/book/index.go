package cmd

import (
	"fmt"

	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/catalog"
)

// indexCache holds the lazily-opened SQLite index shared by the read paths
// within a single command invocation. It is created in Main and closed by
// Main's deferred cleanup.
type indexCache struct {
	index *catalog.Index
}

// get lazily opens the derived index.
func (c *indexCache) get(config *book.Config) (*catalog.Index, error) {
	if c.index == nil {
		idx, err := catalog.OpenIndex(config)
		if err != nil {
			return nil, err
		}
		c.index = idx
	}
	return c.index, nil
}

// sync lazily opens the derived index and reconciles it with the shelf files
// on disk, returning the ready-to-query index.
func (c *indexCache) sync(config *book.Config) (*catalog.Index, error) {
	idx, err := c.get(config)
	if err != nil {
		return nil, err
	}
	if _, err := idx.Sync(config); err != nil {
		return nil, err
	}
	return idx, nil
}

// rebuild wipes and rebuilds the index from shelf TOML files.
func (c *indexCache) rebuild(config *book.Config) (*catalog.RebuildReport, error) {
	idx, err := c.get(config)
	if err != nil {
		return nil, err
	}
	return idx.Rebuild(config)
}

// close releases the cached index, if any.
func (c *indexCache) close() error {
	if c.index != nil {
		return c.index.Close()
	}
	return nil
}

func runIndexRebuild(cache *indexCache, config *book.Config) error {
	report, err := cache.rebuild(config)
	if err != nil {
		return err
	}
	fmt.Printf("indexed %d shelf file(s)\n", report.Indexed)
	return nil
}

func runIndexSync(cache *indexCache, config *book.Config) error {
	idx, err := cache.get(config)
	if err != nil {
		return err
	}

	report, err := idx.Sync(config)
	if err != nil {
		return err
	}
	fmt.Printf("reindexed %d, removed %d, unchanged %d\n", report.Reindexed, report.Removed, report.Unchanged)
	return nil
}
