package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shazow/wifitui/wifi"
)

// The main model for our TUI application
type model struct {
	componentStack []Component

	spinner       spinner.Model
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
		componentStack: []Component{listModel},
		spinner:        s,
		backend:        b,
		loading:        true,
		statusMessage:  "Loading connections...",
	}
	return &m, nil
}

// Init is the first command that is run when the program starts
func (m *model) Init() tea.Cmd {
	if len(m.componentStack) > 0 {
		return tea.Batch(m.spinner.Tick, m.componentStack[0].Init(), refreshNetworks(m.backend))
	}
	return m.spinner.Tick
}

// Update handles all incoming messages and updates the model accordingly
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Global messages that are not passed to components
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		for _, c := range m.componentStack {
			c.Resize(msg.Width, msg.Height)
		}
	case popViewMsg:
		if len(m.componentStack) > 1 {
			m.componentStack = m.componentStack[:len(m.componentStack)-1]
		}
		return m, nil
	case errorMsg:
		m.loading = false
		errorModel := NewErrorModel(msg.err)
		m.componentStack = append(m.componentStack, errorModel)
		return m, nil
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
	// Clear loading status on some messages
	case connectionsLoadedMsg, scanFinishedMsg, secretsLoadedMsg:
		m.loading = false
		m.statusMessage = ""
	case connectionSavedMsg:
		m.loading = true // Show loading while we refresh
		m.statusMessage = "Successfully updated. Refreshing list..."
		return m, tea.Batch(func() tea.Msg { return popViewMsg{} }, refreshNetworks(m.backend))

	}

	// Delegate to the component on the stack
	top := m.componentStack[len(m.componentStack)-1]
	newComp, cmd := top.Update(msg)
	cmds = append(cmds, cmd)

	if newComp != top {
		m.componentStack = append(m.componentStack, newComp)
		cmds = append(cmds, newComp.Init())
	}

	// Spinner update
	var spinnerCmd tea.Cmd
	m.spinner, spinnerCmd = m.spinner.Update(msg)
	cmds = append(cmds, spinnerCmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI based on the current model state
func (m model) View() string {
	var s strings.Builder

	if len(m.componentStack) > 0 {
		top := m.componentStack[len(m.componentStack)-1]
		s.WriteString(top.View())
	}

	if m.loading {
		s.WriteString(fmt.Sprintf("\n\n%s %s", m.spinner.View(), lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Render(m.statusMessage)))
	} else if m.statusMessage != "" {
		s.WriteString(fmt.Sprintf("\n\n%s", lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Render(m.statusMessage)))
	}

	return s.String()
}
