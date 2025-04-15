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

// NEW: Define a specific error for user cancellation
var ErrFormCancelled = errors.New("operation cancelled by user")

type FormModel struct {
	form         *huh.Form
	keyMap       KeyMap
	help         help.Model
	logger       logging.Logger
	cancel       context.CancelFunc // NEW: Store cancel func
	wasCancelled bool               // NEW: Track if the form was cancelled by the user
}

// NEW: Add a method to check if cancelled
func (m FormModel) WasCancelled() bool {
	return m.wasCancelled
}

func NewFormModel(form *huh.Form, logger logging.Logger, cancel context.CancelFunc) FormModel {
	// Apply custom styling to the form
	styledForm := form.
		WithTheme(huh.ThemeCharm()).
		WithShowHelp(true).
		WithWidth(80)

	return FormModel{
		form:   styledForm,
		keyMap: DefaultKeyMap,
		help:   help.New(),
		logger: logger,
		cancel: cancel, // Store it
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
			if m.cancel != nil { // NEW: If cancel func exists
				m.logger.Debug("Calling global cancel function from form")
				m.cancel() // Call it immediately
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		// If window size changes, we might want to adjust the form
		return m, nil
	}

	var cmd tea.Cmd
	formModel, formCmd := m.form.Update(msg)

	// If the form was exited through other means
	if formModel == nil {
		if m.logger != nil {
			m.logger.Info("Form exited unexpectedly")
		}
		return m, tea.Quit
	}

	m.form = formModel.(*huh.Form)
	cmd = formCmd
	return m, cmd
}

func (m FormModel) View() string {
	return m.form.View()
}

// RunForm runs a form with simple CTRL+C handling
func RunForm(form *huh.Form, logger logging.Logger, cancel context.CancelFunc) error {
	// Configure tea.Program
	model := NewFormModel(form, logger, cancel)

	// Use a standard program without fancy options
	program := tea.NewProgram(model)

	// Run the program - CTRL+C will be caught by the global handler in main.go
	finalModel, err := program.Run()

	// Check if the model was cancelled by the user
	if m, ok := finalModel.(FormModel); ok && m.WasCancelled() {
		return ErrFormCancelled // NEW: Return the specific error
	}

	if err != nil {
		// Handle other errors from the form
		return err
	}

	return nil
}
