package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shazow/wifitui/wifi"
)

type ListModel struct {
	list    list.Model
	backend wifi.Backend
	width, height int
}

func NewListModel(b wifi.Backend) *ListModel {
	delegate := itemDelegate{}
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
	l.KeyMap.Quit = key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit"))
	l.AdditionalFullHelpKeys = l.AdditionalShortHelpKeys
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Bold(true)
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(CurrentTheme.Normal)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(CurrentTheme.Primary)

	return &ListModel{
		list:    l,
		backend: b,
	}
}

func (m *ListModel) Init() tea.Cmd {
	return refreshNetworks(m.backend)
}

func (m *ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()
		listBorderStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true).BorderForeground(CurrentTheme.Border)
		bh, bv := listBorderStyle.GetFrameSize()
		extraVerticalSpace := 4
		m.list.SetSize(msg.Width-h-bh, msg.Height-v-bv-extraVerticalSpace)
		m.width = msg.Width
		m.height = msg.Height

	case connectionsLoadedMsg:
		items := make([]list.Item, len(msg))
		for i, c := range msg {
			items[i] = connectionItem{Connection: c}
		}
		m.list.SetItems(items)
		return m, func() tea.Msg { return SetLoadingMsg{Loading: false} }

	case scanFinishedMsg:
		items := make([]list.Item, len(msg))
		for i, c := range msg {
			items[i] = connectionItem{Connection: c}
		}
		m.list.SetItems(items)
		cmds = append(cmds, func() tea.Msg { return SetLoadingMsg{Loading: false} })
		cmds = append(cmds, func() tea.Msg { return SetStatusMsg("Scan finished.") })
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "s":
			cmds = append(cmds, func() tea.Msg { return SetLoadingMsg{Loading: true, Message: "Scanning for networks..."} })
			cmds = append(cmds, scanNetworks(m.backend))
		case "n":
			cmds = append(cmds, func() tea.Msg {
				return PushMsg{Model: NewEditModel(m.backend, connectionItem{}, "")}
			})
		case "f":
			if len(m.list.Items()) > 0 {
				selected, ok := m.list.SelectedItem().(connectionItem)
				if ok && selected.IsKnown {
					cmds = append(cmds, func() tea.Msg {
						return PushMsg{Model: NewForgetModel(m.backend, selected)}
					})
				}
			}
		case "c":
			if len(m.list.Items()) > 0 {
				selected, ok := m.list.SelectedItem().(connectionItem)
				if ok {
					if selected.IsKnown {
						cmds = append(cmds, func() tea.Msg { return SetLoadingMsg{Loading: true, Message: fmt.Sprintf("Connecting to '%s'...", selected.SSID)} })
						cmds = append(cmds, activateConnection(m.backend, selected.SSID))
					} else {
						if shouldDisplayPasswordField(selected.Security) {
							cmds = append(cmds, func() tea.Msg {
								return PushMsg{Model: NewEditModel(m.backend, selected, "")}
							})
						} else {
							cmds = append(cmds, func() tea.Msg { return SetLoadingMsg{Loading: true, Message: fmt.Sprintf("Joining '%s'...", selected.SSID)} })
							cmds = append(cmds, joinNetwork(m.backend, selected.SSID, "", wifi.SecurityOpen, false))
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
				if selected.IsKnown {
					cmds = append(cmds, func() tea.Msg { return SetLoadingMsg{Loading: true, Message: fmt.Sprintf("Loading details for %s...", selected.SSID)} })
					cmds = append(cmds, getSecrets(m.backend, selected))
				} else {
					cmds = append(cmds, func() tea.Msg {
						return PushMsg{Model: NewEditModel(m.backend, selected, "")}
					})
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *ListModel) View() string {
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
