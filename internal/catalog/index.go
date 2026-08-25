// Package catalog handles loading of shelf files and creating/writing of toml
// and json files
package catalog

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	_ "modernc.org/sqlite" // registers the pure-Go "sqlite" driver

	"github.com/polymorcodeus/book/internal/book"
)

// indexSchema creates the derived SQLite index. TOML remains the source of
// truth; these tables are rebuilt from it and are safe to delete at any time.
const indexSchema = `
CREATE TABLE IF NOT EXISTS shelves (
	shelf_id    TEXT PRIMARY KEY,
	name        TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	created_at  TEXT NOT NULL DEFAULT '',
	updated_at  TEXT NOT NULL DEFAULT '',
	file_path   TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS collections (
	collection_id TEXT PRIMARY KEY,
	shelf_id      TEXT NOT NULL,
	name          TEXT NOT NULL,
	description   TEXT NOT NULL DEFAULT '',
	created_at    TEXT NOT NULL DEFAULT '',
	updated_at    TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_collections_shelf ON collections(shelf_id);
CREATE TABLE IF NOT EXISTS marks (
	catalog_id    TEXT PRIMARY KEY,
	collection_id TEXT NOT NULL,
	title         TEXT NOT NULL DEFAULT '',
	url           TEXT NOT NULL DEFAULT '',
	created_at    TEXT NOT NULL DEFAULT '',
	updated_at    TEXT NOT NULL DEFAULT '',
	deleted_at    TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_marks_collection ON marks(collection_id);
CREATE TABLE IF NOT EXISTS tags (
	mark_id TEXT NOT NULL,
	tag     TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag);
CREATE TABLE IF NOT EXISTS file_meta (
	path       TEXT PRIMARY KEY,
	mtime      INTEGER NOT NULL,
	size       INTEGER NOT NULL,
	sha256     TEXT NOT NULL,
	indexed_at TEXT NOT NULL
);
CREATE VIRTUAL TABLE IF NOT EXISTS marks_fts USING fts5(title, url, catalog_id UNINDEXED);
`

// Index is a derived SQLite index over the shelf TOML files. It is the read and
// search path; writes remain TOML-first and the index is refreshed lazily via
// Sync or eagerly via Rebuild.
type Index struct {
	db   *sql.DB
	path string
}

// IndexPath returns the on-disk location of the derived index. It prefers the
// user cache dir ($XDG_CACHE_HOME/book/index.db), falling back to the config
// dir next to the shelf directory when the cache dir is unavailable.
func IndexPath(config *book.Config) string {
	if cache := os.Getenv("XDG_CACHE_HOME"); cache != "" {
		return filepath.Join(cache, "book", "index.db")
	}
	return filepath.Join(filepath.Dir(config.ShelfRoot), "index.db")
}

// OpenIndex opens (creating if needed) the SQLite index and ensures its schema
// is present.
func OpenIndex(config *book.Config) (*Index, error) {
	path := IndexPath(config)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create index directory: %w", err)
	}

	// When falling back to the config dir (which may be git-committed), make
	// sure the disposable index never gets committed.
	if os.Getenv("XDG_CACHE_HOME") == "" {
		ensureIndexGitignore(filepath.Dir(path))
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// A single connection sidesteps SQLITE_BUSY between this process's own
	// read and write statements.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(indexSchema); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Index{db: db, path: path}, nil
}

// Close releases the underlying database connection.
func (ix *Index) Close() error {
	return ix.db.Close()
}

// RebuildReport summarizes a full Rebuild run.
type RebuildReport struct {
	Indexed int
}

// SyncReport summarizes an incremental Sync run.
type SyncReport struct {
	Reindexed int
	Removed   int
	Unchanged int
}

// Rebuild wipes the index and re-indexes every shelf file from disk. It is the
// escape hatch for corruption or drift.
func (ix *Index) Rebuild(config *book.Config) (*RebuildReport, error) {
	files, err := shelfFilePaths(config)
	if err != nil {
		return nil, err
	}

	tx, err := ix.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range []string{
		"DELETE FROM tags",
		"DELETE FROM marks_fts",
		"DELETE FROM marks",
		"DELETE FROM collections",
		"DELETE FROM shelves",
		"DELETE FROM file_meta",
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return nil, err
		}
	}

	report := &RebuildReport{}
	for _, file := range files {
		if err := ix.indexShelfFile(tx, file); err != nil {
			return nil, err
		}
		report.Indexed++
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return report, nil
}

