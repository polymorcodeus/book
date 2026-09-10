package model

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/catalog"
)

type shelfModel struct {
	book       *Book
	shelf      *book.Shelf
	collection *book.Collection
	config     *book.Config
}

type getShelfModel struct {
	get    shelfModel
	action string
}

func (m getShelfModel) Init() tea.Cmd {
	return m.get.book.form.Init()
}

func (m getShelfModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.get.book.width = min(msg.Width, maxWidth) - m.get.book.styles.Base.GetHorizontalFrameSize()
	case tea.KeyPressMsg:
		if cmd, handled := handleCommonKeys(m.get.book.form, msg); handled {
			return m, cmd
		}
	case errMsg:
		m.get.book.err = msg
		return m, tea.Quit
	}

	var cmds []tea.Cmd

	// Process the form
	form, cmd := m.get.book.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.get.book.form = f
		cmds = append(cmds, cmd)
	}

	if m.get.book.form.State == huh.StateCompleted {
		switch m.action {
		case "add":
			editScreen := editShelfForm(m.get.book.shelves, m.get.shelf, m.get.config, m.action)
			return editScreen, editScreen.Init()
		case "list":
			cmds = append(cmds, tea.Quit)
		}
	}

	return m, tea.Batch(cmds...)
}

// View now returns tea.View (bubbletea v2 breaking change).
func (m getShelfModel) View() tea.View {
	if m.get.book.form.State == huh.StateCompleted || m.get.book.width <= 0 {
		return altScreenView("")
	}

	s := m.get.book.styles

	// Form (left side)
	v := strings.TrimSuffix(m.get.book.form.View(), "\n\n")
	form := s.Form.Render(v)

	// Status (right side)
	var status string
	{
		displayShelf := m.get.book.form.GetString("shelf")
		currentShelf := lipglossDimmer(s.StatusHeader, "Picked Shelf", displayShelf)

		status = m.get.book.statusPanel(form, currentShelf, 14)
	}

	errors := m.get.book.form.Errors()
	header := m.get.book.appBoundaryView("book shelf retrieval system")
	if len(errors) > 0 {
		header = m.get.book.appErrorBoundaryView(m.get.book.errorView())
	}
	body := lipgloss.JoinHorizontal(lipgloss.Left, form, status)

	footer := m.get.book.appBoundaryView(m.get.book.form.Help().ShortHelpView(m.get.book.form.KeyBinds()))
	if len(errors) > 0 {
		footer = m.get.book.appErrorBoundaryView("")
	}
	return altScreenView(s.Base.Render(header + "\n" + body + "\n\n" + footer))
}

// ResultView returns the completion output for the caller to print after the
// program exits.
func (m getShelfModel) ResultView() string {
	if m.get.book.form.State != huh.StateCompleted || m.get.book.err != nil || m.action != "list" {
		return ""
	}
	return renderCompletedView(m.get.book.styles, m.get.book.tmpls, "shelf-list", m.get.book.shelves).Content
}

// Error returns the terminal error, if any, for the caller to surface after
// the program exits.
func (m getShelfModel) Error() error {
	return m.get.book.err
}

// GetShelfForm to be used for editing descriptions/names in future
func GetShelfForm(bs *book.BookShelves, config *book.Config, action string) getShelfModel {
	m := shelfModel{book: &Book{width: 0}}
	m.book.styles = NewStyles(config)
	m.book.tmpls = config.Templates
	m.book.shelves = bs
	m.config = config

	var (
		chosenShelf string
		agree       = true
	)

	m.book.form = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Pick your shelf.").
				Options(
					huh.NewOptions(bs.ShelfNames()...)...,
				).
				Key("shelf").
				Value(&chosenShelf),
		).WithHideFunc(func() bool {
			return action != "edit"
		}),

		huh.NewGroup(
			huh.NewConfirm().
				Key("done").
				Title("All done picked?").
				Description("shift+tab to go back.").
				Validate(func(v bool) error {
					if !v {
						return fmt.Errorf("welp, finish up then")
					}
					return nil
				}).
				Value(&agree).
				Affirmative("Yep").
				Negative("Wait, no"),
		).WithHideFunc(func() bool {
			return action != "edit"
		}),
	).
		WithWidth(45).
		WithShowHelp(false).
		WithShowErrors(false).
		WithTheme(config.Theme.HuhTheme(config.Interactive))

	return getShelfModel{
		get:    m,
		action: action,
	}
}

type editShelfModel struct {
	editor shelfModel
	action string
}

func (m editShelfModel) Init() tea.Cmd {
	return m.editor.book.form.Init()
}

func (m editShelfModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.editor.book.width = min(msg.Width, maxWidth) - m.editor.book.styles.Base.GetHorizontalFrameSize()
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+z" {
			switch m.action {
			case "edit", "add":
				getScreen := GetShelfForm(m.editor.book.shelves, m.editor.config, m.action)
				return getScreen, getScreen.Init()
			}
		}
		if cmd, handled := handleCommonKeys(m.editor.book.form, msg); handled {
			return m, cmd
		}
	case shelfSavedMsg:
		return m, tea.Quit
	case errMsg:
		m.editor.book.err = msg
		return m, tea.Quit
	}

	var cmds []tea.Cmd

	// Process the form
	form, cmd := m.editor.book.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.editor.book.form = f
		cmds = append(cmds, cmd)
	}

	if m.editor.book.form.State == huh.StateCompleted {
		newM := m
		if m.action == "add" {
			shelf, err := book.NewShelf(m.editor.shelf.Name, m.editor.shelf.Description)
			if err != nil {
				newM.editor.book.err = err
				return newM, nil
			}
			collection, err := book.NewCollection(shelf, m.editor.collection.Name, m.editor.collection.Description)
			if err != nil {
				newM.editor.book.err = err
				return newM, nil
			}
			shelf.AddCollection(collection)
			shelf.AddFileDetail(newM.editor.config)
			newM.editor.shelf = shelf
			newM.editor.collection = collection
		}
		cmds = append(cmds, newM.editor.updateShelfFileCmd(newM.action))
		return newM, tea.Batch(cmds...)
	}

	return m, tea.Batch(cmds...)
}

