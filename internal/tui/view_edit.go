package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shazow/wifitui/backend"
	"github.com/shazow/wifitui/internal/helpers"
	"github.com/shazow/wifitui/qrwifi"
)

// --- View Logic ---

func (m *model) setupEditView() {
	isNew := m.selectedItem.SSID == ""
	var items []Focusable

	if isNew {
		items = append(items, focusableInt(focusSSID))
	}

	security := m.selectedItem.Security
	if isNew {
		security = backend.SecurityType(m.editSecuritySelection)
	}

	if shouldDisplayPasswordField(security) {
		items = append(items, focusableInt(focusInput))
	}

	if isNew {
		items = append(items, focusableInt(focusSecurity))
	}

	if m.selectedItem.IsKnown {
		items = append(items, focusableInt(focusAutoConnect))
	}

	items = append(items, focusableInt(focusButtons))

	m.editFocusManager = NewFocusManager(items...)
	m.editFocusManager.Focus()
	m.updateFocus(nil) // Set initial focus on inputs
}

// updateFocus handles the logic for focusing/blurring text inputs.
func (m *model) updateFocus(cmd tea.Cmd) tea.Cmd {
	focus := m.editFocusManager.Focused().(focusableInt)

	if focus == focusSSID {
		return m.ssidInput.Focus()
	}
	m.ssidInput.Blur()

	if focus == focusInput {
		m.passwordInput.EchoMode = textinput.EchoNormal
		return m.passwordInput.Focus()
	}
	m.passwordInput.EchoMode = textinput.EchoPassword
	m.passwordInput.Blur()

	return cmd
}

func (m *model) updateEditView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		focused := m.editFocusManager.Focused().(focusableInt)
		if focused == focusSSID {
			m.ssidInput, cmd = m.ssidInput.Update(msg)
			return m, cmd
		}
		if focused == focusInput {
			m.passwordInput, cmd = m.passwordInput.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "tab":
			cmd = m.editFocusManager.Next()
			return m, m.updateFocus(cmd)
		case "shift+tab":
			cmd = m.editFocusManager.Prev()
			return m, m.updateFocus(cmd)
		case "esc":
			m.state = stateListView
			return m, nil

		default:
			switch focused {
			case focusSecurity:
				numOptions := 3 // Open, WEP, WPA/WPA2
				switch msg.String() {
				case "right":
					m.editSecuritySelection = (m.editSecuritySelection + 1) % numOptions
				case "left":
					m.editSecuritySelection = (m.editSecuritySelection - 1 + numOptions) % numOptions
				}
			case focusAutoConnect:
				switch msg.String() {
				case "enter", " ":
					m.editAutoConnect = !m.editAutoConnect
				}
			case focusButtons:
				var numButtons int
				if m.selectedItem.SSID == "" {
					numButtons = 2
				} else if m.selectedItem.IsKnown {
					numButtons = 4
				} else {
					numButtons = 2
				}

				switch msg.String() {
				case "right":
					m.editSelectedButton = (m.editSelectedButton + 1) % numButtons
				case "left":
					m.editSelectedButton = (m.editSelectedButton - 1 + numButtons) % numButtons
				case "enter":
					isNew := m.selectedItem.SSID == ""
					if isNew {
						switch m.editSelectedButton {
						case 0: // Join
							m.loading = true
							ssid := m.ssidInput.Value()
							m.statusMessage = fmt.Sprintf("Joining '%s'...", ssid)
							cmds = append(cmds, joinNetwork(m.backend, ssid, m.passwordInput.Value(), backend.SecurityType(m.editSecuritySelection), true))
						case 1: // Cancel
							m.state = stateListView
						}
					} else if m.selectedItem.IsKnown {
						switch m.editSelectedButton {
						case 0: // Connect
							m.loading = true
							m.statusMessage = fmt.Sprintf("Connecting to '%s'...", m.selectedItem.SSID)
							cmds = append(cmds, activateConnection(m.backend, m.selectedItem.SSID))
						case 1: // Save
							m.loading = true
							m.statusMessage = fmt.Sprintf("Saving settings for %s...", m.selectedItem.SSID)
							cmds = append(cmds, updateSecret(m.backend, m.selectedItem.SSID, m.passwordInput.Value()))
							if m.editAutoConnect != m.selectedItem.AutoConnect {
								cmds = append(cmds, updateAutoConnect(m.backend, m.selectedItem.SSID, m.editAutoConnect))
							}
						case 2: // Forget
							m.state = stateForgetView
						case 3: // Cancel
							m.state = stateListView
						}
					} else {
						switch m.editSelectedButton {
						case 0: // Join
							m.loading = true
							m.statusMessage = fmt.Sprintf("Joining '%s'...", m.selectedItem.SSID)
							cmds = append(cmds, joinNetwork(m.backend, m.selectedItem.SSID, m.passwordInput.Value(), m.selectedItem.Security, m.selectedItem.IsHidden))
						case 1: // Cancel
							m.state = stateListView
						}
					}
				}
			}
		}
	}
	return m, tea.Batch(cmds...)
}

