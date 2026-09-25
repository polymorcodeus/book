package theme

import (
	"errors"
	"strings"
	"testing"

	"github.com/polymorcodeus/book/pkg/book"
)

func TestUIConfigStyledError(t *testing.T) {
	t.Run("plain in non-interactive mode", func(t *testing.T) {
		cfg := &UIConfig{Config: &book.Config{Interactive: false}}
		got := cfg.StyledError(errors.New("boom"))
		if got != "boom" {
			t.Errorf("got %q, want %q", got, "boom")
		}
	})

	t.Run("styled in interactive mode", func(t *testing.T) {
		cfg := &UIConfig{Config: &book.Config{Interactive: true}}
		cfg.Theme = NewTheme(nil)
		got := cfg.StyledError(errors.New("boom"))
		if !strings.Contains(got, "HEAVENS TO MURGATROYD!") {
			t.Errorf("styled error missing header: %q", got)
		}
		if !strings.Contains(got, "boom") {
			t.Errorf("styled error missing message: %q", got)
		}
	})
}
