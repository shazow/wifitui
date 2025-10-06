package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shazow/wifitui/wifi"
)

type WirelessDisabledModel struct {
	backend wifi.Backend
}

func NewWirelessDisabledModel(backend wifi.Backend) *WirelessDisabledModel {
	return &WirelessDisabledModel{
		backend: backend,
	}
}

func (m *WirelessDisabledModel) Init() tea.Cmd {
	return nil
}

func (m *WirelessDisabledModel) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return m, func() tea.Msg {
				err := m.backend.SetWireless(true)
				if err != nil {
					return errorMsg{err}
				}
				// We re-use connectionSavedMsg to trigger a refresh in the main model.
				return connectionSavedMsg{}
			}
		case "q", "esc":
			// We are quitting the program, so no need to pop the view.
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *WirelessDisabledModel) View() string {
	var s strings.Builder
	s.WriteString(lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Render("Wi-Fi is disabled."))
	s.WriteString("\n\n")
	button := lipgloss.NewStyle().
		Foreground(CurrentTheme.Primary).
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Render("Enable WiFi (r)")

	s.WriteString(button)
	s.WriteString("\n\n")
	s.WriteString("Press 'q' to quit.\n")
	return s.String()
}

func (m *WirelessDisabledModel) IsConsumingInput() bool {
	return false
}
