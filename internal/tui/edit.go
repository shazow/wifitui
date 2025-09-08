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
	focusManager      *FocusManager
	ssidAdapter       *TextInput
	passwordAdapter   *TextInput
	securityGroup     *ChoiceComponent
	autoConnectCheckbox *Checkbox
	buttonGroup       *MultiButtonComponent
	passwordRevealed  bool
	selectedItem      connectionItem
}

func NewEditModel(item *connectionItem) EditModel {
	if item == nil {
		item = &connectionItem{}
	}
	isNew := item.SSID == ""
	var items []Focusable

	ssidInput := textinput.New()
	ssidInput.Focus()
	ssidInput.CharLimit = 32
	ssidInput.Width = 30
	if !isNew {
		ssidInput.SetValue(item.SSID)
	}

	passwordInput := textinput.New()
	passwordInput.Focus()
	passwordInput.CharLimit = 64
	passwordInput.Width = 30
	passwordInput.EchoMode = textinput.EchoPassword

	m := EditModel{selectedItem: *item}

	m.ssidAdapter = &TextInput{
		Model: ssidInput,
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
		Model:   passwordInput,
		label:   "Passphrase:",
		OnFocus: onPasswordFocus,
		OnBlur:  onPasswordBlur,
	}

	if isNew {
		items = append(items, m.ssidAdapter)
	}

	security := m.selectedItem.Security
	if isNew {
		m.securityGroup = NewChoiceComponent("Security:", []string{"Open", "WEP", "WPA/WPA2"})
		security = wifi.SecurityType(m.securityGroup.Selected())
	}

	if ShouldDisplayPasswordField(security) {
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
		isNew := m.selectedItem.SSID == ""
		if isNew {
			switch index {
			case 0: // Join
				return func() tea.Msg {
					return joinNetworkMsg{
						ssid:       m.ssidAdapter.Model.Value(),
						password:   m.passwordAdapter.Model.Value(),
						security:   wifi.SecurityType(m.securityGroup.Selected()),
						isHidden:   true,
					}
				}
			case 1: // Cancel
				return func() tea.Msg { return changeViewMsg(stateListView) }
			}
		} else if m.selectedItem.IsKnown {
			switch index {
			case 0: // Connect
				return func() tea.Msg {
					return connectMsg{
						item: m.selectedItem,
						autoConnect: m.autoConnectCheckbox.Checked(),
					}
				}
			case 1: // Save
				return func() tea.Msg {
					return updateSecretMsg{
						item: m.selectedItem,
						newPassword: m.passwordAdapter.Model.Value(),
						autoConnect: m.autoConnectCheckbox.Checked(),
					}
				}
			case 2: // Forget
				return func() tea.Msg { return showForgetViewMsg{item: m.selectedItem} }
			case 3: // Cancel
				return func() tea.Msg { return changeViewMsg(stateListView) }
			}
		} else { // Unknown network
			switch index {
			case 0: // Join
				return func() tea.Msg {
					return joinNetworkMsg{
						ssid:       m.selectedItem.SSID,
						password:   m.passwordAdapter.Model.Value(),
						security:   m.selectedItem.Security,
						isHidden:   m.selectedItem.IsHidden,
					}
				}
			case 1: // Cancel
				return func() tea.Msg { return changeViewMsg(stateListView) }
			}
		}
		return nil
	}
	m.buttonGroup = NewMultiButtonComponent(buttons, buttonAction)
	items = append(items, m.buttonGroup)

	m.focusManager = NewFocusManager(items...)

	if m.selectedItem.IsKnown {
		m.focusManager.SetFocus(m.buttonGroup)
	} else {
		m.focusManager.Focus()
	}
	return m
}

func (m *EditModel) SetPassword(password string) {
	m.passwordAdapter.Model.SetValue(password)
	m.passwordAdapter.Model.CursorEnd()
	if password != "" {
		m.passwordAdapter.Model.EchoMode = textinput.EchoPassword
		m.passwordAdapter.Model.Blur()
	} else {
		m.passwordAdapter.Model.EchoMode = textinput.EchoNormal
	}
}

func (m EditModel) Init() tea.Cmd {
	return nil
}

func (m EditModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			return m, m.focusManager.Next()
		case "shift+tab":
			return m, m.focusManager.Prev()
		case "esc":
			return m, func() tea.Msg { return changeViewMsg(stateListView) }
		case "enter":
			if m.focusManager.Focused() == m.passwordAdapter {
				return m, m.focusManager.Next()
			}
		}
	}

	newFocusable, cmd := m.focusManager.Update(msg)
	cmds = append(cmds, cmd)

	// FIXME: This is a hack to update the underlying models
	if ta, ok := newFocusable.(*TextInput); ok {
		if ta.label == "SSID:" {
			m.ssidAdapter.Model = ta.Model
		} else if ta.label == "Passphrase:" {
			m.passwordAdapter.Model = ta.Model
		}
	}

	return m, tea.Batch(cmds...)
}

func (m EditModel) View() string {
	var s strings.Builder
	s.WriteString(fmt.Sprintf("\n%s\n\n", "Wi-Fi Connection"))

	isNew := m.selectedItem.SSID == ""
	if !isNew {
		var details strings.Builder
		details.WriteString(fmt.Sprintf("SSID: %s\n", m.selectedItem.SSID))
		var security string
		switch m.selectedItem.Security {
		case wifi.SecurityOpen:
			security = "Open"
		case wifi.SecurityWEP:
			security = "WEP"
		case wifi.SecurityWPA:
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
		s.WriteString(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Width(50).Render(details.String()))
		s.WriteString("\n\n")
	}

	for _, item := range m.focusManager.items {
		s.WriteString(item.View())
		s.WriteString("\n\n")
	}

	s.WriteString("\n\n(tab to switch fields, arrows to navigate, enter to select)")

	if m.selectedItem.IsKnown {
		password := m.passwordAdapter.Model.Value()
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

func ShouldDisplayPasswordField(security wifi.SecurityType) bool {
	return security != wifi.SecurityOpen
}
