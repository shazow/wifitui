package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shazow/wifitui/backend"
)

func (m model) updateEditView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.focusManager.IsFocused() {
				m.focusManager.Blur()
				return m, nil
			}
			m.state = stateListView
			m.statusMessage = ""
			m.errorMessage = ""
			m.passwordInput.Model.EchoMode = textinput.EchoNormal
			m.passwordInput.Model.Placeholder = ""
			m.passwordRevealed = false
		case "q":
			return m, tea.Quit
		case "*":
			if m.selectedItem.IsKnown && m.passwordInput.Model.Value() != "" {
				m.passwordRevealed = !m.passwordRevealed
				if m.passwordRevealed {
					m.passwordInput.Model.EchoMode = textinput.EchoNormal
				} else {
					m.passwordInput.Model.EchoMode = textinput.EchoPassword
				}
			}
		default:
			if m.focusManager.IsFocused() {
				var newFocusable Focusable
				newFocusable, cmd = m.focusManager.Update(msg)
				m.focusManager = newFocusable.(*FocusManager)
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) viewEditView() string {
	var s strings.Builder
	// --- Title ---
	title := "Wi-Fi Connection"
	s.WriteString(fmt.Sprintf("\n%s\n\n", title))

	// --- Details Box ---
	var details strings.Builder
	isNew := m.selectedItem.SSID == ""
	if isNew {
		s.WriteString("SSID:\n")
		s.WriteString(m.ssidInput.View())
		s.WriteString("\n\n")
	} else {
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
			details.WriteString(fmt.Sprintf("Last connected: \n  %s (%s)\n", m.selectedItem.LastConnected.Format(time.DateTime), formatDuration(*m.selectedItem.LastConnected)))
		}
		s.WriteString(lipgloss.NewStyle().Width(50).Border(lipgloss.RoundedBorder()).Padding(1, 2).Render(details.String()))
		s.WriteString("\n\n")
	}

	// --- Input field ---
	if shouldDisplayPasswordField(m.selectedItem.Security) {
		s.WriteString("Passphrase:\n")
		s.WriteString(m.passwordInput.View())
	}

	// --- Autoconnect Checkbox ---
	if m.selectedItem.IsKnown {
		s.WriteString("\n\n")
		s.WriteString(m.autoConnectCheckbox.View())
	}

	// --- Security Selection ---
	if m.selectedItem.SSID == "" {
		s.WriteString("\n\nSecurity:\n")
		s.WriteString(m.securityGroup.View())
	}

	// --- Button rendering ---
	s.WriteString("\n\n")
	for _, btn := range m.buttons {
		s.WriteString(btn.View())
		s.WriteString("  ")
	}
	s.WriteString("\n\n(tab to switch fields, arrows to navigate, enter to select)")

	// --- QR Code ---
	if m.selectedItem.IsKnown {
		password := m.passwordInput.Model.Value()
		if m.passwordRevealed && password != "" {
			qrCodeString, err := GenerateWifiQRCode(m.selectedItem.SSID, password, m.selectedItem.IsSecure, m.selectedItem.IsHidden)
			if err == nil {
				s.WriteString("\n\n")
				s.WriteString(qrCodeString)
			}
		}
	}
	return s.String()
}
