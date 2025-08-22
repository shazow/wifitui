package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/godbus/dbus/v5"
	"github.com/google/uuid"
)

// Define some styles for the UI
var (
	docStyle            = lipgloss.NewStyle().Margin(1, 2)
	statusStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render
	errorStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render
	activeStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	knownNetworkStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("40"))
	unknownNetworkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	disabledStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// viewState represents the current screen of the TUI
type viewState int

const (
	stateListView viewState = iota
	stateEditView
	stateJoinView
	stateForgetView
)

// accessPoint holds the information for a single visible Wi-Fi access point.
type accessPoint struct {
	ssid     string
	path     dbus.ObjectPath
	strength uint8
	isSecure bool
}

// connectionItem holds the information for a single Wi-Fi connection in our list
type connectionItem struct {
	ssid      string
	path      dbus.ObjectPath
	settings  map[string]map[string]dbus.Variant
	isActive  bool
	isKnown   bool
	isSecure  bool
	isVisible bool
	strength  uint8
}

// Implement the list.Item interface for our connectionItem
func (i connectionItem) Title() string {
	if !i.isVisible {
		return disabledStyle.Render(i.ssid)
	}

	var styledSSID string
	if i.isKnown {
		styledSSID = knownNetworkStyle.Render(i.ssid)
	} else {
		styledSSID = unknownNetworkStyle.Render(i.ssid)
	}

	if i.isActive {
		return activeStyle.Render("* " + i.ssid)
	}

	return styledSSID
}
func (i connectionItem) Description() string {
	if i.strength > 0 {
		return fmt.Sprintf("Strength: %d%%", i.strength)
	}
	return ""
}
func (i connectionItem) FilterValue() string { return i.ssid }

