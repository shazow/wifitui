package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	Connection
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
func (i connectionItem) FilterValue() string { return i.SSID }

type itemDelegate struct {
	progress progress.Model
}

func newItemDelegate() *itemDelegate {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(30),
		progress.WithoutPercentage(),
	)
	return &itemDelegate{progress: p}
}

func (d *itemDelegate) Height() int {
	return 2
}

func (d *itemDelegate) Spacing() int {
	return 1
}

func (d *itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	// This is where we could handle messages for our delegate.
	// For now, we don't need to do anything here.
	return nil
}

func (d *itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(connectionItem)
	if !ok {
		return
	}

	// Set the progress bar width based on the list width.
	// This is a bit of a hack, but it's the easiest way to make it responsive.
	d.progress.Width = m.Width() - 25

	s := &strings.Builder{}
	s.WriteString(i.Title())
	s.WriteString("\n ")

	if i.IsVisible {
		s.WriteString(d.progress.ViewAs(float64(i.Strength)/100.0))
		s.WriteString(fmt.Sprintf(" %3d%%", i.Strength))
	} else {
		s.WriteString(disabledStyle.Render("Not visible"))
	}

	fmt.Fprint(w, s.String())
}

// Bubbletea messages are used to communicate between the main loop and commands
type (
	connectionsLoadedMsg []Connection // Sent when connections are fetched
	scanFinishedMsg      []Connection // Sent when a scan is finished
	secretsLoadedMsg     string       // Sent when a password is fetched
	connectionSavedMsg   struct{}     // Sent when a password is saved
	errorMsg             struct{ err error }
)

// The main model for our TUI application
type model struct {
	state         viewState
	list          list.Model
	passwordInput textinput.Model
	spinner       spinner.Model
	backend       Backend
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
	delegate := newItemDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Visible Wi-Fi Networks"
	l.SetShowStatusBar(false)
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
		backend:       NewDBusBackend(),
		loading:       true,
		statusMessage: "Loading connections...",
	}
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
							cmds = append(cmds, activateConnection(m.backend, m.selectedItem.Connection))
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
								cmds = append(cmds, joinNetwork(m.backend, m.selectedItem.Connection, ""))
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
						cmds = append(cmds, getSecrets(m.backend, m.selectedItem.Connection))
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
							cmds = append(cmds, joinNetwork(m.backend, m.selectedItem.Connection, ""))
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
				cmds = append(cmds, updateSecret(m.backend, m.selectedItem.Connection, m.passwordInput.Value()))
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
				cmds = append(cmds, joinNetwork(m.backend, m.selectedItem.Connection, m.passwordInput.Value()))
			}
		case stateForgetView:
			switch msg.String() {
			case "y":
				m.loading = true
				m.statusMessage = fmt.Sprintf("Forgetting '%s'...", m.selectedItem.SSID)
				cmds = append(cmds, forgetNetwork(m.backend, m.selectedItem.Connection))
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

func scanNetworks(b Backend) tea.Cmd {
	return func() tea.Msg {
		connections, err := b.BuildNetworkList(true)
		if err != nil {
			return errorMsg{err}
		}
		return scanFinishedMsg(connections)
	}
}

func refreshNetworks(b Backend) tea.Cmd {
	return func() tea.Msg {
		connections, err := b.BuildNetworkList(false)
		if err != nil {
			return errorMsg{err}
		}
		return connectionsLoadedMsg(connections)
	}
}

func activateConnection(b Backend, c Connection) tea.Cmd {
	return func() tea.Msg {
		err := b.ActivateConnection(c)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to activate connection: %w", err)}
		}
		return connectionSavedMsg{} // Re-use this to trigger a refresh
	}
}

func forgetNetwork(b Backend, c Connection) tea.Cmd {
	return func() tea.Msg {
		err := b.ForgetNetwork(c)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to forget connection: %w", err)}
		}
		return connectionSavedMsg{} // Re-use this to trigger a refresh
	}
}

func joinNetwork(b Backend, c Connection, password string) tea.Cmd {
	return func() tea.Msg {
		err := b.JoinNetwork(c, password)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to join network: %w", err)}
		}
		return connectionSavedMsg{}
	}
}

func getSecrets(b Backend, c Connection) tea.Cmd {
	return func() tea.Msg {
		secret, err := b.GetSecrets(c)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to get secrets: %w", err)}
		}
		return secretsLoadedMsg(secret)
	}
}

func updateSecret(b Backend, c Connection, newPassword string) tea.Cmd {
	return func() tea.Msg {
		err := b.UpdateSecret(c, newPassword)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to update connection: %w", err)}
		}
		return connectionSavedMsg{}
	}
}

// main is the entry point of the application
func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
		os.Exit(1)
	}
}
