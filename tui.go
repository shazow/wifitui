package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shazow/wifitui/backend"
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

// connectionItem holds the information for a single Wi-Fi connection in our list
type connectionItem struct {
	backend.Connection
}

// Implement the list.Item interface for our connectionItem
func (i connectionItem) Title() string {
	if !i.IsVisible {
		return disabledStyle.Render(i.SSID)
	}

	var styledSSID string
	if i.IsKnown {
		styledSSID = knownNetworkStyle.Render(i.SSID)
	} else {
		styledSSID = unknownNetworkStyle.Render(i.SSID)
	}

	if i.IsActive {
		return activeStyle.Render("* " + i.SSID)
	}

	return styledSSID
}
func (i connectionItem) Description() string {
	if i.Strength > 0 {
		return fmt.Sprintf("%d%% %s", i.Strength, strings.Repeat("█", int(i.Strength/2)))
	}
	if !i.IsVisible && i.LastConnected != nil {
		return formatDuration(*i.LastConnected)
	}
	return ""
}
func (i connectionItem) FilterValue() string { return i.SSID }

// plainTitle returns the title of the connection item without any styling
func (i connectionItem) plainTitle() string {
	if i.IsActive {
		return "* " + i.SSID
	}
	return i.SSID
}

// plainDescription returns the description of the connection item without any styling
func (i connectionItem) plainDescription() string {
	if i.Strength > 0 {
		return fmt.Sprintf("%d%%", i.Strength)
	}
	if !i.IsVisible && i.LastConnected != nil {
		return formatDuration(*i.LastConnected)
	}
	return ""
}

// formatDuration takes a time and returns a human-readable string like "2 hours ago"
func formatDuration(t time.Time) string {
	d := time.Since(t)
	var s string
	switch {
	case d < time.Minute*2:
		s = fmt.Sprintf("%0.f seconds", d.Seconds())
	case d < time.Hour*2:
		s = fmt.Sprintf("%0.f minutes", d.Minutes())
	case d < time.Hour*48:
		s = fmt.Sprintf("%0.1f hours", d.Hours())
	default:
		days := d.Hours() / 24
		s = fmt.Sprintf("%0.1f days", days)
	}
	return fmt.Sprintf("%s ago", s)
}

// Bubbletea messages are used to communicate between the main loop and commands
type (
	connectionsLoadedMsg []backend.Connection // Sent when connections are fetched
	scanFinishedMsg      []backend.Connection // Sent when a scan is finished
	secretsLoadedMsg     string               // Sent when a password is fetched
	connectionSavedMsg   struct{}             // Sent when a password is saved
	errorMsg             struct{ err error }
)

// The main model for our TUI application
type model struct {
	state         viewState
	list          list.Model
	passwordInput textinput.Model
	spinner       spinner.Model
	backend       backend.Backend
	loading       bool
	statusMessage string
	errorMessage  string
	selectedItem  connectionItem
}

// initialModel creates the starting state of our application
func initialModel() (model, error) {
	be, err := NewBackend()
	if err != nil {
		return model{}, fmt.Errorf("failed to initialize backend: %w", err)
	}
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
	l.Title = "Wi-Fi Networks"
	l.SetShowStatusBar(true)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "scan")),
			key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "forget")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "connect")),
		}
	}
	// Make 'q' the only quit key
	l.KeyMap.Quit = key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit"))
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "scan")),
			key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "forget")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "connect")),
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
		backend:       be,
		loading:       true,
		statusMessage: "Loading connections...",
	}, nil
}

// Init is the first command that is run when the program starts
func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, refreshNetworks(m.backend))
}

