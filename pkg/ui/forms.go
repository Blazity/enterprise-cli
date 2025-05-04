package ui

import (
	"context"
	"errors"

	"github.com/blazity/enterprise-cli/pkg/logging"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

type KeyMap struct {
	Quit key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Quit},
	}
}

var DefaultKeyMap = KeyMap{
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c", "q"),
		key.WithHelp("ctrl+c/q", "quit"),
	),
}

var ErrFormCancelled = errors.New("operation cancelled by user")

type FormModel struct {
	form         *huh.Form
	keyMap       KeyMap
	help         help.Model
	logger       logging.Logger
	cancel       context.CancelFunc
	wasCancelled bool
}

func (m FormModel) WasCancelled() bool {
	return m.wasCancelled
}

func NewFormModel(form *huh.Form, cancel context.CancelFunc) FormModel {
	logger := logging.GetLogger()
	styledForm := form.
		WithTheme(huh.ThemeCharm()).
		WithShowHelp(true).
		WithWidth(80)

	return FormModel{
		form:   styledForm,
		keyMap: DefaultKeyMap,
		help:   help.New(),
		logger: logger,
		cancel: cancel,
	}
}

func (m FormModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.logger != nil {
				m.logger.Debug("Form quit via 'q' key")
			}
			m.wasCancelled = true
			if m.cancel != nil {
				m.logger.Debug("Calling global cancel function from form")
				m.cancel()
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		return m, nil
	}

	formModel, formCmd := m.form.Update(msg)

	if formModel == nil {
		if m.logger != nil {
			m.logger.Info("Form exited unexpectedly")
		}
		return m, tea.Quit
	}

	m.form = formModel.(*huh.Form)

	if m.form.State == huh.StateCompleted {
		if m.logger != nil {
			m.logger.Debug("Form completed successfully")
		}
		return m, tea.Quit
	}

	return m, formCmd
}

func (m FormModel) View() string {
	return m.form.View()
}

func RunForm(form *huh.Form, cancel context.CancelFunc) error {
	model := NewFormModel(form, cancel)

	program := tea.NewProgram(model)

	finalModel, err := program.Run()

	if m, ok := finalModel.(FormModel); ok && m.WasCancelled() {
		return ErrFormCancelled
	}

	if err != nil {
		return err
	}

	return nil
}
