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

func (m *model) updateEditView(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If a text input is focused, only treat Tab and Enter as hotkeys;
		// all other keys should go to the input itself (so '*' and 'q' can be typed).
		if m.editFocus == focusInput || m.editFocus == focusSSID {
			switch msg.String() {
			case "tab":
				// handled below in the main switch
			case "esc":
				// Exit the input focus instead of leaving the page
				if m.editFocus == focusSSID {
					m.ssidInput.Blur()
				} else {
					m.passwordInput.Blur()
					if m.selectedItem.IsKnown && m.passwordInput.Value() != "" {
						m.passwordRevealed = false
						m.passwordInput.EchoMode = textinput.EchoPassword
					}
				}
				m.editFocus = focusButtons
				return tea.Batch(cmds...)
			case "enter":
				if m.editFocus == focusSSID {
					// let the input handle it as well
					m.ssidInput, cmd = m.ssidInput.Update(msg)
					cmds = append(cmds, cmd)
					// and continue to allow any view-level enter behavior if needed
				} else {
					// For password, we just want to change focus
					m.passwordInput.Blur()
					m.editFocus = focusButtons
					return tea.Batch(cmds...)
				}
			default:
				// Forward to the focused input and return immediately
				if m.editFocus == focusSSID {
					m.ssidInput, cmd = m.ssidInput.Update(msg)
				} else {
					m.passwordInput, cmd = m.passwordInput.Update(msg)
				}
				cmds = append(cmds, cmd)
				return tea.Batch(cmds...)
			}
		}

		// Handle key presses for the edit view when not typing into inputs
		switch msg.String() {
		case "tab":
			isNew := m.selectedItem.SSID == ""
			if isNew {
				// Cycle through SSID, password, security, and buttons
				switch m.editFocus {
				case focusSSID:
					if shouldDisplayPasswordField(backend.SecurityType(m.editSecuritySelection)) {
						m.editFocus = focusInput
						m.ssidInput.Blur()
						m.passwordInput.Focus()
					} else {
						m.editFocus = focusSecurity
						m.ssidInput.Blur()
					}
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
				// Cycle through password, autoconnect, and buttons
				switch m.editFocus {
				case focusInput:
					m.editFocus = focusAutoConnect
					m.passwordInput.Blur()
					if m.selectedItem.IsKnown && m.passwordInput.Value() != "" {
						m.passwordRevealed = false
						m.passwordInput.EchoMode = textinput.EchoPassword
					}
				case focusAutoConnect:
					m.editFocus = focusButtons
				case focusButtons:
					if shouldDisplayPasswordField(m.selectedItem.Security) {
						m.editFocus = focusInput
						m.passwordInput.Focus()
						if m.selectedItem.IsKnown && m.passwordInput.Value() != "" {
							m.passwordRevealed = true
							m.passwordInput.EchoMode = textinput.EchoNormal
						}
					} else {
						m.editFocus = focusAutoConnect
					}
				default:
					if shouldDisplayPasswordField(m.selectedItem.Security) {
						m.editFocus = focusInput
						m.passwordInput.Focus()
					} else {
						m.editFocus = focusAutoConnect
					}

				}
			}
		case "esc":
			m.state = stateListView
			m.statusMessage = ""
			m.errorMessage = ""
			// Reset password input to default state
			m.passwordInput.EchoMode = textinput.EchoNormal
			m.passwordInput.Placeholder = ""
			m.passwordRevealed = false
		case "q":
			// Quit from the viewer when not typing into inputs
			return tea.Quit
		case "*":
			// Allow revealing password only for known networks with a password
			if m.selectedItem.IsKnown && m.passwordInput.Value() != "" {
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
			case focusAutoConnect:
				switch msg.String() {
				case "enter", " ":
					m.editAutoConnect = !m.editAutoConnect
				}
			case focusButtons:
				var numButtons int
				if m.selectedItem.SSID == "" {
					numButtons = 2 // Join, Cancel
				} else if m.selectedItem.IsKnown {
					numButtons = 4 // Connect, Save, Forget, Cancel
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
							m.passwordInput.EchoMode = textinput.EchoNormal
							m.passwordInput.Placeholder = ""
							m.passwordRevealed = false
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
							m.statusMessage = fmt.Sprintf("Saving settings for %s...", m.selectedItem.SSID)
							m.errorMessage = ""
							// NOTE(shazow): We can't tell if the password changed, so we always save it.
							cmds = append(cmds, updateSecret(m.backend, m.selectedItem.SSID, m.passwordInput.Value()))
							if m.editAutoConnect != m.selectedItem.AutoConnect {
								cmds = append(cmds, updateAutoConnect(m.backend, m.selectedItem.SSID, m.editAutoConnect))
							}
						case 2: // Forget
							m.state = stateForgetView
							m.statusMessage = fmt.Sprintf("Forget network '%s'? (y/n)", m.selectedItem.SSID)
							m.errorMessage = ""
						case 3: // Cancel
							m.state = stateListView
							m.statusMessage = ""
							m.errorMessage = ""
							m.passwordInput.EchoMode = textinput.EchoNormal
							m.passwordInput.Placeholder = ""
							m.passwordRevealed = false
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
							m.passwordInput.EchoMode = textinput.EchoNormal
							m.passwordInput.Placeholder = ""
							m.passwordRevealed = false
						}
					}
				}
			}
		}
	}
	return tea.Batch(cmds...)
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
	if shouldDisplayPasswordField(m.selectedItem.Security) {
		s.WriteString("Passphrase:\n")
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
	}

	// --- Autoconnect Checkbox ---
	if m.selectedItem.IsKnown {
		s.WriteString("\n\n")
		var checkbox string
		if m.editAutoConnect {
			checkbox = "[x]"
		} else {
			checkbox = "[ ]"
		}
		label := "Auto Connect"
		var styledCheckbox string
		if m.editFocus == focusAutoConnect {
			styledCheckbox = focusedButtonStyle.Render(fmt.Sprintf("%s %s", checkbox, label))
		} else {
			styledCheckbox = normalButtonStyle.Render(fmt.Sprintf("%s %s", checkbox, label))
		}
		s.WriteString(styledCheckbox)
	}

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
		buttons = []string{"Connect", "Save", "Forget", "Cancel"}
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
