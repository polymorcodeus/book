package book

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
