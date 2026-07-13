// Package theme skins bubbletea components and other interactive views using
// user provided theme.json or defaults if not present
package theme

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// -----------------------------------------------------------------------------
// Configuration Types
// -----------------------------------------------------------------------------

// ColorSpec represents a colour value. In JSON it can be:
//   - A plain string: hex ("#FF5FAF"), ANSI 256 index ("212"), or ANSI name.
//   - An object with "light" and "dark" keys for adaptive colours.
type ColorSpec struct {
	Raw   string `json:"color,omitempty"`
	Light string `json:"light,omitempty"`
	Dark  string `json:"dark,omitempty"`
}

// UnmarshalJSON handles both string and object forms for a ColorSpec.
func (c *ColorSpec) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		c.Raw = s
		return nil
	}
	type alt ColorSpec
	return json.Unmarshal(data, (*alt)(c))
}

// IsAdaptive reports whether the ColorSpec has light/dark variants.
func (c ColorSpec) IsAdaptive() bool {
	return c.Light != "" || c.Dark != ""
}

// StyleDef defines a reusable lipgloss style. Only non-zero fields are emitted.
type StyleDef struct {
	Foreground       string `json:"foreground,omitempty"`
	Background       string `json:"background,omitempty"`
	BorderForeground string `json:"border_foreground,omitempty"`
	Bold             bool   `json:"bold,omitempty"`
	Italic           bool   `json:"italic,omitempty"`
	Underline        bool   `json:"underline,omitempty"`
	Strikethrough    bool   `json:"strikethrough,omitempty"`
	Faint            bool   `json:"faint,omitempty"`
	Blink            bool   `json:"blink,omitempty"`
	Reverse          bool   `json:"reverse,omitempty"`
}

// ThemeConfig is the raw JSON representation of a user theme.
type ThemeConfig struct {
	Palette map[string]ColorSpec `json:"palette"`
	Styles  map[string]StyleDef  `json:"styles"`
	Huh     map[string]StyleDef  `json:"huh"`
}

// -----------------------------------------------------------------------------
// Compiled Theme (immutable)
// -----------------------------------------------------------------------------

// Theme is a compiled, ready-to-use theme with resolved colors and styles.
type Theme struct {
	hasDarkBg      bool
	colors         map[string]color.Color
	styles         map[string]lipgloss.Style
	huhDefinitions map[string]StyleDef
}

// NewTheme compiles a ThemeConfig into a Theme with resolved colors and styles.
func NewTheme(cfg *ThemeConfig) *Theme {
	if cfg == nil {
		cfg = DefaultThemeConfig()
	}

	hasDark := lipgloss.HasDarkBackground(os.Stdin, os.Stdout)

	t := &Theme{
		hasDarkBg:      hasDark,
		colors:         make(map[string]color.Color),
		styles:         make(map[string]lipgloss.Style),
		huhDefinitions: cfg.Huh,
	}

	for name, spec := range cfg.Palette {
		t.colors[name] = t.resolveColorSpec(spec)
	}
	for name, def := range cfg.Styles {
		t.styles[name] = t.buildStyle(def)
	}

	return t
}

// Color looks up a named color from the palette, falling back to parsing the name as a literal color.
func (t *Theme) Color(name string) color.Color {
	if c, ok := t.colors[name]; ok {
		return c
	}
	return parseLiteralColor(name)
}

// Style looks up a named style, returning an empty style if the name is not found.
func (t *Theme) Style(name string) lipgloss.Style {
	if s, ok := t.styles[name]; ok {
		return s
	}
	return lipgloss.NewStyle()
}

