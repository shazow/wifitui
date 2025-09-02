package main

import (
	"fmt"
	"strings"

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
	listBorderStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true)
	dialogBoxStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).BorderForeground(lipgloss.Color("205"))

	// Signal strength colors are now defined as hex constants
)

const (
	colorSignalLow  = "#BC3C00"
	colorSignalHigh = "#00FF00"
)

// viewState represents the current screen of the TUI
type viewState int

const (
	stateListView viewState = iota
	stateEditView
	stateForgetView
)


// connectionItem holds the information for a single Wi-Fi connection in our list
type connectionItem struct {
	backend.Connection
}

func (i connectionItem) Title() string { return i.SSID }
func (i connectionItem) Description() string {
	if i.Strength > 0 {
		return fmt.Sprintf("%d%%", i.Strength)
	}
	if !i.IsVisible && i.LastConnected != nil {
		return formatDuration(*i.LastConnected)
	}
	return ""
}
func (i connectionItem) FilterValue() string { return i.Title() }

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
	state                 viewState
	list                  list.Model
	passwordInput         *Input
	ssidInput             *Input
	buttons               []*Button
	securityGroup         *RadioGroup
	autoConnectCheckbox   *Checkbox
	focusManager          *FocusManager
	buttonFocusManager    *FocusManager
	spinner               spinner.Model
	backend               backend.Backend
	loading               bool
	statusMessage         string
	errorMessage          string
	selectedItem          connectionItem
	width, height         int
	passwordRevealed      bool
	pendingEditItem       *connectionItem
}

// initialModel creates the starting state of our application
func initialModel(b backend.Backend) (model, error) {
	// Configure the spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Configure the password input field
	ti := NewInput()
	ti.Model.Focus()
	ti.Model.CharLimit = 64
	ti.Model.Width = 30
	ti.Model.EchoMode = textinput.EchoNormal // Show password visibly

	// Configure the SSID input field
	si := NewInput()
	si.Model.Focus()
	si.Model.CharLimit = 32
	si.Model.Width = 30

	// Configure the list
	delegate := itemDelegate{}
	defaultDel := list.NewDefaultDelegate()
	delegate.Styles = defaultDel.Styles
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = fmt.Sprintf("%-31s %s", "WiFi Network", "Signal")
	l.SetShowStatusBar(false)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "scan")),
			key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new network")),
			key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "forget")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "connect")),
		}
	}
	// Make 'q' the only quit key
	l.KeyMap.Quit = key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit"))
	l.AdditionalFullHelpKeys = l.AdditionalShortHelpKeys

	// Enable the fuzzy finder
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return model{
		state:                 stateListView,
		list:                  l,
		passwordInput:         ti,
		ssidInput:             si,
		securityGroup:         NewRadioGroup([]string{"Open", "WEP", "WPA/WPA2"}, 0),
		autoConnectCheckbox:   NewCheckbox("Auto Connect", false),
		focusManager:          NewFocusManager(),
		buttonFocusManager:    NewFocusManager(),
		spinner:               s,
		backend:               b,
		loading:               true,
		statusMessage:         "Loading connections...",
		passwordRevealed:      false,
		pendingEditItem:       nil,
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
		bh, bv := listBorderStyle.GetFrameSize()
		// Account for title and status bar
		extraVerticalSpace := 4
		m.list.SetSize(msg.Width-h-bh, msg.Height-v-bv-extraVerticalSpace)
		m.width = msg.Width
		m.height = msg.Height

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
		if m.pendingEditItem != nil {
			m.selectedItem = *m.pendingEditItem
			m.pendingEditItem = nil
		}
		m.passwordInput.Model.SetValue(string(msg))
		m.passwordInput.Model.CursorEnd()
		m.buttons = []*Button{
			NewButton("Connect", 0, func() tea.Cmd {
				return activateConnection(m.backend, m.selectedItem.SSID)
			}),
			NewButton("Save", 1, func() tea.Cmd {
				var cmds []tea.Cmd
				cmds = append(cmds, updateSecret(m.backend, m.selectedItem.SSID, m.passwordInput.Model.Value()))
				if m.autoConnectCheckbox.Checked != m.selectedItem.AutoConnect {
					cmds = append(cmds, updateAutoConnect(m.backend, m.selectedItem.SSID, m.autoConnectCheckbox.Checked))
				}
				return tea.Batch(cmds...)
			}),
			NewButton("Forget", 2, func() tea.Cmd {
				m.state = stateForgetView
				m.statusMessage = fmt.Sprintf("Forget network '%s'? (y/n)", m.selectedItem.SSID)
				m.errorMessage = ""
				return nil
			}),
			NewButton("Cancel", 3, func() tea.Cmd {
				m.state = stateListView
				m.statusMessage = ""
				m.errorMessage = ""
				m.passwordInput.Model.EchoMode = textinput.EchoNormal
				m.passwordInput.Model.Placeholder = ""
				m.passwordRevealed = false
				return nil
			}),
		}
		m.buttonFocusManager.SetComponents(Focusables(m.buttons)...)
		m.autoConnectCheckbox.Checked = m.selectedItem.AutoConnect
		m.focusManager.SetComponents(m.passwordInput, m.autoConnectCheckbox, m.buttonFocusManager)
		m.focusManager.Focus()
		if string(msg) != "" {
			m.passwordInput.Model.EchoMode = textinput.EchoPassword
			m.passwordInput.Model.Placeholder = "(press * to reveal)"
		} else {
			m.passwordInput.Model.EchoMode = textinput.EchoNormal
			m.passwordInput.Model.Placeholder = ""
		}
		m.state = stateEditView
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
	}

	// Pass messages to the active view to handle
	switch m.state {
	case stateListView:
		return m.updateListView(msg)
	case stateEditView:
		return m.updateEditView(msg)
	case stateForgetView:
		return m.updateForgetView(msg)
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
		s.WriteString(m.viewListView())
	case stateEditView:
		s.WriteString(m.viewEditView())
	case stateForgetView:
		s.WriteString(m.viewForgetView())
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
		backend.SortConnections(connections)
		return scanFinishedMsg(connections)
	}
}

func refreshNetworks(b backend.Backend) tea.Cmd {
	return func() tea.Msg {
		connections, err := b.BuildNetworkList(false)
		if err != nil {
			return errorMsg{err}
		}
		backend.SortConnections(connections)
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

func joinNetwork(b backend.Backend, ssid string, password string, security backend.SecurityType, isHidden bool) tea.Cmd {
	return func() tea.Msg {
		err := b.JoinNetwork(ssid, password, security, isHidden)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to join network: %w", err)}
		}
		return connectionSavedMsg{}
	}
}

func updateAutoConnect(b backend.Backend, ssid string, autoConnect bool) tea.Cmd {
	return func() tea.Msg {
		err := b.SetAutoConnect(ssid, autoConnect)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to update autoconnect: %w", err)}
		}
		// We can reuse connectionSavedMsg to trigger a refresh
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