// Update handles all incoming messages and updates the model accordingly
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	// Custom messages from our backend commands
	case connectionsLoadedMsg:
		m.loading = false
		m.statusMessage = ""
		items := make([]list.Item, len(msg))
		for i, c := range msg {
			items[i] = connectionItem{Connection: c}
		}
		m.list.SetItems(items)
	case scanFinishedMsg:
		m.loading = false
		m.statusMessage = "Scan finished."
		items := make([]list.Item, len(msg))
		for i, c := range msg {
			items[i] = connectionItem{Connection: c}
		}
		m.list.SetItems(items)
	case secretsLoadedMsg:
		m.loading = false
		m.statusMessage = "Secret loaded. Press 'esc' to go back."
		m.passwordInput.SetValue(string(msg))
		m.passwordInput.CursorEnd()
	case connectionSavedMsg:
		m.loading = true // Show loading while we refresh
		m.statusMessage = fmt.Sprintf("Successfully updated '%s'. Refreshing list...", m.selectedItem.SSID)
		m.state = stateListView
		return m, refreshNetworks(m.backend)
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
				cmds = append(cmds, scanNetworks(m.backend))
			case "f":
				if len(m.list.Items()) > 0 {
					selected, ok := m.list.SelectedItem().(connectionItem)
					if ok && selected.IsKnown {
						m.selectedItem = selected
						m.state = stateForgetView
						m.statusMessage = fmt.Sprintf("Forget network '%s'? (y/n)", m.selectedItem.SSID)
					}
				}
			case "c":
				if len(m.list.Items()) > 0 {
					selected, ok := m.list.SelectedItem().(connectionItem)
					if ok {
						m.selectedItem = selected
						if selected.IsKnown {
							m.loading = true
							m.statusMessage = fmt.Sprintf("Connecting to '%s'...", m.selectedItem.SSID)
							cmds = append(cmds, activateConnection(m.backend, m.selectedItem.SSID))
						} else {
							// For unknown networks, 'connect' is the same as 'join'
							if selected.IsSecure {
								m.state = stateJoinView
								m.statusMessage = fmt.Sprintf("Enter password for %s", m.selectedItem.SSID)
								m.errorMessage = ""
								m.passwordInput.SetValue("")
								m.passwordInput.Focus()
							} else {
								m.loading = true
								m.statusMessage = fmt.Sprintf("Joining '%s'...", m.selectedItem.SSID)
								m.errorMessage = ""
								cmds = append(cmds, joinNetwork(m.backend, m.selectedItem.SSID, ""))
							}
						}
					}
				}
			case "enter":
				if len(m.list.Items()) > 0 {
					selected, ok := m.list.SelectedItem().(connectionItem)
					if !ok {
						break
					}
					m.selectedItem = selected
					if selected.IsKnown {
						m.state = stateEditView
						m.loading = true
						m.statusMessage = fmt.Sprintf("Fetching secret for %s...", m.selectedItem.SSID)
						m.errorMessage = ""
						m.passwordInput.SetValue("")
						m.passwordInput.Focus()
						cmds = append(cmds, getSecrets(m.backend, m.selectedItem.SSID))
					} else {
						if selected.IsSecure {
							m.state = stateJoinView
							m.statusMessage = fmt.Sprintf("Enter password for %s", m.selectedItem.SSID)
							m.errorMessage = ""
							m.passwordInput.SetValue("")
							m.passwordInput.Focus()
						} else {
							m.loading = true
							m.statusMessage = fmt.Sprintf("Joining '%s'...", m.selectedItem.SSID)
							m.errorMessage = ""
							cmds = append(cmds, joinNetwork(m.backend, m.selectedItem.SSID, ""))
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
				m.statusMessage = fmt.Sprintf("Saving password for %s...", m.selectedItem.SSID)
				m.errorMessage = ""
				cmds = append(cmds, updateSecret(m.backend, m.selectedItem.SSID, m.passwordInput.Value()))
			}
		case stateJoinView:
			switch msg.String() {
			case "q", "esc":
				m.state = stateListView
				m.statusMessage = ""
				m.errorMessage = ""
			case "enter":
				m.loading = true
				m.statusMessage = fmt.Sprintf("Joining '%s'...", m.selectedItem.SSID)
				m.errorMessage = ""
				cmds = append(cmds, joinNetwork(m.backend, m.selectedItem.SSID, m.passwordInput.Value()))
			}
		case stateForgetView:
			switch msg.String() {
			case "y":
				m.loading = true
				m.statusMessage = fmt.Sprintf("Forgetting '%s'...", m.selectedItem.SSID)
				cmds = append(cmds, forgetNetwork(m.backend, m.selectedItem.SSID))
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
		s.WriteString(fmt.Sprintf("\nEditing Wi-Fi Connection: %s\n\n", m.selectedItem.SSID))
		s.WriteString(m.passwordInput.View())
		s.WriteString("\n\n(press enter to save, esc to cancel)")

		// Add QR code if we have a password
		password := m.passwordInput.Value()
		if password != "" {
			qrCodeString, err := GenerateWifiQRCode(m.selectedItem.SSID, password, m.selectedItem.IsSecure, m.selectedItem.IsHidden)
			if err == nil {
				s.WriteString("\n\n")
				s.WriteString(qrCodeString)
			}
		}
	case stateJoinView:
		s.WriteString(fmt.Sprintf("\nJoining Wi-Fi Network: %s\n\n", m.selectedItem.SSID))
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

// --- Commands that interact with the backend ---

func scanNetworks(b backend.Backend) tea.Cmd {
	return func() tea.Msg {
		connections, err := b.BuildNetworkList(true)
		if err != nil {
			return errorMsg{err}
		}
		return scanFinishedMsg(connections)
	}
}

func refreshNetworks(b backend.Backend) tea.Cmd {
	return func() tea.Msg {
		connections, err := b.BuildNetworkList(false)
		if err != nil {
			return errorMsg{err}
		}
		return connectionsLoadedMsg(connections)
	}
}

func activateConnection(b backend.Backend, ssid string) tea.Cmd {
	return func() tea.Msg {
		err := b.ActivateConnection(ssid)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to activate connection: %w", err)}
		}
		return connectionSavedMsg{} // Re-use this to trigger a refresh
	}
}

func forgetNetwork(b backend.Backend, ssid string) tea.Cmd {
	return func() tea.Msg {
		err := b.ForgetNetwork(ssid)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to forget connection: %w", err)}
		}
		return connectionSavedMsg{} // Re-use this to trigger a refresh
	}
}

func joinNetwork(b backend.Backend, ssid string, password string) tea.Cmd {
	return func() tea.Msg {
		err := b.JoinNetwork(ssid, password)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to join network: %w", err)}
		}
		return connectionSavedMsg{}
	}
}

func getSecrets(b backend.Backend, ssid string) tea.Cmd {
	return func() tea.Msg {
		secret, err := b.GetSecrets(ssid)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to get secrets: %w", err)}
		}
		return secretsLoadedMsg(secret)
	}
}

func updateSecret(b backend.Backend, ssid string, newPassword string) tea.Cmd {
	return func() tea.Msg {
		err := b.UpdateSecret(ssid, newPassword)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to update connection: %w", err)}
		}
		return connectionSavedMsg{}
	}
}
