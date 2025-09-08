package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shazow/wifitui/wifi"
)

type ForgetModel struct {
	backend    wifi.Backend
	connection connectionItem
	width, height int
}

func NewForgetModel(b wifi.Backend, c connectionItem) tea.Model {
	return &ForgetModel{
		backend:    b,
		connection: c,
	}
}

func (m *ForgetModel) Init() tea.Cmd {
	return nil
}

func (m *ForgetModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "enter", "f":
			cmd := forgetNetwork(m.backend, m.connection.SSID)
			return m, tea.Sequence(
				func() tea.Msg { return SetLoadingMsg{Loading: true, Message: fmt.Sprintf("Forgetting '%s'...", m.connection.SSID)} },
				cmd,
			)
		case "n", "q", "esc":
			return m, func() tea.Msg { return PopMsg{} }
		}
	case connectionSavedMsg:
		// When the network is forgotten, we pop back to the list view
		return m, func() tea.Msg { return PopMsg{} }
	}
	return m, nil
}

func (m *ForgetModel) View() string {
	question := lipgloss.NewStyle().Width(50).Align(lipgloss.Center).Render(fmt.Sprintf("Forget network '%s'? (Y/n)", m.connection.SSID))
	dialog := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).BorderForeground(CurrentTheme.Primary).Render(question)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
}

// A placeholder model for views that are not yet implemented.
type placeholderModel struct {
	name string
}

func (m placeholderModel) Init() tea.Cmd                           { return nil }
func (m placeholderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m placeholderModel) View() string                            { return "TODO: " + m.name }
