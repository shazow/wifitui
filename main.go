package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/godbus/dbus/v5"
)

// Define some styles for the UI
var (
	docStyle    = lipgloss.NewStyle().Margin(1, 2)
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Render
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render
	activeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
)

// viewState represents the current screen of the TUI
type viewState int

const (
	stateListView viewState = iota
	stateEditView
)

// connectionItem holds the information for a single Wi-Fi connection in our list
type connectionItem struct {
	ssid     string
	path     dbus.ObjectPath
	settings map[string]map[string]dbus.Variant
	isActive bool
}

// Implement the list.Item interface for our connectionItem
func (i connectionItem) Title() string {
	if i.isActive {
		return activeStyle.Render("* " + i.ssid)
	}
	return i.ssid
}
func (i connectionItem) Description() string { return string(i.path) }
func (i connectionItem) FilterValue() string { return i.ssid }

// Bubbletea messages are used to communicate between the main loop and commands
type (
	connectionsLoadedMsg []list.Item // Sent when connections are fetched
	secretsLoadedMsg     string      // Sent when a password is fetched
	connectionSavedMsg   struct{}    // Sent when a password is saved
	errorMsg             struct{ err error }
)

// The main model for our TUI application
type model struct {
	state         viewState
	list          list.Model
	passwordInput textinput.Model
	spinner       spinner.Model
	loading       bool
	statusMessage string
	errorMessage  string
	selectedItem  connectionItem
}

// initialModel creates the starting state of our application
func initialModel() model {
	// Configure the spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Configure the password input field
	ti := textinput.New()
	ti.Placeholder = "Password"
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 30
	ti.EchoMode = textinput.EchoNormal // Show password visibly

	// Configure the list
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Search Known Wi-Fi Networks"
	l.SetShowStatusBar(true)
	// Enable the fuzzy finder
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return model{
		state:         stateListView,
		list:          l,
		passwordInput: ti,
		spinner:       s,
		loading:       true,
		statusMessage: "Loading connections...",
	}
}

// Init is the first command that is run when the program starts
func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchConnections)
}

