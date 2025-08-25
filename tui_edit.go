package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m model) updateEditView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle key presses for the edit view
		switch msg.String() {
		case "tab":
			if m.editFocus == focusInput {
				m.editFocus = focusButtons
				m.passwordInput.Blur()
			} else {
				m.editFocus = focusInput
				m.passwordInput.Focus()
			}
		case "esc":
			m.state = stateListView
			m.statusMessage = ""
			m.errorMessage = ""

		default:
			if m.editFocus == focusInput {
				// Pass all other key presses to the input field
				m.passwordInput, cmd = m.passwordInput.Update(msg)
				cmds = append(cmds, cmd)
			} else { // m.editFocus == focusButtons
				numButtons := 2
				if m.selectedItem.IsKnown {
					numButtons = 3
				}

				switch msg.String() {
				case "right":
					m.editSelectedButton = (m.editSelectedButton + 1) % numButtons
				case "left":
					m.editSelectedButton = (m.editSelectedButton - 1 + numButtons) % numButtons
				case "enter":
					if m.selectedItem.IsKnown {
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
							cmds = append(cmds, joinNetwork(m.backend, m.selectedItem.SSID, m.passwordInput.Value()))
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

	// --- Details Box ---
	var details strings.Builder
	details.WriteString(fmt.Sprintf("SSID: %s\n", m.selectedItem.SSID))
	security := "Open"
	if m.selectedItem.IsSecure {
		security = "Secure"
	}
	details.WriteString(fmt.Sprintf("Security: %s\n", security))
	if m.selectedItem.Strength > 0 {
		details.WriteString(fmt.Sprintf("Signal: %d%%\n", m.selectedItem.Strength))
	}
	if m.selectedItem.IsKnown && m.selectedItem.LastConnected != nil {
		details.WriteString(fmt.Sprintf("Last connected: \n  %s (%s)\n", m.selectedItem.LastConnected.Format(time.DateTime), formatDuration(*m.selectedItem.LastConnected)))
	}

	s.WriteString(lipgloss.NewStyle().Width(50).Border(lipgloss.RoundedBorder()).Padding(1, 2).Render(details.String()))
	s.WriteString("\n")
	s.WriteString(m.passwordInput.View())

	// --- Button rendering ---
	var buttonRow strings.Builder
	var buttons []string
	if m.selectedItem.IsKnown {
		buttons = []string{"Connect", "Save", "Cancel"}
	} else {
		buttons = []string{"Join", "Cancel"}
	}
	focusedButtonStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	normalButtonStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

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
		if password != "" {
			qrCodeString, err := GenerateWifiQRCode(m.selectedItem.SSID, password, m.selectedItem.IsSecure, m.selectedItem.IsHidden)
			if err == nil {
				s.WriteString("\n\n")
				s.WriteString(qrCodeString)
			}
		}
	}
	return s.String()
}
