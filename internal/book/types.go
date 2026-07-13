package book

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/polymorcodeus/book/internal/theme"
)

const errorBullet string = "󰯷" // "nf-md-alpha_e_box_outline

type TOMLFile interface {
	FileDetail() string
}

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

func (c *Config) LoadTheme(interactive bool) error {
	c.Theme = theme.NewTheme(&theme.ThemeConfig{})

	// load theme file or use defaults for TUI/interactive features
	if interactive {
		raw, err := theme.LoadThemeConfig(c.ThemeFile)
		if err != nil {
			return err
		}
		c.Theme = theme.NewTheme(raw) // nil raw = defaults
	}
	return nil
}

func (c *Config) LoadTemplates() error {
	c.Templates = make(map[string]ViewTemplate)

	// Start with defaults
	maps.Copy(c.Templates, DefaultViewTemplates)

	data, err := os.ReadFile(c.TemplateFile)
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
		base, ok := c.Templates[k]
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
		c.Templates[k] = base
	}

	return nil
}

func (c *Config) StyledError(e error) string {
	if !c.Interactive {
		return e.Error()
	}
	// return styled error only in interactive mode
	return c.Theme.Style("highlight").Render("HEAVENS TO MURGATROYD!") + "\n" +
		c.Theme.Style("error").Render(errorBullet, e.Error())
}

type FileConfig struct {
	CatalogFormat string `toml:"catalog_format"`
	ShelfRoot     string `toml:"shelf_directory"`
	Autoconfirm   *bool  `toml:"autoconfirm"`   // edit to bypass --confirm for non-interactive adds
	Interactive   *bool  `toml:"interactive"`   // edit to bypass TUI - false by default
	ThemeFile     string `toml:"theme_file"`    // path to theme file, typical BOOK_THEME
	TemplateFile  string `toml:"template_file"` // path to theme file, typical BOOK_TEMPLATE
	ConfigFile    string `toml:"-"`             // path to config file, typical BOOK_CONFIG
}

func (f *FileConfig) FileDetail() string {
	return f.ConfigFile
}

type BookShelves []Shelf

func (bs *BookShelves) AddShelf(shelf Shelf) {
	// Dereference bs (*bs) to get the slice, append,
	// and reassign the result to the dereferenced pointer
	*bs = append(*bs, shelf)
}

func (bs *BookShelves) Shelf(s string) *Shelf {
	for i := range *bs {
		if (*bs)[i].Name == s {
			return &(*bs)[i]
		}
	}
	return &Shelf{}
}

func (bs *BookShelves) ShelfNames() []string {
	shelvesNames := make([]string, 0, len(*bs))
	for _, p := range *bs {
		shelvesNames = append(shelvesNames, p.Name)
	}
	return shelvesNames
}

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

func (bs *BookShelves) VerifyUniqueURL(id string) error {
	for _, b := range *bs {
		for _, c := range b.Collections {
			for _, m := range c.Marks {
				if m.Id == id {
					errMessage := fmt.Sprintf("duplicate URL Found!\n\n%s", m.FullDetail())
					return fmt.Errorf("%s", errMessage)
				}
			}
		}
	}
	return nil
}

type Shelf struct {
	Name        string `toml:"shelf_name" json:"shelf_name"`
	Description string `toml:"shelf_desc,omitempty" json:"shelf_desc,omitempty"`
	Collections map[string]*Collection
	FilePath    string `toml:"-" json:"-"`
}

func (s *Shelf) AddFileDetail(c *Config) {
	formattedName := strings.ReplaceAll(strings.TrimSpace(s.Name), " ", "_")
	fileName := fmt.Sprintf("%s.%s", formattedName, c.CatalogFormat)

	s.FilePath = filepath.Join(c.ShelfRoot, fileName)
}

func (s *Shelf) FileDetail() string {
	return s.FilePath
}

func (s *Shelf) Collection(c string) *Collection {
	return s.Collections[c]
}

func (s *Shelf) AddCollection(c *Collection) {
	if s.Collections == nil {
		s.Collections = make(map[string]*Collection)
	}
	s.Collections[c.Name] = c
}

func (s *Shelf) CollectionsNames() []string {
	names := make([]string, 0, len(s.Collections))
	for name := range s.Collections {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

type Collection struct {
	Shelf       *Shelf  `toml:"-" json:"-"`
	Name        string  `toml:"collection_name" json:"collection_name"`
	Description string  `toml:"collection_desc,omitempty" json:"collection_desc,omitempty"`
	Marks       []*Mark `toml:"marks" json:"marks"`
}

func (c *Collection) MarksNames() []string {
	markNames := make([]string, 0, len(c.Marks))
	for _, m := range c.Marks {
		markNames = append(markNames, m.Name)
	}
	return markNames
}

func (c *Collection) AllTags() []string {
	var tags []string
	for _, m := range c.Marks {
		tags = append(tags, m.Tags...)
	}
	slices.Sort(tags)
	return tags
}

func (c *Collection) Mark(m string) *Mark {
	for _, n := range c.Marks {
		if n.Name == m {
			return n
		}
	}
	return nil
}

func (c *Collection) AddMark(m *Mark) {
	c.Marks = append(c.Marks, m)
}

func (c *Collection) DeleteMark(m *Mark) {
	c.Marks = slices.DeleteFunc(c.Marks, func(d *Mark) bool {
		return d == m
	})
}

type Mark struct {
	Shelf      *Shelf      `toml:"-" json:"-"`
	Collection *Collection `toml:"-" json:"-"`

	Id   string   `toml:"catalog_id" json:"catalog_id"`
	Name string   `toml:"title" json:"title"`
	Url  string   `toml:"url" json:"url"`
	Tags []string `toml:"tags" json:"tags"`
}

func (m *Mark) Description() string {
	return fmt.Sprintf("Title: %s\nURL: %s\nTags: %s", m.Name, m.Url, strings.Join(m.Tags, ","))
}

func (m *Mark) FullDetail() string {
	return fmt.Sprintf("Shelf: %s\nCollection: %s\nTitle: %s\nURL: %s\nTags: %s", m.Shelf.Name, m.Collection.Name, m.Name, m.Url, strings.Join(m.Tags, ","))
}

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

func StructIsEmpty[T any](ptr *T) bool {
	if ptr == nil {
		return true
	}

	val := reflect.ValueOf(ptr).Elem()

	// This will return true if all fields within the struct have their
	// zero values (e.g., 0 for int, "" for string, nil for pointers, etc.).
	return val.IsZero()
}

func GenerateID(url string) string {
	hash := sha256.Sum256([]byte(url))
	// Return the first 8 characters of the hex representation
	return fmt.Sprintf("%x", hash)[:8]
}

// MergeTags combines multiple tag slices, deduplicates, removes empty strings,
// and returns a sorted slice. Order of arguments determines priority (earlier
// slices' items appear first in result).
func MergeTags(sources ...[]string) []string {
	merged := DedupUnique(sources...)
	return slices.DeleteFunc(merged, func(e string) bool { return e == "" })
}

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
