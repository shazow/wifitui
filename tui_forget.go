package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m model) updateForgetView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y":
			m.loading = true
			m.statusMessage = fmt.Sprintf("Forgetting '%s'...", m.selectedItem.SSID)
			m.errorMessage = ""
			cmds = append(cmds, forgetNetwork(m.backend, m.selectedItem.SSID))
		case "n", "q", "esc":
			m.state = stateListView
			m.statusMessage = ""
			m.errorMessage = ""
		}
	}
	return m, tea.Batch(cmds...)
}

func (m model) viewForgetView() string {
	question := lipgloss.NewStyle().Width(50).Align(lipgloss.Center).Render(m.statusMessage)
	dialog := dialogBoxStyle.Render(question)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
}
