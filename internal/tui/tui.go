package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shazow/wifitui/internal/helpers"
	"github.com/shazow/wifitui/wifi"
)

// The main model for our TUI application
type model struct {
	stack *ComponentStack

	spinner       spinner.Model
	backend       wifi.Backend
	loading       bool
	statusMessage string

	listModel *ListModel

	networkChangeCancel   context.CancelFunc
	networkRefreshPending bool
}

// NetworkManager can send several AP/device signals for one scan update.
// Debounce them so AP property churn triggers one cached refresh instead of
// repeated D-Bus list reads and redraws.
const networkChangeDebounce = 150 * time.Millisecond

// TODO: Consider adding a general change-watching interface to wifi.Backend so
// every backend can expose network-change hints through one TUI code path.
type networkChangeWatcher interface {
	WatchNetworkChanges(context.Context) (<-chan struct{}, error)
}

// NewModel creates the starting state of our application
func NewModel(b wifi.Backend) (*model, error) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(CurrentTheme.Primary)

	window := &WindowState{}
	listModel := NewListModelWithWindow(window)

	m := model{
		stack:     NewComponentStack(listModel),
		spinner:   s,
		backend:   b,
		listModel: listModel,
	}
	return &m, nil
}

type radioEnabledMsg struct{}
type networkWatchStartedMsg struct {
	changes <-chan struct{}
	cancel  context.CancelFunc
}
type networkChangedMsg struct {
	changes <-chan struct{}
}
type networkDebouncedMsg struct{}
type updateNetworkMsg struct {
	item networkItem
	wifi.UpdateOptions
}

