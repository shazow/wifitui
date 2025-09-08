package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ForgetModel struct {
	item         connectionItem
	width, height int
}

func NewForgetModel(item connectionItem, width, height int) ForgetModel {
	return ForgetModel{item: item, width: width, height: height}
}

func (m ForgetModel) Init() tea.Cmd {
	return nil
}

func (m ForgetModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "enter", "f":
			return m, func() tea.Msg { return forgetNetworkMsg{item: m.item} }
		case "n", "q", "esc":
			return m, func() tea.Msg { return changeViewMsg(stateListView) }
		}
	}
	return m, nil
}

func (m ForgetModel) View() string {
	question := fmt.Sprintf("Forget network '%s'? (Y/n)", m.item.SSID)
	questionStyle := lipgloss.NewStyle().Width(50).Align(lipgloss.Center).Render(question)
	dialog := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).BorderForeground(CurrentTheme.Primary).Render(questionStyle)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
}