func (m model) viewEditView() string {
	var s strings.Builder
	s.WriteString(fmt.Sprintf("\n%s\n\n", "Wi-Fi Connection"))

	focused := m.editFocusManager.Focused().(focusableInt)
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

	if isNew {
		s.WriteString("SSID:\n")
		ssidStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			Padding(0, 1)
		if focused == focusSSID {
			ssidStyle = ssidStyle.BorderForeground(lipgloss.Color("205"))
		}
		s.WriteString(ssidStyle.Render(m.ssidInput.View()))
		s.WriteString("\n\n")
	}

	security := m.selectedItem.Security
	if isNew {
		security = backend.SecurityType(m.editSecuritySelection)
	}
	if shouldDisplayPasswordField(security) {
		s.WriteString("Passphrase:\n")
		passwordStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			Padding(0, 1)
		if focused == focusInput {
			passwordStyle = passwordStyle.BorderForeground(lipgloss.Color("205"))
		}
		s.WriteString(passwordStyle.Render(m.passwordInput.View()))
		s.WriteString("\n\n")
	}

	if m.selectedItem.IsKnown {
		s.WriteString("\n\n")
		var checkbox string
		if m.editAutoConnect {
			checkbox = "[x]"
		} else {
			checkbox = "[ ]"
		}
		label := "Auto Connect"
		style := blurredStyle
		if focused == focusAutoConnect {
			style = focusedStyle
		}
		s.WriteString(style.Render(fmt.Sprintf("%s %s", checkbox, label)))
		s.WriteString("\n\n")
	}

	if isNew {
		s.WriteString("\n\nSecurity:\n")
		securityOptions := []string{"Open", "WEP", "WPA/WPA2"}
		var securityRow strings.Builder
		for i, label := range securityOptions {
			style := blurredStyle
			if focused == focusSecurity && i == m.editSecuritySelection {
				style = focusedStyle
			}
			securityRow.WriteString(style.Render(fmt.Sprintf("[ %s ]", label)))
			securityRow.WriteString("  ")
		}
		s.WriteString(securityRow.String())
		s.WriteString("\n\n")
	}

	var buttonRow strings.Builder
	var buttons []string
	if isNew {
		buttons = []string{"Join", "Cancel"}
	} else if m.selectedItem.IsKnown {
		buttons = []string{"Connect", "Save", "Forget", "Cancel"}
	} else {
		buttons = []string{"Join", "Cancel"}
	}
	for i, label := range buttons {
		style := blurredStyle
		if focused == focusButtons && i == m.editSelectedButton {
			style = focusedStyle
		}
		buttonRow.WriteString(style.Render(fmt.Sprintf("[ %s ]", label)))
		buttonRow.WriteString("  ")
	}
	s.WriteString(buttonRow.String())
	s.WriteString("\n\n(tab to switch fields, arrows to navigate, enter to select)")

	// --- QR Code ---
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
