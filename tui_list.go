package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/shazow/wifitui/backend"
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

func (m model) updateListView(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.ssidInput.Focus()
			m.editFocus = focusSSID
			m.editSelectedButton = 0
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
						if selected.Security != backend.SecurityOpen && selected.Security != backend.SecurityUnknown {
							m.state = stateEditView
							m.statusMessage = fmt.Sprintf("Enter password for %s", m.selectedItem.SSID)
							m.errorMessage = ""
							m.passwordInput.SetValue("")
							m.passwordInput.Focus()
							m.editFocus = focusInput
							m.editSelectedButton = 0
						} else {
							m.loading = true
							m.statusMessage = fmt.Sprintf("Joining '%s'...", m.selectedItem.SSID)
							m.errorMessage = ""
							cmds = append(cmds, joinNetwork(m.backend, m.selectedItem.SSID, "", backend.SecurityOpen, false))
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
					m.statusMessage = fmt.Sprintf("Fetching secret for %s...", m.selectedItem.SSID)
					m.errorMessage = ""
					m.pendingEditItem = &m.selectedItem
					cmds = append(cmds, getSecrets(m.backend, m.selectedItem.SSID))
				} else {
					if selected.Security != backend.SecurityOpen && selected.Security != backend.SecurityUnknown {
						m.state = stateEditView
						m.statusMessage = fmt.Sprintf("Enter password for %s", m.selectedItem.SSID)
						m.errorMessage = ""
						m.passwordInput.SetValue("")
						m.passwordInput.Focus()
						m.editFocus = focusInput
						m.editSelectedButton = 0
					} else {
						m.loading = true
						m.statusMessage = fmt.Sprintf("Joining '%s'...", m.selectedItem.SSID)
						m.errorMessage = ""
						cmds = append(cmds, joinNetwork(m.backend, m.selectedItem.SSID, "", backend.SecurityOpen, false))
					}
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
	viewBuilder.WriteString(listBorderStyle.Render(m.list.View()))

	// Custom status bar
	statusText := ""
	if len(m.list.Items()) > 0 {
		statusText = fmt.Sprintf("%d/%d", m.list.Index()+1, len(m.list.Items()))
	}
	viewBuilder.WriteString("\n")
	viewBuilder.WriteString(statusText)
	return docStyle.Render(viewBuilder.String())
}
