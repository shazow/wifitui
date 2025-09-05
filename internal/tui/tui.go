package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/shazow/wifitui/wifi"
	"github.com/shazow/wifitui/internal/helpers"
)


// viewState represents the current screen of the TUI
type viewState int

const (
	stateListView viewState = iota
	stateEditView
	stateForgetView
	stateErrorView
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

// Bubbletea messages are used to communicate between the main loop and commands
type (
	connectionsLoadedMsg []wifi.Connection // Sent when connections are fetched
	scanFinishedMsg      []wifi.Connection // Sent when a scan is finished
	secretsLoadedMsg     string // Sent when a password is fetched
	connectionSavedMsg   struct{}
	errorMsg             struct{ err error }
	changeViewMsg        viewState
	showForgetViewMsg    struct{}
)

// The main model for our TUI application
type model struct {
	state                 viewState
	list                  list.Model
	passwordInput         textinput.Model
	ssidInput             textinput.Model
	spinner               spinner.Model
	backend               wifi.Backend
	loading               bool
	statusMessage         string
	errorMessage          string
	selectedItem          connectionItem
	width, height         int
	editFocusManager      *FocusManager
	ssidAdapter           *TextInput
	passwordAdapter       *TextInput
	securityGroup         *ChoiceComponent
	autoConnectCheckbox   *Checkbox
	buttonGroup           *MultiButtonComponent
	passwordRevealed      bool
	pendingEditItem       *connectionItem
}

// NewModel creates the starting state of our application
func NewModel(b wifi.Backend) (*model, error) {
	// Configure the spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = CurrentTheme.FocusedStyle

	// Configure the password input field
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 30
	ti.EchoMode = textinput.EchoNormal // Show password visibly

	// Configure the SSID input field
	si := textinput.New()
	si.Focus()
	si.CharLimit = 32
	si.Width = 30

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
	l.Styles.Title = CurrentTheme.ListTitleStyle
	l.Styles.FilterPrompt = CurrentTheme.ListFilterPromptStyle
	l.Styles.FilterCursor = CurrentTheme.ListFilterCursorStyle

	m := model{
		state:            stateListView,
		list:             l,
		passwordInput:    ti,
		ssidInput:        si,
		spinner:          s,
		backend:          b,
		loading:          true,
		statusMessage:    "Loading connections...",
		passwordRevealed: false,
		pendingEditItem:  nil,
	}
	return &m, nil
}

// Init is the first command that is run when the program starts
func (m *model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, refreshNetworks(m.backend))
}

// Update handles all incoming messages and updates the model accordingly
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := CurrentTheme.DocStyle.GetFrameSize()
		bh, bv := CurrentTheme.ListBorderStyle.GetFrameSize()
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
		m.statusMessage = "Network loaded. Press 'esc' to go back."
		if m.pendingEditItem != nil {
			m.selectedItem = *m.pendingEditItem
			m.pendingEditItem = nil
		}
		m.passwordInput.SetValue(string(msg))
		m.passwordInput.CursorEnd()
		if string(msg) != "" {
			m.passwordInput.EchoMode = textinput.EchoPassword
			m.passwordInput.Blur()
		} else {
			m.passwordInput.EchoMode = textinput.EchoNormal
		}
		m.state = stateEditView
		m.setupEditView()
	case connectionSavedMsg:
		m.loading = true // Show loading while we refresh
		m.statusMessage = fmt.Sprintf("Successfully updated '%s'. Refreshing list...", m.selectedItem.SSID)
		m.state = stateListView
		return m, refreshNetworks(m.backend)
	case errorMsg:
		m.loading = false
		m.errorMessage = msg.err.Error()
		m.state = stateErrorView
	case changeViewMsg:
		m.state = viewState(msg)
	case showForgetViewMsg:
		m.state = stateForgetView
		m.statusMessage = fmt.Sprintf("Forget network '%s'? (Y/n)", m.selectedItem.SSID)
		m.errorMessage = ""

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
	case stateErrorView:
		return m.updateErrorView(msg)
	}

	// Always update the spinner. It will handle its own tick messages.
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI based on the current model state
func (m model) View() string {
	var s strings.Builder

	switch m.state {
	case stateListView:
		s.WriteString(m.viewListView())
	case stateEditView:
		s.WriteString(m.viewEditView())
	case stateForgetView:
		s.WriteString(m.viewForgetView())
	case stateErrorView:
		s.WriteString(m.viewErrorView())
	}

	if m.loading {
		s.WriteString(fmt.Sprintf("\n\n%s %s", m.spinner.View(), CurrentTheme.StatusMessageStyle.Render(m.statusMessage)))
	} else if m.statusMessage != "" {
		s.WriteString(fmt.Sprintf("\n\n%s", CurrentTheme.StatusMessageStyle.Render(m.statusMessage)))
	}

	return s.String()
}

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
		// We can reuse connectionSavedMsg to trigger a refresh
		return connectionSavedMsg{}
	}
}

func getSecrets(b wifi.Backend, ssid string) tea.Cmd {
	return func() tea.Msg {
		secret, err := b.GetSecrets(ssid)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to get secrets: %w", err)}
		}
		return secretsLoadedMsg(secret)
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
