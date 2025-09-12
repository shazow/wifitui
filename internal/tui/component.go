package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/shazow/wifitui/internal/helpers"
	"github.com/shazow/wifitui/wifi"
)

// Component is the interface for a TUI component.
type Component interface {
	Update(msg tea.Msg) (Component, tea.Cmd)
	View() string
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
