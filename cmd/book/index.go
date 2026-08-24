package cmd

import (
	"fmt"

	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/catalog"
)

// index holds the lazily-opened SQLite index shared by the read paths within a
// single command invocation.
var index *catalog.Index

// syncIndex lazily opens the derived index and reconciles it with the shelf
// files on disk, returning the ready-to-query index.
func syncIndex(config *book.Config) (*catalog.Index, error) {
	if index == nil {
		idx, err := catalog.OpenIndex(config)
		if err != nil {
			return nil, err
		}
		index = idx
	}
	if _, err := index.Sync(config); err != nil {
		return nil, err
	}
	return index, nil
}

func runIndexRebuild(config *book.Config) error {
	idx, err := catalog.OpenIndex(config)
	if err != nil {
		return err
	}
	defer func() { _ = idx.Close() }()

	report, err := idx.Rebuild(config)
	if err != nil {
		return err
	}
	fmt.Printf("indexed %d shelf file(s)\n", report.Indexed)
	return nil
}

func runIndexSync(config *book.Config) error {
	idx, err := catalog.OpenIndex(config)
	if err != nil {
		return err
	}
	defer func() { _ = idx.Close() }()

	report, err := idx.Sync(config)
	if err != nil {
		return err
	}
	fmt.Printf("reindexed %d, removed %d, unchanged %d\n", report.Reindexed, report.Removed, report.Unchanged)
	return nil
}
