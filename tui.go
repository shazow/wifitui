package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
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
	colorSignalLow = "#BC3C00"
	colorSignalHigh  = "#00FF00"
)

// viewState represents the current screen of the TUI
type viewState int

const (
	stateListView viewState = iota
	stateEditView
	stateJoinView
	stateForgetView
)

const (
	focusInput = iota
	focusButtons
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

// itemDelegate is our custom list delegate
type itemDelegate struct {
	list.DefaultDelegate
}

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(connectionItem)
	if !ok {
		// Fallback to default render for any other item types
		d.DefaultDelegate.Render(w, m, index, listItem)
		return
	}

	// Get plain title and description
	title := i.Title()

	// Define column width for SSID
	ssidColumnWidth := 30
	titleLen := len(title)

	// Truncate title if it's too long
	if titleLen > ssidColumnWidth {
		title = title[:ssidColumnWidth-3] + "…"
		titleLen = ssidColumnWidth
	}
	padding := strings.Repeat(" ", ssidColumnWidth-titleLen)

	// Apply custom styling based on connection state
	if !i.IsVisible {
		title = disabledStyle.Render(title)
	} else if i.IsActive {
		title = activeStyle.Render(title)
	} else if i.IsKnown {
		title = knownNetworkStyle.Render(title)
	} else {
		title = unknownNetworkStyle.Render(title)
	}

	// Prepare description parts
	strengthPart := i.Description()
	connectedPart := ""
	if i.IsActive {
		connectedPart = " (Connected)"
	}

	var desc string
	var descStyle lipgloss.Style

	// Determine base styles
	if index == m.Index() {
		title = "▶" + d.Styles.SelectedTitle.Render(title)
		descStyle = d.Styles.SelectedDesc
	} else {
		title = d.Styles.NormalTitle.MarginLeft(1).Render(title)
		descStyle = d.Styles.NormalDesc
	}

	// Now construct the description string with styles
	if i.Strength > 0 {
		start, _ := colorful.Hex(colorSignalLow)
		end, _ := colorful.Hex(colorSignalHigh)
		p := float64(i.Strength) / 100.0
		blend := start.BlendRgb(end, p)
		signalColor := lipgloss.Color(blend.Hex())

		// Combine base desc style with our signal color
		finalSignalStyle := descStyle.Foreground(signalColor)
		desc = finalSignalStyle.Render(strengthPart) + descStyle.Render(connectedPart)
	} else {
		// No strength, just use the base desc style
		desc = descStyle.Render(strengthPart + connectedPart)
	}

	// Render with padding to create columns
	fmt.Fprintf(w, "%s%s %s", title, padding, desc)
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
	state              viewState
	list               list.Model
	passwordInput      textinput.Model
	spinner            spinner.Model
	backend            backend.Backend
	loading            bool
	statusMessage      string
	errorMessage       string
	selectedItem       connectionItem
	width, height      int
	editFocus          int
	editSelectedButton int
}

