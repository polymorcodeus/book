package model

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/polymorcodeus/book/pkg/book"
)

// plainStyles returns Styles with no color or spacing so rendered output is a
// stable string in tests.
func plainStyles() *Styles {
	return &Styles{
		Primary:   lipgloss.NewStyle(),
		StatusBox: lipgloss.NewStyle(),
		None:      lipgloss.NewStyle(),
	}
}

func TestRenderViewSections(t *testing.T) {
	tmpl := book.ViewTemplate{
		PrimaryTitle:   "To add mark:",
		SecondaryTitle: "With URL:",
		ListTitle:      "With tags:",
	}
	got := renderView(plainStyles(), tmpl, viewData{
		Primary:   "The Go Programming Language",
		Secondary: "https://go.dev",
		List:      []string{"lang", "official"},
	})

	want := "To add mark:\nThe Go Programming Language\n\n" +
		"With URL:\nhttps://go.dev\n\n" +
		"With tags:\n" + listBullet + " lang\n" + listBullet + " official"
	if got != want {
		t.Errorf("renderView() = %q, want %q", got, want)
	}
}

func TestRenderViewSkipsEmptySections(t *testing.T) {
	tmpl := book.ViewTemplate{ListTitle: "Shelves:"}
	got := renderView(plainStyles(), tmpl, viewData{List: []string{"archive"}})

	if strings.Contains(got, "To add") {
		t.Errorf("renderView() rendered an unset section: %q", got)
	}
	if want := "Shelves:\n" + listBullet + " archive"; got != want {
		t.Errorf("renderView() = %q, want %q", got, want)
	}
}

func TestRenderCompletedViewRendersParentFirst(t *testing.T) {
	got := renderCompletedView(plainStyles(), book.DefaultViewTemplates, "mark-add", viewData{
		Primary:   "The Go Programming Language",
		Secondary: "https://go.dev",
		List:      []string{"lang"},
		Parent:    &viewData{Primary: "work", Secondary: "golang"},
	}).Content

	shelf := strings.Index(got, "Chosen shelf:\nwork")
	collection := strings.Index(got, "Chosen collection:\ngolang")
	mark := strings.Index(got, "To add mark:\nThe Go Programming Language")
	if shelf < 0 || collection < 0 || mark < 0 {
		t.Fatalf("renderCompletedView() missing a section: %q", got)
	}
	if shelf >= collection || collection >= mark {
		t.Errorf("parent section not rendered above the mark: %q", got)
	}
	if !strings.Contains(got, "golang\n\nTo add mark:") {
		t.Errorf("parent and mark sections are not separated by a blank line: %q", got)
	}
}

func TestRenderCompletedViewOmitsParentWhenUnset(t *testing.T) {
	got := renderCompletedView(plainStyles(), book.DefaultViewTemplates, "shelf-list", viewData{
		List: []string{"archive", "dev"},
	}).Content

	if strings.Contains(got, "Chosen collection:") {
		t.Errorf("renderCompletedView() rendered a parent section with no Parent set: %q", got)
	}
	if !strings.Contains(got, "You've knocked over all your shelves:") {
		t.Errorf("renderCompletedView() missing the list title: %q", got)
	}
}
