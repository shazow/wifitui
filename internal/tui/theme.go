package tui

import "github.com/charmbracelet/lipgloss"

// Theme contains the styles for the application.
type Theme struct {
	ListBorderStyle     lipgloss.Style
	DocStyle            lipgloss.Style
	DisabledStyle       lipgloss.Style
	ActiveStyle         lipgloss.Style
	KnownNetworkStyle   lipgloss.Style
	UnknownNetworkStyle lipgloss.Style
	StatusMessageStyle  lipgloss.Style
	ErrorMessageStyle   lipgloss.Style
	FocusedStyle        lipgloss.Style
	BlurredStyle        lipgloss.Style
	ErrorViewStyle      lipgloss.Style
	TextInputStyle      lipgloss.Style
	DialogBoxStyle        lipgloss.Style
	DetailsBoxStyle       lipgloss.Style
	ListTitleStyle        lipgloss.Style
	ListFilterPromptStyle lipgloss.Style
	ListFilterCursorStyle lipgloss.Style
	QuestionStyle         lipgloss.Style

	ColorSignalHigh string
	ColorSignalLow  string

	FocusedColor lipgloss.TerminalColor
}

// CurrentTheme is the active theme for the application.
var CurrentTheme = NewDefaultTheme()

// NewDefaultTheme creates a new default theme.
func NewDefaultTheme() Theme {
	return Theme{
		ListBorderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(lipgloss.Color("240")),
		DocStyle: lipgloss.NewStyle().
			Margin(1, 2),
		DisabledStyle: lipgloss.NewStyle().
			Strikethrough(true).
			Foreground(lipgloss.Color("244")),
		ActiveStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")), // A nice aqua
		KnownNetworkStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("40")),
		UnknownNetworkStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")),
		StatusMessageStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")),
		ErrorMessageStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")), // Red
		FocusedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).Bold(true),
		BlurredStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")),
		ErrorViewStyle: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder(), true).
			BorderForeground(lipgloss.Color("9")). // Red
			Padding(1, 2),
		TextInputStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			Padding(0, 1),
		DialogBoxStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			BorderForeground(lipgloss.Color("205")),
		ListTitleStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true),
		ListFilterPromptStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")),
		ListFilterCursorStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")),
		DetailsBoxStyle: lipgloss.NewStyle().
			Width(50).
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2),
		QuestionStyle: lipgloss.NewStyle().
			Width(50).
			Align(lipgloss.Center),

		ColorSignalHigh: "#00FF00",
		ColorSignalLow:  "#BC3C00",

		FocusedColor: lipgloss.Color("205"),
	}
}
