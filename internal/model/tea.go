// Package model handles all bubble TUI components
package model

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/polymorcodeus/book/internal/book"
)

const maxWidth = 120

// Styles holds the pre-built lipgloss styles for the TUI.
type Styles struct {
	Base,
	HeaderText,
	Status,
	StatusBox,
	StatusHeader,
	Highlight,
	Dim,
	ErrorHeaderText,
	Help,
	Primary,
	PrimaryAccent,
	Text,
	None,
	Form lipgloss.Style
}

// NewStyles builds a Styles instance from the current theme configuration.
func NewStyles(config *book.Config) *Styles {
	s := Styles{}
	s.Base = lipgloss.NewStyle().
		Padding(1, 4, 0, 1)
	s.HeaderText = lipgloss.NewStyle().
		Foreground(config.Theme.Color("primary")).
		Bold(true).
		Padding(0, 1, 0, 2)
	s.Status = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(config.Theme.Color("primary")).
		PaddingLeft(1).
		MarginTop(1)
	s.StatusBox = lipgloss.NewStyle().
		Margin(0, 1).
		Padding(1, 2).
		Width(48)
	s.StatusHeader = lipgloss.NewStyle().
		Foreground(config.Theme.Color("secondary_accent")).
		Bold(true)
	s.ErrorHeaderText = s.HeaderText.
		Foreground(config.Theme.Color("error"))
	s.Form = lipgloss.NewStyle().
		Margin(1, 0)
	s.Help = config.Theme.Style("help")
	s.Dim = config.Theme.Style("dimmed")
	s.Primary = config.Theme.Style("primary")
	s.PrimaryAccent = config.Theme.Style("primary_accent")
	s.Highlight = config.Theme.Style("highlight")
	s.Text = config.Theme.Style("text")
	s.None = lipgloss.NewStyle()
	return &s
}

type errMsg struct{ error }

type shelfSavedMsg struct{}

// Book is the shared TUI state container used by all screen models.
type Book struct {
	err     error
	styles  *Styles
	tmpls   map[string]book.ViewTemplate
	width   int
	form    *huh.Form
	shelves *book.BookShelves
}

func (b Book) errorView() string {
	var s strings.Builder
	for _, err := range b.form.Errors() {
		s.WriteString(err.Error())
	}
	return s.String()
}

func (b Book) appBoundaryView(text string) string {
	return lipgloss.PlaceHorizontal(
		b.width,
		lipgloss.Left,
		b.styles.HeaderText.Render(text),
		lipgloss.WithWhitespaceChars("❯"),
		lipgloss.WithWhitespaceStyle(b.styles.Primary),
	)
}

func (b Book) appErrorBoundaryView(text string) string {
	return lipgloss.PlaceHorizontal(
		b.width,
		lipgloss.Left,
		b.styles.ErrorHeaderText.Render(text),
		lipgloss.WithWhitespaceChars("❯"),
		lipgloss.WithWhitespaceStyle(b.styles.PrimaryAccent),
	)
}

// RootScreen wraps a tea.Model and delegates the BubbleTea lifecycle to it.
type RootScreen struct {
	Model tea.Model
}

func (m RootScreen) Init() tea.Cmd {
	return m.Model.Init()
}

func (m RootScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.Model.Update(msg)
}

// View now returns tea.View (bubbletea v2 breaking change).
func (m RootScreen) View() tea.View {
	return m.Model.View()
}

// SwitchScreen replaces the wrapped model and re-initializes it.
func (m RootScreen) SwitchScreen(model tea.Model) (tea.Model, tea.Cmd) {
	m.Model = model
	return m.Model, m.Model.Init()
}

func lipglossDimmer(style lipgloss.Style, title string, dim string) string {
	dimmer := lipgloss.NewStyle().Foreground(lipgloss.Color("237"))

	if dim == "" {
		return dimmer.Render(title) + "\n" + dim + "\n\n"
	}
	return style.Render(title) + "\n" + dim + "\n\n"
}

const listBullet string = "󱥸" // "nf-md-dots_circle"

func lipglossList(gloss lipgloss.Style, l []string) string {
	parts := make([]string, len(l))
	for i, s := range l {
		parts[i] = gloss.Render(listBullet, s)
	}
	return strings.Join(parts, "\n")
}

func renderCompletedView(s *Styles, tmpls map[string]book.ViewTemplate, key string, entity book.Templatable) tea.View {
	tmpl := tmpls[key]

	var b strings.Builder
	if strings.HasPrefix(key, "mark-") {
		if mark, ok := entity.(*book.Mark); ok {
			fmt.Fprintf(&b, "%s", renderView(s, tmpls["mark-parent"], mark.Collection))
		}
	}
	fmt.Fprintf(&b, "%s", renderView(s, tmpl, entity))

	return tea.NewView(s.StatusBox.Render(b.String()) + "\n")
}

func renderView(styles *Styles, tmpl book.ViewTemplate, entity book.Templatable) string {
	var b strings.Builder

	if tmpl.PrimaryTitle != "" && entity.Primary() != "" {
		fmt.Fprintf(&b, "%s\n%s\n\n", tmpl.PrimaryTitle, styles.Primary.Render(entity.Primary()))
	}
	if tmpl.SecondaryTitle != "" && entity.Secondary() != "" {
		fmt.Fprintf(&b, "%s\n%s", tmpl.SecondaryTitle, styles.Primary.Render(entity.Secondary()))
	}

	// needed for mark rendering
	if entity.Secondary() != "" && len(entity.List()) > 0 {
		fmt.Fprintf(&b, "\n\n")
	}

	if tmpl.ListTitle != "" && len(entity.List()) > 0 {
		fmt.Fprintf(&b, "%s\n%s", tmpl.ListTitle, lipglossList(styles.Primary, entity.List()))
	}

	return b.String()
}