// Bubbletea messages are used to communicate between the main loop and commands
type (
	connectionsLoadedMsg []list.Item // Sent when connections are fetched
	scanFinishedMsg      []list.Item // Sent when a scan is finished
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
	l.Title = "Visible Wi-Fi Networks"
	l.SetShowStatusBar(true)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "scan")),
			key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "forget")),
		}
	}
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
	return tea.Batch(m.spinner.Tick, refreshNetworks)
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
	case scanFinishedMsg:
		m.loading = false
		m.statusMessage = "Scan finished."
		m.list.SetItems(msg)
	case secretsLoadedMsg:
		m.loading = false
		m.statusMessage = "Secret loaded. Press 'esc' to go back."
		m.passwordInput.SetValue(string(msg))
		m.passwordInput.CursorEnd()
	case connectionSavedMsg:
		m.loading = true // Show loading while we refresh
		m.statusMessage = fmt.Sprintf("Successfully updated '%s'. Refreshing list...", m.selectedItem.ssid)
		m.state = stateListView
		return m, refreshNetworks
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
			switch msg.String() {
			case "q":
				if m.list.FilterState() != list.Filtering {
					return m, tea.Quit
				}
			case "s":
				m.loading = true
				m.statusMessage = "Scanning for networks..."
				cmds = append(cmds, scanNetworks)
			case "f":
				if len(m.list.Items()) > 0 {
					selected, ok := m.list.SelectedItem().(connectionItem)
					if ok && selected.isKnown {
						m.selectedItem = selected
						m.state = stateForgetView
						m.statusMessage = fmt.Sprintf("Forget network '%s'? (y/n)", m.selectedItem.ssid)
					}
				}
			case "enter":
				if len(m.list.Items()) > 0 {
					selected, ok := m.list.SelectedItem().(connectionItem)
					if !ok {
						break
					}
					m.selectedItem = selected
					if selected.isKnown {
						m.state = stateEditView
						m.loading = true
						m.statusMessage = fmt.Sprintf("Fetching secret for %s...", m.selectedItem.ssid)
						m.errorMessage = ""
						m.passwordInput.SetValue("")
						m.passwordInput.Focus()
						cmds = append(cmds, getSecrets(m.selectedItem))
					} else {
						if selected.isSecure {
							m.state = stateJoinView
							m.statusMessage = fmt.Sprintf("Enter password for %s", m.selectedItem.ssid)
							m.errorMessage = ""
							m.passwordInput.SetValue("")
							m.passwordInput.Focus()
						} else {
							m.loading = true
							m.statusMessage = fmt.Sprintf("Joining '%s'...", m.selectedItem.ssid)
							m.errorMessage = ""
							cmds = append(cmds, joinNetwork(m.selectedItem, ""))
						}
					}
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
		case stateJoinView:
			switch msg.String() {
			case "q", "esc":
				m.state = stateListView
				m.statusMessage = ""
				m.errorMessage = ""
			case "enter":
				m.loading = true
				m.statusMessage = fmt.Sprintf("Joining '%s'...", m.selectedItem.ssid)
				m.errorMessage = ""
				cmds = append(cmds, joinNetwork(m.selectedItem, m.passwordInput.Value()))
			}
		case stateForgetView:
			switch msg.String() {
			case "y":
				m.loading = true
				m.statusMessage = fmt.Sprintf("Forgetting '%s'...", m.selectedItem.ssid)
				cmds = append(cmds, forgetNetwork(m.selectedItem))
			case "n", "q", "esc":
				m.state = stateListView
				m.statusMessage = ""
			}
		}
	}

	// Pass messages to the active components for their own internal updates
	switch m.state {
	case stateListView:
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	case stateEditView, stateJoinView:
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
	case stateJoinView:
		s.WriteString(fmt.Sprintf("\nJoining Wi-Fi Network: %s\n\n", m.selectedItem.ssid))
		s.WriteString(m.passwordInput.View())
		s.WriteString("\n\n(press enter to join, esc to cancel)")
	case stateForgetView:
		// The status message is used as the prompt
		s.WriteString(m.list.View())
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
	nmDest             = "org.freedesktop.NetworkManager"
	nmPath             = "/org/freedesktop/NetworkManager"
	nmIface            = "org.freedesktop.NetworkManager"
	nmSettingsPath     = "/org/freedesktop/NetworkManager/Settings"
	nmSettingsIface    = "org.freedesktop.NetworkManager.Settings"
	nmConnIface        = "org.freedesktop.NetworkManager.Settings.Connection"
	nmActiveConnIface  = "org.freedesktop.NetworkManager.Connection.Active"
	nmDeviceIface      = "org.freedesktop.NetworkManager.Device"
	nmWirelessIface    = "org.freedesktop.NetworkManager.Device.Wireless"
	nmAccessPointIface = "org.freedesktop.NetworkManager.AccessPoint"
)

func scanNetworks() tea.Msg {
	conn, err := dbus.SystemBus()
	if err != nil {
		return errorMsg{err}
	}
	defer conn.Close()

	wirelessDevice, err := getWirelessDevice(conn)
	if err != nil {
		return errorMsg{err}
	}

	obj := conn.Object(nmDest, wirelessDevice)
	err = obj.Call(nmWirelessIface+".RequestScan", 0, map[string]dbus.Variant{}).Store()
	if err != nil {
		return errorMsg{err}
	}

	// For simplicity, we're not waiting for LastScan to update.
	// In a real app, you'd listen for PropertiesChanged signal.

	var apPaths []dbus.ObjectPath
	err = obj.Call(nmWirelessIface+".GetAllAccessPoints", 0).Store(&apPaths)
	if err != nil {
		return errorMsg{err}
	}

	var items []list.Item
	for _, path := range apPaths {
		apObj := conn.Object(nmDest, path)
		ssidVar, err := apObj.GetProperty(nmAccessPointIface + ".Ssid")
		if err != nil {
			continue
		}
		ssidBytes := ssidVar.Value().([]byte)

		strengthVar, err := apObj.GetProperty(nmAccessPointIface + ".Strength")
		if err != nil {
			continue
		}
		strength := strengthVar.Value().(byte)

		flagsVar, err := apObj.GetProperty(nmAccessPointIface + ".Flags")
		if err != nil {
			continue
		}
		flags := flagsVar.Value().(uint32)

		// NM_802_11_AP_FLAGS_PRIVACY
		isSecure := (flags&0x1) != 0

		items = append(items, connectionItem{
			ssid:      string(ssidBytes),
			path:      path,
			settings:  nil, // Not a known connection
			isActive:  false,
			isKnown:   false,
			isSecure:  isSecure,
			strength:  strength,
			isVisible: true,
		})
	}

	return scanFinishedMsg(items)
}

