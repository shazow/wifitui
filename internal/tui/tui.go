package tui

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	wifilog "github.com/shazow/wifitui/internal/log"
	"github.com/shazow/wifitui/wifi"
)

// The main model for our TUI application
type model struct {
	componentStack []Component

	spinner   spinner.Model
	backend   wifi.Backend
	loading   bool
	latestLog slog.Record
	width     int
	height    int
}

// NewModel creates the starting state of our application
func NewModel(b wifi.Backend) (*model, error) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(CurrentTheme.Primary)

	listModel := NewListModel()

	m := model{
		componentStack: []Component{listModel},
		spinner:        s,
		backend:        b,
		loading:        true,
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
	case wifilog.LogMsg:
		m.latestLog = slog.Record(msg)
		if m.latestLog.Level >= slog.LevelInfo {
			m.loading = false
		}
		return m, nil
	case popViewMsg:
		if len(m.componentStack) > 1 {
			m.componentStack = m.componentStack[:len(m.componentStack)-1]
		}
		return m, nil
	case errorMsg:
		m.loading = false
		errorModel := NewErrorModel(msg.err)
		m.componentStack = append(m.componentStack, errorModel)
		return m, nil
	case scanMsg:
		m.loading = true
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
		return m, func() tea.Msg {
			err := m.backend.JoinNetwork(msg.ssid, msg.password, msg.security, msg.isHidden)
			if err != nil {
				return errorMsg{fmt.Errorf("failed to join network: %w", err)}
			}
			return connectionSavedMsg{}
		}
	case loadSecretsMsg:
		m.loading = true
		return m, func() tea.Msg {
			secret, err := m.backend.GetSecrets(msg.item.SSID)
			if err != nil {
				return errorMsg{fmt.Errorf("failed to get secrets: %w", err)}
			}
			return secretsLoadedMsg{item: msg.item, secret: secret}
		}
	case updateSecretMsg:
		m.loading = true
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
		return m, func() tea.Msg {
			err := m.backend.ForgetNetwork(msg.item.SSID)
			if err != nil {
				return errorMsg{fmt.Errorf("failed to forget connection: %w", err)}
			}
			return connectionSavedMsg{} // Re-use this to trigger a refresh
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "l":
			logModel := NewLogViewModel()
			m.componentStack = append(m.componentStack, logModel)
		}
	// Clear loading status on some messages
	case connectionsLoadedMsg, scanFinishedMsg, secretsLoadedMsg:
		m.loading = false
	case connectionSavedMsg:
		m.loading = true // Show loading while we refresh
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
	top := m.componentStack[len(m.componentStack)-1]
	newComp, cmd := top.Update(msg)
	cmds = append(cmds, cmd)

	if newComp != top {
		m.componentStack = append(m.componentStack, newComp)
	}

	// Spinner update
	var spinnerCmd tea.Cmd
	m.spinner, spinnerCmd = m.spinner.Update(msg)
	cmds = append(cmds, spinnerCmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI based on the current model state
func (m model) View() string {
	var s strings.Builder

	if len(m.componentStack) > 0 {
		top := m.componentStack[len(m.componentStack)-1]
		s.WriteString(top.View())
	}

	// Status bar
	var status string
	if m.loading {
		status = m.spinner.View()
	}

	if !m.latestLog.Time.IsZero() {
		var style lipgloss.Style
		switch m.latestLog.Level {
		case slog.LevelError:
			style = lipgloss.NewStyle().Foreground(CurrentTheme.Error)
		default:
			style = lipgloss.NewStyle().Foreground(CurrentTheme.Normal)
		}
		status += style.Render(m.latestLog.Message)
	}
	if status != "" {
		s.WriteString("\n\n" + status)
	}

	return s.String()
}
