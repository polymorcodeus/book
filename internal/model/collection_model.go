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

type collectionModel struct {
	book       *Book
	shelf      *book.Shelf
	collection *book.Collection
	config     *book.Config
}

type getCollectionModel struct {
	get    collectionModel
	action string
}

func (m getCollectionModel) Init() tea.Cmd {
	return m.get.book.form.Init()
}

func (m getCollectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.get.book.width = min(msg.Width, maxWidth) - m.get.book.styles.Base.GetHorizontalFrameSize()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+p":
			return m, m.get.book.form.PrevGroup()
		case "ctrl+c":
			return m, tea.Interrupt
		case "esc":
			return m, tea.Quit
		}
	case errMsg:
		m.get.book.err = msg
		return m, nil
	}

	var cmds []tea.Cmd

	// Process the form
	form, cmd := m.get.book.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.get.book.form = f
		cmds = append(cmds, cmd)
	}

	// Initialize display
	if m.get.shelf == nil {
		m.get.shelf = &book.Shelf{}
	}

	if m.get.book.form.State == huh.StateCompleted {
		m.get.shelf = m.get.book.shelves.Shelf(m.get.book.form.GetString("shelf"))
		switch m.action {
		case "add":
			editScreen := editCollectionForm(m.get.book.shelves, m.get.shelf, m.get.config, m.action)
			return editScreen, editScreen.Init()
		case "list":
			cmds = append(cmds, tea.Quit)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m getCollectionModel) View() tea.View {
	s := m.get.book.styles
	t := m.get.book.tmpls

	if m.action == "list" && m.get.book.form.State == huh.StateCompleted {
		return renderCompletedView(s, t, "collection-list", m.get.shelf)
	}

	// Form (left side)
	v := strings.TrimSuffix(m.get.book.form.View(), "\n\n")
	form := s.Form.Render(v)

	// Status (right side)
	var status string
	{
		var (
			currentShelf string
			displayShelf string
		)

		if m.get.shelf != nil {
			displayShelf = m.get.book.form.GetString("shelf")
		}

		currentShelf = lipglossDimmer(s.StatusHeader, "Picked Shelf", displayShelf)

		const statusWidth = 68
		statusMarginLeft := m.get.book.width - statusWidth - lipgloss.Width(form) - s.Status.GetMarginRight()
		status = s.Status.
			Height(10).
			Width(statusWidth).
			MarginLeft(statusMarginLeft).
			Render(currentShelf)
	}

	errors := m.get.book.form.Errors()
	header := m.get.book.appBoundaryView("book collection retrieval system")
	if len(errors) > 0 {
		header = m.get.book.appErrorBoundaryView(m.get.book.errorView())
	}
	body := lipgloss.JoinHorizontal(lipgloss.Left, form, status)

	footer := m.get.book.appBoundaryView(m.get.book.form.Help().ShortHelpView(m.get.book.form.KeyBinds()))
	if len(errors) > 0 {
		footer = m.get.book.appErrorBoundaryView("")
	}
	return tea.NewView(s.Base.Render(header + "\n" + body + "\n\n" + footer))
}

// GetCollectionForm to be used for editing descriptions/names in future
func GetCollectionForm(bs *book.BookShelves, config *book.Config, action string) getCollectionModel {
	m := collectionModel{book: &Book{width: maxWidth}}
	m.book.styles = NewStyles(config)
	m.book.tmpls = config.Templates
	m.book.shelves = bs
	m.config = config

	var (
		chosenShelf      string
		chosenCollection string
		agree            = true
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
		),

		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Pick your collection.").
				Options(
					huh.NewOptions(bs.Shelf(chosenShelf).CollectionsNames()...)...,
				).
				Key("collection").
				Value(&chosenCollection),
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

	return getCollectionModel{
		get:    m,
		action: action,
	}
}

type editCollectionModel struct {
	editor collectionModel
	action string
}

func (m editCollectionModel) Init() tea.Cmd {
	return m.editor.book.form.Init()
}

func (m editCollectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.editor.book.width = min(msg.Width, maxWidth) - m.editor.book.styles.Base.GetHorizontalFrameSize()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+z":
			switch m.action {
			case "edit", "add":
				getScreen := GetCollectionForm(m.editor.book.shelves, m.editor.config, m.action)
				return getScreen, getScreen.Init()
			}
		case "ctrl+c":
			return m, tea.Interrupt
		case "esc":
			return m, tea.Quit
		}
	case shelfSavedMsg:
		return m, tea.Quit
	case errMsg:
		m.editor.book.err = msg
		return m, nil
	}

	var cmds []tea.Cmd

	// Process the form
	form, cmd := m.editor.book.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.editor.book.form = f
		cmds = append(cmds, cmd)
	}

	if m.editor.book.form.State == huh.StateCompleted {
		m.editor.collection.Shelf = m.editor.shelf
		cmds = append(cmds, m.editor.updateShelfFileCmd(m.action))
	}

	return m, tea.Batch(cmds...)
}

