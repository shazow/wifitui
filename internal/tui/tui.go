package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shazow/wifitui/wifi"
)

type scheduledScanMsg struct{}

func scheduleScan(interval time.Duration) tea.Cmd {
	return func() tea.Msg {
		<-time.After(interval)
		return scheduledScanMsg{}
	}
}

var scanIntervals = []time.Duration{
	1 * time.Second,
	2 * time.Second,
	5 * time.Second,
	10 * time.Second,
}

// The main model for our TUI application
type model struct {
	componentStack []Component

	spinner           spinner.Model
	backend           wifi.Backend
	loading           bool
	statusMessage     string
	width, height     int
	isScanningActive  bool
	scanIntervalIndex int
}

// NewModel creates the starting state of our application
func NewModel(b wifi.Backend) (*model, error) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(CurrentTheme.Primary)

	listModel := NewListModel()

	m := model{
		componentStack:    []Component{listModel},
		spinner:           s,
		backend:           b,
		loading:           true,
		statusMessage:     "Loading connections...",
		isScanningActive:  true,
		scanIntervalIndex: 0,
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
		if len(m.componentStack) > 1 {
			m.componentStack = m.componentStack[:len(m.componentStack)-1]
		}
		return m, nil
	case errorMsg:
		m.loading = false
		if errors.Is(msg.err, wifi.ErrWirelessDisabled) {
			m.statusMessage = ""
			disabledModel := NewWirelessDisabledModel(m.backend)
			m.componentStack = append(m.componentStack, disabledModel)
			return m, nil
		}
		errorModel := NewErrorModel(msg.err)
		m.componentStack = append(m.componentStack, errorModel)
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
		if msg.String() == "r" {
			// This is a global keybinding to toggle the radio
			m.loading = true
			m.statusMessage = "Toggling Wi-Fi radio..."

			// Reset scan interval if we're enabling the radio
			enabled, err := m.backend.IsWirelessEnabled()
			if err != nil {
				return m, func() tea.Msg { return errorMsg{err} }
			}
			if !enabled {
				m.scanIntervalIndex = 0
			}

			var cmds []tea.Cmd
			// If we are in the disabled view, pop it.
			if _, ok := m.componentStack[len(m.componentStack)-1].(*WirelessDisabledModel); ok {
				cmds = append(cmds, func() tea.Msg { return popViewMsg{} })
			}

			toggleCmd := func() tea.Msg {
				err := m.backend.SetWireless(!enabled)
				if err != nil {
					return errorMsg{err}
				}
				return scanMsg{}
			}
			cmds = append(cmds, toggleCmd)

			return m, tea.Batch(cmds...)
		}
		if msg.String() == "S" {
			m.isScanningActive = !m.isScanningActive
			if m.isScanningActive {
				m.statusMessage = "Active scanning enabled."
				m.scanIntervalIndex = 0
				return m, scheduleScan(scanIntervals[m.scanIntervalIndex])
			}
			m.statusMessage = "Active scanning disabled."
			return m, nil
		}
	case scheduledScanMsg:
		if m.isScanningActive {
			return m, func() tea.Msg { return scanMsg{} }
		}
		return m, nil
	case scanFinishedMsg:
		m.loading = false
		m.statusMessage = ""
		if m.isScanningActive {
			if m.scanIntervalIndex < len(scanIntervals)-1 {
				m.scanIntervalIndex++
			}
			return m, scheduleScan(scanIntervals[m.scanIntervalIndex])
		}
		return m, nil
	// Clear loading status on some messages
	case connectionsLoadedMsg:
		m.loading = false
		m.statusMessage = ""
		if m.isScanningActive {
			return m, scheduleScan(scanIntervals[m.scanIntervalIndex])
		}
		return m, nil
	case secretsLoadedMsg:
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

	var footer []string
	if m.loading {
		footer = append(footer, fmt.Sprintf("%s %s", m.spinner.View(), lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Render(m.statusMessage)))
	} else if m.statusMessage != "" {
		footer = append(footer, lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Render(m.statusMessage))
	}

	scanStatus := "Off"
	if m.isScanningActive {
		scanStatus = "On"
	}
	footer = append(footer, fmt.Sprintf("[S] Active Scan: %s", scanStatus))

	if len(footer) > 0 {
		s.WriteString("\n\n")
		s.WriteString(strings.Join(footer, " • "))
	}

	return s.String()
}
