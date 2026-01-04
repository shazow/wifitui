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

type startForgettingMsg struct{}

type connectionFailedMsg struct{ err error }

type EditModel struct {
	focusManager        *FocusManager
	ssidAdapter         *TextInput
	passwordAdapter     *TextInput
	securityGroup       *ChoiceComponent
	autoConnectCheckbox *Checkbox
	buttonGroup         *MultiButtonComponent
	passwordRevealed    bool
	isForgetting        bool
	secretsLoaded       bool
	selectedItem        connectionItem
}

func NewEditModel(item *connectionItem) *EditModel {
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
	passwordInput.Width = 45
	passwordInput.EchoMode = textinput.EchoPassword

	m := EditModel{selectedItem: *item}

	m.ssidAdapter = &TextInput{
		Model: ssidInput,
		label: "SSID:",
	}
	onPasswordFocus := func(ti *textinput.Model) tea.Cmd {
		ti.EchoMode = textinput.EchoNormal
		m.passwordRevealed = true
		// Load secrets on first focus for known networks
		if m.selectedItem.IsKnown && !m.secretsLoaded {
			return func() tea.Msg { return loadSecretsMsg{item: m.selectedItem} }
		}
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
						ssid:     m.ssidAdapter.Model.Value(),
						password: m.passwordAdapter.Model.Value(),
						security: wifi.SecurityType(m.securityGroup.Selected()),
						isHidden: true,
					}
				}
			case 1: // Cancel
				return func() tea.Msg { return popViewMsg{} }
			}
		} else if m.selectedItem.IsKnown {
			switch index {
			case 0: // Connect
				return func() tea.Msg {
					autoConnect := m.autoConnectCheckbox.Checked()
					return connectMsg{
						item:        m.selectedItem,
						autoConnect: autoConnect,
					}
				}
			case 1: // Save
				return func() tea.Msg {
					newPassword := m.passwordAdapter.Model.Value()
					autoConnect := m.autoConnectCheckbox.Checked()
					return updateConnectionMsg{
						item: m.selectedItem,
						UpdateOptions: wifi.UpdateOptions{
							Password:    &newPassword,
							AutoConnect: &autoConnect,
						},
					}
				}
			case 2: // Forget
				return func() tea.Msg { return startForgettingMsg{} }
			case 3: // Cancel
				return func() tea.Msg { return popViewMsg{} }
			}
		} else { // Unknown network
			switch index {
			case 0: // Join
				return func() tea.Msg {
					return joinNetworkMsg{
						ssid:     m.selectedItem.SSID,
						password: m.passwordAdapter.Model.Value(),
						security: m.selectedItem.Security,
						isHidden: m.selectedItem.IsHidden,
					}
				}
			case 1: // Cancel
				return func() tea.Msg { return popViewMsg{} }
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
	return &m
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

func (m *EditModel) Update(msg tea.Msg) (Component, tea.Cmd) {
	var cmds []tea.Cmd

	if m.isForgetting {
		finished, cmd := forgetHandler(msg, m.selectedItem)
		if finished {
			m.isForgetting = false
			return m, cmd
		}
		// Don't consume other events if we're not finished
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		newWidth := int(float64(msg.Width) * 0.8)
		if newWidth > 80 {
			newWidth = 80
		}
		m.ssidAdapter.Model.Width = newWidth
		m.passwordAdapter.Model.Width = newWidth
		return m, nil
	case secretsLoadedMsg:
		m.secretsLoaded = true
		m.SetPassword(msg.secret)
		return m, nil
	case startForgettingMsg:
		m.isForgetting = true
		return m, nil
	case connectionFailedMsg:
		return m, tea.Batch(
			m.focusManager.SetFocus(m.passwordAdapter),
			func() tea.Msg {
				return statusMsg{status: msg.err.Error()}
			},
		)
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			return m, m.focusManager.Next()
		case "shift+tab":
			return m, m.focusManager.Prev()
		case "esc":
			return m, func() tea.Msg { return popViewMsg{} }
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

func (m *EditModel) IsConsumingInput() bool {
	return m.ssidAdapter.Model.Focused() || m.passwordAdapter.Model.Focused()
}

func (m *EditModel) View() string {
	var s strings.Builder
	s.WriteString("\n  ")
	s.WriteString(lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Bold(true).Render("WiFi Connection"))
	s.WriteString("\n")

	formatLabel := lipgloss.NewStyle().Foreground(CurrentTheme.Subtle)

	isNew := m.selectedItem.SSID == ""
	if !isNew {
		var details strings.Builder
		details.WriteString(formatLabel.Render("SSID: "))
		details.WriteString(fmt.Sprintf("%s\n", m.selectedItem.SSID))
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
		details.WriteString(formatLabel.Render("Security: "))
		details.WriteString(fmt.Sprintf("%s", security))
		if m.selectedItem.Strength() > 0 {
			details.WriteString("\n")
			details.WriteString(formatLabel.Render("Signal: "))
			details.WriteString(CurrentTheme.FormatSignalStrength(m.selectedItem.Strength()))
		}
		if len(m.selectedItem.AccessPoints) > 0 {
			details.WriteString("\n\n")
			details.WriteString(formatLabel.Render("Access Points:"))
			for _, ap := range m.selectedItem.AccessPoints {
				bssid := ap.BSSID
				if bssid == "" {
					bssid = "(unknown)"
				}
				details.WriteString("\n  ")
				details.WriteString(CurrentTheme.FormatSignalStrength(ap.Strength))
				details.WriteString(fmt.Sprintf("  %dMHz  %s", ap.Frequency, bssid))
			}
		}
		if m.selectedItem.IsKnown && m.selectedItem.LastConnected != nil {
			details.WriteString("\n\n")
			details.WriteString(formatLabel.Render("Last Connected:"))
			details.WriteString(fmt.Sprintf("\n  %s (%s)", m.selectedItem.LastConnected.Format(time.DateTime), helpers.FormatDuration(*m.selectedItem.LastConnected)))
		}

		s.WriteString(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Width(50).Render(details.String()))
		s.WriteString("\n\n")
	}

	for _, item := range m.focusManager.items {
		s.WriteString(item.View())
		s.WriteString("\n\n")
	}

	if m.isForgetting {
		s.WriteString(lipgloss.NewStyle().Foreground(CurrentTheme.Error).Render("Forget this network? (Y/n)"))
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

// forgetHandler handles the key presses for the forget confirmation.
// It returns whether the forget flow is finished, and a command to execute.
func forgetHandler(msg tea.Msg, item connectionItem) (finished bool, cmd tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "enter":
			return true, func() tea.Msg {
				return forgetNetworkMsg{item: item}
			}
		case "n", "esc":
			return true, nil
		}
	}
	return false, nil
}
