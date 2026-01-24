package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	var icon string
	if i.IsVisible {
		switch i.Security {
		case wifi.SecurityOpen:
			icon = CurrentTheme.NetworkOpenIcon
		case wifi.SecurityUnknown:
			icon = CurrentTheme.NetworkUnknownIcon
		default:
			icon = CurrentTheme.NetworkSecureIcon
		}
	}
	if i.IsKnown {
		icon = CurrentTheme.NetworkSavedIcon
	}
	title = icon + title

	// Define column width for SSID
	ssidColumnWidth := 30
	titleWidth := lipgloss.Width(title)

	// Truncate title if it's too long
	if titleWidth > ssidColumnWidth {
		title = truncateString(title, ssidColumnWidth)
		titleWidth = ssidColumnWidth
	}
	padding := strings.Repeat(" ", ssidColumnWidth-titleWidth)

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

	countPart := ""
	if len(i.AccessPoints) > 1 {
		countPart = fmt.Sprintf(" (%d APs)", len(i.AccessPoints))
	}

	var desc string
	var sb strings.Builder
	if i.Strength() > 0 {
		sb.WriteString(CurrentTheme.FormatSignalStrength(i.Strength()))
		sb.WriteString(countPart)
		sb.WriteString(connectedPart)
		desc = sb.String()
	} else {
		// Networks that are not visible have Strength=0, show their time ago instead
		sb.WriteString(strengthPart)
		sb.WriteString(countPart)
		sb.WriteString(connectedPart)
		desc = lipgloss.NewStyle().Foreground(CurrentTheme.Subtle).Render(sb.String())
	}

	// Now combine and render the full line
	var line string
	if index == m.Index() {
		// Selected item
		if d.listModel.isForgetting {
			desc = lipgloss.NewStyle().Foreground(CurrentTheme.Error).Render("Forget? (Y/n)")
		}
		line = lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Render("▶ ") + title + padding + " " + desc
	} else {
		// Normal item
		line = "  " + title + padding + " " + desc
	}
	fmt.Fprint(w, line)
}

type ListModel struct {
	list         list.Model
	isForgetting bool
	scanner      *ScanSchedule
	numScans     int // Number of scans with results since enter
	width        int
	height       int
	showFullHelp bool
}

// IsConsumingInput returns whether the model is focused on a text input.
func (m *ListModel) IsConsumingInput() bool {
	// The list model does not have any text inputs.
	if m.list.FilterState() == list.Filtering {
		return true
	}
	return false
}

func (m *ListModel) OnEnter() tea.Cmd {
	m.numScans = 0
	return tea.Batch(
		m.scanner.SetSchedule(ScanFast),
		// Start a scan right away
		func() tea.Msg { return scanMsg{} },
	)
}

func NewListModel() *ListModel {
	// m needs to be a pointer to be assigned to listModel
	m := &ListModel{}
	m.scanner = NewScanSchedule(func() tea.Msg { return scanMsg{} })

	delegate := itemDelegate{
		listModel: m,
	}
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = fmt.Sprintf("%-29s %s", CurrentTheme.TitleIcon+"WiFi Network", "Signal")
	l.SetShowStatusBar(false)
	l.SetShowHelp(false) // We manage help rendering manually to customize ShortHelp content

	// Reuse the list's built-in help model, but style it
	l.Help.Styles.ShortKey = lipgloss.NewStyle().Foreground(CurrentTheme.Primary)
	l.Help.Styles.ShortDesc = lipgloss.NewStyle().Foreground(CurrentTheme.Subtle)
	l.Help.Styles.FullKey = lipgloss.NewStyle().Foreground(CurrentTheme.Primary)
	l.Help.Styles.FullDesc = lipgloss.NewStyle().Foreground(CurrentTheme.Subtle)

	// Define custom Short Help keys
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "scan")),
			key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "forget")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "connect")),
		}
	}
	// Make 'q' the only quit key
	l.KeyMap.Quit = key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit"))

	// Disable internal help toggle logic since we handle it
	l.KeyMap.ShowFullHelp.SetEnabled(false)
	l.KeyMap.CloseFullHelp.SetEnabled(false)

	l.AdditionalFullHelpKeys = func() []key.Binding {
		return append([]key.Binding{
			key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new network")),
			key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "active scan")),
		}, l.AdditionalShortHelpKeys()...)
	}

	// Enable the fuzzy finder
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Bold(true)
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(CurrentTheme.Normal)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(CurrentTheme.Primary)
	m.list = l
	return m
}

func (m *ListModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.updateListSize()
}

func (m *ListModel) SetItems(items []list.Item) {
	m.list.SetItems(items)
}