// Init is the first command that is run when the program starts
func (m *model) Init() tea.Cmd {
	var cmds []tea.Cmd
	// Call OnEnter on the initial component
	if enterable, ok := m.stack.components[0].(Enterable); ok {
		cmds = append(cmds, enterable.OnEnter())
	}

	cmds = append(cmds, startNetworkChangeWatcher(m.backend))
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
	case networkWatchStartedMsg:
		m.networkChangeCancel = msg.cancel
		return m, waitForNetworkChange(msg.changes)
	case networkChangedMsg:
		cmds = append(cmds, waitForNetworkChange(msg.changes))
		if !m.networkRefreshPending {
			m.networkRefreshPending = true
			cmds = append(cmds, tea.Tick(networkChangeDebounce, func(time.Time) tea.Msg {
				return networkDebouncedMsg{}
			}))
		}
		return m, tea.Batch(cmds...)
	case networkDebouncedMsg:
		m.networkRefreshPending = false
		if m.loading {
			m.networkRefreshPending = true
			return m, tea.Tick(networkChangeDebounce, func(time.Time) tea.Msg {
				return networkDebouncedMsg{}
			})
		}
		return m, func() tea.Msg {
			result, err := m.backend.ListNetworks(wifi.ScanNever)
			if err != nil {
				return errorMsg{err}
			}
			networks := result.Networks
			wifi.SortNetworks(networks)
			return networksLoadedMsg(networks)
		}
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
		m.statusMessage = "Scanning for networks..."
		m.loading = true
		return m, func() tea.Msg {
			result, err := m.backend.ListNetworks(msg.mode)
			if err != nil {
				return errorMsg{err}
			}
			networks := result.Networks
			wifi.SortNetworks(networks)
			return scanFinishedMsg{
				networks: networks,
				scanErr:  result.ScanError,
			}
		}
	case connectMsg:
		var batch []tea.Cmd = []tea.Cmd{
			func() tea.Msg {
				return statusMsg{status: fmt.Sprintf("Connecting to %q...", msg.item.SSID), loading: true}
			},
		}
		if msg.autoConnect != msg.item.AutoConnect {
			batch = append(batch, func() tea.Msg {
				err := m.backend.UpdateNetwork(msg.item.SSID, wifi.UpdateOptions{AutoConnect: &msg.autoConnect})
				if err != nil {
					return errorMsg{fmt.Errorf("failed to update autoconnect: %w", err)}
				}
				return networkSavedMsg{}
			})
		}
		batch = append(batch, func() tea.Msg {
			err := m.backend.ActivateNetwork(msg.item.SSID)
			if err != nil {
				return errorMsg{fmt.Errorf("failed to activate connection: %w", err)}
			}
			return networkSavedMsg{} // Re-use this to trigger a refresh
		})
		return m, tea.Batch(batch...)
	case joinNetworkMsg:
		return m, tea.Batch(
			func() tea.Msg { return statusMsg{status: fmt.Sprintf("Joining %q...", msg.ssid), loading: true} },
			func() tea.Msg {
				err := m.backend.JoinNetwork(msg.ssid, msg.password, msg.security, msg.isHidden)
				if err != nil {
					return errorMsg{fmt.Errorf("failed to join network: %w", err)}
				}
				return networkSavedMsg{}
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
	case updateNetworkMsg:
		return m, tea.Batch(
			func() tea.Msg { return statusMsg{status: fmt.Sprintf("Saving %q...", msg.item.SSID), loading: true} },
			func() tea.Msg {
				err := m.backend.UpdateNetwork(msg.item.SSID, msg.UpdateOptions)
				if err != nil {
					return errorMsg{fmt.Errorf("failed to update connection: %w", err)}
				}
				return networkSavedMsg{}
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
				return networkSavedMsg{forgottenSSID: msg.item.SSID} // Re-use this to trigger a refresh
			},
		)
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			if m.networkChangeCancel != nil {
				m.networkChangeCancel()
			}
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
	case secretsLoadedMsg:
		// Clear loading status
		cmds = append(cmds, func() tea.Msg { return statusMsg{} })
	case networksLoadedMsg:
		// Clear loading status
		cmds = append(cmds, func() tea.Msg { return statusMsg{} })
	case scanFinishedMsg:
		m.loading = false
		m.statusMessage = ""
		if msg.scanErr != nil {
			m.statusMessage = fmt.Sprintf("Scan failed: %s", helpers.FormatScanFailure(msg.scanErr))
		}
	case networkSavedMsg:
		return m, tea.Batch(
			func() tea.Msg { return statusMsg{status: "Saved. Refreshing...", loading: true} },
			func() tea.Msg { return popViewMsg{} },
			func() tea.Msg {
				result, err := m.backend.ListNetworks(wifi.ScanNever)
				if err != nil {
					return errorMsg{err}
				}
				networks := result.Networks
				if msg.forgottenSSID != "" {
					// Remove the forgotten network from the list.
					// If visible, mark as not known; if not visible, remove entirely.
					filtered := networks[:0]
					for _, conn := range networks {
						if conn.SSID == msg.forgottenSSID {
							if conn.IsVisible {
								conn.IsKnown = false
								conn.AutoConnect = false
								filtered = append(filtered, conn)
							}
							// If not visible, don't add to filtered list (removes it)
						} else {
							filtered = append(filtered, conn)
						}
					}
					networks = filtered
				}
				wifi.SortNetworks(networks)
				return networksLoadedMsg(networks)
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

func startNetworkChangeWatcher(b wifi.Backend) tea.Cmd {
	watcher, ok := b.(networkChangeWatcher)
	if !ok {
		return nil
	}

	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		changes, err := watcher.WatchNetworkChanges(ctx)
		if err != nil {
			cancel()
			return nil
		}
		return networkWatchStartedMsg{
			changes: changes,
			cancel:  cancel,
		}
	}
}

func waitForNetworkChange(changes <-chan struct{}) tea.Cmd {
	if changes == nil {
		return nil
	}
	return func() tea.Msg {
		_, ok := <-changes
		if !ok {
			return nil
		}
		return networkChangedMsg{changes: changes}
	}
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
