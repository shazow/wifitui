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
		// Handle key presses for the edit view
		switch msg.String() {
		case "tab":
			isNew := m.selectedItem.SSID == ""
			if isNew {
				// Cycle through SSID, password, security, and buttons
				switch m.editFocus {
				case focusSSID:
					m.editFocus = focusInput
					m.ssidInput.Blur()
					m.passwordInput.Focus()
				case focusInput:
					m.editFocus = focusSecurity
					m.passwordInput.Blur()
				case focusSecurity:
					m.editFocus = focusButtons
				case focusButtons:
					m.editFocus = focusSSID
					m.ssidInput.Focus()
				}
			} else {
				// Cycle through password and buttons
				if m.editFocus == focusInput {
					m.editFocus = focusButtons
					m.passwordInput.Blur()
				} else {
					m.editFocus = focusInput
					m.passwordInput.Focus()
				}
			}
		case "esc":
			m.state = stateListView
			m.statusMessage = ""
			m.errorMessage = ""
		case "*":
			if m.editFocus == focusInput {
				m.passwordRevealed = !m.passwordRevealed
				if m.passwordRevealed {
					m.passwordInput.EchoMode = textinput.EchoNormal
				} else {
					m.passwordInput.EchoMode = textinput.EchoPassword
				}
			}
		default:
			switch m.editFocus {
			case focusSSID:
				m.ssidInput, cmd = m.ssidInput.Update(msg)
				cmds = append(cmds, cmd)
			case focusInput:
				m.passwordInput, cmd = m.passwordInput.Update(msg)
				cmds = append(cmds, cmd)
			case focusSecurity:
				numOptions := 3 // Open, WEP, WPA/WPA2
				switch msg.String() {
				case "right":
					m.editSecuritySelection = (m.editSecuritySelection + 1) % numOptions
				case "left":
					m.editSecuritySelection = (m.editSecuritySelection - 1 + numOptions) % numOptions
				}
			case focusButtons:
				var numButtons int
				if m.selectedItem.SSID == "" {
					numButtons = 2 // Join, Cancel
				} else if m.selectedItem.IsKnown {
					numButtons = 3 // Connect, Save, Cancel
				} else {
					numButtons = 2 // Join, Cancel
				}

				switch msg.String() {
				case "right":
					m.editSelectedButton = (m.editSelectedButton + 1) % numButtons
				case "left":
					m.editSelectedButton = (m.editSelectedButton - 1 + numButtons) % numButtons
				case "enter":
					isNew := m.selectedItem.SSID == ""
					if isNew {
						// New network buttons: Join, Cancel
						switch m.editSelectedButton {
						case 0: // Join
							m.loading = true
							ssid := m.ssidInput.Value()
							m.statusMessage = fmt.Sprintf("Joining '%s'...", ssid)
							m.errorMessage = ""
							cmds = append(cmds, joinNetwork(m.backend, ssid, m.passwordInput.Value(), backend.SecurityType(m.editSecuritySelection), true))
						case 1: // Cancel
							m.state = stateListView
							m.statusMessage = ""
							m.errorMessage = ""
						}
					} else if m.selectedItem.IsKnown {
						// Known network buttons: Connect, Save, Cancel
						switch m.editSelectedButton {
						case 0: // Connect
							m.loading = true
							m.statusMessage = fmt.Sprintf("Connecting to '%s'...", m.selectedItem.SSID)
							cmds = append(cmds, activateConnection(m.backend, m.selectedItem.SSID))
						case 1: // Save
							m.loading = true
							m.statusMessage = fmt.Sprintf("Saving password for %s...", m.selectedItem.SSID)
							m.errorMessage = ""
							cmds = append(cmds, updateSecret(m.backend, m.selectedItem.SSID, m.passwordInput.Value()))
						case 2: // Cancel
							m.state = stateListView
							m.statusMessage = ""
							m.errorMessage = ""
						}
					} else {
						// Unknown network buttons: Join, Cancel
						switch m.editSelectedButton {
						case 0: // Join
							m.loading = true
							m.statusMessage = fmt.Sprintf("Joining '%s'...", m.selectedItem.SSID)
							m.errorMessage = ""
							cmds = append(cmds, joinNetwork(m.backend, m.selectedItem.SSID, m.passwordInput.Value(), m.selectedItem.Security, m.selectedItem.IsHidden))
						case 1: // Cancel
							m.state = stateListView
							m.statusMessage = ""
							m.errorMessage = ""
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
	// --- Title ---
	title := "Wi-Fi Connection"
	s.WriteString(fmt.Sprintf("\n%s\n\n", title))

	// --- Styles ---
	focusedButtonStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	normalButtonStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

	// --- Details Box ---
	var details strings.Builder
	isNew := m.selectedItem.SSID == ""
	if isNew {
		s.WriteString("SSID:\n")
		var ssidInputView string
		if m.editFocus == focusSSID {
			ssidInputView = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder(), true).
				BorderForeground(lipgloss.Color("205")).
				Padding(0, 1).
				Render(m.ssidInput.View())
		} else {
			ssidInputView = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder(), true).
				Padding(0, 1).
				Render(m.ssidInput.View())
		}
		s.WriteString(ssidInputView)
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
	s.WriteString("Passphrase (optional for open networks):\n")
	var inputView string
	if m.editFocus == focusInput {
		inputView = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(lipgloss.Color("205")).
			Padding(0, 1).
			Render(m.passwordInput.View())
	} else {
		inputView = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			Padding(0, 1).
			Render(m.passwordInput.View())
	}
	s.WriteString(inputView)

	// --- Security Selection ---
	if m.selectedItem.SSID == "" {
		s.WriteString("\n\nSecurity:\n")
		securityOptions := []string{"Open", "WEP", "WPA/WPA2"}
		var securityRow strings.Builder
		for i, label := range securityOptions {
			style := normalButtonStyle
			if m.editFocus == focusSecurity && i == m.editSecuritySelection {
				style = focusedButtonStyle
			}
			securityRow.WriteString(style.Render(fmt.Sprintf("[ %s ]", label)))
			securityRow.WriteString("  ")
		}
		s.WriteString(securityRow.String())
	}

	// --- Button rendering ---
	var buttonRow strings.Builder
	var buttons []string
	if m.selectedItem.SSID == "" {
		buttons = []string{"Join", "Cancel"}
	} else if m.selectedItem.IsKnown {
		buttons = []string{"Connect", "Save", "Cancel"}
	} else {
		buttons = []string{"Join", "Cancel"}
	}

	for i, label := range buttons {
		style := normalButtonStyle
		if m.editFocus == focusButtons && i == m.editSelectedButton {
			style = focusedButtonStyle
		}
		buttonRow.WriteString(style.Render(fmt.Sprintf("[ %s ]", label)))
		buttonRow.WriteString("  ")
	}

	s.WriteString("\n\n")
	s.WriteString(buttonRow.String())
	s.WriteString("\n\n(tab to switch fields, arrows to navigate, enter to select)")

	// --- QR Code ---
	if m.selectedItem.IsKnown {
		password := m.passwordInput.Value()
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
