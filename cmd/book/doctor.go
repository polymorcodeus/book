package cmd

import (
	"fmt"

	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/catalog"
)

// doctor inspects the catalog for post-merge problems: duplicate marks, schema
// drift, index staleness, and stray debris. With fix it auto-resolves merge
// duplicates and rewrites the affected shelf files.
func doctor(config *book.Config, fix bool) error {
	var shelves book.BookShelves
	if err := catalog.LoadShelves(&shelves, config); err != nil {
		return err
	}

	var duplicates, conflicts []book.MarkConflict
	for _, c := range shelves.DetectDuplicates() {
		if c.TrueConflict {
			conflicts = append(conflicts, c)
		} else {
			duplicates = append(duplicates, c)
		}
	}

	v1Files, err := catalog.V1ShelfFiles(config.ShelfRoot, config.CatalogFormat)
	if err != nil {
		return err
	}
	debris, err := catalog.StrayDebris(config.ShelfRoot, config.CatalogFormat)
	if err != nil {
		return err
	}
	stale, err := indexStaleFiles(config)
	if err != nil {
		return err
	}

	report := doctorReport{
		Duplicates: duplicates,
		Conflicts:  conflicts,
		V1Files:    v1Files,
		Debris:     debris,
		Stale:      stale,
	}

	if fix {
		if !config.Autoconfirm {
			return fmt.Errorf("set --confirm to fix merge duplicates")
		}
		removed, changed := shelves.ResolveDuplicates()
		report.Fixed = removed
		for _, s := range changed {
			if err := catalog.UpdateShelfFile(s); err != nil {
				return err
			}
			report.FixedFiles = append(report.FixedFiles, s.FilePath)
		}
		// Only reconcile the index when no duplicate IDs remain: the index's
		// primary key on catalog_id cannot represent unresolved conflicts.
		if len(changed) > 0 && len(shelves.DetectDuplicates()) == 0 {
			if _, err := syncIndex(config); err != nil {
				return err
			}
		}
	}

	printDoctorReport(report)
	return nil
}

// indexStaleFiles returns shelf paths whose index entries are out of date, or
// nil when the index has not been built yet.
func indexStaleFiles(config *book.Config) ([]string, error) {
	exists, err := catalog.VerifyExists(catalog.IndexPath(config))
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	idx, err := catalog.OpenIndex(config)
	if err != nil {
		return nil, err
	}
	defer func() { _ = idx.Close() }()
	return idx.StaleFiles(config)
}

type doctorReport struct {
	Duplicates []book.MarkConflict
	Conflicts  []book.MarkConflict
	V1Files    []string
	Debris     []string
	Stale      []string
	Fixed      int
	FixedFiles []string
}

func printDoctorReport(r doctorReport) {
	clean := len(r.Duplicates) == 0 && len(r.Conflicts) == 0 &&
		len(r.V1Files) == 0 && len(r.Debris) == 0 && len(r.Stale) == 0 && r.Fixed == 0

	fmt.Println("book doctor")
	fmt.Println()

	if clean {
		fmt.Println("catalog is clean")
		return
	}

	if len(r.Duplicates) > 0 {
		fmt.Printf("duplicate marks (%d)\n", len(r.Duplicates))
		for _, d := range r.Duplicates {
			fmt.Printf("  %s  %s  (%d copies)\n", d.ID, d.URL, len(d.Marks))
			for _, m := range d.Marks {
				fmt.Printf("    - %s / %s\n", m.Shelf.Name, m.Collection.Name)
			}
		}
		if r.Fixed == 0 {
			fmt.Println("  run `book doctor --fix` to auto-merge these")
		}
		fmt.Println()
	}

	if len(r.Conflicts) > 0 {
		fmt.Printf("conflicting marks (%d, manual resolution needed)\n", len(r.Conflicts))
		for _, c := range r.Conflicts {
			fmt.Printf("  %s  %s\n", c.ID, c.URL)
			for _, m := range c.Marks {
				fmt.Printf("    - %s / %s  title=%q tags=%v deleted=%t\n",
					m.Shelf.Name, m.Collection.Name, m.Name, m.Tags, m.IsDeleted())
			}
		}
		fmt.Println()
	}

	if len(r.V1Files) > 0 {
		fmt.Println("v1 schema files (run `book migrate`)")
		for _, f := range r.V1Files {
			fmt.Printf("  %s\n", f)
		}
		fmt.Println()
	}

	if len(r.Stale) > 0 {
		fmt.Println("stale index entries (run `book index sync`)")
		for _, f := range r.Stale {
			fmt.Printf("  %s\n", f)
		}
		fmt.Println()
	}

	if len(r.Debris) > 0 {
		fmt.Println("stray debris")
		for _, f := range r.Debris {
			fmt.Printf("  %s\n", f)
		}
		fmt.Println()
	}

	if r.Fixed > 0 {
		fmt.Printf("fixed: removed %d duplicate mark(s) from %d shelf file(s)\n", r.Fixed, len(r.FixedFiles))
	}
}
