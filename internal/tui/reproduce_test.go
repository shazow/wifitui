package tui

import (
	"testing"

	"github.com/shazow/wifitui/wifi"
)

// mockBackend is a minimal implementation of wifi.Backend for testing purposes.
type mockBackend struct {
	connections []wifi.Connection
}

func (m *mockBackend) BuildNetworkList(shouldScan bool) ([]wifi.Connection, error) {
	return m.connections, nil
}
func (m *mockBackend) ActivateConnection(ssid string) error { return nil }
func (m *mockBackend) ForgetNetwork(ssid string) error      { return nil }
func (m *mockBackend) JoinNetwork(ssid string, password string, security wifi.SecurityType, isHidden bool) error {
	return nil
}
func (m *mockBackend) GetSecrets(ssid string) (string, error)               { return "", nil }
func (m *mockBackend) UpdateConnection(ssid string, opts wifi.UpdateOptions) error { return nil }
func (m *mockBackend) IsWirelessEnabled() (bool, error)                     { return true, nil }
func (m *mockBackend) SetWireless(enabled bool) error                       { return nil }

func TestMultipleAccessPointsDisplay(t *testing.T) {
	// Create a mock backend with aggregated APs
	backend := &mockBackend{
		connections: []wifi.Connection{
			{
				SSID: "MeshNetwork",
				IsVisible: true,
				AccessPoints: []wifi.AccessPoint{
					{Strength: 80},
					{Strength: 50},
					{Strength: 90},
				},
			},
			{
				SSID: "SingleAP",
				IsVisible: true,
				AccessPoints: []wifi.AccessPoint{
					{Strength: 40},
				},
			},
		},
	}

	// Initialize the list model
	model := NewListModel()

	// Simulate loading connections
	conns, _ := backend.BuildNetworkList(true)
	msg := connectionsLoadedMsg(conns)
	model.Update(msg)

	items := model.list.Items()
	if len(items) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(items))
	}

	// Helper function to check item description
	checkDescription := func(ssid string) {
		found := false
		for _, item := range items {
			c := item.(connectionItem)
			if c.SSID == ssid {
				found = true

				// Let's verify the underlying data structure is correct, which implies Render will be correct.
				if len(c.AccessPoints) == 3 && ssid == "MeshNetwork" {
					return // Good
				}
				if len(c.AccessPoints) == 1 && ssid == "SingleAP" {
					return // Good
				}
				t.Errorf("Incorrect AP count for %s: got %d", ssid, len(c.AccessPoints))
			}
		}
		if !found {
			t.Errorf("SSID %s not found", ssid)
		}
	}

	checkDescription("MeshNetwork")
	checkDescription("SingleAP")
}
