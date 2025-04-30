package ui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2ECC71")).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E74C3C")).
			Bold(true)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3498DB")).
			Bold(true).
			Underline(true).
			PaddingBottom(1)

	SubHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9B59B6")).
			Bold(true)

	HighlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F39C12")).
			Bold(true)

	// Form styles for improved TUI
	FormTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3498DB")).
			Bold(true)

	FormDescriptionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7F8C8D")).
				Italic(true)

	FormFocusedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F39C12")).
				Bold(true)

	// Style for scrollable indicators
	ScrollableIndicatorStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#95A5A6")).
					Italic(false).
					Faint(false)
)

func Success(s string) string {
	return SuccessStyle.Render(s)
}

func Error(s string) string {
	return ErrorStyle.Render(s)
}

func Header(s string) string {
	return HeaderStyle.Render(s)
}

func SubHeader(s string) string {
	return SubHeaderStyle.Render(s)
}

func Highlight(s string) string {
	return HighlightStyle.Render(s)
}

// ScrollableIndicator returns a styled scrollable indicator string
func ScrollableIndicator(s string) string {
	return ScrollableIndicatorStyle.Render(s)
}

func LegibleProviderName(providerName string) string {
	displayNames := map[string]string{
		"aws":   "AWS",
		"azure": "Azure",
		"gcp":   "GCP",
	}

	displayName, exists := displayNames[providerName]
	if !exists {
		displayName = providerName
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3498DB")).
		Bold(true).
		Render(displayName)
}
