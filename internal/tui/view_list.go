package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
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

	// Use a table to render the row
	row := []string{title, desc}
	t := table.New().
		Rows([][]string{row}...).
		Border(lipgloss.HiddenBorder()).
		Width(m.Width() - 2). // Subtract padding
		Wrap(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			if index == m.Index() {
				return lipgloss.NewStyle().
					Border(lipgloss.ThickBorder(), false, false, false, true).
					BorderForeground(CurrentTheme.Primary).
					Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})

	fmt.Fprint(w, t.Render())
}

func shouldDisplayPasswordField(security wifi.SecurityType) bool {
	return security != wifi.SecurityOpen
}