// View now returns tea.View (bubbletea v2 breaking change).
func (m editShelfModel) View() tea.View {
	if m.editor.book.form.State == huh.StateCompleted || m.editor.book.width <= 0 {
		return altScreenView("")
	}

	s := m.editor.book.styles

	// Form (left side)
	v := strings.TrimSuffix(m.editor.book.form.View(), "\n\n")
	form := s.Form.Render(v)

	// Status (right side)
	var status string
	{
		var (
			editShelfName      string
			editShelfDesc      string
			editCollectionName string
			editCollectionDesc string
		)

		editShelfName = s.StatusHeader.Render("Shelf Name") + "\n" + m.editor.shelf.Name + "\n\n"
		editShelfDesc = lipglossDimmer(s.StatusHeader, "Shelf Description", m.editor.shelf.Description)
		editCollectionName = lipglossDimmer(s.StatusHeader, "Collection Name", m.editor.collection.Name)
		editCollectionDesc = lipglossDimmer(s.StatusHeader, "Collection Description", m.editor.collection.Description)

		status = m.editor.book.statusPanel(form, editShelfName+editShelfDesc+editCollectionName+editCollectionDesc, 14)
	}

	errors := m.editor.book.form.Errors()
	header := m.editor.book.appBoundaryView("book shelf editing system")
	if len(errors) > 0 {
		header = m.editor.book.appErrorBoundaryView(m.editor.book.errorView())
	}
	body := lipgloss.JoinHorizontal(lipgloss.Left, form, status)

	footer := m.editor.book.appBoundaryView(m.editor.book.form.Help().ShortHelpView(m.editor.book.form.KeyBinds()))
	if len(errors) > 0 {
		footer = m.editor.book.appErrorBoundaryView("")
	}
	return altScreenView(s.Base.Render(header + "\n" + body + "\n\n" + footer))
}

// ResultView returns the completion output for the caller to print after the
// program exits.
func (m editShelfModel) ResultView() string {
	if m.editor.book.form.State != huh.StateCompleted || m.editor.book.err != nil {
		return ""
	}
	return renderCompletedView(m.editor.book.styles, m.editor.book.tmpls, "shelf-add", m.editor.collection).Content
}

// Error returns the terminal error, if any, for the caller to surface after
// the program exits.
func (m editShelfModel) Error() error {
	return m.editor.book.err
}

func editShelfForm(bs *book.BookShelves, shelf *book.Shelf, config *book.Config, action string) editShelfModel {
	m := shelfModel{book: &Book{width: 0}}
	m.book.styles = NewStyles(config)
	m.book.tmpls = config.Templates
	m.book.shelves = bs
	if shelf == nil {
		shelf = &book.Shelf{}
	}
	m.shelf = shelf
	m.collection = &book.Collection{}
	m.config = config

	var (
		agree = true
	)

	collectionNote := huh.NewNote().
		Title(fmt.Sprintf("Your shelf is feeling empty...\n%s", m.book.styles.Dim.Render("Let's add a token collection!!")))

	m.book.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Name of new shelf?").
				Description("Cannot be easily changed so choose wisely.").
				Validate(func(s string) error {
					return bs.ValidateNewShelfName(s)
				}).
				Value(&m.shelf.Name),

			huh.NewInput().
				Title("Description of shelf?").
				Description("Mostly ignored, feel free to skip it.").
				Value(&m.shelf.Description),
		),

		huh.NewGroup(
			collectionNote,

			huh.NewInput().
				Title("Name of new collection?").
				Description("This will be immortalized, be certain.").
				Validate(func(s string) error {
					return m.shelf.ValidateNewCollectionName(s)
				}).
				Value(&m.collection.Name),

			huh.NewInput().
				Title("Description of collection?").
				Description("Ala ignored like shelf description.").
				Value(&m.collection.Description),
		).WithHideFunc(func() bool {
			return action != "add"
		}),

		huh.NewGroup(
			huh.NewConfirm().
				Key("done").
				TitleFunc(func() string {
					if action == "edit" {
						return "Done editing?"
					}
					return "Ready to add shelf?"
				}, &action).
				Description("ctrl+z to start over.").
				Validate(func(v bool) error {
					if !v {
						return fmt.Errorf("welp, finish up then")
					}
					return nil
				}).
				Value(&agree).
				Affirmative("Yep").
				Negative("Wait, no"),
		),
	).
		WithWidth(45).
		WithShowHelp(false).
		WithShowErrors(false).
		WithTheme(config.Theme.HuhTheme(config.Interactive))

	return editShelfModel{
		editor: m,
		action: action,
	}
}

func (m *shelfModel) updateShelfFileCmd(action string) tea.Cmd {
	return func() tea.Msg {
		if action == "add" {
			if err := catalog.UpdateShelfFile(m.shelf); err != nil {
				return errMsg{err}
			}
			return shelfSavedMsg{}
		}
		return nil
	}
}
