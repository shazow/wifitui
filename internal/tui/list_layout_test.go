package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestListModel_DynamicLayout(t *testing.T) {
	m := NewListModel()

	// Helper to send resize and check width
	checkLayout := func(width int, expectedSSIDWidth int) {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 20})

		if m.ssidColumnWidth != expectedSSIDWidth {
			t.Errorf("Width %d: expected ssidColumnWidth %d, got %d", width, expectedSSIDWidth, m.ssidColumnWidth)
		}

		// We can also verify that the list size was set correctly relative to the minimum width
		// but checking internal list state is harder without access to private fields of bubbles/list.
	}

	// Test cases based on the plan:
	// Min width 30. Max 60.
	// Overhead ~40.
	// Logic: ssid = width - 40.

	// Case 1: Small window (below min reasonable width 70)
	// If width=40, effective=70.
	// Margins/Border overhead = 6. Available = 64.
	// Content overhead = 33. Target = 64 - 33 = 31.
	checkLayout(40, 31)

	// Case 2: Medium window
	// If width=80. Available = 74.
	// Target = 74 - 33 = 41.
	checkLayout(80, 41)

	// Case 3: Large window
	// If width=120, ssid=120-40=80 -> clamped to 60.
	checkLayout(120, 60)
}