// Update handles all incoming messages and updates the model accordingly
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	// Custom messages from our D-Bus commands
	case connectionsLoadedMsg:
		m.loading = false
		m.statusMessage = ""
		m.list.SetItems(msg)
	case secretsLoadedMsg:
		m.loading = false
		m.statusMessage = "Secret loaded. Press 'esc' to go back."
		m.passwordInput.SetValue(string(msg))
		m.passwordInput.CursorEnd()
	case connectionSavedMsg:
		m.loading = false
		m.statusMessage = fmt.Sprintf("Password for '%s' saved!", m.selectedItem.ssid)
		m.state = stateListView
	case errorMsg:
		m.loading = false
		m.errorMessage = msg.err.Error()

	// Handle key presses
	case tea.KeyMsg:
		// Global quit
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		switch m.state {
		case stateListView:
			// Handle quit and enter in list view
			if m.list.FilterState() != list.Filtering && msg.String() == "q" {
				return m, tea.Quit
			}
			if msg.String() == "enter" && len(m.list.Items()) > 0 {
				selected, ok := m.list.SelectedItem().(connectionItem)
				if ok {
					m.selectedItem = selected
					m.state = stateEditView
					m.loading = true
					m.statusMessage = fmt.Sprintf("Fetching secret for %s...", m.selectedItem.ssid)
					m.errorMessage = ""
					m.passwordInput.SetValue("")
					m.passwordInput.Focus()
					cmds = append(cmds, getSecrets(m.selectedItem))
				}
			}
		case stateEditView:
			// Handle quit, escape, and enter in edit view
			switch msg.String() {
			case "q", "esc":
				m.state = stateListView
				m.statusMessage = ""
				m.errorMessage = ""
			case "enter":
				m.loading = true
				m.statusMessage = fmt.Sprintf("Saving password for %s...", m.selectedItem.ssid)
				m.errorMessage = ""
				cmds = append(cmds, updateSecret(m.selectedItem, m.passwordInput.Value()))
			}
		}
	}

	// Pass messages to the active components for their own internal updates
	switch m.state {
	case stateListView:
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	case stateEditView:
		m.passwordInput, cmd = m.passwordInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Always update the spinner. It will handle its own tick messages.
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI based on the current model state
func (m model) View() string {
	if m.errorMessage != "" {
		return docStyle.Render(fmt.Sprintf("Error: %s\n\nPress 'q' to quit.", errorStyle(m.errorMessage)))
	}

	var s strings.Builder

	switch m.state {
	case stateListView:
		s.WriteString(docStyle.Render(m.list.View()))
	case stateEditView:
		s.WriteString(fmt.Sprintf("\nEditing Wi-Fi Connection: %s\n\n", m.selectedItem.ssid))
		s.WriteString(m.passwordInput.View())
		s.WriteString("\n\n(press enter to save, esc to cancel)")
	}

	if m.loading {
		s.WriteString(fmt.Sprintf("\n\n%s %s", m.spinner.View(), statusStyle(m.statusMessage)))
	} else if m.statusMessage != "" {
		s.WriteString(fmt.Sprintf("\n\n%s", statusStyle(m.statusMessage)))
	}

	return s.String()
}

// --- D-Bus Logic ---

const (
	nmDest          = "org.freedesktop.NetworkManager"
	nmPath          = "/org/freedesktop/NetworkManager"
	nmIface         = "org.freedesktop.NetworkManager"
	nmSettingsPath  = "/org/freedesktop/NetworkManager/Settings"
	nmSettingsIface = "org.freedesktop.NetworkManager.Settings"
	nmConnIface     = "org.freedesktop.NetworkManager.Settings.Connection"
	nmActiveConnIface = "org.freedesktop.NetworkManager.Connection.Active"
)

// fetchConnections is a tea.Cmd that gets connections from D-Bus
func fetchConnections() tea.Msg {
	conn, err := dbus.SystemBus()
	if err != nil {
		return errorMsg{err}
	}
	defer conn.Close()

	// First, find the active Wi-Fi connection's settings path, if any
	activeWifiPath := getActiveWifiConnectionPath(conn)

	// Now, get all saved connections
	settingsObj := conn.Object(nmDest, nmSettingsPath)
	var connectionPaths []dbus.ObjectPath
	err = settingsObj.Call(nmSettingsIface+".ListConnections", 0).Store(&connectionPaths)
	if err != nil {
		return errorMsg{err}
	}

	var activeItem list.Item
	var otherItems []list.Item

	for _, path := range connectionPaths {
		connObj := conn.Object(nmDest, path)
		settings, err := getSettings(connObj)
		if err != nil {
			continue
		}

		if connType, ok := settings["connection"]["type"]; ok && connType.Value() == "802-11-wireless" {
			ssidBytes, ok := settings["802-11-wireless"]["ssid"].Value().([]byte)
			if !ok {
				continue
			}

			isActive := (activeWifiPath != "" && path == activeWifiPath)
			item := connectionItem{
				ssid:     string(ssidBytes),
				path:     path,
				settings: settings,
				isActive: isActive,
			}

			if isActive {
				activeItem = item
			} else {
				otherItems = append(otherItems, item)
			}
		}
	}
	
	finalItems := []list.Item{}
	if activeItem != nil {
		finalItems = append(finalItems, activeItem)
	}
	finalItems = append(finalItems, otherItems...)


	return connectionsLoadedMsg(finalItems)
}

// getActiveWifiConnectionPath finds the settings path of the currently active Wi-Fi connection
func getActiveWifiConnectionPath(conn *dbus.Conn) dbus.ObjectPath {
	nmObj := conn.Object(nmDest, nmPath)
	
	var activeConnPaths []dbus.ObjectPath
	variant, err := nmObj.GetProperty(nmIface + ".ActiveConnections")
	if err != nil {
		return ""
	}
	activeConnPaths = variant.Value().([]dbus.ObjectPath)

	for _, path := range activeConnPaths {
		activeConnObj := conn.Object(nmDest, path)
		connTypeVar, err := activeConnObj.GetProperty(nmActiveConnIface + ".Type")
		if err != nil {
			continue
		}
		if connType, ok := connTypeVar.Value().(string); ok && connType == "802-11-wireless" {
			settingsPathVar, err := activeConnObj.GetProperty(nmActiveConnIface + ".Connection")
			if err != nil {
				continue
			}
			if settingsPath, ok := settingsPathVar.Value().(dbus.ObjectPath); ok {
				return settingsPath
			}
		}
	}
	return ""
}


// getSecrets is a tea.Cmd that fetches the password for a connection
func getSecrets(item connectionItem) tea.Cmd {
	return func() tea.Msg {
		if _, ok := item.settings["802-11-wireless-security"]; !ok {
			return secretsLoadedMsg("")
		}

		conn, err := dbus.SystemBus()
		if err != nil {
			return errorMsg{err}
		}
		defer conn.Close()

		obj := conn.Object(nmDest, item.path)

		var secrets map[string]map[string]dbus.Variant
		err = obj.Call(nmConnIface+".GetSecrets", 0, "802-11-wireless-security").Store(&secrets)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to get secrets (did you authenticate?): %w", err)}
		}

		psk, ok := secrets["802-11-wireless-security"]["psk"]
		if !ok {
			return secretsLoadedMsg("")
		}

		return secretsLoadedMsg(psk.Value().(string))
	}
}

// updateSecret is a tea.Cmd that updates the password for a connection
func updateSecret(item connectionItem, newPassword string) tea.Cmd {
	return func() tea.Msg {
		conn, err := dbus.SystemBus()
		if err != nil {
			return errorMsg{err}
		}
		defer conn.Close()

		obj := conn.Object(nmDest, item.path)
		currentSettings, err := getSettings(obj)
		if err != nil {
			return errorMsg{err}
		}

		if _, ok := currentSettings["802-11-wireless-security"]; !ok {
			currentSettings["802-11-wireless-security"] = make(map[string]dbus.Variant)
		}
		currentSettings["802-11-wireless-security"]["psk"] = dbus.MakeVariant(newPassword)

		err = obj.Call(nmConnIface+".Update", 0, currentSettings).Store()
		if err != nil {
			return errorMsg{fmt.Errorf("failed to update connection: %w", err)}
		}

		return connectionSavedMsg{}
	}
}

// getSettings is a helper to fetch all settings for a connection object
func getSettings(obj dbus.BusObject) (map[string]map[string]dbus.Variant, error) {
	var settings map[string]map[string]dbus.Variant
	err := obj.Call(nmConnIface+".GetSettings", 0).Store(&settings)
	if err != nil {
		return nil, err
	}
	return settings, nil
}

// main is the entry point of the application
func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
		os.Exit(1)
	}
}

