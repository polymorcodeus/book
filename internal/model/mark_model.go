package model

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/polymorcodeus/book/internal/book"
	"github.com/polymorcodeus/book/internal/catalog"
	"github.com/polymorcodeus/book/internal/web"
)

type markModel struct {
	book       *Book
	shelf      *book.Shelf
	collection *book.Collection
	mark       *book.Mark
	config     *book.Config
}

func (m markModel) verifyCollection() bool {
	collection := m.book.form.GetString("collection")
	validCollections := m.shelf.CollectionsNames()

	return slices.Contains(validCollections, collection)
}

func (m markModel) verifyMark() bool {
	mark := m.book.form.GetString("mark")

	// Still needed for custom banner title
	if m.collection != nil {
		validMarks := m.collection.MarksNames()
		return slices.Contains(validMarks, mark)
	}
	return false
}

func (m getMarkModel) reloadFromForm() getMarkModel {
	shelfName := m.get.book.form.GetString("shelf")
	collectionName := m.get.book.form.GetString("collection")

	shelf, _ := m.get.book.shelves.Shelf(shelfName)
	m.get.shelf = shelf
	if shelf == nil {
		m.get.collection = nil
		if m.action != "add" {
			m.get.mark = nil
		}
		return m
	}

	m.get.collection = shelf.Collection(collectionName)
	if m.get.collection == nil {
		if m.action != "add" {
			m.get.mark = nil
		}
		return m
	}

	if m.action != "add" {
		m.get.mark = m.get.collection.Mark(m.get.book.form.GetString("mark"))
	}
	return m
}

func (m markModel) withParents() markModel {
	if m.mark == nil {
		return m
	}
	shelf, _ := m.book.shelves.Shelf(m.book.form.GetString("shelf"))
	if shelf == nil {
		return m
	}
	newMark := *m.mark
	newMark.Shelf = shelf
	newMark.Collection = shelf.Collection(m.book.form.GetString("collection"))
	m.mark = &newMark
	return m
}

func (m *markModel) updateShelfFileCmd(action string) tea.Cmd {
	return func() tea.Msg {
		switch action {
		case "add":
			m.mark.RecordAdd()
			m.mark.Collection.AddMark(m.mark)
		case "edit":
			m.mark.Touch()
		case "delete":
			m.mark.RecordDelete()
		}
		if err := catalog.UpdateShelfFile(m.mark.Shelf); err != nil {
			return errMsg{err}
		}
		return shelfSavedMsg{}
	}
}

type getMarkModel struct {
	get    markModel
	action string
}

func (m getMarkModel) Init() tea.Cmd {
	return m.get.book.form.Init()
}

func (m getMarkModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.get.book.width = min(msg.Width, maxWidth) - m.get.book.styles.Base.GetHorizontalFrameSize()
	case tea.KeyPressMsg:
		if cmd, handled := handleCommonKeys(m.get.book.form, msg); handled {
			return m, cmd
		}
	case shelfSavedMsg:
		return m, tea.Quit
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

	// Keep model state in sync with form selections.
	if m.get.book.form.State != huh.StateCompleted {
		m = m.reloadFromForm()
	}

	if m.get.book.form.State == huh.StateCompleted {
		// Reload mark and model from final form fields
		if m.action != "add" {
			m = m.reloadFromForm()
		}

		switch m.action {
		case "add", "edit":
			m.get = m.get.withParents()
			editScreen := editMarkForm(m.get.book.shelves, m.get.mark, m.get.config, m.action)
			return editScreen, editScreen.Init()
		case "get":
			cmd = func() tea.Msg {
				if err := web.OpenURL(m.get.mark.URL); err != nil {
					return errMsg{err}
				}
				return nil
			}
			cmds = append(cmds, cmd, tea.Quit)
		case "list":
			cmds = append(cmds, tea.Quit)
		case "delete":
			cmds = append(cmds, m.get.updateShelfFileCmd(m.action))
		}
	}

	return m, tea.Batch(cmds...)
}

