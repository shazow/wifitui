package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shazow/wifitui/internal/helpers"
	"github.com/shazow/wifitui/wifi"
)

// viewState represents the current screen of the TUI
type viewState int

const (
	stateListView viewState = iota
	stateEditView
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
	changeViewMsg   viewState
	showEditViewMsg struct{ item *connectionItem }
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

// The main model for our TUI application
type model struct {
	state      viewState
	listModel  *ListModel
	editModel  EditModel
	errorModel ErrorModel

	spinner spinner.Model
	backend       wifi.Backend
	loading       bool
	statusMessage string
	width, height int
}

// NewModel creates the starting state of our application
func NewModel(b wifi.Backend) (*model, error) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(CurrentTheme.Primary)

	listModel := NewListModel()

	m := model{
		state:         stateListView,
		listModel:     listModel,
		spinner:       s,
		backend:       b,
		loading:       true,
		statusMessage: "Loading connections...",
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
		m.width = msg.Width
		m.height = msg.Height
		// Account for title and status bar
		h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()
		listBorderStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true).BorderForeground(CurrentTheme.Border)
		bh, bv := listBorderStyle.GetFrameSize()
		extraVerticalSpace := 4
		m.listModel.SetSize(msg.Width-h-bh, msg.Height-v-bv-extraVerticalSpace)

	// Custom messages from our backend commands
	case connectionsLoadedMsg:
		m.loading = false
		m.statusMessage = ""
		items := make([]list.Item, len(msg))
		for i, c := range msg {
			items[i] = connectionItem{Connection: c}
		}
		m.listModel.SetItems(items)
	case scanFinishedMsg:
		m.loading = false
		m.statusMessage = "Scan finished."
		items := make([]list.Item, len(msg))
		for i, c := range msg {
			items[i] = connectionItem{Connection: c}
		}
		m.listModel.SetItems(items)
	case secretsLoadedMsg:
		m.loading = false
		m.statusMessage = "Network loaded. Press 'esc' to go back."
		m.editModel = NewEditModel(&msg.item)
		m.editModel.SetPassword(msg.secret)
		m.state = stateEditView
	case connectionSavedMsg:
		m.loading = true // Show loading while we refresh
		m.statusMessage = "Successfully updated. Refreshing list..."
		m.state = stateListView
		return m, refreshNetworks(m.backend)
	case errorMsg:
		m.loading = false
		m.errorModel = NewErrorModel(msg.err)
		m.state = stateErrorView
	case changeViewMsg:
		m.state = viewState(msg)
	case showEditViewMsg:
		m.state = stateEditView
		m.editModel = NewEditModel(msg.item)
	case scanMsg:
		m.loading = true
		m.statusMessage = "Scanning for networks..."
		return m, scanNetworks(m.backend)
	case connectMsg:
		m.loading = true
		m.statusMessage = fmt.Sprintf("Connecting to '%s'...", msg.item.SSID)
		var batch []tea.Cmd
		if msg.autoConnect != msg.item.AutoConnect {
			batch = append(batch, updateAutoConnect(m.backend, msg.item.SSID, msg.autoConnect))
		}
		batch = append(batch, activateConnection(m.backend, msg.item.SSID))
		return m, tea.Batch(batch...)
	case joinNetworkMsg:
		m.loading = true
		m.statusMessage = fmt.Sprintf("Joining '%s'...", msg.ssid)
		return m, joinNetwork(m.backend, msg.ssid, msg.password, msg.security, msg.isHidden)
	case loadSecretsMsg:
		m.loading = true
		m.statusMessage = fmt.Sprintf("Loading details for %s...", msg.item.SSID)
		return m, getSecrets(m.backend, msg.item)
	case updateSecretMsg:
		m.loading = true
		m.statusMessage = fmt.Sprintf("Saving settings for %s...", msg.item.SSID)
		var batch []tea.Cmd
		batch = append(batch, updateSecret(m.backend, msg.item.SSID, msg.newPassword))
		if msg.autoConnect != msg.item.AutoConnect {
			batch = append(batch, updateAutoConnect(m.backend, msg.item.SSID, msg.autoConnect))
		}
		return m, tea.Batch(batch...)
	case forgetNetworkMsg:
		m.loading = true
		m.statusMessage = fmt.Sprintf("Forgetting '%s'...", msg.item.SSID)
		return m, forgetNetwork(m.backend, msg.item.SSID)
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	// Pass messages to the active view to handle
	var newModel tea.Model
	switch m.state {
	case stateListView:
		newModel, cmd = m.listModel.Update(msg)
		m.listModel = newModel.(*ListModel)
	case stateEditView:
		newModel, cmd = m.editModel.Update(msg)
		m.editModel = newModel.(EditModel)
	case stateErrorView:
		newModel, cmd = m.errorModel.Update(msg)
		m.errorModel = newModel.(ErrorModel)
	}
	cmds = append(cmds, cmd)

	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI based on the current model state
func (m model) View() string {
	var s strings.Builder

	switch m.state {
	case stateListView:
		s.WriteString(m.listModel.View())
	case stateEditView:
		s.WriteString(m.editModel.View())
	case stateErrorView:
		s.WriteString(m.errorModel.View())
	}

	if m.loading {
		s.WriteString(fmt.Sprintf("\n\n%s %s", m.spinner.View(), lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Render(m.statusMessage)))
	} else if m.statusMessage != "" {
		s.WriteString(fmt.Sprintf("\n\n%s", lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Render(m.statusMessage)))
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
