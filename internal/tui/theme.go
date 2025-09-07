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

	SignalHigh lipgloss.TerminalColor
	SignalLow  lipgloss.TerminalColor
}

// CurrentTheme is the active theme for the application.
var CurrentTheme = NewDefaultTheme()

// NewDefaultTheme creates a new default theme.
func NewDefaultTheme() Theme {
	return Theme{
		Primary:   lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#D359E3"}, // Purple/Pink
		Subtle:    lipgloss.AdaptiveColor{Light: "#BDBDBD", Dark: "#616161"}, // Gray
		Success:   lipgloss.AdaptiveColor{Light: "#388E3C", Dark: "#81C784"}, // Green
		Error:     lipgloss.AdaptiveColor{Light: "#D32F2F", Dark: "#E57373"}, // Red
		Normal:    lipgloss.AdaptiveColor{Light: "#212121", Dark: "#FFFFFF"}, // Black/White
		Disabled:  lipgloss.AdaptiveColor{Light: "#E0E0E0", Dark: "#424242"}, // Lighter/Darker Gray
		Border:    lipgloss.AdaptiveColor{Light: "#BDBDBD", Dark: "#616161"}, // Gray

		SignalHigh: lipgloss.AdaptiveColor{Light: "#00B300", Dark: "#00FF00"},
		SignalLow:  lipgloss.AdaptiveColor{Light: "#D05F00", Dark: "#BC3C00"},
	}
}
