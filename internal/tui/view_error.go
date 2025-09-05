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
	errorViewStyle := CurrentTheme.Box.Border(lipgloss.DoubleBorder()).BorderForeground(CurrentTheme.ErrorColor)
	return CurrentTheme.Doc.Render(errorViewStyle.Render(fmt.Sprintf("Error: %s", m.errorMessage)))
}
