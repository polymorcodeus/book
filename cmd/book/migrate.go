package cmd

import (
	"fmt"

	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/catalog"
)

func migrate(config *book.Config) error {
	if !config.Autoconfirm {
		return fmt.Errorf("set --confirm to migrate shelf files")
	}

	report, err := catalog.MigrateShelfDir(config.ShelfRoot, catalog.MigrateOptions{})
	if err != nil {
		return err
	}

	fmt.Printf("migrated %d shelf(s); %d already v2", report.Upgraded, report.AlreadyV2)
	if len(report.Backups) > 0 {
		fmt.Printf("; backups: %d", len(report.Backups))
	}
	fmt.Println()
	return nil
}
