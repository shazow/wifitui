package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/shazow/wifitui/wifi"
)

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

	title := i.Title()

	// Add icons for security
	var icon string = "  ️ "
	switch i.Security {
	case wifi.SecurityUnknown:
		if i.IsVisible {
			icon = "❓ "
		}
	case wifi.SecurityOpen:
		icon = "🔓 "
	default:
		icon = "🔒 "
	}
	title = icon + title

	// Define column width for SSID
	ssidColumnWidth := 30
	titleLen := len(title)

	// Truncate title if it's too long
	if titleLen > ssidColumnWidth {
		title = title[:ssidColumnWidth-1] + "…"
		titleLen = ssidColumnWidth
	}
	padding := strings.Repeat(" ", ssidColumnWidth-titleLen)

	// Apply custom styling based on connection state
	var titleStyle lipgloss.Style
	if !i.IsVisible {
		titleStyle = lipgloss.NewStyle().Foreground(CurrentTheme.Disabled)
	} else if i.IsActive {
		titleStyle = lipgloss.NewStyle().Foreground(CurrentTheme.Success)
	} else if i.IsKnown {
		titleStyle = lipgloss.NewStyle().Foreground(CurrentTheme.Success)
	} else {
		titleStyle = lipgloss.NewStyle().Foreground(CurrentTheme.Normal)
	}
	title = titleStyle.Render(title)

	// Prepare description parts
	strengthPart := i.Description()
	connectedPart := ""
	if i.IsActive {
		connectedPart = " (Connected)"
	}

	var desc string
	if i.Strength > 0 {
		// FIXME: This can be simplified
		var signalHigh, signalLow string
		if adaptiveHigh, ok := CurrentTheme.SignalHigh.TerminalColor.(lipgloss.AdaptiveColor); ok {
			if adaptiveLow, ok := CurrentTheme.SignalLow.TerminalColor.(lipgloss.AdaptiveColor); ok {
				if lipgloss.HasDarkBackground() {
					signalHigh = adaptiveHigh.Dark
					signalLow = adaptiveLow.Dark
				} else {
					signalHigh = adaptiveHigh.Light
					signalLow = adaptiveLow.Light
				}
			}
		}
		start, _ := colorful.Hex(signalLow)
		end, _ := colorful.Hex(signalHigh)
		p := float64(i.Strength) / 100.0
		blend := start.BlendRgb(end, p)
		signalColor := lipgloss.Color(blend.Hex())

		// Style only the signal part with color
		desc = lipgloss.NewStyle().Foreground(signalColor).Render(strengthPart) + connectedPart
	} else {
		desc = lipgloss.NewStyle().Foreground(CurrentTheme.Subtle).Render(strengthPart + connectedPart)
	}

	// Now combine and render the full line
	var line string
	var lineStyle lipgloss.Style
	if index == m.Index() {
		// Selected item
		line = title + padding + " " + desc
		lineStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder(), false, false, false, true). // Left border
			BorderForeground(CurrentTheme.Primary)
	} else {
		// Normal item
		line = title + padding + " " + desc
		lineStyle = lipgloss.NewStyle().PaddingLeft(1)
	}
	fmt.Fprint(w, lineStyle.Render(line))
}

func (m *model) updateListView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle quit and enter in list view
		switch msg.String() {
		case "q":
			if m.list.FilterState() != list.Filtering {
				return m, tea.Quit
			}
		case "n":
			m.state = stateEditView
			m.statusMessage = "Enter details for new network"
			m.errorMessage = ""
			m.selectedItem = connectionItem{}
			m.passwordInput.SetValue("")
			m.ssidInput.SetValue("")
			m.setupEditView()
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
					m.statusMessage = fmt.Sprintf("Forget network '%s'? (Y/n)", m.selectedItem.SSID)
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
						if shouldDisplayPasswordField(selected.Security) {
							m.state = stateEditView
							m.statusMessage = fmt.Sprintf("Enter password for %s", m.selectedItem.SSID)
							m.errorMessage = ""
							m.passwordInput.SetValue("")
							m.setupEditView()
						} else {
							m.loading = true
							m.statusMessage = fmt.Sprintf("Joining '%s'...", m.selectedItem.SSID)
							m.errorMessage = ""
							cmds = append(cmds, joinNetwork(m.backend, m.selectedItem.SSID, "", wifi.SecurityOpen, false))
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
					m.loading = true
					m.statusMessage = fmt.Sprintf("Loading details for %s...", m.selectedItem.SSID)
					m.errorMessage = ""
					m.pendingEditItem = &m.selectedItem
					cmds = append(cmds, getSecrets(m.backend, m.selectedItem.SSID))
				} else {
					// For unknown networks, 'enter' should open the edit view
					m.state = stateEditView
					m.statusMessage = fmt.Sprintf("Editing network %s", m.selectedItem.SSID)
					m.errorMessage = ""
					m.passwordInput.SetValue("")
					m.setupEditView()
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) viewListView() string {
	var viewBuilder strings.Builder
	listBorderStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true).BorderForeground(CurrentTheme.Border)
	viewBuilder.WriteString(listBorderStyle.Render(m.list.View()))

	// Custom status bar
	statusText := ""
	if len(m.list.Items()) > 0 {
		statusText = fmt.Sprintf("%d/%d", m.list.Index()+1, len(m.list.Items()))
	}
	viewBuilder.WriteString("\n")
	viewBuilder.WriteString(statusText)
	return lipgloss.NewStyle().Margin(1, 2).Render(viewBuilder.String())
}

func shouldDisplayPasswordField(security wifi.SecurityType) bool {
	return security != wifi.SecurityOpen
}
