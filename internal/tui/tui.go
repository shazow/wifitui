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
}

// NewModel creates the starting state of our application
func NewModel(b wifi.Backend) (*model, error) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(CurrentTheme.Primary)

	listModel := NewListModel()

	m := model{
		stack:   NewComponentStack(listModel),
		spinner: s,
		backend: b,
	}
	return &m, nil
}

type radioEnabledMsg struct{}
type updateConnectionMsg struct {
	item connectionItem
	wifi.UpdateOptions
}

// Init is the first command that is run when the program starts
func (m *model) Init() tea.Cmd {
	var cmds []tea.Cmd
	// Call OnEnter on the initial component
	if enterable, ok := m.stack.components[0].(Enterable); ok {
		cmds = append(cmds, enterable.OnEnter())
	}

	cmds = append(cmds, m.spinner.Tick)
	return tea.Batch(cmds...)
}

// Update handles all incoming messages and updates the model accordingly
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Global messages that are not passed to components
	switch msg := msg.(type) {
	case statusMsg:
		m.statusMessage = msg.status
		m.loading = msg.loading
		return m, nil
	case popViewMsg:
		cmd := m.stack.Pop()
		return m, cmd
	case radioEnabledMsg:
		cmd := m.stack.Pop() // Pop the disabled view
		return m, cmd
	case errorMsg:
		m.loading = false
		m.statusMessage = ""
		// If we're in the Edit view, pass the error up so it can be displayed.
		if _, ok := m.stack.Top().(*EditModel); ok {
			return m, func() tea.Msg {
				return connectionFailedMsg{err: msg.err}
			}
		}

		if errors.Is(msg.err, wifi.ErrWirelessDisabled) {
			disabledModel := NewWirelessDisabledModel(m.backend)
			cmd := m.stack.Push(disabledModel)
			return m, cmd
		}
		errorModel := NewErrorModel(msg.err)
		cmd := m.stack.Push(errorModel)
		return m, cmd
	case scanMsg:
		if m.loading {
			// Skip additional scans while we're still loading
			return m, nil
		}
		return m, tea.Batch(
			func() tea.Msg { return statusMsg{status: "Scanning for networks...", loading: true} },
			func() tea.Msg {
				connections, err := m.backend.BuildNetworkList(true)
				if err != nil {
					return errorMsg{err}
				}
				wifi.SortConnections(connections)
				return scanFinishedMsg(connections)
			},
		)
	case connectMsg:
		var batch []tea.Cmd = []tea.Cmd{
			func() tea.Msg {
				return statusMsg{status: fmt.Sprintf("Connecting to %q...", msg.item.SSID), loading: true}
			},
		}
		if msg.autoConnect != msg.item.AutoConnect {
			batch = append(batch, func() tea.Msg {
				err := m.backend.UpdateConnection(msg.item.SSID, wifi.UpdateOptions{AutoConnect: &msg.autoConnect})
				if err != nil {
					return errorMsg{fmt.Errorf("failed to update autoconnect: %w", err)}
				}
				return connectionSavedMsg{}
			})
		}
		batch = append(batch, func() tea.Msg {
			err := m.backend.ActivateConnection(msg.item.SSID, msg.bssid)
			if err != nil {
				return errorMsg{fmt.Errorf("failed to activate connection: %w", err)}
			}
			return connectionSavedMsg{} // Re-use this to trigger a refresh
		})
		return m, tea.Batch(batch...)
	case joinNetworkMsg:
		return m, tea.Batch(
			func() tea.Msg { return statusMsg{status: fmt.Sprintf("Joining %q...", msg.ssid), loading: true} },
			func() tea.Msg {
				err := m.backend.JoinNetwork(msg.ssid, msg.password, msg.security, msg.isHidden, msg.bssid)
				if err != nil {
					return errorMsg{fmt.Errorf("failed to join network: %w", err)}
				}
				return connectionSavedMsg{}
			},
		)
	case loadSecretsMsg:
		return m, tea.Batch(
			func() tea.Msg { return statusMsg{status: fmt.Sprintf("Loading %q...", msg.item.SSID), loading: true} },
			func() tea.Msg {
				secret, err := m.backend.GetSecrets(msg.item.SSID)
				if err != nil {
					return errorMsg{fmt.Errorf("failed to get secrets: %w", err)}
				}
				return secretsLoadedMsg{item: msg.item, secret: secret}
			},
		)
	case updateConnectionMsg:
		return m, tea.Batch(
			func() tea.Msg { return statusMsg{status: fmt.Sprintf("Saving %q...", msg.item.SSID), loading: true} },
			func() tea.Msg {
				err := m.backend.UpdateConnection(msg.item.SSID, msg.UpdateOptions)
				if err != nil {
					return errorMsg{fmt.Errorf("failed to update connection: %w", err)}
				}
				return connectionSavedMsg{}
			},
		)
	case forgetNetworkMsg:
		return m, tea.Batch(
			func() tea.Msg {
				return statusMsg{status: fmt.Sprintf("Forgetting %q...", msg.item.SSID), loading: true}
			},
			func() tea.Msg {
				err := m.backend.ForgetNetwork(msg.item.SSID)
				if err != nil {
					return errorMsg{fmt.Errorf("failed to forget connection: %w", err)}
				}
				return connectionSavedMsg{forgottenSSID: msg.item.SSID} // Re-use this to trigger a refresh
			},
		)
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// If a text input is focused, don't process global keybindings.
		if m.stack.IsConsumingInput() {
			break
		}

		switch msg.String() {
		case "r":
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

			return m, tea.Batch(
				func() tea.Msg { return statusMsg{status: "Disabling WiFi radio...", loading: true} },
				func() tea.Msg {
					err := m.backend.SetWireless(false)
					if err != nil {
						return errorMsg{err}
					}
					// By returning this error, we trigger the main loop to push the WirelessDisabledModel.
					return errorMsg{wifi.ErrWirelessDisabled}
				},
			)
		}
	case connectionsLoadedMsg, secretsLoadedMsg, scanFinishedMsg:
		// Clear loading status
		cmds = append(cmds, func() tea.Msg { return statusMsg{} })
	case connectionSavedMsg:
		return m, tea.Batch(
			func() tea.Msg { return statusMsg{status: "Saved. Refreshing...", loading: true} },
			func() tea.Msg { return popViewMsg{} },
			func() tea.Msg {
				connections, err := m.backend.BuildNetworkList(false)
				if err != nil {
					return errorMsg{err}
				}
				if msg.forgottenSSID != "" {
					// Filter out the forgotten SSID from connections if it is still present
					// This handles the race condition where the backend might return the forgotten network
					for i := range connections {
						if connections[i].SSID == msg.forgottenSSID {
							connections[i].IsKnown = false
							connections[i].AutoConnect = false
						}
					}
				}
				wifi.SortConnections(connections)
				return connectionsLoadedMsg(connections)
			},
		)

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
	s.WriteString("\n")

	if m.loading {
		s.WriteString(m.spinner.View())
	}
	s.WriteString(lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Render(m.statusMessage))

	return s.String()
}
