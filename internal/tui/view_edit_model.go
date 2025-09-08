package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shazow/wifitui/internal/helpers"
	"github.com/shazow/wifitui/qrwifi"
	"github.com/shazow/wifitui/wifi"
)

type EditModel struct {
	backend             wifi.Backend
	connection          connectionItem
	passwordInput       textinput.Model
	ssidInput           textinput.Model
	focusManager        *FocusManager
	ssidAdapter         *TextInput
	passwordAdapter     *TextInput
	securityGroup       *ChoiceComponent
	autoConnectCheckbox *Checkbox
	buttonGroup         *MultiButtonComponent
	passwordRevealed    bool
	width, height       int
}

func NewEditModel(b wifi.Backend, c connectionItem, password string) tea.Model {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 30
	ti.SetValue(password)
	if password != "" {
		ti.EchoMode = textinput.EchoPassword
	}

	si := textinput.New()
	si.Focus()
	si.CharLimit = 32
	si.Width = 30
	si.SetValue(c.SSID)

	m := &EditModel{
		backend:       b,
		connection:    c,
		passwordInput: ti,
		ssidInput:     si,
	}
	m.setupView()
	return m
}

func (m *EditModel) setupView() {
	isNew := m.connection.SSID == ""
	var items []Focusable

	m.ssidAdapter = &TextInput{
		Model: m.ssidInput,
		label: "SSID:",
	}
	onPasswordFocus := func(ti *textinput.Model) tea.Cmd {
		ti.EchoMode = textinput.EchoNormal
		m.passwordRevealed = true
		return nil
	}
	onPasswordBlur := func(ti *textinput.Model) {
		ti.EchoMode = textinput.EchoPassword
		m.passwordRevealed = false
	}
	m.passwordAdapter = &TextInput{
		Model:   m.passwordInput,
		label:   "Passphrase:",
		OnFocus: onPasswordFocus,
		OnBlur:  onPasswordBlur,
	}

	if isNew {
		items = append(items, m.ssidAdapter)
	}

	security := m.connection.Security
	if isNew {
		m.securityGroup = NewChoiceComponent("Security:", []string{"Open", "WEP", "WPA/WPA2"})
		security = wifi.SecurityType(m.securityGroup.Selected())
	}

	if shouldDisplayPasswordField(security) {
		items = append(items, m.passwordAdapter)
	}

	if isNew {
		items = append(items, m.securityGroup)
	}

	if m.connection.IsKnown {
		m.autoConnectCheckbox = NewCheckbox("Auto Connect", m.connection.AutoConnect)
		items = append(items, m.autoConnectCheckbox)
	}

	var buttons []string
	if isNew {
		buttons = []string{"Join", "Cancel"}
	} else if m.connection.IsKnown {
		buttons = []string{"Connect", "Save", "Forget", "Cancel"}
	} else {
		buttons = []string{"Join", "Cancel"}
	}
	buttonAction := func(index int) tea.Cmd {
		var cmds []tea.Cmd
		isNew := m.connection.SSID == ""
		if isNew {
			switch index {
			case 0: // Join
				ssid := m.ssidInput.Value()
				cmds = append(cmds, func() tea.Msg { return SetLoadingMsg{Loading: true, Message: fmt.Sprintf("Joining '%s'...", ssid)} })
				cmds = append(cmds, joinNetwork(m.backend, ssid, m.passwordAdapter.Model.Value(), wifi.SecurityType(m.securityGroup.Selected()), true))
			case 1: // Cancel
				return func() tea.Msg { return PopMsg{} }
			}
		} else if m.connection.IsKnown {
			switch index {
			case 0: // Connect
				cmds = append(cmds, func() tea.Msg { return SetLoadingMsg{Loading: true, Message: fmt.Sprintf("Connecting to '%s'...", m.connection.SSID)} })
				if m.autoConnectCheckbox.Checked() != m.connection.AutoConnect {
					cmds = append(cmds, updateAutoConnect(m.backend, m.connection.SSID, m.autoConnectCheckbox.Checked()))
				}
				cmds = append(cmds, activateConnection(m.backend, m.connection.SSID))
			case 1: // Save
				cmds = append(cmds, func() tea.Msg { return SetLoadingMsg{Loading: true, Message: fmt.Sprintf("Saving settings for %s...", m.connection.SSID)} })
				cmds = append(cmds, updateSecret(m.backend, m.connection.SSID, m.passwordAdapter.Model.Value()))
				if m.autoConnectCheckbox.Checked() != m.connection.AutoConnect {
					cmds = append(cmds, updateAutoConnect(m.backend, m.connection.SSID, m.autoConnectCheckbox.Checked()))
				}
			case 2: // Forget
				return func() tea.Msg { return PushMsg{Model: NewForgetModel(m.backend, m.connection)} }
			case 3: // Cancel
				return func() tea.Msg { return PopMsg{} }
			}
		} else {
			switch index {
			case 0: // Join
				cmds = append(cmds, func() tea.Msg { return SetLoadingMsg{Loading: true, Message: fmt.Sprintf("Joining '%s'...", m.connection.SSID)} })
				cmds = append(cmds, joinNetwork(m.backend, m.connection.SSID, m.passwordAdapter.Model.Value(), m.connection.Security, m.connection.IsHidden))
			case 1: // Cancel
				return func() tea.Msg { return PopMsg{} }
			}
		}
		return tea.Batch(cmds...)
	}
	m.buttonGroup = NewMultiButtonComponent(buttons, buttonAction)
	items = append(items, m.buttonGroup)

	m.focusManager = NewFocusManager(items...)

	if m.connection.IsKnown {
		m.focusManager.SetFocus(m.buttonGroup)
	} else {
		m.focusManager.Focus()
	}
}

