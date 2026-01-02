package wifi

import (
	"testing"
	"time"
)

func TestSortConnections(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-1 * time.Hour)
	earlier2 := now.Add(-2 * time.Hour)

	tests := []struct {
		name     string
		input    []Connection
		expected []string // SSIDs in expected order
	}{
		{
			name: "Sort by strength",
			input: []Connection{
				{SSID: "Weak", AccessPoints: []AccessPoint{{Strength: 10}}, IsVisible: true},
				{SSID: "Strong", AccessPoints: []AccessPoint{{Strength: 90}}, IsVisible: true},
				{SSID: "Medium", AccessPoints: []AccessPoint{{Strength: 50}}, IsVisible: true},
			},
			expected: []string{"Strong", "Medium", "Weak"},
		},
		{
			name: "Active first",
			input: []Connection{
				{SSID: "Strong", AccessPoints: []AccessPoint{{Strength: 90}}, IsVisible: true},
				{SSID: "Active", AccessPoints: []AccessPoint{{Strength: 10}}, IsVisible: true, IsActive: true},
			},
			expected: []string{"Active", "Strong"},
		},
		{
			name: "Visible before hidden known",
			input: []Connection{
				{SSID: "KnownHidden", IsKnown: true, LastConnected: &now},
				{SSID: "Visible", AccessPoints: []AccessPoint{{Strength: 10}}, IsVisible: true},
			},
			expected: []string{"Visible", "KnownHidden"},
		},
		{
			name: "Known sorted by LastConnected",
			input: []Connection{
				{SSID: "Old", IsKnown: true, LastConnected: &earlier2},
				{SSID: "New", IsKnown: true, LastConnected: &earlier},
				{SSID: "Never", IsKnown: true}, // LastConnected is nil
			},
			expected: []string{"New", "Old", "Never"},
		},
		{
			name: "Fallback to SSID",
			input: []Connection{
				{SSID: "B", AccessPoints: []AccessPoint{{Strength: 50}}, IsVisible: true},
				{SSID: "A", AccessPoints: []AccessPoint{{Strength: 50}}, IsVisible: true},
			},
			expected: []string{"A", "B"},
		},
		{
			name: "Complex Mix",
			input: []Connection{
				{SSID: "KnownOld", IsKnown: true, LastConnected: &earlier2},
				{SSID: "Active", IsActive: true, IsVisible: true, AccessPoints: []AccessPoint{{Strength: 60}}},
				{SSID: "VisibleWeak", IsVisible: true, AccessPoints: []AccessPoint{{Strength: 20}}},
				{SSID: "KnownNew", IsKnown: true, LastConnected: &earlier},
				{SSID: "VisibleStrong", IsVisible: true, AccessPoints: []AccessPoint{{Strength: 80}}},
			},
			expected: []string{"Active", "VisibleStrong", "VisibleWeak", "KnownNew", "KnownOld"},
		},
		// Testing duplicate SSIDs with different strengths (should just sort by strength, though duplicates shouldn't happen ideally)
		{
			name: "Duplicate SSIDs",
			input: []Connection{
				{SSID: "MyWifi", AccessPoints: []AccessPoint{{Strength: 30}}, IsVisible: true},
				{SSID: "MyWifi", AccessPoints: []AccessPoint{{Strength: 80}}, IsVisible: true},
			},
			expected: []string{"MyWifi", "MyWifi"}, // Stable sort maintains relative order if equal, but strength differs so 80 comes first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of input
			conns := make([]Connection, len(tt.input))
			copy(conns, tt.input)

			SortConnections(conns)

			if len(conns) != len(tt.expected) {
				t.Fatalf("Expected %d connections, got %d", len(tt.expected), len(conns))
			}

			for i, ssid := range tt.expected {
				if conns[i].SSID != ssid {
					t.Errorf("Index %d: expected SSID %q, got %q", i, ssid, conns[i].SSID)
				}
			}
			// Special check for duplicate test case to ensure order
			if tt.name == "Duplicate SSIDs" {
				if conns[0].Strength() != 80 {
					t.Errorf("Expected strongest duplicate first")
				}
			}
		})
	}
}

func TestSortConnectionsStability(t *testing.T) {
	// Test stability for items that are equal
	input := []Connection{
		{SSID: "A", AccessPoints: []AccessPoint{{Strength: 50}}, IsVisible: true},
		{SSID: "B", AccessPoints: []AccessPoint{{Strength: 50}}, IsVisible: true},
		{SSID: "A", AccessPoints: []AccessPoint{{Strength: 50}}, IsVisible: true},
	}

	// Expected: A, A, B (because A < B, and stable sort keeps original order of As)
	// Actually, wait. SSID "A" < "B". So both As come before B.
	// Between the two As, stable sort preserves order.

	conns := make([]Connection, len(input))
	copy(conns, input)

	SortConnections(conns)

	if conns[0].SSID != "A" || conns[1].SSID != "A" || conns[2].SSID != "B" {
		t.Errorf("Sort order incorrect: %v", conns)
	}
}