func (m editCollectionModel) View() tea.View {
	s := m.editor.book.styles
	t := m.editor.book.tmpls

	switch m.editor.book.form.State {
	case huh.StateCompleted:
		return renderCompletedView(s, t, "collection-add", m.editor.collection)
	default:
		// Form (left side)
		v := strings.TrimSuffix(m.editor.book.form.View(), "\n\n")
		form := s.Form.Render(v)

		// Status (right side)
		var status string
		{
			var (
				editShelfName      string
				editCollectionName string
				editCollectionDesc string
			)

			editShelfName = s.StatusHeader.Render("Picked Shelf") + "\n" + m.editor.shelf.Name + "\n\n"
			editCollectionName = lipglossDimmer(s.StatusHeader, "Collection Name", m.editor.collection.Name)
			editCollectionDesc = lipglossDimmer(s.StatusHeader, "Collection Description", m.editor.collection.Description)

			const statusWidth = 68
			statusMarginLeft := m.editor.book.width - statusWidth - lipgloss.Width(form) - s.Status.GetMarginRight()
			status = s.Status.
				Height(10).
				Width(statusWidth).
				MarginLeft(statusMarginLeft).
				Render(editShelfName + editCollectionName + editCollectionDesc)
		}

		errors := m.editor.book.form.Errors()
		header := m.editor.book.appBoundaryView("book collection editing system")
		if len(errors) > 0 {
			header = m.editor.book.appErrorBoundaryView(m.editor.book.errorView())
		}
		body := lipgloss.JoinHorizontal(lipgloss.Left, form, status)

		footer := m.editor.book.appBoundaryView(m.editor.book.form.Help().ShortHelpView(m.editor.book.form.KeyBinds()))
		if len(errors) > 0 {
			footer = m.editor.book.appErrorBoundaryView("")
		}
		return tea.NewView(s.Base.Render(header + "\n" + body + "\n\n" + footer))
	}
}

func editCollectionForm(bs *book.BookShelves, shelf *book.Shelf, config *book.Config, action string) editCollectionModel {
	m := collectionModel{book: &Book{width: maxWidth}}
	m.book.styles = NewStyles(config)
	m.book.tmpls = config.Templates
	m.book.shelves = bs
	m.shelf = shelf
	m.collection = &book.Collection{}
	m.config = config

	var (
		agree = true
	)

	m.book.form = huh.NewForm(

		huh.NewGroup(
			huh.NewInput().
				Title("Name of new collection?").
				Description("This will be immortalized, be certain.").
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
					return "Ready to update shelf?"
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

	return editCollectionModel{
		editor: m,
		action: action,
	}
}

func (m *collectionModel) updateShelfFileCmd(action string) tea.Cmd {
	return func() tea.Msg {
		if action == "add" {
			m.shelf.AddCollection(m.collection)
		}
		if err := catalog.UpdateShelfFile(m.shelf); err != nil {
			return errMsg{err}
		}
		return shelfSavedMsg{}
	}
}
