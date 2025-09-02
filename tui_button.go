package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	focusedButtonStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	normalButtonStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
)

// Button is a custom UI component for handling button logic and rendering.
type Button struct {
	Label   string
	ID      int // To identify the button
	focused bool
	OnClick func() tea.Cmd
}

// NewButton creates a new button.
func NewButton(label string, id int, onClick func() tea.Cmd) *Button {
	return &Button{
		Label:   label,
		ID:      id,
		OnClick: onClick,
	}
}

// Update handles messages for the button.
func (b *Button) Update(msg tea.Msg) (Focusable, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
		if b.OnClick != nil {
			return b, b.OnClick()
		}
	}
	return b, nil
}

// View renders the button.
func (b *Button) View() string {
	style := normalButtonStyle
	if b.focused {
		style = focusedButtonStyle
	}
	return style.Render("[ " + b.Label + " ]")
}

// Focus sets the focus on the button.
func (b *Button) Focus() {
	b.focused = true
}

// Blur removes the focus from the button.
func (b *Button) Blur() {
	b.focused = false
}

// IsFocused returns whether the button is focused.
func (b *Button) IsFocused() bool {
	return b.focused
}
