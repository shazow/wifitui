package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shazow/wifitui/wifi"
)

func TestListModel_DynamicLayout(t *testing.T) {
	m := NewListModel()

	// Helper to send resize and check width
	checkLayout := func(width int, expectedSSIDWidth int) {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 20})

		if m.ssidColumnWidth != expectedSSIDWidth {
			t.Errorf("Width %d: expected ssidColumnWidth %d, got %d", width, expectedSSIDWidth, m.ssidColumnWidth)
		}

		if len(m.list.Title) == 0 {
			t.Errorf("Width %d: Title is empty", width)
		}
	}

	// Case 1: Empty list, width=80.
	// m.ssidLongestWidth initialized to 30.
	// Target = 30 + 2 = 32.
	// Clamped to 32.
	checkLayout(80, 32)
}

func TestListModel_ContentAwareLayout(t *testing.T) {
	m := NewListModel()

	checkContentLayout := func(ssid string, windowWidth int, expectedWidth int) {
		// Create a connection
		conns := []wifi.Connection{
			{
				SSID:      ssid,
				IsVisible: true,
				Security:  wifi.SecurityWPA,
			},
		}

		// Load connections
		m.Update(connectionsLoadedMsg(conns))

		// Manually calculate and set width (simulating tui.go logic)
		item := connectionItem{Connection: conns[0]}
		w := lipgloss.Width(getIcon(item) + item.Title()) + 2
		m.SetColumnWidth(w)

		// Trigger resize to update layout
		m.Update(tea.WindowSizeMsg{Width: windowWidth, Height: 20})

		if m.ssidColumnWidth != expectedWidth {
			t.Errorf("SSID %q (len %d), Window %d: expected %d, got %d",
				ssid, len(ssid), windowWidth, expectedWidth, m.ssidColumnWidth)
		}
	}

	// Icon width for Secure is "🔒 " -> 3?
	// Let's verify icon width dynamically or assume 3.
	// CurrentTheme is used.
	// "🔒 " is typically 2 cells for emoji + 1 space = 3.

	// Case 1: Short SSID "Home" (4)
	// Width = 3 + 4 = 7.
	// Target = 9.
	// Clamped min = 30.
	checkContentLayout("Home", 100, 30)

	// Case 2: Medium SSID (35 chars)
	// 35 chars.
	// Width = 3 + 35 = 38.
	// Target = 40.
	// Window 100 -> Available 100-6-33 = 61.
	// 40 fits.
	longSSID := strings.Repeat("a", 35)

	// Determine expected width precisely
	icon := CurrentTheme.NetworkSecureIcon
	contentWidth := lipgloss.Width(icon + longSSID)
	// contentWidth should be around 38 if icon is 3.
	expected := contentWidth + 2
	if expected < 30 { expected = 30 } // Should not happen for 35 chars

	checkContentLayout(longSSID, 100, expected)

	// Case 3: Long SSID (55 chars)
	// 55 chars.
	// Width = 3 + 55 = 58.
	// Target = 60.
	// Clamped max = 60.
	veryLongSSID := strings.Repeat("b", 55)
	checkContentLayout(veryLongSSID, 120, 60)

	// Case 4: Constrained by window
	// SSID 45 chars (~48 width -> 50 target).
	// Window 80.
	// Available = 80 - 6 (margins) - 33 (overhead) = 41.
	// 50 > 41.
	// Should be clamped to 41.
	mediumSSID := strings.Repeat("c", 45) // Width ~48

	// Calculate exact expected
	avail := 80 - 6 - 33 // 41
	checkContentLayout(mediumSSID, 80, avail)
}