// initialModel creates the starting state of our application
func initialModel(b backend.Backend) (model, error) {
	// Configure the spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Configure the password input field
	ti := textinput.New()
	ti.Placeholder = "Passphrase"
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 30
	ti.EchoMode = textinput.EchoNormal // Show password visibly

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
		state:              stateListView,
		list:               l,
		passwordInput:      ti,
		spinner:            s,
		backend:            b,
		loading:            true,
		statusMessage:      "Loading connections...",
		editFocus:          focusInput,
		editSelectedButton: 0,
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
						m.errorMessage = ""
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
						m.editFocus = focusInput
						m.editSelectedButton = 0
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
			// Handle key presses for the edit view
			switch msg.String() {
			case "tab":
				if m.editFocus == focusInput {
					m.editFocus = focusButtons
					m.passwordInput.Blur()
				} else {
					m.editFocus = focusInput
					m.passwordInput.Focus()
				}
			case "esc":
				m.state = stateListView
				m.statusMessage = ""
				m.errorMessage = ""

			default:
				if m.editFocus == focusInput {
					// Pass all other key presses to the input field
					m.passwordInput, cmd = m.passwordInput.Update(msg)
					cmds = append(cmds, cmd)
				} else { // m.editFocus == focusButtons
					switch msg.String() {
					case "right":
						m.editSelectedButton = (m.editSelectedButton + 1) % 3
					case "left":
						m.editSelectedButton = (m.editSelectedButton - 1 + 3) % 3
					case "enter":
						switch m.editSelectedButton {
						case 0: // Connect
							m.loading = true
							m.statusMessage = fmt.Sprintf("Connecting to '%s'...", m.selectedItem.SSID)
							cmds = append(cmds, activateConnection(m.backend, m.selectedItem.SSID))
						case 1: // Save
							m.loading = true
							m.statusMessage = fmt.Sprintf("Saving password for %s...", m.selectedItem.SSID)
							m.errorMessage = ""
							cmds = append(cmds, updateSecret(m.backend, m.selectedItem.SSID, m.passwordInput.Value()))
						case 2: // Cancel
							m.state = stateListView
							m.statusMessage = ""
							m.errorMessage = ""
						}
					}
				}
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
				m.errorMessage = ""
				cmds = append(cmds, forgetNetwork(m.backend, m.selectedItem.SSID))
			case "n", "q", "esc":
				m.state = stateListView
				m.statusMessage = ""
				m.errorMessage = ""
			}
		}
	}

	// Pass messages to the active components for their own internal updates
	switch m.state {
	case stateListView:
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	case stateJoinView:
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

	if m.state == stateForgetView {
		question := lipgloss.NewStyle().Width(50).Align(lipgloss.Center).Render(m.statusMessage)
		dialog := dialogBoxStyle.Render(question)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
	}

	var s strings.Builder

	switch m.state {
	case stateListView:
		var viewBuilder strings.Builder
		viewBuilder.WriteString(listBorderStyle.Render(m.list.View()))

		// Custom status bar
		statusText := ""
		if len(m.list.Items()) > 0 {
			statusText = fmt.Sprintf("%d/%d", m.list.Index()+1, len(m.list.Items()))
		}
		viewBuilder.WriteString("\n")
		viewBuilder.WriteString(statusText)
		s.WriteString(docStyle.Render(viewBuilder.String()))
	case stateEditView:
		var details strings.Builder
		details.WriteString(fmt.Sprintf("SSID: %s\n", m.selectedItem.SSID))
		security := "Open"
		if m.selectedItem.IsSecure {
			security = "Secure"
		}
		details.WriteString(fmt.Sprintf("Security: %s\n", security))
		if m.selectedItem.Strength > 0 {
			details.WriteString(fmt.Sprintf("Signal: %d%%\n", m.selectedItem.Strength))
		}
		if m.selectedItem.LastConnected != nil {
			details.WriteString(fmt.Sprintf("Last connected: %s\n", formatDuration(*m.selectedItem.LastConnected)))
		}

		s.WriteString(lipgloss.NewStyle().Width(50).Border(lipgloss.RoundedBorder()).Padding(1, 2).Render(details.String()))
		s.WriteString("\n")
		s.WriteString(m.passwordInput.View())

		// --- Button rendering ---
		var buttonRow strings.Builder
		buttons := []string{"Connect", "Save", "Cancel"}
		focusedButtonStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
		normalButtonStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

		for i, label := range buttons {
			style := normalButtonStyle
			if m.editFocus == focusButtons && i == m.editSelectedButton {
				style = focusedButtonStyle
			}
			buttonRow.WriteString(style.Render(fmt.Sprintf("[ %s ]", label)))
			buttonRow.WriteString("  ")
		}

		s.WriteString("\n\n")
		s.WriteString(buttonRow.String())
		s.WriteString("\n\n(tab to switch fields, arrows to navigate, enter to select)")

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
		var details strings.Builder
		details.WriteString(fmt.Sprintf("SSID: %s\n", m.selectedItem.SSID))
		security := "Open"
		if m.selectedItem.IsSecure {
			security = "Secure"
		}
		details.WriteString(fmt.Sprintf("Security: %s\n", security))
		if m.selectedItem.Strength > 0 {
			details.WriteString(fmt.Sprintf("Signal: %d%%\n", m.selectedItem.Strength))
		}

		s.WriteString(lipgloss.NewStyle().Width(50).Border(lipgloss.RoundedBorder()).Padding(1, 2).Render(details.String()))
		s.WriteString("\n")
		s.WriteString(m.passwordInput.View())
		s.WriteString("\n\n(press enter to join, esc to cancel)")
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