// Sync reconciles the index with the shelf files on disk. It stats each file
// (fast path); only when mtime or size change does it re-hash the file and
// re-index it. Files that no longer exist are pruned from the index.
func (ix *Index) Sync(config *book.Config) (*SyncReport, error) {
	files, err := shelfFilePaths(config)
	if err != nil {
		return nil, err
	}

	tx, err := ix.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	report := &SyncReport{}
	seen := make(map[string]bool, len(files))
	for _, file := range files {
		seen[file] = true
		changed, err := ix.fileChanged(tx, file)
		if err != nil {
			return nil, err
		}
		if !changed {
			report.Unchanged++
			continue
		}
		if err := ix.indexShelfFile(tx, file); err != nil {
			return nil, err
		}
		report.Reindexed++
	}

	if err := ix.pruneRemoved(tx, seen, report); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return report, nil
}

// StaleFiles returns the shelf file paths whose index entries are out of date:
// files whose content changed since indexing, files never indexed, and paths
// that were indexed but no longer exist on disk. An empty result means the
// index is current.
func (ix *Index) StaleFiles(config *book.Config) ([]string, error) {
	files, err := shelfFilePaths(config)
	if err != nil {
		return nil, err
	}

	tx, err := ix.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var stale []string
	seen := make(map[string]bool, len(files))
	for _, file := range files {
		seen[file] = true
		changed, err := ix.fileChanged(tx, file)
		if err != nil {
			return nil, err
		}
		if changed {
			stale = append(stale, file)
		}
	}

	// Paths indexed but no longer present on disk.
	rows, err := tx.Query(`SELECT path FROM file_meta`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if !seen[path] {
			stale = append(stale, path)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	sort.Strings(stale)
	return stale, nil
}

// UpsertShelf writes a single shelf (and its collections, marks, and tags) into
// the index. The caller must have already persisted the shelf to TOML.
func (ix *Index) UpsertShelf(s *book.Shelf) error {
	tx, err := ix.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := ix.deleteShelf(tx, s.ID, s.FilePath); err != nil {
		return err
	}
	if err := insertShelf(tx, s); err != nil {
		return err
	}
	if err := ix.recordFileMeta(tx, s.FilePath); err != nil {
		return err
	}
	return tx.Commit()
}

// ShelfNames returns the names of all indexed shelves, sorted.
func (ix *Index) ShelfNames() ([]string, error) {
	rows, err := ix.db.Query(`SELECT name FROM shelves ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// CollectionNames returns the names of all collections in a shelf, sorted.
func (ix *Index) CollectionNames(shelfName string) ([]string, error) {
	var exists bool
	if err := ix.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM shelves WHERE name = ?)`, shelfName).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("shelf %q not found", shelfName)
	}

	rows, err := ix.db.Query(`
		SELECT c.name
		FROM collections c
		JOIN shelves s ON s.shelf_id = c.shelf_id
		WHERE s.name = ?
		ORDER BY c.name`, shelfName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// Collection returns a reconstructed collection (with marks and tags) for the
// named shelf and collection. Soft-deleted marks are excluded.
func (ix *Index) Collection(shelfName, collectionName string) (*book.Collection, error) {
	var shelfID string
	err := ix.db.QueryRow(`SELECT shelf_id FROM shelves WHERE name = ?`, shelfName).Scan(&shelfID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("shelf %q not found", shelfName)
	}
	if err != nil {
		return nil, err
	}

	col := &book.Collection{}
	err = ix.db.QueryRow(`
		SELECT collection_id, name, description, created_at, updated_at
		FROM collections
		WHERE shelf_id = ? AND name = ?`, shelfID, collectionName).
		Scan(&col.ID, &col.Name, &col.Description, &col.CreatedAt, &col.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("collection %q not found in shelf %q", collectionName, shelfName)
	}
	if err != nil {
		return nil, err
	}

	rows, err := ix.db.Query(`
		SELECT catalog_id, title, url, created_at, updated_at, deleted_at
		FROM marks
		WHERE collection_id = ? AND deleted_at = ''
		ORDER BY rowid`, col.ID)
	if err != nil {
		return nil, err
	}

	var marks []*book.Mark
	for rows.Next() {
		m := &book.Mark{}
		if err := rows.Scan(&m.ID, &m.Name, &m.URL, &m.CreatedAt, &m.UpdatedAt, &m.DeletedAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		marks = append(marks, m)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	// Load tags after closing the marks cursor; with a single connection the
	// nested tag query cannot run while the cursor still holds it.
	for _, m := range marks {
		if err := ix.loadTags(m); err != nil {
			return nil, err
		}
	}
	col.Marks = marks
	return col, nil
}

// SearchResult is a single match from a full-text search over the index.
type SearchResult struct {
	ID         string   `json:"catalog_id" toml:"catalog_id"`
	Shelf      string   `json:"shelf" toml:"shelf"`
	Collection string   `json:"collection" toml:"collection"`
	Title      string   `json:"title" toml:"title"`
	URL        string   `json:"url" toml:"url"`
	Tags       []string `json:"tags" toml:"tags"`
}

// Search runs an FTS5 query over mark titles and URLs, excluding soft-deleted
// marks. The query string uses FTS5 match syntax and may be empty to search by
// filters alone. tagClauses filters results to marks matching every clause
// (AND), where each clause is a set of tags of which at least one must match
// (OR). An empty tagClauses applies no tag filter.
func (ix *Index) Search(query, shelfName, collectionName string, tagClauses [][]string) ([]SearchResult, error) {
	var sqlQuery string
	var args []any

	if query != "" {
		sqlQuery = `
			SELECT m.catalog_id, m.title, m.url, c.name, s.name
			FROM marks_fts
			JOIN marks m ON m.catalog_id = marks_fts.catalog_id
			JOIN collections c ON c.collection_id = m.collection_id
			JOIN shelves s ON s.shelf_id = c.shelf_id
			WHERE marks_fts MATCH ? AND m.deleted_at = ''`
		args = append(args, query)
	} else {
		sqlQuery = `
			SELECT m.catalog_id, m.title, m.url, c.name, s.name
			FROM marks m
			JOIN collections c ON c.collection_id = m.collection_id
			JOIN shelves s ON s.shelf_id = c.shelf_id
			WHERE m.deleted_at = ''`
	}

	if shelfName != "" {
		sqlQuery += " AND s.name = ?"
		args = append(args, shelfName)
	}
	if collectionName != "" {
		sqlQuery += " AND c.name = ?"
		args = append(args, collectionName)
	}
	for i, clause := range tagClauses {
		alias := fmt.Sprintf("tf%d", i)
		sqlQuery += fmt.Sprintf(
			" AND EXISTS (SELECT 1 FROM tags %s WHERE %s.mark_id = m.catalog_id AND %s.tag IN (%s))",
			alias, alias, alias, placeholders(len(clause)))
		for _, tag := range clause {
			args = append(args, tag)
		}
	}
	if query != "" {
		sqlQuery += " ORDER BY rank"
	} else {
		sqlQuery += " ORDER BY s.name, c.name, m.title"
	}

	rows, err := ix.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}

	type match struct {
		id, title, url, collection, shelf string
	}
	var matches []match
	for rows.Next() {
		var m match
		if err := rows.Scan(&m.id, &m.title, &m.url, &m.collection, &m.shelf); err != nil {
			_ = rows.Close()
			return nil, err
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(matches))
	for _, m := range matches {
		mark := &book.Mark{ID: m.id}
		if err := ix.loadTags(mark); err != nil {
			return nil, err
		}
		results = append(results, SearchResult{
			ID:         m.id,
			Shelf:      m.shelf,
			Collection: m.collection,
			Title:      m.title,
			URL:        m.url,
			Tags:       mark.Tags,
		})
	}
	return results, nil
}

// DeletedMarks returns every soft-deleted mark, optionally filtered by shelf and
// collection name, ordered by shelf, collection, and title.
func (ix *Index) DeletedMarks(shelfName, collectionName string) ([]SearchResult, error) {
	sqlQuery := `
		SELECT m.catalog_id, m.title, m.url, c.name, s.name
		FROM marks m
		JOIN collections c ON c.collection_id = m.collection_id
		JOIN shelves s ON s.shelf_id = c.shelf_id
		WHERE m.deleted_at != ''`
	var args []any

	if shelfName != "" {
		sqlQuery += " AND s.name = ?"
		args = append(args, shelfName)
	}
	if collectionName != "" {
		sqlQuery += " AND c.name = ?"
		args = append(args, collectionName)
	}
	sqlQuery += " ORDER BY s.name, c.name, m.title"

	rows, err := ix.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}

	type match struct {
		id, title, url, collection, shelf string
	}
	var matches []match
	for rows.Next() {
		var m match
		if err := rows.Scan(&m.id, &m.title, &m.url, &m.collection, &m.shelf); err != nil {
			_ = rows.Close()
			return nil, err
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(matches))
	for _, m := range matches {
		mark := &book.Mark{ID: m.id}
		if err := ix.loadTags(mark); err != nil {
			return nil, err
		}
		results = append(results, SearchResult{
			ID:         m.id,
			Shelf:      m.shelf,
			Collection: m.collection,
			Title:      m.title,
			URL:        m.url,
			Tags:       mark.Tags,
		})
	}
	return results, nil
}

// loadTags populates the mark's Tags from the tags table, sorted.
func (ix *Index) loadTags(m *book.Mark) error {
	rows, err := ix.db.Query(`SELECT tag FROM tags WHERE mark_id = ? ORDER BY tag`, m.ID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return err
		}
		m.Tags = append(m.Tags, tag)
	}
	return rows.Err()
}

// fileChanged reports whether the file's content differs from what is indexed.
// It returns true on a stat fast-path miss only after confirming the hash also
// differs, so a mere touch does not trigger a reindex.
func (ix *Index) fileChanged(tx *sql.Tx, path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	var (
		storedMtime int64
		storedSize  int64
		storedHash  string
	)
	err = tx.QueryRow(`SELECT mtime, size, sha256 FROM file_meta WHERE path = ?`, path).
		Scan(&storedMtime, &storedSize, &storedHash)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	mtime := info.ModTime().UnixNano()
	size := info.Size()
	if mtime == storedMtime && size == storedSize {
		return false, nil
	}

	sum, err := hashFile(path)
	if err != nil {
		return false, err
	}
	return sum != storedHash, nil
}

// indexShelfFile decodes a single shelf file and replaces its rows in the
// index, then records the file metadata used for future invalidation.
func (ix *Index) indexShelfFile(tx *sql.Tx, path string) error {
	var shelf book.Shelf
	if _, err := toml.DecodeFile(path, &shelf); err != nil {
		return err
	}
	shelf.FilePath = path

	// Ensure v2 identity fields exist so primary keys are never empty. This is
	// in-memory only; the TOML file is not rewritten here.
	if !shelf.IsV2() {
		MigrateShelf(&shelf)
	}
	for _, c := range shelf.Collections {
		for _, m := range c.Marks {
			if m.ID == "" {
				m.ID = book.GenerateID(m.URL)
			}
		}
	}

	if err := ix.deleteShelf(tx, shelf.ID, path); err != nil {
		return err
	}
	if err := insertShelf(tx, &shelf); err != nil {
		return err
	}
	return ix.recordFileMeta(tx, path)
}

// deleteShelf removes every row belonging to a shelf, identified by shelf_id,
// plus the file metadata for its path.
func (ix *Index) deleteShelf(tx *sql.Tx, shelfID, filePath string) error {
	if _, err := tx.Exec(`
		DELETE FROM marks_fts WHERE catalog_id IN (
			SELECT m.catalog_id FROM marks m
			JOIN collections c ON c.collection_id = m.collection_id
			WHERE c.shelf_id = ?
		)`, shelfID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		DELETE FROM tags WHERE mark_id IN (
			SELECT m.catalog_id FROM marks m
			JOIN collections c ON c.collection_id = m.collection_id
			WHERE c.shelf_id = ?
		)`, shelfID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM marks WHERE collection_id IN (SELECT collection_id FROM collections WHERE shelf_id = ?)`, shelfID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM collections WHERE shelf_id = ?`, shelfID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM shelves WHERE shelf_id = ?`, shelfID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM file_meta WHERE path = ?`, filePath); err != nil {
		return err
	}
	return nil
}

// insertShelf writes a shelf and all of its nested collections, marks, and tags.
func insertShelf(tx *sql.Tx, s *book.Shelf) error {
	if _, err := tx.Exec(`
		INSERT INTO shelves (shelf_id, name, description, created_at, updated_at, file_path)
		VALUES (?, ?, ?, ?, ?, ?)`,
		s.ID, s.Name, s.Description, s.CreatedAt, s.UpdatedAt, s.FilePath); err != nil {
		return err
	}

	for _, c := range s.Collections {
		if _, err := tx.Exec(`
			INSERT INTO collections (collection_id, shelf_id, name, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			c.ID, s.ID, c.Name, c.Description, c.CreatedAt, c.UpdatedAt); err != nil {
			return err
		}

		for _, m := range c.Marks {
			if _, err := tx.Exec(`
				INSERT INTO marks (catalog_id, collection_id, title, url, created_at, updated_at, deleted_at)
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				m.ID, c.ID, m.Name, m.URL, m.CreatedAt, m.UpdatedAt, m.DeletedAt); err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO marks_fts (title, url, catalog_id) VALUES (?, ?, ?)`, m.Name, m.URL, m.ID); err != nil {
				return err
			}
			for _, tag := range m.Tags {
				if _, err := tx.Exec(`INSERT INTO tags (mark_id, tag) VALUES (?, ?)`, m.ID, tag); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// recordFileMeta stores the stat and hash of a shelf file for invalidation.
func (ix *Index) recordFileMeta(tx *sql.Tx, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	sum, err := hashFile(path)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT OR REPLACE INTO file_meta (path, mtime, size, sha256, indexed_at)
		VALUES (?, ?, ?, ?, ?)`,
		path, info.ModTime().UnixNano(), info.Size(), sum, book.NowTimestamp())
	return err
}

// pruneRemoved deletes index rows for shelf files that no longer exist on disk.
func (ix *Index) pruneRemoved(tx *sql.Tx, seen map[string]bool, report *SyncReport) error {
	rows, err := tx.Query(`SELECT shelf_id, file_path FROM shelves`)
	if err != nil {
		return err
	}

	var stale []struct{ id, path string }
	for rows.Next() {
		var id, path string
		if err := rows.Scan(&id, &path); err != nil {
			_ = rows.Close()
			return err
		}
		if !seen[path] {
			stale = append(stale, struct{ id, path string }{id, path})
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, d := range stale {
		if err := ix.deleteShelf(tx, d.id, d.path); err != nil {
			return err
		}
		report.Removed++
	}
	return nil
}

// shelfFilePaths returns the shelf TOML files in the shelf directory.
func shelfFilePaths(config *book.Config) ([]string, error) {
	globDir := fmt.Sprintf("%s/*.%s", config.ShelfRoot, config.CatalogFormat)
	files, err := filepath.Glob(globDir)
	if err != nil {
		return nil, err
	}

	out := files[:0]
	for _, file := range files {
		if filepath.Base(file) == filepath.Base(config.ConfigFile) {
			continue
		}
		out = append(out, file)
	}
	return out, nil
}

// placeholders returns a comma-separated list of n SQL "?" placeholders.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// hashFile returns the lowercase hex SHA-256 of the file's contents.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ensureIndexGitignore best-effort adds the disposable index to the config
// directory's .gitignore so it is never committed alongside the TOML shelves.
func ensureIndexGitignore(dir string) {
	gi := filepath.Join(dir, ".gitignore")
	if data, err := os.ReadFile(gi); err == nil {
		if strings.Contains(string(data), "index.db") {
			return
		}
	}

	f, err := os.OpenFile(gi, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = fmt.Fprintln(f, "index.db")
}
