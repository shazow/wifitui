package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/shazow/wifitui/wifi"
)

// itemDelegate is our custom list delegate
type itemDelegate struct {
	list.DefaultDelegate
	listModel *ListModel
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
	if i.IsKnown {
		icon = "💾 "
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
		titleStyle = lipgloss.NewStyle().Foreground(CurrentTheme.Saved)
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
		if d.listModel.isForgetting {
			forgetPrompt := lipgloss.NewStyle().Foreground(CurrentTheme.Error).Render(" Forget? (Y/n)")
			line += forgetPrompt
		}
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

type ListModel struct {
	list         list.Model
	isForgetting bool
}

func NewListModel() ListModel {
	m := ListModel{}
	delegate := itemDelegate{
		listModel: &m,
	}
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
	l.Styles.Title = lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Bold(true)
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(CurrentTheme.Normal)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(CurrentTheme.Primary)
	m.list = l
	return m
}

func (m *ListModel) SetSize(w, h int) {
	m.list.SetSize(w, h)
}

func (m *ListModel) SetItems(items []list.Item) {
	m.list.SetItems(items)
}

func (m ListModel) Init() tea.Cmd {
	return nil
}

func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	oldIndex := m.list.Index()

	if m.isForgetting {
		selected, ok := m.list.SelectedItem().(connectionItem)
		if !ok {
			m.isForgetting = false
		} else {
			finished, cmd := forgetHandler(msg, selected)
			if finished {
				m.isForgetting = false
				return m, cmd
			}
			// Don't consume other events if we're not finished
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "q":
			if m.list.FilterState() != list.Filtering {
				return m, tea.Quit
			}
		case "n":
			return m, func() tea.Msg { return showEditViewMsg{item: nil} }
		case "s":
			return m, func() tea.Msg { return scanMsg{} }
		case "f":
			if len(m.list.Items()) > 0 {
				selected, ok := m.list.SelectedItem().(connectionItem)
				if ok && selected.IsKnown {
					m.isForgetting = true
					return m, nil
				}
			}
		case "c":
			if len(m.list.Items()) > 0 {
				selected, ok := m.list.SelectedItem().(connectionItem)
				if ok {
					if selected.IsKnown {
						return m, func() tea.Msg { return connectMsg{item: selected} }
					} else {
						return m, func() tea.Msg { return showEditViewMsg{item: &selected} }
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
					return m, func() tea.Msg { return loadSecretsMsg{item: selected} }
				} else {
					return m, func() tea.Msg { return showEditViewMsg{item: &selected} }
				}
			}
		}
	}

	var cmd tea.Cmd
	var newModel list.Model
	newModel, cmd = m.list.Update(msg)
	m.list = newModel
	cmds = append(cmds, cmd)

	if m.isForgetting && m.list.Index() != oldIndex {
		m.isForgetting = false
	}

	return m, tea.Batch(cmds...)
}

func (m ListModel) View() string {
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
