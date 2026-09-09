// Package book data models, catalog theme and templates, and their methods
package book

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/polymorcodeus/book/internal/theme"
)

const errorBullet string = "󰯷" // "nf-md-alpha_e_box_outline

// TOMLFile defines exportable TOML files
type TOMLFile interface {
	FileDetail() string
}

// Config holds internal application configuration settings, including loaded files
type Config struct {
	CatalogFormat string                  `toml:"catalog_format"`
	ShelfRoot     string                  `toml:"shelf_directory"`
	Autoconfirm   bool                    `toml:"autoconfirm"`   // edit to bypass --confirm for non-interactive adds
	Interactive   bool                    `toml:"interactive"`   // edit to bypass TUI - false by default
	ConfigFile    string                  `toml:"-"`             // path to config file, typical BOOK_CONFIG
	ThemeFile     string                  `toml:"theme_file"`    // path to theme file, typical BOOK_THEME
	TemplateFile  string                  `toml:"template_file"` // path to theme file, typical BOOK_TEMPLATE
	Theme         *theme.Theme            `toml:"-"`             // loaded at run time
	Templates     map[string]ViewTemplate `toml:"-"`             // loaded at run time
}

// LoadTheme loads the theme file or falls back to defaults.
func (cfg *Config) LoadTheme(interactive bool) error {
	cfg.Theme = theme.NewTheme(&theme.ThemeConfig{})

	// load theme file or use defaults for TUI/interactive features
	if interactive {
		raw, err := theme.LoadThemeConfig(cfg.ThemeFile)
		if err != nil {
			return err
		}
		cfg.Theme = theme.NewTheme(raw) // nil raw = defaults
	}
	return nil
}

// LoadTemplates loads user template overrides on top of built-in defaults.
func (cfg *Config) LoadTemplates() error {
	cfg.Templates = make(map[string]ViewTemplate)

	// Start with defaults
	maps.Copy(cfg.Templates, DefaultViewTemplates)

	data, err := os.ReadFile(cfg.TemplateFile)
	if os.IsNotExist(err) {
		// Not an error — user hasn't customized, defaults are fine
		return nil
	}
	if err != nil {
		return fmt.Errorf("read templates: %w", err)
	}

	var userTmpls map[string]ViewTemplate
	if err := json.Unmarshal(data, &userTmpls); err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}

	// Overlay user partials onto defaults
	for k, user := range userTmpls {
		base, ok := cfg.Templates[k]
		if !ok {
			// Unknown key — skip or warn
			continue
		}
		if user.PrimaryTitle != "" {
			base.PrimaryTitle = user.PrimaryTitle
		}
		if user.SecondaryTitle != "" {
			base.SecondaryTitle = user.SecondaryTitle
		}
		if user.ListTitle != "" {
			base.ListTitle = user.ListTitle
		}
		cfg.Templates[k] = base
	}

	return nil
}

// StyledError returns a styled error string for interactive mode, or plain text otherwise.
func (cfg *Config) StyledError(e error) string {
	if !cfg.Interactive {
		return e.Error()
	}
	// return styled error only in interactive mode
	return cfg.Theme.Style("highlight").Render("HEAVENS TO MURGATROYD!") + "\n" +
		cfg.Theme.Style("error").Render(errorBullet, e.Error())
}

// FileConfig holds externally writable application configuration settings
type FileConfig struct {
	CatalogFormat string `toml:"catalog_format"`
	ShelfRoot     string `toml:"shelf_directory"`
	Autoconfirm   *bool  `toml:"autoconfirm"`   // edit to bypass --confirm for non-interactive adds
	Interactive   *bool  `toml:"interactive"`   // edit to bypass TUI - false by default
	ThemeFile     string `toml:"theme_file"`    // path to theme file, typical BOOK_THEME
	TemplateFile  string `toml:"template_file"` // path to theme file, typical BOOK_TEMPLATE
	ConfigFile    string `toml:"-"`             // path to config file, typical BOOK_CONFIG
}

// FileDetail returns the file path used to create the config TOML file.
func (f *FileConfig) FileDetail() string {
	return f.ConfigFile
}