// HuhTheme returns a huh.ThemeFunc that overlays custom styles onto a base theme.
func (t *Theme) HuhTheme(interactive bool) huh.ThemeFunc {
	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		base := huh.ThemeBase(isDark)
		if interactive {
			base = huh.ThemeCharm(isDark)
		}

		apply := func(target *lipgloss.Style, key string) {
			if def, ok := t.huhDefinitions[key]; ok {
				*target = t.overlayStyleDef(*target, def)
			}
		}

		apply(&base.Focused.Title, "focused_title")
		apply(&base.Focused.Description, "focused_description")
		apply(&base.Focused.SelectedOption, "focused_selected_option")
		apply(&base.Focused.UnselectedOption, "focused_unselected_option")
		apply(&base.Focused.ErrorIndicator, "focused_error_indicator")
		apply(&base.Focused.ErrorMessage, "focused_error_message")
		apply(&base.Focused.SelectSelector, "focused_select_selector")
		apply(&base.Focused.NextIndicator, "focused_next_indicator")
		apply(&base.Focused.PrevIndicator, "focused_prev_indicator")
		apply(&base.Focused.FocusedButton, "focused_focused_button")
		apply(&base.Focused.BlurredButton, "focused_blurred_button")
		apply(&base.Focused.Directory, "focused_directory")
		apply(&base.Focused.File, "focused_file")
		apply(&base.Focused.Option, "focused_option")
		apply(&base.Focused.MultiSelectSelector, "focused_multi_select_selector")
		apply(&base.Focused.SelectedPrefix, "focused_selected_prefix")
		apply(&base.Focused.UnselectedPrefix, "focused_unselected_prefix")
		apply(&base.Focused.Card, "focused_card")
		apply(&base.Focused.NoteTitle, "focused_note_title")
		apply(&base.Focused.Next, "focused_next")

		apply(&base.Blurred.Title, "blurred_title")
		apply(&base.Blurred.Description, "blurred_description")
		apply(&base.Blurred.SelectedOption, "blurred_selected_option")
		apply(&base.Blurred.UnselectedOption, "blurred_unselected_option")
		apply(&base.Blurred.ErrorIndicator, "blurred_error_indicator")
		apply(&base.Blurred.ErrorMessage, "blurred_error_message")
		apply(&base.Blurred.SelectSelector, "blurred_select_selector")
		apply(&base.Blurred.NextIndicator, "blurred_next_indicator")
		apply(&base.Blurred.PrevIndicator, "blurred_prev_indicator")
		apply(&base.Blurred.FocusedButton, "blurred_focused_button")
		apply(&base.Blurred.BlurredButton, "blurred_blurred_button")
		apply(&base.Blurred.Directory, "blurred_directory")
		apply(&base.Blurred.File, "blurred_file")
		apply(&base.Blurred.Option, "blurred_option")
		apply(&base.Blurred.MultiSelectSelector, "blurred_multi_select_selector")
		apply(&base.Blurred.SelectedPrefix, "blurred_selected_prefix")
		apply(&base.Blurred.UnselectedPrefix, "blurred_unselected_prefix")
		apply(&base.Blurred.Card, "blurred_card")
		apply(&base.Blurred.NoteTitle, "blurred_note_title")
		apply(&base.Blurred.Next, "blurred_next")

		apply(&base.Focused.TextInput.Cursor, "focused_textinput_cursor")
		apply(&base.Focused.TextInput.CursorText, "focused_textinput_cursor_text")
		apply(&base.Focused.TextInput.Placeholder, "focused_textinput_placeholder")
		apply(&base.Focused.TextInput.Prompt, "focused_textinput_prompt")
		apply(&base.Focused.TextInput.Text, "focused_textinput_text")

		apply(&base.Blurred.TextInput.Cursor, "blurred_textinput_cursor")
		apply(&base.Blurred.TextInput.CursorText, "blurred_textinput_cursor_text")
		apply(&base.Blurred.TextInput.Placeholder, "blurred_textinput_placeholder")
		apply(&base.Blurred.TextInput.Prompt, "blurred_textinput_prompt")
		apply(&base.Blurred.TextInput.Text, "blurred_textinput_text")

		apply(&base.Group.Title, "group_title")
		apply(&base.Group.Description, "group_description")

		return base
	})
}

// -----------------------------------------------------------------------------
// File Loading
// -----------------------------------------------------------------------------

// LoadThemeConfig reads and parses a theme.json file into a ThemeConfig.
func LoadThemeConfig(path string) (*ThemeConfig, error) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("load theme %q: %w", path, err)
	}

	var cfg ThemeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse theme %q: %w", path, err)
	}
	return &cfg, nil
}

// -----------------------------------------------------------------------------
// Internal
// -----------------------------------------------------------------------------

