package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ErrorModel struct {
	err error
}

func NewErrorModel(err error) *ErrorModel {
	return &ErrorModel{err: err}
}

func (m *ErrorModel) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		// Any key press dismisses the error
		return m, func() tea.Msg { return popViewMsg{} }
	}
	return m, nil
}

func (m *ErrorModel) View() string {
	errorViewStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder(), true).
		BorderForeground(CurrentTheme.Error).
		Padding(1, 2)
	return lipgloss.NewStyle().Margin(1, 2).Render(errorViewStyle.Render(fmt.Sprintf("Error: %s", m.err)))
}

// IsConsumingInput returns whether the model is focused on a text input.
func (m *ErrorModel) IsConsumingInput() bool {
	return false
}
