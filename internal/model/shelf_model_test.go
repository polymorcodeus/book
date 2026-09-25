package model

import (
	"maps"
	"strings"
	"testing"

	"github.com/polymorcodeus/book/internal/theme"
	"github.com/polymorcodeus/book/pkg/book"
)

func testUIConfig() *theme.UIConfig {
	cfg := &theme.UIConfig{
		Config:    &book.Config{Interactive: true},
		Theme:     theme.NewTheme(nil),
		Templates: make(map[string]book.ViewTemplate),
	}
	maps.Copy(cfg.Templates, book.DefaultViewTemplates)
	return cfg
}

func TestGetShelfFormSkipProgram(t *testing.T) {
	shelf, err := book.NewShelf("archive", "")
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	var bs book.BookShelves
	bs.AddShelf(*shelf)

	t.Run("list skips the program and renders immediately", func(t *testing.T) {
		m := GetShelfForm(&bs, testUIConfig(), "list")
		if !m.SkipProgram() {
			t.Fatal("list action should skip the tea program")
		}
		if got := m.ResultView(); !strings.Contains(got, "archive") {
			t.Errorf("ResultView missing shelf name, got %q", got)
		}
	})

	t.Run("other actions keep the program", func(t *testing.T) {
		for _, action := range []string{"add", "edit", "get"} {
			m := GetShelfForm(&bs, testUIConfig(), action)
			if m.SkipProgram() {
				t.Errorf("action %q should run the tea program", action)
			}
		}
	})
}
