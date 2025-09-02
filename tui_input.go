package main

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	focusedInputStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder(), true).
				BorderForeground(lipgloss.Color("205")).
				Padding(0, 1)
	normalInputStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder(), true).
				Padding(0, 1)
)

// Input is a wrapper around the textinput.Model to make it a more
// self-contained component.
type Input struct {
	Model   textinput.Model
	Focused bool
}

// NewInput creates a new input component.
func NewInput() *Input {
	ti := textinput.New()
	return &Input{
		Model: ti,
	}
}

// Update handles messages for the input.
func (i *Input) Update(msg tea.Msg) (Focusable, tea.Cmd) {
	var cmd tea.Cmd
	i.Model, cmd = i.Model.Update(msg)
	return i, cmd
}

// View renders the input field.
func (i *Input) View() string {
	style := normalInputStyle
	if i.Focused {
		style = focusedInputStyle
	}
	return style.Render(i.Model.View())
}

// Focus sets the focus on the input field.
func (i *Input) Focus() {
	i.Focused = true
	i.Model.Focus()
}

// Blur removes the focus from the input field.
func (i *Input) Blur() {
	i.Focused = false
	i.Model.Blur()
}

// IsFocused returns whether the input field is focused.
func (i *Input) IsFocused() bool {
	return i.Focused
}
