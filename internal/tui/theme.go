package tui

import "github.com/charmbracelet/lipgloss"

// Theme contains the colors for the application.
type Theme struct {
	Primary   lipgloss.TerminalColor
	Subtle    lipgloss.TerminalColor
	Success   lipgloss.TerminalColor
	Error     lipgloss.TerminalColor
	Normal    lipgloss.TerminalColor
	Disabled  lipgloss.TerminalColor
	Border    lipgloss.TerminalColor

	SignalHigh string
	SignalLow  string
}

// CurrentTheme is the active theme for the application.
var CurrentTheme = NewDefaultTheme()

// NewDefaultTheme creates a new default theme.
func NewDefaultTheme() Theme {
	return Theme{
		Primary:   lipgloss.Color("205"),
		Subtle:    lipgloss.Color("250"),
		Success:   lipgloss.Color("86"),
		Error:     lipgloss.Color("196"),
		Normal:    lipgloss.Color("255"),
		Disabled:  lipgloss.Color("244"),
		Border:    lipgloss.Color("240"),

		SignalHigh: "#00FF00",
		SignalLow:  "#BC3C00",
	}
}