func (m getMarkModel) View() tea.View {
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
		var (
			currentShelf      string
			currentCollection string
			currentMark       string
			displayShelf      string
			displayCollection string
			displayMark       string
		)

		if m.get.shelf != nil {
			displayShelf = m.get.shelf.Name

			if m.get.verifyCollection() {
				displayCollection = m.get.collection.Name

				if m.get.verifyMark() {
					displayMark = m.get.mark.Name + "\n\n" + m.get.mark.URL + "\n\n" + lipglossList(s.None, m.get.mark.Tags) + "\n"
				}
			}
		}
		currentShelf = lipglossDimmer(s.StatusHeader, "Picked Shelf", displayShelf)
		currentCollection = lipglossDimmer(s.StatusHeader, "Picked Collection", displayCollection)
		currentMark = lipglossDimmer(s.StatusHeader, "Picked Mark", displayMark)

		status = m.get.book.statusPanel(form, currentShelf+currentCollection+currentMark, 28)
	}

	errors := m.get.book.form.Errors()
	header := m.get.book.appBoundaryView("book mark retrieval system")
	if m.action == "delete" && m.get.verifyMark() {
		header = m.get.book.appBoundaryView("book mark garbage system")
	}
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
func (m getMarkModel) ResultView() string {
	if m.get.book.form.State != huh.StateCompleted || m.get.book.err != nil {
		return ""
	}
	s := m.get.book.styles
	t := m.get.book.tmpls
	switch m.action {
	case "get":
		return renderCompletedView(s, t, "mark-get", m.get.mark).Content
	case "list":
		return renderCompletedView(s, t, "mark-list", m.get.collection).Content
	case "delete":
		return renderCompletedView(s, t, "mark-delete", m.get.mark).Content
	}
	return ""
}

// Error returns the terminal error, if any, for the caller to surface after
// the program exits.
func (m getMarkModel) Error() error {
	return m.get.book.err
}

// GetMarkForm returns a TUI model for navigating shelves, collections, and marks.
func GetMarkForm(bs *book.BookShelves, mark *book.Mark, config *book.Config, action string) getMarkModel {
	m := markModel{book: &Book{width: 0}}
	m.book.styles = NewStyles(config)
	m.book.tmpls = config.Templates
	m.book.shelves = bs
	m.mark = mark
	m.config = config

	var (
		chosenShelf      string
		chosenCollection string
		chosenMark       string
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

			huh.NewSelect[string]().
				Title("Pick your collection.").
				OptionsFunc(func() []huh.Option[string] {
					// Prevent empty collections from being loaded
					var opts []string
					shelf, _ := bs.Shelf(chosenShelf)
					if shelf == nil {
						return []huh.Option[string]{}
					}
					for _, col := range shelf.Collections {
						if (col.HasActiveMarks() && action != "add") || action == "add" {
							opts = append(opts, col.Name)
						}
					}
					return huh.NewOptions(opts...)
				}, &chosenShelf).
				Key("collection").
				Value(&chosenCollection).
				Height(19),
		),

		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Pick your mark.").
				OptionsFunc(func() []huh.Option[string] {
					shelf, _ := bs.Shelf(chosenShelf)
					if shelf == nil {
						return []huh.Option[string]{}
					}
					collection := shelf.Collection(chosenCollection)
					if collection == nil {
						return []huh.Option[string]{}
					}
					opts := collection.MarksNames()
					return huh.NewOptions(opts...)
				}, &chosenCollection).
				Key("mark").
				Value(&chosenMark).
				Height(19),
		).WithHideFunc(func() bool {
			return action == "add" || action == "list"
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
		),
	).
		WithWidth(45).
		WithShowHelp(false).
		WithShowErrors(false).
		WithTheme(config.Theme.HuhTheme(config.Interactive))
	return getMarkModel{
		get:    m,
		action: action,
	}
}

type editMarkModel struct {
	editor markModel
	action string
}

func (m editMarkModel) Init() tea.Cmd {
	return m.editor.book.form.Init()
}

