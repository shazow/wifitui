package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

import "fmt"

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
	return CurrentTheme.DocStyle.Render(fmt.Sprintf("Error: %s", CurrentTheme.ErrorMessageStyle.Render(m.errorMessage)))
}