// DefaultThemeConfig returns the built-in default theme configuration.
func DefaultThemeConfig() *ThemeConfig {
	return &ThemeConfig{
		Palette: map[string]ColorSpec{
			"primary":          {Light: "#FF80AB", Dark: "#FF4081"},
			"primary_accent":   {Light: "#FF5FAF", Dark: "#FE5F86"},
			"secondary":        {Light: "#c5adf9", Dark: "#7d56f4"},
			"secondary_accent": {Light: "#64FCDA", Dark: "#04b575"},
			"highlight":        {Light: "#f5d76e", Dark: "#ffd640"},
			"error":            {Raw: "#ff5c57"},
			"text":             {Light: "#14121a", Dark: "#f5f1fa"},
		},
		Styles: map[string]StyleDef{
			"primary":          {Foreground: "primary"},
			"primary_accent":   {Foreground: "primary_accent"},
			"secondary":        {Foreground: "secondary"},
			"secondary_accent": {Foreground: "secondary_accent"},
			"highlight":        {Foreground: "highlight", Bold: true},
			"error":            {Foreground: "error", Bold: true},
			"text":             {Foreground: "text"},
			"dimmed":           {Foreground: "243"},
			"help":             {Foreground: "240"},
		},
		Huh: map[string]StyleDef{
			"focused_title":           {Foreground: "primary", Bold: true},
			"focused_selected_option": {Foreground: "secondary_accent"},
			"focused_description":     {Foreground: "243", Italic: true},
			"blurred_description":     {Foreground: "243", Italic: true},
		},
	}
}

func (t *Theme) resolveColorSpec(spec ColorSpec) color.Color {
	if spec.IsAdaptive() {
		light := parseLiteralColor(spec.Light)
		dark := parseLiteralColor(spec.Dark)
		return lipgloss.LightDark(t.hasDarkBg)(light, dark)
	}
	return parseLiteralColor(spec.Raw)
}

func parseLiteralColor(v string) color.Color {
	v = strings.ToLower(strings.TrimSpace(v))

	switch v {
	case "black":
		return lipgloss.Black
	case "red":
		return lipgloss.Red
	case "green":
		return lipgloss.Green
	case "yellow":
		return lipgloss.Yellow
	case "blue":
		return lipgloss.Blue
	case "magenta":
		return lipgloss.Magenta
	case "cyan":
		return lipgloss.Cyan
	case "white":
		return lipgloss.White
	case "brightblack", "bright_black", "gray", "grey":
		return lipgloss.BrightBlack
	case "brightred", "bright_red":
		return lipgloss.BrightRed
	case "brightgreen", "bright_green":
		return lipgloss.BrightGreen
	case "brightyellow", "bright_yellow":
		return lipgloss.BrightYellow
	case "brightblue", "bright_blue":
		return lipgloss.BrightBlue
	case "brightmagenta", "bright_magenta":
		return lipgloss.BrightMagenta
	case "brightcyan", "bright_cyan":
		return lipgloss.BrightCyan
	case "brightwhite", "bright_white":
		return lipgloss.BrightWhite
	case "none", "":
		return nil
	}

	return lipgloss.Color(v)
}

func (t *Theme) parseColorValue(v string) color.Color {
	v = strings.TrimSpace(v)
	if v == "" || strings.EqualFold(v, "none") {
		return nil
	}
	if c, ok := t.colors[v]; ok {
		return c
	}
	return parseLiteralColor(v)
}

func (t *Theme) buildStyle(def StyleDef) lipgloss.Style {
	s := lipgloss.NewStyle()
	if def.Foreground != "" {
		s = s.Foreground(t.parseColorValue(def.Foreground))
	}
	if def.Background != "" {
		s = s.Background(t.parseColorValue(def.Background))
	}
	if def.BorderForeground != "" {
		s = s.BorderForeground(t.parseColorValue(def.BorderForeground))
	}
	if def.Bold {
		s = s.Bold(true)
	}
	if def.Italic {
		s = s.Italic(true)
	}
	if def.Underline {
		s = s.Underline(true)
	}
	if def.Strikethrough {
		s = s.Strikethrough(true)
	}
	if def.Faint {
		s = s.Faint(true)
	}
	if def.Blink {
		s = s.Blink(true)
	}
	if def.Reverse {
		s = s.Reverse(true)
	}
	return s
}

func (t *Theme) overlayStyleDef(base lipgloss.Style, def StyleDef) lipgloss.Style {
	s := base
	if def.Foreground != "" {
		s = s.Foreground(t.parseColorValue(def.Foreground))
	}
	if def.Background != "" {
		s = s.Background(t.parseColorValue(def.Background))
	}
	if def.BorderForeground != "" {
		s = s.BorderForeground(t.parseColorValue(def.BorderForeground))
	}
	if def.Bold {
		s = s.Bold(true)
	}
	if def.Italic {
		s = s.Italic(true)
	}
	if def.Underline {
		s = s.Underline(true)
	}
	if def.Strikethrough {
		s = s.Strikethrough(true)
	}
	if def.Faint {
		s = s.Faint(true)
	}
	if def.Blink {
		s = s.Blink(true)
	}
	if def.Reverse {
		s = s.Reverse(true)
	}
	return s
}
