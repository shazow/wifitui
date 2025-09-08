package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/shazow/wifitui/internal/helpers"
	"github.com/shazow/wifitui/wifi"
)

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

//- Messages -------------------------------------------------------------------
// These are messages that are handled by the root model (the Stack) or passed
// between view models.

type (
	// connectionsLoadedMsg is sent when the list of connections is loaded.
	connectionsLoadedMsg []wifi.Connection
	// scanFinishedMsg is sent when a network scan is complete.
	scanFinishedMsg []wifi.Connection
	// secretsLoadedMsg is sent when secrets for a network are loaded.
	secretsLoadedMsg struct {
		Password string
		Conn     connectionItem
	}
	// connectionSavedMsg is sent when a connection is successfully saved.
	connectionSavedMsg struct{}
)

// NewModel creates the starting state of our application.
// It returns the root model for the TUI.
func NewModel(b wifi.Backend) (tea.Model, error) {
	initialView := NewListModel(b)
	stack := NewStack(b, initialView)
	return stack, nil
}

// --- Commands that interact with the backend ---

func scanNetworks(b wifi.Backend) tea.Cmd {
	return func() tea.Msg {
		connections, err := b.BuildNetworkList(true)
		if err != nil {
			return ShowErrorMsg{err}
		}
		wifi.SortConnections(connections)
		return scanFinishedMsg(connections)
	}
}

func refreshNetworks(b wifi.Backend) tea.Cmd {
	return func() tea.Msg {
		connections, err := b.BuildNetworkList(false)
		if err != nil {
			return ShowErrorMsg{err}
		}
		wifi.SortConnections(connections)
		return connectionsLoadedMsg(connections)
	}
}

func activateConnection(b wifi.Backend, ssid string) tea.Cmd {
	return func() tea.Msg {
		err := b.ActivateConnection(ssid)
		if err != nil {
			return ShowErrorMsg{fmt.Errorf("failed to activate connection: %w", err)}
		}
		return connectionSavedMsg{} // Re-use this to trigger a refresh
	}
}

func forgetNetwork(b wifi.Backend, ssid string) tea.Cmd {
	return func() tea.Msg {
		err := b.ForgetNetwork(ssid)
		if err != nil {
			return ShowErrorMsg{fmt.Errorf("failed to forget connection: %w", err)}
		}
		return connectionSavedMsg{} // Re-use this to trigger a refresh
	}
}

func joinNetwork(b wifi.Backend, ssid string, password string, security wifi.SecurityType, isHidden bool) tea.Cmd {
	return func() tea.Msg {
		err := b.JoinNetwork(ssid, password, security, isHidden)
		if err != nil {
			return ShowErrorMsg{fmt.Errorf("failed to join network: %w", err)}
		}
		return connectionSavedMsg{}
	}
}

func updateAutoConnect(b wifi.Backend, ssid string, autoConnect bool) tea.Cmd {
	return func() tea.Msg {
		err := b.SetAutoConnect(ssid, autoConnect)
		if err != nil {
			return ShowErrorMsg{fmt.Errorf("failed to update autoconnect: %w", err)}
		}
		// We can reuse connectionSavedMsg to trigger a refresh
		return connectionSavedMsg{}
	}
}

func getSecrets(b wifi.Backend, conn connectionItem) tea.Cmd {
	return func() tea.Msg {
		secret, err := b.GetSecrets(conn.SSID)
		if err != nil {
			return ShowErrorMsg{fmt.Errorf("failed to get secrets: %w", err)}
		}
		return secretsLoadedMsg{Password: secret, Conn: conn}
	}
}

func updateSecret(b wifi.Backend, ssid string, newPassword string) tea.Cmd {
	return func() tea.Msg {
		err := b.UpdateSecret(ssid, newPassword)
		if err != nil {
			return ShowErrorMsg{fmt.Errorf("failed to update connection: %w", err)}
		}
		return connectionSavedMsg{}
	}
}
