package book

type Templatable interface {
	Primary() string
	Secondary() string
	List() []string
}

// Template interface methods
func (s *BookShelves) Primary() string   { return "" }
func (s *BookShelves) Secondary() string { return "" }
func (s *BookShelves) List() []string    { return s.ShelfNames() }

func (s *Shelf) Primary() string   { return s.Name }
func (s *Shelf) Secondary() string { return "" }
func (s *Shelf) List() []string    { return s.CollectionsNames() }

func (s *Collection) Primary() string   { return s.Shelf.Name }
func (s *Collection) Secondary() string { return s.Name }
func (s *Collection) List() []string    { return s.MarksNames() }

func (s *Mark) Primary() string   { return s.Name }
func (s *Mark) Secondary() string { return s.Url }
func (s *Mark) List() []string    { return s.Tags }

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