func getWirelessDevice(conn *dbus.Conn) (dbus.ObjectPath, error) {
	nmObj := conn.Object(nmDest, nmPath)
	var devices []dbus.ObjectPath
	err := nmObj.Call(nmIface+".GetDevices", 0).Store(&devices)
	if err != nil {
		return "", err
	}

	for _, devicePath := range devices {
		deviceObj := conn.Object(nmDest, devicePath)
		deviceTypeVar, err := deviceObj.GetProperty(nmDeviceIface + ".DeviceType")
		if err != nil {
			continue
		}
		// NM_DEVICE_TYPE_WIFI = 2
		if deviceType, ok := deviceTypeVar.Value().(uint32); ok && deviceType == 2 {
			return devicePath, nil
		}
	}

	return "", fmt.Errorf("no wireless device found")
}

func forgetNetwork(item connectionItem) tea.Cmd {
	return func() tea.Msg {
		conn, err := dbus.SystemBus()
		if err != nil {
			return errorMsg{err}
		}
		defer conn.Close()

		obj := conn.Object(nmDest, item.path)
		err = obj.Call(nmConnIface+".Delete", 0).Store()
		if err != nil {
			return errorMsg{fmt.Errorf("failed to forget connection: %w", err)}
		}

		return connectionSavedMsg{} // Re-use this to trigger a refresh
	}
}

func joinNetwork(item connectionItem, password string) tea.Cmd {
	return func() tea.Msg {
		conn, err := dbus.SystemBus()
		if err != nil {
			return errorMsg{err}
		}
		defer conn.Close()

		wirelessDevice, err := getWirelessDevice(conn)
		if err != nil {
			return errorMsg{err}
		}

		connection := map[string]map[string]dbus.Variant{
			"connection": {
				"id":   dbus.MakeVariant(item.ssid),
				"uuid": dbus.MakeVariant(uuid.New().String()),
				"type": dbus.MakeVariant("802-11-wireless"),
			},
			"802-11-wireless": {
				"mode": dbus.MakeVariant("infrastructure"),
				"ssid": dbus.MakeVariant([]byte(item.ssid)),
			},
			"ipv4": {"method": dbus.MakeVariant("auto")},
			"ipv6": {"method": dbus.MakeVariant("auto")},
		}

		if item.isSecure {
			connection["802-11-wireless-security"] = map[string]dbus.Variant{
				"key-mgmt": dbus.MakeVariant("wpa-psk"),
				"psk":      dbus.MakeVariant(password),
			}
		}

		nmObj := conn.Object(nmDest, nmPath)
		var activeConnectionPath dbus.ObjectPath
		var newConnectionPath dbus.ObjectPath
		err = nmObj.Call(
			nmIface+".AddAndActivateConnection",
			0,
			connection,
			wirelessDevice,
			item.path, // specific AP path
		).Store(&newConnectionPath, &activeConnectionPath)

		if err != nil {
			return errorMsg{fmt.Errorf("failed to add/activate connection: %w", err)}
		}

		return connectionSavedMsg{}
	}
}