func (m *EditModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *EditModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			return m, m.focusManager.Next()
		case "shift+tab":
			return m, m.focusManager.Prev()
		case "esc":
			return m, func() tea.Msg { return PopMsg{} }
		case "enter":
			if m.focusManager.Focused() == m.passwordAdapter {
				return m, m.focusManager.Next()
			}
		}
	case connectionSavedMsg:
		// When a connection is saved, we pop back to the list view
		return m, func() tea.Msg { return PopMsg{} }
	}

	newFocusable, cmd := m.focusManager.Update(msg)
	cmds = append(cmds, cmd)

	if ta, ok := newFocusable.(*TextInput); ok {
		if ta == m.ssidAdapter {
			m.ssidInput = ta.Model
		} else if ta == m.passwordAdapter {
			m.passwordInput = ta.Model
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *EditModel) View() string {
	var s strings.Builder
	s.WriteString(fmt.Sprintf("\n%s\n\n", "Wi-Fi Connection"))

	isNew := m.connection.SSID == ""
	if !isNew {
		var details strings.Builder
		details.WriteString(fmt.Sprintf("SSID: %s\n", m.connection.SSID))
		var security string
		switch m.connection.Security {
		case wifi.SecurityOpen:
			security = "Open"
		case wifi.SecurityWEP:
			security = "WEP"
		case wifi.SecurityWPA:
			security = "WPA/WPA2"
		default:
			if m.connection.IsSecure {
				security = "Secure"
			} else {
				security = "Open"
			}
		}
		details.WriteString(fmt.Sprintf("Security: %s\n", security))
		if m.connection.Strength > 0 {
			details.WriteString(fmt.Sprintf("Signal: %d%%\n", m.connection.Strength))
		}
		if m.connection.IsKnown && m.connection.LastConnected != nil {
			details.WriteString(fmt.Sprintf("Last connected: \n  %s (%s)\n", m.connection.LastConnected.Format(time.DateTime), helpers.FormatDuration(*m.connection.LastConnected)))
		}
		s.WriteString(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Width(50).Render(details.String()))
		s.WriteString("\n\n")
	}

	for _, item := range m.focusManager.items {
		s.WriteString(item.View())
		s.WriteString("\n\n")
	}

	s.WriteString("\n\n(tab to switch fields, arrows to navigate, enter to select)")

	if m.connection.IsKnown {
		password := m.passwordAdapter.Model.Value()
		if m.passwordRevealed && password != "" {
			qrCodeString, err := qrwifi.GenerateWifiQRCode(m.connection.SSID, password, m.connection.IsSecure, m.connection.IsHidden)
			if err == nil {
				s.WriteString("\n\n")
				s.WriteString(qrCodeString)
			}
		}
	}

	return s.String()
}