func (m editMarkModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.editor.book.width = min(msg.Width, maxWidth) - m.editor.book.styles.Base.GetHorizontalFrameSize()
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+z" {
			switch m.action {
			case "edit":
				getScreen := GetMarkForm(m.editor.book.shelves, nil, m.editor.config, m.action)
				return getScreen, getScreen.Init()
			case "add":
				getScreen := GetMarkForm(m.editor.book.shelves, m.editor.mark, m.editor.config, m.action)
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
		newM.editor = newM.editor.withParents()
		cmds = append(cmds, newM.editor.updateShelfFileCmd(newM.action))
		return newM, tea.Batch(cmds...)
	}

	return m, tea.Batch(cmds...)
}

func (m editMarkModel) View() tea.View {
	if m.editor.book.form.State == huh.StateCompleted || m.editor.book.width <= 0 {
		return altScreenView("")
	}

	s := m.editor.book.styles

	// Form (left side)
	v := strings.TrimSuffix(m.editor.book.form.View(), "\n\n")
	form := s.Form.Render(v)

	var shelf = m.editor.mark.Shelf.Name

	// Status (right side)
	var status string
	{
		var (
			currentShelf      string
			currentCollection string
			currentMark       string
		)

		currentShelf = s.StatusHeader.Render("Picked Shelf") + "\n" + shelf + "\n\n"
		currentCollection = s.StatusHeader.Render("Picked Collection") + "\n" + m.editor.mark.Collection.Name + "\n\n"

		currentMark = s.StatusHeader.Render("Editing Mark") + "\n" + m.editor.mark.Name
		currentMark += "\n\n" + m.editor.mark.URL + "\n\n" + lipglossList(s.None, m.editor.mark.Tags) + "\n"

		status = m.editor.book.statusPanel(form, currentShelf+currentCollection+currentMark, 28)
	}

	errors := m.editor.book.form.Errors()
	header := m.editor.book.appBoundaryView("book mark editing system")
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
func (m editMarkModel) ResultView() string {
	if m.editor.book.form.State != huh.StateCompleted || m.editor.book.err != nil {
		return ""
	}
	s := m.editor.book.styles
	t := m.editor.book.tmpls
	switch m.action {
	case "add":
		return renderCompletedView(s, t, "mark-add", m.editor.mark).Content
	case "edit":
		return renderCompletedView(s, t, "mark-edit", m.editor.mark).Content
	}
	return ""
}

// Error returns the terminal error, if any, for the caller to surface after
// the program exits.
func (m editMarkModel) Error() error {
	return m.editor.book.err
}

func editMarkForm(bs *book.BookShelves, mark *book.Mark, config *book.Config, action string) editMarkModel {
	m := markModel{book: &Book{width: 0}}
	m.book.styles = NewStyles(config)
	m.book.tmpls = config.Templates
	m.book.shelves = bs
	m.mark = mark

	var (
		agree    = true
		tempTags string
	)

	m.book.form = huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Title("Review title.").
				Key("markTitle").
				Value(&m.mark.Name).
				WithWidth(25).
				WithHeight(3),

			huh.NewText().
				Title("Additional Tags?").
				Description("separate tags with new line").
				Key("markFreeTags").
				Value(&tempTags).
				WithWidth(25).
				WithHeight(5),

			huh.NewMultiSelect[string]().
				Title("Review tags.").
				Description("additional and collection tags shown").
				OptionsFunc(func() []huh.Option[string] {
					collectTags := []string{}
					shelf, _ := bs.Shelf(m.mark.Shelf.Name)
					if shelf != nil {
						collection := shelf.Collection(m.mark.Collection.Name)
						if collection != nil {
							collectTags = collection.AllTags()
						}
					}
					userTags := book.SplitTagLines(tempTags)
					return huh.NewOptions(book.MergeTags(m.mark.Tags, userTags, collectTags)...)
				}, &tempTags).
				Key("markTags").
				Value(&m.mark.Tags),

			huh.NewConfirm().
				Key("done").
				TitleFunc(func() string {
					if action == "edit" {
						return "Done editing?"
					}
					return "Ready to add mark?"
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
	return editMarkModel{
		editor: m,
		action: action,
	}
}