// BookShelves is the top-level container for all shelf data.
type BookShelves []Shelf

// AddShelf appends a new shelf to the collection.
func (bs *BookShelves) AddShelf(shelf Shelf) {
	// Dereference bs (*bs) to get the slice, append,
	// and reassign the result to the dereferenced pointer
	*bs = append(*bs, shelf)
}

// Shelf returns a shelf by name, or a zero-value Shelf if not found.
func (bs *BookShelves) Shelf(s string) *Shelf {
	for i := range *bs {
		if (*bs)[i].Name == s {
			return &(*bs)[i]
		}
	}
	return &Shelf{}
}

// ShelfNames returns the names of all loaded shelves.
func (bs *BookShelves) ShelfNames() []string {
	shelvesNames := make([]string, 0, len(*bs))
	for _, p := range *bs {
		shelvesNames = append(shelvesNames, p.Name)
	}
	return shelvesNames
}

// LoadParents sets back-pointers from marks to their parent shelf and collection.
func (bs *BookShelves) LoadParents() {
	// Loads Shelf and Collection pointers in Marks
	for i := range *bs {
		shelf := &(*bs)[i]
		for _, c := range shelf.Collections {
			c.Shelf = shelf
			for _, m := range c.Marks {
				m.Shelf = shelf
				m.Collection = c
			}
		}
	}
}

// VerifyUniqueURL returns an error if the given ID already exists in any mark
// other than exclude. A collision with a soft-deleted mark points at
// `book mark restore` rather than re-adding the URL.
func (bs *BookShelves) VerifyUniqueURL(id string, exclude *Mark) error {
	for _, b := range *bs {
		for _, c := range b.Collections {
			for _, m := range c.Marks {
				if m == exclude || m.ID != id {
					continue
				}
				if m.IsDeleted() {
					return fmt.Errorf("url already trashed\n\n%s\n\nrestore it with:\nbook mark restore --shelf %s --collection %s --url %s",
						m.FullDetail(), m.Shelf.Name, m.Collection.Name, m.URL)
				}
				return fmt.Errorf("duplicate url found\n\n%s", m.FullDetail())
			}
		}
	}
	return nil
}

// PurgeDeletedMarks hard-removes soft-deleted marks older than cutoff from every
// shelf, returning the number of marks removed.
func (bs *BookShelves) PurgeDeletedMarks(cutoff time.Time) int {
	total := 0
	for i := range *bs {
		total += (*bs)[i].PurgeDeletedMarks(cutoff)
	}
	return total
}

// SoftDeletedByID returns the soft-deleted mark whose ID matches, or nil. IDs
// are globally unique, so no shelf or collection scoping is needed. The
// returned mark retains its Shelf and Collection back-pointers after
// LoadParents.
func (bs *BookShelves) SoftDeletedByID(id string) *Mark {
	for i := range *bs {
		for _, c := range (*bs)[i].Collections {
			for _, m := range c.Marks {
				if m.ID == id && m.IsDeleted() {
					return m
				}
			}
		}
	}
	return nil
}

// Shelf is a named container for collections stored in a single TOML file.
type Shelf struct {
	SchemaVersion *int   `toml:"schema_version,omitempty" json:"schema_version,omitempty"`
	ID            string `toml:"shelf_id,omitempty" json:"shelf_id,omitempty"`
	Name          string `toml:"shelf_name" json:"shelf_name"`
	Description   string `toml:"shelf_desc,omitempty" json:"shelf_desc,omitempty"`
	CreatedAt     string `toml:"created_at,omitempty" json:"created_at,omitempty"`
	UpdatedAt     string `toml:"updated_at,omitempty" json:"updated_at,omitempty"`
	Collections   map[string]*Collection
	FilePath      string `toml:"-" json:"-"`
}

// AddFileDetail sets the on-disk file path for the shelf based on its name.
func (s *Shelf) AddFileDetail(c *Config) {
	formattedName := strings.ReplaceAll(strings.TrimSpace(s.Name), " ", "_")
	fileName := fmt.Sprintf("%s.%s", formattedName, c.CatalogFormat)

	s.FilePath = filepath.Join(c.ShelfRoot, fileName)
}

