package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shazow/wifitui/backend"
	"github.com/shazow/wifitui/internal/helpers"
	"github.com/shazow/wifitui/qrwifi"
)

func (m *model) setupEditView() {
	isNew := m.selectedItem.SSID == ""
	var items []Focusable

	m.ssidAdapter = NewTextInputAdapter(m.ssidInput, false)
	m.passwordAdapter = NewTextInputAdapter(m.passwordInput, true)

	if isNew {
		items = append(items, m.ssidAdapter)
	}

	security := m.selectedItem.Security
	if isNew {
		m.securityGroup = NewChoiceComponent("Security:", []string{"Open", "WEP", "WPA/WPA2"})
		security = backend.SecurityType(m.securityGroup.Selected())
	}

	if shouldDisplayPasswordField(security) {
		items = append(items, m.passwordAdapter)
	}

	if isNew {
		items = append(items, m.securityGroup)
	}

	if m.selectedItem.IsKnown {
		m.autoConnectCheckbox = NewCheckbox("Auto Connect", m.selectedItem.AutoConnect)
		items = append(items, m.autoConnectCheckbox)
	}

	var buttons []string
	if isNew {
		buttons = []string{"Join", "Cancel"}
	} else if m.selectedItem.IsKnown {
		buttons = []string{"Connect", "Save", "Forget", "Cancel"}
	} else {
		buttons = []string{"Join", "Cancel"}
	}
	buttonAction := func(index int) tea.Cmd {
		var cmds []tea.Cmd
		isNew := m.selectedItem.SSID == ""
		if isNew {
			switch index {
			case 0: // Join
				m.loading = true
				ssid := m.ssidInput.Value()
				m.statusMessage = fmt.Sprintf("Joining '%s'...", ssid)
				cmds = append(cmds, joinNetwork(m.backend, ssid, m.passwordInput.Value(), backend.SecurityType(m.securityGroup.Selected()), true))
			case 1: // Cancel
				return func() tea.Msg { return changeViewMsg(stateListView) }
			}
		} else if m.selectedItem.IsKnown {
			switch index {
			case 0: // Connect
				m.loading = true
				m.statusMessage = fmt.Sprintf("Connecting to '%s'...", m.selectedItem.SSID)
				cmds = append(cmds, activateConnection(m.backend, m.selectedItem.SSID))
			case 1: // Save
				m.loading = true
				m.statusMessage = fmt.Sprintf("Saving settings for %s...", m.selectedItem.SSID)
				cmds = append(cmds, updateSecret(m.backend, m.selectedItem.SSID, m.passwordInput.Value()))
				if m.autoConnectCheckbox.Checked() != m.selectedItem.AutoConnect {
					cmds = append(cmds, updateAutoConnect(m.backend, m.selectedItem.SSID, m.autoConnectCheckbox.Checked()))
				}
			case 2: // Forget
				return func() tea.Msg { return changeViewMsg(stateForgetView) }
			case 3: // Cancel
				return func() tea.Msg { return changeViewMsg(stateListView) }
			}
		} else {
			switch index {
			case 0: // Join
				m.loading = true
				m.statusMessage = fmt.Sprintf("Joining '%s'...", m.selectedItem.SSID)
				cmds = append(cmds, joinNetwork(m.backend, m.selectedItem.SSID, m.passwordInput.Value(), m.selectedItem.Security, m.selectedItem.IsHidden))
			case 1: // Cancel
				return func() tea.Msg { return changeViewMsg(stateListView) }
			}
		}
		return tea.Batch(cmds...)
	}
	m.buttonGroup = NewMultiButtonComponent(buttons, buttonAction)
	items = append(items, m.buttonGroup)

	m.editFocusManager = NewFocusManager(items...)
	m.editFocusManager.Focus()
}

func (m *model) updateEditView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			blurred, cmd := m.editFocusManager.Next()
			if ta, ok := blurred.(*TextInputAdapter); ok {
				if ta == m.passwordAdapter {
					m.passwordInput = ta.Model
				}
			}
			return m, cmd
		case "shift+tab":
			blurred, cmd := m.editFocusManager.Prev()
			if ta, ok := blurred.(*TextInputAdapter); ok {
				if ta == m.passwordAdapter {
					m.passwordInput = ta.Model
				}
			}
			return m, cmd
		case "esc":
			m.state = stateListView
			return m, nil
		}
	}

	newFocusable, cmd := m.editFocusManager.Update(msg)
	cmds = append(cmds, cmd)

	if ta, ok := newFocusable.(*TextInputAdapter); ok {
		if ta == m.ssidAdapter {
			m.ssidInput = ta.Model
		} else if ta == m.passwordAdapter {
			m.passwordInput = ta.Model
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) viewEditView() string {
	var s strings.Builder
	s.WriteString(fmt.Sprintf("\n%s\n\n", "Wi-Fi Connection"))

	isNew := m.selectedItem.SSID == ""
	if !isNew {
		var details strings.Builder
		details.WriteString(fmt.Sprintf("SSID: %s\n", m.selectedItem.SSID))
		var security string
		switch m.selectedItem.Security {
		case backend.SecurityOpen:
			security = "Open"
		case backend.SecurityWEP:
			security = "WEP"
		case backend.SecurityWPA:
			security = "WPA/WPA2"
		default:
			if m.selectedItem.IsSecure {
				security = "Secure"
			} else {
				security = "Open"
			}
		}
		details.WriteString(fmt.Sprintf("Security: %s\n", security))
		if m.selectedItem.Strength > 0 {
			details.WriteString(fmt.Sprintf("Signal: %d%%\n", m.selectedItem.Strength))
		}
		if m.selectedItem.IsKnown && m.selectedItem.LastConnected != nil {
			details.WriteString(fmt.Sprintf("Last connected: \n  %s (%s)\n", m.selectedItem.LastConnected.Format(time.DateTime), helpers.FormatDuration(*m.selectedItem.LastConnected)))
		}
		s.WriteString(lipgloss.NewStyle().Width(50).Border(lipgloss.RoundedBorder()).Padding(1, 2).Render(details.String()))
		s.WriteString("\n\n")
	}

	for _, item := range m.editFocusManager.items {
		if ta, ok := item.(*TextInputAdapter); ok {
			if ta == m.ssidAdapter {
				s.WriteString("SSID:\n")
			} else {
				s.WriteString("Passphrase:\n")
			}
			style := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder(), true).
				Padding(0, 1)
			if m.editFocusManager.Focused() == item {
				style = style.BorderForeground(lipgloss.Color("205"))
			}
			s.WriteString(style.Render(item.View()))
		} else {
			s.WriteString(item.View())
		}
		s.WriteString("\n\n")
	}

	s.WriteString("\n\n(tab to switch fields, arrows to navigate, enter to select)")

	if m.selectedItem.IsKnown {
		password := m.passwordInput.Value()
		if m.passwordRevealed && password != "" {
			qrCodeString, err := qrwifi.GenerateWifiQRCode(m.selectedItem.SSID, password, m.selectedItem.IsSecure, m.selectedItem.IsHidden)
			if err == nil {
				s.WriteString("\n\n")
				s.WriteString(qrCodeString)
			}
		}
	}

	return s.String()
}