func (m *ListModel) updateListSize() {
	h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()
	listBorderStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true).BorderForeground(CurrentTheme.Border)
	bh, bv := listBorderStyle.GetFrameSize()

	availableWidth := m.width - h - bh

	// Measure help height
	m.list.Help.Width = availableWidth
	// We render help using 'm' (ListModel) as the KeyMap to get our custom ShortHelp
	helpView := m.list.Help.View(m)
	helpHeight := lipgloss.Height(helpView)

	extraVerticalSpace := v + bv
	// We add 2 for the padding newlines we insert in View
	// Wait, do we want newlines? The previous implementation had them.
	// If we use help.View(), it's tight.
	// Let's stick to tight for now, or add 1 line padding.
	// The original screenshot had padding.
	// Let's add 1 line padding if help is shown.
	if helpHeight > 0 {
		helpHeight += 2 // \n\n
	}

	m.list.SetSize(availableWidth, m.height-extraVerticalSpace-helpHeight)
}

func (m *ListModel) Update(msg tea.Msg) (Component, tea.Cmd) {
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
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateListSize()
	case connectionsLoadedMsg:
		items := make([]list.Item, len(msg))
		for i, c := range msg {
			items[i] = connectionItem{Connection: c}
		}
		m.list.SetItems(items)
		return m, nil
	case scanFinishedMsg:
		items := make([]list.Item, len(msg))
		for i, c := range msg {
			items[i] = connectionItem{Connection: c}
		}
		m.list.SetItems(items)
		if len(items) > 0 {
			m.numScans++
		}
		if m.numScans == 3 {
			m.scanner.SetSchedule(ScanSlow)
		}
		return m, nil
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "?":
			m.showFullHelp = !m.showFullHelp
			m.list.Help.ShowAll = m.showFullHelp
			m.updateListSize()
			return m, nil
		case "q":
			if m.list.FilterState() != list.Filtering {
				return m, tea.Quit
			}
		case "n":
			editModel := NewEditModel(nil)
			return editModel, nil
		case "s":
			return m, func() tea.Msg { return scanMsg{} }
		case "S":
			enabled, cmd := m.scanner.Toggle()
			var msg string
			if enabled {
				msg = "Active Scan enabled"
			} else {
				msg = "Active Scan disabled"
			}
			return m, tea.Batch(cmd, func() tea.Msg {
				return statusMsg{status: msg}
			})
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
						editModel := NewEditModel(&selected)
						return editModel, nil
					}
				}
			}
		case "enter":
			if len(m.list.Items()) > 0 {
				selected, ok := m.list.SelectedItem().(connectionItem)
				if !ok {
					break
				}
				editModel := NewEditModel(&selected)
				return editModel, nil
			}
		}
	}

	newList, newCmd := m.list.Update(msg)
	m.list = newList
	cmds = append(cmds, newCmd)

	if m.isForgetting && m.list.Index() != oldIndex {
		m.isForgetting = false
	}

	cmds = append(cmds, m.scanner.Update(msg))
	return m, tea.Batch(cmds...)
}

func (m *ListModel) OnLeave() tea.Cmd {
	m.isForgetting = false
	return m.scanner.SetSchedule(ScanOff)
}

func (m *ListModel) View() string {
	listBorderStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true).BorderForeground(CurrentTheme.Border)

	// We render help manually using 'm' (ListModel) to control keys
	helpView := m.list.Help.View(m)

	// Add padding
	help := fmt.Sprintf("\n\n %s ", helpView)

	// Combine List + Help inside the border
	content := m.list.View() + help

	listView := listBorderStyle.Render(content)

	statusText := ""
	if len(m.list.Items()) > 0 {
		statusText = fmt.Sprintf("%d/%d", m.list.Index()+1, len(m.list.Items()))
	}

	return lipgloss.NewStyle().Margin(1, 2).Render(listView + "\n" + statusText)
}

// ShortHelp returns the specific bindings we want to show in the short help view.
// This allows us to exclude default list navigation keys (Up/Down) that we don't want in the short view.
func (m *ListModel) ShortHelp() []key.Binding {
	keys := []key.Binding{
		m.list.KeyMap.Filter,
	}
	keys = append(keys, m.list.AdditionalShortHelpKeys()...)
	keys = append(keys, m.list.KeyMap.Quit)
	keys = append(keys, key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "more")))
	return keys
}

// FullHelp delegates to the list's FullHelp, which includes all defaults (including Up/Down) + Additional keys.
func (m *ListModel) FullHelp() [][]key.Binding {
	return m.list.FullHelp()
}

// truncateString truncates a string to maxW visual width, appending an ellipsis if truncated.
func truncateString(s string, maxW int) string {
	if lipgloss.Width(s) <= maxW {
		return s
	}
	target := maxW - 1
	var w int
	var sb strings.Builder
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > target {
			break
		}
		sb.WriteRune(r)
		w += rw
	}
	sb.WriteString("…")
	return sb.String()
}
