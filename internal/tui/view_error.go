package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *model) updateErrorView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		// Any key press dismisses the error
		m.errorMessage = ""
		m.state = stateListView
	}
	return m, nil
}

func (m model) viewErrorView() string {
	errorViewStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder(), true).
		BorderForeground(CurrentTheme.Error).
		Padding(1, 2)
	return lipgloss.NewStyle().Margin(1, 2).Render(errorViewStyle.Render(fmt.Sprintf("Error: %s", m.errorMessage)))
}
