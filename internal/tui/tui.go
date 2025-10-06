package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shazow/wifitui/wifi"
)

// The main model for our TUI application
type model struct {
	stack *ComponentStack

	spinner       spinner.Model
	backend       wifi.Backend
	loading       bool
	statusMessage string
	width, height int
}

// NewModel creates the starting state of our application
func NewModel(b wifi.Backend) (*model, error) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(CurrentTheme.Primary)

	listModel := NewListModel()

	m := model{
		stack:         NewComponentStack(listModel),
		spinner:       s,
		backend:       b,
		loading:       true,
		statusMessage: "Loading connections...",
	}
	return &m, nil
}

// Init is the first command that is run when the program starts
func (m *model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		connections, err := m.backend.BuildNetworkList(false)
		if err != nil {
			return errorMsg{err}
		}
		wifi.SortConnections(connections)
		return connectionsLoadedMsg(connections)
	})
}

// Update handles all incoming messages and updates the model accordingly
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Global messages that are not passed to components
	switch msg := msg.(type) {
	case popViewMsg:
		m.stack.Pop()
		return m, nil
	case errorMsg:
		m.loading = false
		if errors.Is(msg.err, wifi.ErrWirelessDisabled) {
			disabledModel := NewWirelessDisabledModel(m.backend)
			m.stack.Push(disabledModel)
			return m, nil
		}
		errorModel := NewErrorModel(msg.err)
		m.stack.Push(errorModel)
		return m, nil
	case scanMsg:
		m.loading = true
		m.statusMessage = "Scanning for networks..."
		return m, func() tea.Msg {
			connections, err := m.backend.BuildNetworkList(true)
			if err != nil {
				return errorMsg{err}
			}
			wifi.SortConnections(connections)
			return scanFinishedMsg(connections)
		}
	case connectMsg:
		m.loading = true
		m.statusMessage = fmt.Sprintf("Connecting to '%s'...", msg.item.SSID)
		var batch []tea.Cmd
		if msg.autoConnect != msg.item.AutoConnect {
			batch = append(batch, func() tea.Msg {
				err := m.backend.SetAutoConnect(msg.item.SSID, msg.autoConnect)
				if err != nil {
					return errorMsg{fmt.Errorf("failed to update autoconnect: %w", err)}
				}
				return connectionSavedMsg{}
			})
		}
		batch = append(batch, func() tea.Msg {
			err := m.backend.ActivateConnection(msg.item.SSID)
			if err != nil {
				return errorMsg{fmt.Errorf("failed to activate connection: %w", err)}
			}
			return connectionSavedMsg{} // Re-use this to trigger a refresh
		})
		return m, tea.Batch(batch...)
	case joinNetworkMsg:
		m.loading = true
		m.statusMessage = fmt.Sprintf("Joining '%s'...", msg.ssid)
		return m, func() tea.Msg {
			err := m.backend.JoinNetwork(msg.ssid, msg.password, msg.security, msg.isHidden)
			if err != nil {
				return errorMsg{fmt.Errorf("failed to join network: %w", err)}
			}
			return connectionSavedMsg{}
		}
	case loadSecretsMsg:
		m.loading = true
		m.statusMessage = fmt.Sprintf("Loading details for %s...", msg.item.SSID)
		return m, func() tea.Msg {
			secret, err := m.backend.GetSecrets(msg.item.SSID)
			if err != nil {
				return errorMsg{fmt.Errorf("failed to get secrets: %w", err)}
			}
			return secretsLoadedMsg{item: msg.item, secret: secret}
		}
	case updateSecretMsg:
		m.loading = true
		m.statusMessage = fmt.Sprintf("Saving settings for %s...", msg.item.SSID)
		var batch []tea.Cmd
		batch = append(batch, func() tea.Msg {
			err := m.backend.UpdateSecret(msg.item.SSID, msg.newPassword)
			if err != nil {
				return errorMsg{fmt.Errorf("failed to update connection: %w", err)}
			}
			return connectionSavedMsg{}
		})
		if msg.autoConnect != msg.item.AutoConnect {
			batch = append(batch, func() tea.Msg {
				err := m.backend.SetAutoConnect(msg.item.SSID, msg.autoConnect)
				if err != nil {
					return errorMsg{fmt.Errorf("failed to update autoconnect: %w", err)}
				}
				return connectionSavedMsg{}
			})
		}
		return m, tea.Batch(batch...)
	case forgetNetworkMsg:
		m.loading = true
		m.statusMessage = fmt.Sprintf("Forgetting '%s'...", msg.item.SSID)
		return m, func() tea.Msg {
			err := m.backend.ForgetNetwork(msg.item.SSID)
			if err != nil {
				return errorMsg{fmt.Errorf("failed to forget connection: %w", err)}
			}
			return connectionSavedMsg{} // Re-use this to trigger a refresh
		}
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// If a text input is focused, don't process global keybindings.
		if m.stack.IsConsumingInput() {
			break
		}

		if msg.String() == "r" {
			// This is a global keybinding to toggle the radio.
			// We only handle it here if the radio is currently enabled.
			// If it's disabled, we let the WirelessDisabledModel handle it.
			enabled, err := m.backend.IsWirelessEnabled()
			if err != nil {
				return m, func() tea.Msg { return errorMsg{err} }
			}
			if !enabled {
				// Let the component on the stack handle it.
				break
			}

			m.loading = true
			m.statusMessage = "Disabling Wi-Fi radio..."
			return m, func() tea.Msg {
				err := m.backend.SetWireless(false)
				if err != nil {
					return errorMsg{err}
				}
				// By returning this error, we trigger the main loop to push the WirelessDisabledModel.
				return errorMsg{wifi.ErrWirelessDisabled}
			}
		}
	// Clear loading status on some messages
	case connectionsLoadedMsg, scanFinishedMsg, secretsLoadedMsg:
		m.loading = false
		m.statusMessage = ""
	case connectionSavedMsg:
		m.loading = true // Show loading while we refresh
		m.statusMessage = "Successfully updated. Refreshing list..."
		return m, tea.Batch(func() tea.Msg { return popViewMsg{} }, func() tea.Msg {
			connections, err := m.backend.BuildNetworkList(false)
			if err != nil {
				return errorMsg{err}
			}
			wifi.SortConnections(connections)
			return connectionsLoadedMsg(connections)
		})

	}

	// Delegate to the component on the stack
	cmds = append(cmds, m.stack.Update(msg))

	// Spinner update
	var spinnerCmd tea.Cmd
	m.spinner, spinnerCmd = m.spinner.Update(msg)
	cmds = append(cmds, spinnerCmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI based on the current model state
func (m model) View() string {
	var s strings.Builder

	s.WriteString(m.stack.View())

	if m.loading {
		s.WriteString(fmt.Sprintf("\n\n%s %s", m.spinner.View(), lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Render(m.statusMessage)))
	} else if m.statusMessage != "" {
		s.WriteString(fmt.Sprintf("\n\n%s", lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Render(m.statusMessage)))
	}

	return s.String()
}
