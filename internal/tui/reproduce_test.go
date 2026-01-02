package tui

import (
	"testing"

	"github.com/shazow/wifitui/wifi"
	"github.com/shazow/wifitui/wifi/mock"
)

func TestMultipleAccessPointsDisplay(t *testing.T) {
	// Create a mock backend with aggregated APs
	b, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	mb := b.(*mock.MockBackend)

	// Configure specific connections
	mb.VisibleConnections = []wifi.Connection{
		{
			SSID:      "MeshNetwork",
			IsVisible: true,
			AccessPoints: []wifi.AccessPoint{
				{Strength: 80},
				{Strength: 50},
				{Strength: 90},
			},
		},
		{
			SSID:      "SingleAP",
			IsVisible: true,
			AccessPoints: []wifi.AccessPoint{
				{Strength: 40},
			},
		},
	}
	mb.KnownConnections = nil // Reset known connections to avoid interference
	mb.ActionSleep = 0
	mb.DisableRandomization = true

	// Initialize the list model
	model := NewListModel()

	// Simulate loading connections
	conns, _ := mb.BuildNetworkList(true)
	msg := connectionsLoadedMsg(conns)
	model.Update(msg)

	items := model.list.Items()
	// We expect 2 items: MeshNetwork and SingleAP
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

				// Let's verify the underlying data structure is correct
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
