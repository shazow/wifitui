package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/shazow/wifitui/internal/helpers"
	"github.com/shazow/wifitui/wifi"
)

// Component is the interface for a TUI component.
type Component interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Component, tea.Cmd)
	View() string
	Resize(width, height int)
}

// popViewMsg is a message to pop the current view from the stack.
type popViewMsg struct{}

// connectionItem holds the information for a single Wi-Fi connection in our list
type connectionItem struct {
	wifi.Connection
}

func (i connectionItem) Title() string { return i.SSID }
func (i connectionItem) Description() string {
	if i.Strength > 0 {
		return fmt.Sprintf("%d%%", i.Strength)
	}
	if !i.IsVisible && i.LastConnected != nil {
		return helpers.FormatDuration(*i.LastConnected)
	}
	return ""
}
func (i connectionItem) FilterValue() string { return i.Title() }

// Bubbletea messages are used to communicate between the main loop and commands
type (
	// From backend
	connectionsLoadedMsg []wifi.Connection // Sent when connections are fetched
	scanFinishedMsg      []wifi.Connection // Sent when a scan is finished
	secretsLoadedMsg     struct {
		item   connectionItem
		secret string
	}
	connectionSavedMsg struct{}
	errorMsg           struct{ err error }

	// To main model
	scanMsg         struct{}
	connectMsg       struct {
		item        connectionItem
		autoConnect bool
	}
	joinNetworkMsg struct {
		ssid     string
		password string
		security wifi.SecurityType
		isHidden bool
	}
	loadSecretsMsg struct{ item connectionItem }
	updateSecretMsg struct {
		item        connectionItem
		newPassword string
		autoConnect bool
	}
	forgetNetworkMsg struct{ item connectionItem }
)

// --- Commands that interact with the backend ---

func scanNetworks(b wifi.Backend) tea.Cmd {
	return func() tea.Msg {
		connections, err := b.BuildNetworkList(true)
		if err != nil {
			return errorMsg{err}
		}
		wifi.SortConnections(connections)
		return scanFinishedMsg(connections)
	}
}

func refreshNetworks(b wifi.Backend) tea.Cmd {
	return func() tea.Msg {
		connections, err := b.BuildNetworkList(false)
		if err != nil {
			return errorMsg{err}
		}
		wifi.SortConnections(connections)
		return connectionsLoadedMsg(connections)
	}
}

func activateConnection(b wifi.Backend, ssid string) tea.Cmd {
	return func() tea.Msg {
		err := b.ActivateConnection(ssid)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to activate connection: %w", err)}
		}
		return connectionSavedMsg{} // Re-use this to trigger a refresh
	}
}

func forgetNetwork(b wifi.Backend, ssid string) tea.Cmd {
	return func() tea.Msg {
		err := b.ForgetNetwork(ssid)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to forget connection: %w", err)}
		}
		return connectionSavedMsg{} // Re-use this to trigger a refresh
	}
}

func joinNetwork(b wifi.Backend, ssid string, password string, security wifi.SecurityType, isHidden bool) tea.Cmd {
	return func() tea.Msg {
		err := b.JoinNetwork(ssid, password, security, isHidden)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to join network: %w", err)}
		}
		return connectionSavedMsg{}
	}
}

func updateAutoConnect(b wifi.Backend, ssid string, autoConnect bool) tea.Cmd {
	return func() tea.Msg {
		err := b.SetAutoConnect(ssid, autoConnect)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to update autoconnect: %w", err)}
		}
		return connectionSavedMsg{}
	}
}

func getSecrets(b wifi.Backend, item connectionItem) tea.Cmd {
	return func() tea.Msg {
		secret, err := b.GetSecrets(item.SSID)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to get secrets: %w", err)}
		}
		return secretsLoadedMsg{item: item, secret: secret}
	}
}

func updateSecret(b wifi.Backend, ssid string, newPassword string) tea.Cmd {
	return func() tea.Msg {
		err := b.UpdateSecret(ssid, newPassword)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to update connection: %w", err)}
		}
		return connectionSavedMsg{}
	}
}
