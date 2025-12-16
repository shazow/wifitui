package wifi

import (
	"reflect"
	"testing"
	"time"
)

func TestSortConnections(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	twoDaysAgo := now.Add(-48 * time.Hour)

	tests := []struct {
		name        string
		connections []Connection
		expected    []Connection
	}{
		{
			name: "Sort by active",
			connections: []Connection{
				{SSID: "Inactive", IsActive: false},
				{SSID: "Active", IsActive: true},
			},
			expected: []Connection{
				{SSID: "Active", IsActive: true},
				{SSID: "Inactive", IsActive: false},
			},
		},
		{
			name: "Sort by visible",
			connections: []Connection{
				{SSID: "NotVisible", IsVisible: false},
				{SSID: "Visible", IsVisible: true},
			},
			expected: []Connection{
				{SSID: "Visible", IsVisible: true},
				{SSID: "NotVisible", IsVisible: false},
			},
		},
		{
			name: "Sort by strength",
			connections: []Connection{
				{SSID: "Weak", IsVisible: true, AccessPoints: []AccessPoint{{Strength: 10}}},
				{SSID: "Strong", IsVisible: true, AccessPoints: []AccessPoint{{Strength: 90}}},
			},
			expected: []Connection{
				{SSID: "Strong", IsVisible: true, AccessPoints: []AccessPoint{{Strength: 90}}},
				{SSID: "Weak", IsVisible: true, AccessPoints: []AccessPoint{{Strength: 10}}},
			},
		},
		{
			name: "Sort by last connected",
			connections: []Connection{
				{SSID: "TwoDaysAgo", IsVisible: false, LastConnected: &twoDaysAgo},
				{SSID: "Yesterday", IsVisible: false, LastConnected: &yesterday},
				{SSID: "Never", IsVisible: false, LastConnected: nil},
			},
			expected: []Connection{
				{SSID: "Yesterday", IsVisible: false, LastConnected: &yesterday},
				{SSID: "TwoDaysAgo", IsVisible: false, LastConnected: &twoDaysAgo},
				{SSID: "Never", IsVisible: false, LastConnected: nil},
			},
		},
		{
			name: "Sort by SSID",
			connections: []Connection{
				{SSID: "B"},
				{SSID: "A"},
			},
			expected: []Connection{
				{SSID: "A"},
				{SSID: "B"},
			},
		},
		{
			name: "Complex sort",
			connections: []Connection{
				{SSID: "Known B", IsVisible: false, LastConnected: &twoDaysAgo},
				{SSID: "Visible Weak", IsVisible: true, AccessPoints: []AccessPoint{{Strength: 20}}},
				{SSID: "Active WiFi", IsActive: true, IsVisible: true, AccessPoints: []AccessPoint{{Strength: 80}}},
				{SSID: "Known A", IsVisible: false, LastConnected: &yesterday},
				{SSID: "Visible Strong", IsVisible: true, AccessPoints: []AccessPoint{{Strength: 90}}},
			},
			expected: []Connection{
				{SSID: "Active WiFi", IsActive: true, IsVisible: true, AccessPoints: []AccessPoint{{Strength: 80}}},
				{SSID: "Visible Strong", IsVisible: true, AccessPoints: []AccessPoint{{Strength: 90}}},
				{SSID: "Visible Weak", IsVisible: true, AccessPoints: []AccessPoint{{Strength: 20}}},
				{SSID: "Known A", IsVisible: false, LastConnected: &yesterday},
				{SSID: "Known B", IsVisible: false, LastConnected: &twoDaysAgo},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SortConnections(tt.connections)
			if !reflect.DeepEqual(tt.connections, tt.expected) {
				t.Errorf("SortConnections() got = %v, want %v", tt.connections, tt.expected)
			}
		})
	}
}
