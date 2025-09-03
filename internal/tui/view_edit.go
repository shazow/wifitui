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

	ssidAdapter := NewTextInputAdapter(m.ssidInput)
	passwordAdapter := NewTextInputAdapter(m.passwordInput)

	if isNew {
		m.ssidInput.Focus()
		items = append(items, ssidAdapter)
		m.editSecurityChoice = NewRadioGroup([]string{"Open", "WEP", "WPA/WPA2"}, 0)
		if shouldDisplayPasswordField(backend.SecurityType(m.editSecurityChoice.Selected())) {
			items = append(items, passwordAdapter)
		}
		items = append(items, m.editSecurityChoice)
	} else {
		m.passwordInput.Focus()
		if shouldDisplayPasswordField(m.selectedItem.Security) {
			items = append(items, passwordAdapter)
		}
		if m.selectedItem.IsKnown {
			m.editConnectCheck = NewCheckbox("Auto Connect", m.editAutoConnect)
			items = append(items, m.editConnectCheck)
		}
	}

	var buttons []string
	if isNew {
		buttons = []string{"Join", "Cancel"}
	} else if m.selectedItem.IsKnown {
		buttons = []string{"Connect", "Save", "Forget", "Cancel"}
	} else {
		buttons = []string{"Join", "Cancel"}
	}
	m.editButtons = NewButtonGroup(buttons, m.editAction)
	items = append(items, m.editButtons)

	m.editFocusManager = NewFocusManager(items...)
	m.editFocusManager.Focus()
}

func (m model) updateEditView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			return m, m.editFocusManager.Next()
		case "shift+tab":
			return m, m.editFocusManager.Prev()
		case "esc":
			m.state = stateListView
			m.statusMessage = ""
			m.errorMessage = ""
			return m, nil
		case "q":
			return m, tea.Quit
		case "*":
			if m.selectedItem.IsKnown && m.passwordInput.Value() != "" {
				m.passwordRevealed = !m.passwordRevealed
			}
		}
	}

	var cmd tea.Cmd
	newFocusable, cmd := m.editFocusManager.Update(msg)
	cmds = append(cmds, cmd)

	// Persist the new focusable to the model
	switch v := newFocusable.(type) {
	case *TextInputAdapter:
		// This is a bit of a hack, but we need to update the model's text input
		// since the adapter just wraps it.
		focused := m.editFocusManager.Focused()
		if focused == v {
			// This is the SSID or password input. We need to figure out which one.
			// We can do this by comparing the value.
			if v.Value() == m.ssidInput.Value() {
				m.ssidInput = v.Model
			} else {
				m.passwordInput = v.Model
			}
		}
	case *Checkbox:
		m.editConnectCheck = v
		m.editAutoConnect = v.Checked()
	case *RadioGroup:
		m.editSecurityChoice = v
	case *ButtonGroup:
		m.editButtons = v
	}

	return m, tea.Batch(cmds...)
}

func (m model) viewEditView() string {
	var s strings.Builder
	s.WriteString(fmt.Sprintf("\n%s\n\n", "Wi-Fi Connection"))

	isNew := m.selectedItem.SSID == ""
	if isNew {
		s.WriteString("SSID:\n")
		s.WriteString(m.ssidInput.View())
		s.WriteString("\n\n")
	} else {
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

	if shouldDisplayPasswordField(m.selectedItem.Security) {
		s.WriteString("Passphrase:\n")
		s.WriteString(m.passwordInput.View())
	}

	if m.selectedItem.IsKnown {
		s.WriteString("\n\n")
		s.WriteString(m.editConnectCheck.View())
	}

	if isNew {
		s.WriteString("\n\nSecurity:\n")
		s.WriteString(m.editSecurityChoice.View())
	}

	s.WriteString("\n\n")
	s.WriteString(m.editButtons.View())
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
