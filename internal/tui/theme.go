package tui

import "github.com/charmbracelet/lipgloss"

// Theme contains the styles for the application.
type Theme struct {
	// Text styles
	Primary   lipgloss.Style
	Subtle    lipgloss.Style
	Success   lipgloss.Style
	Error     lipgloss.Style
	Normal    lipgloss.Style
	Disabled  lipgloss.Style

	// Box styles
	Box      lipgloss.Style
	ListBorderStyle lipgloss.Style

	// List item styles
	ListItemStyle         lipgloss.Style
	SelectedListItemStyle lipgloss.Style

	// Layout styles
	Doc      lipgloss.Style
	Question lipgloss.Style

	// Colors for non-style contexts
	SignalHighColor string
	SignalLowColor  string

	PrimaryColor lipgloss.TerminalColor
	SubtleColor  lipgloss.TerminalColor
	ErrorColor   lipgloss.TerminalColor
}

// CurrentTheme is the active theme for the application.
var CurrentTheme = NewDefaultTheme()

// NewDefaultTheme creates a new default theme.
func NewDefaultTheme() Theme {
	primary := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	subtle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	success := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	normal := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))

	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)

	return Theme{
		Primary:   primary,
		Subtle:    subtle,
		Success:   success,
		Error:     errorStyle,
		Normal:    normal,
		Disabled:  subtle.Strikethrough(true).Foreground(lipgloss.Color("244")),

		Box:      box,
		ListBorderStyle: box.Border(lipgloss.RoundedBorder(), true).BorderForeground(lipgloss.Color("240")),

		ListItemStyle: lipgloss.NewStyle().PaddingLeft(2),
		SelectedListItemStyle: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("205")).
			PaddingLeft(1),

		Doc:      lipgloss.NewStyle().Margin(1, 2),
		Question: lipgloss.NewStyle().Width(50).Align(lipgloss.Center),

		SignalHighColor: "#00FF00",
		SignalLowColor:  "#BC3C00",

		PrimaryColor: lipgloss.Color("205"),
		SubtleColor:  lipgloss.Color("240"),
		ErrorColor:   lipgloss.Color("196"),
	}
}