// FileDetail returns the file path of the shelf's TOML file.
func (s *Shelf) FileDetail() string {
	return s.FilePath
}

// IsV2 reports whether the shelf uses the v2 schema (has schema_version set).
func (s *Shelf) IsV2() bool {
	return s.SchemaVersion != nil && *s.SchemaVersion >= 2
}

// Collection returns a collection by name from the shelf.
func (s *Shelf) Collection(c string) *Collection {
	return s.Collections[c]
}

// AddCollection registers a collection in the shelf.
func (s *Shelf) AddCollection(c *Collection) {
	if s.Collections == nil {
		s.Collections = make(map[string]*Collection)
	}
	s.Collections[c.Name] = c
}

// Touch updates the shelf's UpdatedAt if it tracks v2 timestamps.
func (s *Shelf) Touch() {
	if s.IsV2() {
		s.UpdatedAt = NowTimestamp()
	}
}

// PurgeDeletedMarks hard-removes soft-deleted marks older than cutoff from every
// collection in the shelf, returning the number of marks removed.
func (s *Shelf) PurgeDeletedMarks(cutoff time.Time) int {
	total := 0
	for _, c := range s.Collections {
		total += c.PurgeDeletedMarks(cutoff)
	}
	return total
}

// CollectionsNames returns the names of all collections in the shelf, sorted.
func (s *Shelf) CollectionsNames() []string {
	names := make([]string, 0, len(s.Collections))
	for name := range s.Collections {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Collection is a named grouping of bookmarks within a shelf.
type Collection struct {
	Shelf       *Shelf  `toml:"-" json:"-"`
	ID          string  `toml:"collection_id,omitempty" json:"collection_id,omitempty"`
	Name        string  `toml:"collection_name" json:"collection_name"`
	Description string  `toml:"collection_desc,omitempty" json:"collection_desc,omitempty"`
	CreatedAt   string  `toml:"created_at,omitempty" json:"created_at,omitempty"`
	UpdatedAt   string  `toml:"updated_at,omitempty" json:"updated_at,omitempty"`
	Marks       []*Mark `toml:"marks" json:"marks"`
}

// MarksNames returns the names of all non-deleted marks in the collection.
func (c *Collection) MarksNames() []string {
	markNames := make([]string, 0, len(c.Marks))
	for _, m := range c.Marks {
		if m.IsDeleted() {
			continue
		}
		markNames = append(markNames, m.Name)
	}
	return markNames
}

// HasActiveMarks reports whether the collection contains any non-deleted mark.
func (c *Collection) HasActiveMarks() bool {
	for _, m := range c.Marks {
		if !m.IsDeleted() {
			return true
		}
	}
	return false
}

// AllTags returns every tag across all non-deleted marks in the collection, sorted.
func (c *Collection) AllTags() []string {
	var tags []string
	for _, m := range c.Marks {
		if m.IsDeleted() {
			continue
		}
		tags = append(tags, m.Tags...)
	}
	slices.Sort(tags)
	return tags
}

// Mark returns a non-deleted mark by name from the collection.
func (c *Collection) Mark(m string) *Mark {
	for _, n := range c.Marks {
		if n.Name == m && !n.IsDeleted() {
			return n
		}
	}
	return nil
}

// AddMark appends a mark to the collection.
func (c *Collection) AddMark(m *Mark) {
	c.Marks = append(c.Marks, m)
}

// Touch updates the collection's UpdatedAt and cascades to its shelf when the
// shelf tracks v2 timestamps.
func (c *Collection) Touch() {
	now := NowTimestamp()
	if c.UpdatedAt != "" {
		c.UpdatedAt = now
	}
	if c.Shelf != nil && c.Shelf.IsV2() {
		c.Shelf.UpdatedAt = now
	}
}

// DeleteMark soft-deletes the given mark by stamping its DeletedAt field. The
// mark remains in the collection until gc purges it, so accidental removals can
// be restored.
func (c *Collection) DeleteMark(m *Mark) {
	if m == nil || m.DeletedAt != "" {
		return
	}
	m.DeletedAt = NowTimestamp()
}

// RemoveMark hard-removes the given mark from the collection without stamping
// deleted_at. It is used by book doctor to drop merge-duplicated marks and must
// not be used for user-initiated removal (which should soft-delete instead).
func (c *Collection) RemoveMark(m *Mark) {
	c.Marks = slices.DeleteFunc(c.Marks, func(x *Mark) bool { return x == m })
}

// PurgeDeletedMarks hard-removes soft-deleted marks whose DeletedAt timestamp is
// strictly before cutoff, returning the number of marks removed. Marks with an
// empty or unparseable DeletedAt are retained.
func (c *Collection) PurgeDeletedMarks(cutoff time.Time) int {
	removed := 0
	c.Marks = slices.DeleteFunc(c.Marks, func(m *Mark) bool {
		if m == nil || m.DeletedAt == "" {
			return false
		}
		t, err := time.Parse(time.RFC3339, m.DeletedAt)
		if err != nil || !t.Before(cutoff) {
			return false
		}
		removed++
		return true
	})
	return removed
}

// Mark is a single bookmark with a title, URL, tags, and back-references.
type Mark struct {
	Shelf      *Shelf      `toml:"-" json:"-"`
	Collection *Collection `toml:"-" json:"-"`

	ID        string   `toml:"catalog_id" json:"catalog_id"`
	Name      string   `toml:"title" json:"title"`
	URL       string   `toml:"url" json:"url"`
	Tags      []string `toml:"tags" json:"tags"`
	CreatedAt string   `toml:"created_at,omitempty" json:"created_at,omitempty"`
	UpdatedAt string   `toml:"updated_at,omitempty" json:"updated_at,omitempty"`
	DeletedAt string   `toml:"deleted_at,omitempty" json:"deleted_at,omitempty"`
}

// Description returns a human-readable summary of the mark.
func (m *Mark) Description() string {
	return fmt.Sprintf("Title: %s\nURL: %s\nTags: %s", m.Name, m.URL, strings.Join(m.Tags, ","))
}

// IsDeleted reports whether the mark has been soft-deleted.
func (m *Mark) IsDeleted() bool {
	return m.DeletedAt != ""
}

// Touch updates the mark's UpdatedAt and cascades to its collection and shelf
// when they track v2 timestamps.
func (m *Mark) Touch() {
	now := NowTimestamp()
	m.UpdatedAt = now
	if m.Collection != nil && m.Collection.UpdatedAt != "" {
		m.Collection.UpdatedAt = now
	}
	if m.Shelf != nil && m.Shelf.IsV2() {
		m.Shelf.UpdatedAt = now
	}
}

// RecordAdd sets CreatedAt and UpdatedAt and cascades the update to the mark's
// collection and shelf.
func (m *Mark) RecordAdd() {
	m.CreatedAt = NowTimestamp()
	m.Touch()
}

// RecordDelete soft-deletes the mark and cascades the update to its collection
// and shelf.
func (m *Mark) RecordDelete() {
	if m.Collection != nil {
		m.Collection.DeleteMark(m)
	}
	m.Touch()
}

// FullDetail returns a verbose summary including shelf, collection, title, URL, and tags.
func (m *Mark) FullDetail() string {
	return fmt.Sprintf("Shelf: %s\nCollection: %s\nTitle: %s\nURL: %s\nTags: %s", m.Shelf.Name, m.Collection.Name, m.Name, m.URL, strings.Join(m.Tags, ","))
}

// DedupUnique concatenates and deduplicates multiple slices while preserving first-seen order.
func DedupUnique[T comparable](slice ...[]T) []T {
	merged := slices.Concat(slice...)
	seen := make(map[T]struct{}, len(merged))
	unique := make([]T, 0, len(merged))
	for _, v := range merged {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			unique = append(unique, v)
		}
	}
	return unique
}

// StructIsEmpty reports whether the given struct pointer is nil or contains only zero values.
func StructIsEmpty[T any](ptr *T) bool {
	if ptr == nil {
		return true
	}

	val := reflect.ValueOf(ptr).Elem()

	// This will return true if all fields within the struct have their
	// zero values (e.g., 0 for int, "" for string, nil for pointers, etc.).
	return val.IsZero()
}

// GenerateID returns the first 8 hex characters of the SHA-256 hash of a URL.
func GenerateID(url string) string {
	hash := sha256.Sum256([]byte(url))
	// Return the first 8 characters of the hex representation
	return fmt.Sprintf("%x", hash)[:8]
}

// GenerateShelfID returns a deterministic ID for a shelf from its name.
func GenerateShelfID(name string) string {
	return GenerateID(name)
}

// GenerateCollectionID returns a deterministic ID for a collection from its
// shelf name and collection name.
func GenerateCollectionID(shelfName, collectionName string) string {
	return GenerateID(shelfName + "/" + collectionName)
}

// NowTimestamp returns the current UTC time formatted as RFC3339.
func NowTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// IntPtr returns a pointer to the given int value.
func IntPtr(v int) *int {
	return &v
}

// MergeTags combines multiple tag slices, deduplicates, removes empty strings,
// and returns a sorted slice. Order of arguments determines priority (earlier
// slices' items appear first in result).
func MergeTags(sources ...[]string) []string {
	merged := DedupUnique(sources...)
	return slices.DeleteFunc(merged, func(e string) bool { return e == "" })
}

// SplitTags parses a comma-separated tag string into trimmed, non-empty tags.
func SplitTags(input string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	parts := strings.Split(input, ",")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// SplitTagLines parses whitespace-separated tag text (as entered in the TUI
// free-tag field) into trimmed, non-empty tags.
func SplitTagLines(input string) []string {
	return strings.Fields(input)
}

// ValidateURL checks that raw is a parseable, absolute URL suitable for a mark.
func ValidateURL(raw string) error {
	if _, err := url.ParseRequestURI(raw); err != nil {
		return err
	}
	return nil
}

// NewMarkFromInput builds a Mark from raw input, validating the URL and
// generating a stable catalog ID. It does not resolve titles or parent pointers.
func NewMarkFromInput(rawURL string, tags []string) (Mark, error) {
	if err := ValidateURL(rawURL); err != nil {
		return Mark{}, err
	}
	return Mark{
		ID:   GenerateID(rawURL),
		URL:  rawURL,
		Tags: tags,
	}, nil
}

// TitleFetchResult carries the outcome of fetching a page title for a mark.
type TitleFetchResult struct {
	Title       string
	Unavailable bool
}

// ResolveMarkTitle selects the title for a new mark. A caller-provided title
// always wins. When fetching is unavailable, interactive callers receive an
// empty string (so the TUI can prompt later), while non-interactive callers
// receive an error asking them to provide --title.
func ResolveMarkTitle(providedTitle, url string, fetched TitleFetchResult, interactive bool) (string, error) {
	if providedTitle != "" {
		return providedTitle, nil
	}
	if fetched.Unavailable {
		if interactive {
			return "", nil
		}
		return "", fmt.Errorf("couldn't fetch title for %s; provide --title", url)
	}
	return fetched.Title, nil
}

// ValidateNewShelfName returns an error if name is empty or already in use.
func (bs *BookShelves) ValidateNewShelfName(name string) error {
	if name == "" {
		return fmt.Errorf("shelf name is required")
	}
	if slices.Contains(bs.ShelfNames(), name) {
		return fmt.Errorf("shelf already exists")
	}
	return nil
}

// ParseTagFilter parses the search --tags grammar into AND clauses of OR tags.
// A plus (+) separates AND clauses and a comma (,) separates OR alternatives
// within a clause, so "a,b+c" means (a OR b) AND c. Empty groups (for example
// "a,", ",a", "a+", or "+a") are rejected. A blank input yields nil.
func ParseTagFilter(input string) ([][]string, error) {
	if strings.TrimSpace(input) == "" {
		return nil, nil
	}

	clauses := strings.Split(input, "+")
	result := make([][]string, 0, len(clauses))
	for _, clause := range clauses {
		rawTags := strings.Split(clause, ",")
		tags := make([]string, 0, len(rawTags))
		for _, raw := range rawTags {
			tag := strings.TrimSpace(raw)
			if tag == "" {
				return nil, fmt.Errorf("empty tag group in %q", input)
			}
			tags = append(tags, tag)
		}
		result = append(result, tags)
	}
	return result, nil
}

// PrintCatalog serializes an item as JSON or TOML to stdout.
func PrintCatalog[T any](item T, format string) error {
	switch format {
	case "json":
		jsonData, err := json.MarshalIndent(item, "", "  ")
		if err != nil {
			return err
		}
		fmt.Print(string(jsonData))
	case "toml":
		if err := toml.NewEncoder(os.Stdout).Encode(item); err != nil {
			return err
		}
	}
	return nil
}

// NewShelf creates a new v2 shelf with the given name and description.
func NewShelf(name, description string) (*Shelf, error) {
	if name == "" {
		return nil, fmt.Errorf("shelf name is required")
	}
	now := NowTimestamp()
	return &Shelf{
		SchemaVersion: IntPtr(2),
		ID:            GenerateShelfID(name),
		Name:          name,
		Description:   description,
		CreatedAt:     now,
		UpdatedAt:     now,
		Collections:   make(map[string]*Collection),
	}, nil
}

// RemoveShelf removes a shelf by name from the loaded shelves and returns the
// removed shelf so the caller can delete its on-disk file.
func (bs *BookShelves) RemoveShelf(name string) (Shelf, error) {
	for i := range *bs {
		if (*bs)[i].Name == name {
			removed := (*bs)[i]
			*bs = append((*bs)[:i], (*bs)[i+1:]...)
			return removed, nil
		}
	}
	return Shelf{}, fmt.Errorf("shelf %q not found", name)
}

// NewCollection creates a new collection and wires it to the given shelf.
func NewCollection(shelf *Shelf, name, description string) (*Collection, error) {
	if shelf == nil || shelf.Name == "" {
		return nil, fmt.Errorf("shelf is required")
	}
	if name == "" {
		return nil, fmt.Errorf("collection name is required")
	}
	now := NowTimestamp()
	return &Collection{
		Shelf:       shelf,
		ID:          GenerateCollectionID(shelf.Name, name),
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// RemoveCollection removes a collection by name from the shelf.
func (s *Shelf) RemoveCollection(name string) error {
	if _, ok := s.Collections[name]; !ok {
		return fmt.Errorf("collection %q not found in shelf %q", name, s.Name)
	}
	delete(s.Collections, name)
	return nil
}

// FindMarkByID returns the first mark matching id across all shelves and
// collections, or nil if none is found.
func (bs *BookShelves) FindMarkByID(id string) *Mark {
	for i := range *bs {
		shelf := &(*bs)[i]
		for _, c := range shelf.Collections {
			for _, m := range c.Marks {
				if m.ID == id {
					m.Shelf = shelf
					m.Collection = c
					return m
				}
			}
		}
	}
	return nil
}

// FindMarkByURL returns the first non-deleted mark matching url across all
// shelves and collections, or nil if none is found.
func (bs *BookShelves) FindMarkByURL(url string) *Mark {
	for i := range *bs {
		shelf := &(*bs)[i]
		for _, c := range shelf.Collections {
			for _, m := range c.Marks {
				if m.URL == url && !m.IsDeleted() {
					m.Shelf = shelf
					m.Collection = c
					return m
				}
			}
		}
	}
	return nil
}

// UpdateMark applies edits to a mark. Empty strings leave title and url
// unchanged; a nil tags slice leaves tags unchanged, while an empty (non-nil)
// slice clears them. If url is provided it is validated and the mark's catalog
// ID is regenerated.
func (m *Mark) UpdateMark(title, url string, tags []string) error {
	if url != "" {
		if err := ValidateURL(url); err != nil {
			return err
		}
		m.URL = url
		m.ID = GenerateID(url)
	}
	if title != "" {
		m.Name = title
	}
	if tags != nil {
		m.Tags = tags
	}
	return nil
}
