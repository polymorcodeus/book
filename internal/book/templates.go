package book

// Templatable defines the view-model interface used to render TUI success screens.
type Templatable interface {
	Primary() string
	Secondary() string
	List() []string
}

// Primary returns an empty string for BookShelves.
func (bs *BookShelves) Primary() string   { return "" }
// Secondary returns an empty string for BookShelves.
func (bs *BookShelves) Secondary() string { return "" }
// List returns the names of all shelves.
func (bs *BookShelves) List() []string    { return bs.ShelfNames() }

// Primary returns the shelf name.
func (s *Shelf) Primary() string   { return s.Name }
// Secondary returns an empty string for Shelf.
func (s *Shelf) Secondary() string { return "" }
// List returns the names of all collections in the shelf.
func (s *Shelf) List() []string    { return s.CollectionsNames() }

// Primary returns the parent shelf name.
func (c *Collection) Primary() string   { return c.Shelf.Name }
// Secondary returns the collection name.
func (c *Collection) Secondary() string { return c.Name }
// List returns the names of all marks in the collection.
func (c *Collection) List() []string    { return c.MarksNames() }

// Primary returns the mark title.
func (m *Mark) Primary() string   { return m.Name }
// Secondary returns the mark URL.
func (m *Mark) Secondary() string { return m.URL }
// List returns the mark's tags.
func (m *Mark) List() []string    { return m.Tags }

// ViewTemplate used to templatize TUI success screens
type ViewTemplate struct {
	PrimaryTitle   string `json:"primary_title,omitempty"`
	SecondaryTitle string `json:"secondary_title,omitempty"`
	ListTitle      string `json:"list_title,omitempty"`
}

// Built-in defaults. Every key should have a complete entry here.
var (
	defaultShelfTitle      = "Chosen shelf:"
	defaultCollectionTitle = "Chosen collection:"

	// Base templates for common combinations
	shelfOnly          = ViewTemplate{PrimaryTitle: defaultShelfTitle}
	shelfAndCollection = ViewTemplate{
		PrimaryTitle:   defaultShelfTitle,
		SecondaryTitle: defaultCollectionTitle,
	}
)

func withList(t ViewTemplate, title string) ViewTemplate {
	t.ListTitle = title
	return t
}

func withSecondary(t ViewTemplate, title string) ViewTemplate {
	t.SecondaryTitle = title
	return t
}

func markTemplate(primary string) ViewTemplate {
	return ViewTemplate{
		PrimaryTitle:   primary,
		SecondaryTitle: "With URL:",
		ListTitle:      "With tags:",
	}
}

// DefaultViewTemplates provides the built-in TUI success-screen templates.
var DefaultViewTemplates = map[string]ViewTemplate{
	"shelf-list": {ListTitle: "You've knocked over all your shelves:"},
	"shelf-add":  {PrimaryTitle: "You added shelf:", SecondaryTitle: "Along chosen collection:"},

	"collection-list": withList(shelfOnly, "With the list of collections:"),
	"collection-add":  withSecondary(shelfOnly, "To add the collection:"),

	"mark-parent": shelfAndCollection,
	"mark-list":   withList(shelfAndCollection, "With the list of marks:"),
	"mark-add":    markTemplate("To add mark:"),
	"mark-edit":   markTemplate("To edit mark:"),
	"mark-get":    markTemplate("To open mark:"),
	"mark-delete": markTemplate("To delete mark:"),
}

// UserViewTemplates holds user-defined template overrides loaded at runtime.
var UserViewTemplates = map[string]ViewTemplate{}

// GetTemplate merges user overrides over the default for a given key.
func GetTemplate(key string) ViewTemplate {
	def, hasDef := DefaultViewTemplates[key]
	user, hasUser := UserViewTemplates[key]

	switch {
	case !hasDef && !hasUser:
		return ViewTemplate{}
	case !hasUser:
		return def
	case !hasDef:
		return user
	}

	// Overlay: user field non-empty → override default.
	if user.PrimaryTitle != "" {
		def.PrimaryTitle = user.PrimaryTitle
	}
	if user.SecondaryTitle != "" {
		def.SecondaryTitle = user.SecondaryTitle
	}
	if user.ListTitle != "" {
		def.ListTitle = user.ListTitle
	}
	return def
}