// refreshNetworks is a tea.Cmd that gets connections from D-Bus
func refreshNetworks() tea.Msg {
	conn, err := dbus.SystemBus()
	if err != nil {
		return errorMsg{err}
	}
	defer conn.Close()

	// 1. Get all saved connections and map them by SSID
	knownConnections := make(map[string]connectionItem)
	settingsObj := conn.Object(nmDest, nmSettingsPath)
	var connectionPaths []dbus.ObjectPath
	err = settingsObj.Call(nmSettingsIface+".ListConnections", 0).Store(&connectionPaths)
	if err != nil {
		return errorMsg{err}
	}
	for _, path := range connectionPaths {
		connObj := conn.Object(nmDest, path)
		settings, err := getSettings(connObj)
		if err != nil {
			continue
		}
		if connType, ok := settings["connection"]["type"]; ok && connType.Value() == "802-11-wireless" {
			if ssidBytes, ok := settings["802-11-wireless"]["ssid"].Value().([]byte); ok {
				ssid := string(ssidBytes)
				knownConnections[ssid] = connectionItem{
					ssid:     ssid,
					path:     path,
					settings: settings,
					isKnown:  true,
				}
			}
		}
	}

	// 2. Get all visible access points and map them by SSID
	visibleAPs := make(map[string]accessPoint)
	wirelessDevice, err := getWirelessDevice(conn)
	if err == nil {
		var apPaths []dbus.ObjectPath
		devObj := conn.Object(nmDest, wirelessDevice)
		err = devObj.Call(nmWirelessIface+".GetAllAccessPoints", 0).Store(&apPaths)
		if err == nil {
			for _, path := range apPaths {
				apObj := conn.Object(nmDest, path)
				ssidVar, err := apObj.GetProperty(nmAccessPointIface + ".Ssid")
				if err != nil || ssidVar.Value() == nil {
					continue
				}
				ssidBytes, ok := ssidVar.Value().([]byte)
				if !ok || len(ssidBytes) == 0 {
					continue
				}
				ssid := string(ssidBytes)

				if _, exists := visibleAPs[ssid]; exists {
					continue // Already seen a stronger signal for this SSID
				}

				strengthVar, _ := apObj.GetProperty(nmAccessPointIface + ".Strength")
				flagsVar, _ := apObj.GetProperty(nmAccessPointIface + ".Flags")

				visibleAPs[ssid] = accessPoint{
					ssid:     ssid,
					path:     path,
					strength: strengthVar.Value().(byte),
					isSecure: (flagsVar.Value().(uint32) & 0x1) != 0,
				}
			}
		}
	}

	// 3. Build the final list
	var visibleItems []list.Item
	var nonVisibleItems []list.Item
	processedSSIDs := make(map[string]bool)
	activeWifiPath := getActiveWifiConnectionPath(conn)

	// Process visible APs first
	for ssid, ap := range visibleAPs {
		processedSSIDs[ssid] = true
		var item connectionItem
		if known, ok := knownConnections[ssid]; ok {
			item = known
			item.isActive = (activeWifiPath != "" && item.path == activeWifiPath)
			item.isSecure = ap.isSecure
			item.strength = ap.strength
		} else {
			item = connectionItem{
				ssid:     ssid,
				path:     ap.path,
				isKnown:  false,
				isSecure: ap.isSecure,
				strength: ap.strength,
			}
		}
		item.isVisible = true
		visibleItems = append(visibleItems, item)
	}

	// Add known connections that are not visible
	for ssid, conn := range knownConnections {
		if _, processed := processedSSIDs[ssid]; !processed {
			conn.isVisible = false
			nonVisibleItems = append(nonVisibleItems, conn)
		}
	}

	// Sort: active on top, then visible, then non-visible
	var activeItem list.Item
	var otherVisibleItems []list.Item
	for _, item := range visibleItems {
		if connItem, ok := item.(connectionItem); ok && connItem.isActive {
			activeItem = item
		} else {
			otherVisibleItems = append(otherVisibleItems, item)
		}
	}

	finalItems := []list.Item{}
	if activeItem != nil {
		finalItems = append(finalItems, activeItem)
	}
	finalItems = append(finalItems, otherVisibleItems...)
	finalItems = append(finalItems, nonVisibleItems...)

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
