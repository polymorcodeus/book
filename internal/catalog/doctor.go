package catalog

import (
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/polymorcodeus/book/internal/book"
)

// V1ShelfFiles returns the shelf TOML files in dir that still use the v1 schema
// (no schema_version key). These should be upgraded with `book migrate`.
func V1ShelfFiles(dir, catalogFormat string) ([]string, error) {
	files, err := filepath.Glob(fmt.Sprintf("%s/*.%s", dir, catalogFormat))
	if err != nil {
		return nil, err
	}

	var v1 []string
	for _, file := range files {
		var shelf book.Shelf
		if _, err := toml.DecodeFile(file, &shelf); err != nil {
			return nil, err
		}
		if !shelf.IsV2() {
			v1 = append(v1, file)
		}
	}
	return v1, nil
}

// StrayDebris returns leftover temp and backup files in dir that a crashed
// write or `book migrate` may have left behind.
func StrayDebris(dir, catalogFormat string) ([]string, error) {
	var debris []string
	for _, suffix := range []string{"tmp", "bak"} {
		matches, err := filepath.Glob(fmt.Sprintf("%s/*.%s.%s", dir, catalogFormat, suffix))
		if err != nil {
			return nil, err
		}
		debris = append(debris, matches...)
	}
	return debris, nil
}
