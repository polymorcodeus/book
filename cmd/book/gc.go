package cmd

import (
	"fmt"
	"time"

	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/catalog"
)

// gc purges soft-deleted marks older than retentionDays from the shelf TOML
// files and reconciles the derived index.
func gc(config *book.Config, retentionDays int) error {
	if !config.Autoconfirm {
		return fmt.Errorf("set --confirm to run gc")
	}
	if retentionDays < 0 {
		return fmt.Errorf("--retention-days must be non-negative")
	}

	var shelves book.BookShelves
	if err := catalog.LoadShelves(&shelves, config); err != nil {
		return err
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)

	var purged, changed int
	for i := range shelves {
		shelf := &shelves[i]
		n := shelf.PurgeDeletedMarks(cutoff)
		if n == 0 {
			continue
		}
		if err := catalog.UpdateShelfFile(shelf); err != nil {
			return err
		}
		purged += n
		changed++
	}

	if changed > 0 {
		// Reconcile the derived index so purged marks disappear from search.
		if _, err := syncIndex(config); err != nil {
			return err
		}
	}

	fmt.Printf("purged %d mark(s) from %d shelf(s)\n", purged, changed)
	return nil
}
