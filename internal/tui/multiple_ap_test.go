package tui

import (
	"testing"

	"github.com/shazow/wifitui/wifi"
	"github.com/shazow/wifitui/wifi/mock"
)

func TestMultipleAPs(t *testing.T) {
	// Setup mock backend
	backend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	mb := backend.(*mock.MockBackend)

	// Verify we have a network with multiple APs (as per updated mock fixture)
	conns, err := mb.BuildNetworkList(true)
	if err != nil {
		t.Fatalf("BuildNetworkList failed: %v", err)
	}

	var multiAPConn *wifi.Connection
	for _, c := range conns {
		if c.SSID == "MultiAPTestNetwork" {
			// In BuildNetworkList we iterate range, so c is a copy, but AccessPoints is a slice so it refs the same underlying array
			// But careful with pointer semantics in loops.
			// Let's use index or just check fields.
			if len(c.AccessPoints) > 1 {
				multiAPConn = &c
				break
			}
		}
	}

	if multiAPConn == nil {
		t.Fatal("Expected to find network 'MultiAPTestNetwork' with multiple APs")
	}

	if len(multiAPConn.AccessPoints) != 2 {
		t.Errorf("Expected 2 APs, got %d", len(multiAPConn.AccessPoints))
	}

	// Verify EditModel shows AP selection
	item := connectionItem{Connection: *multiAPConn}
	editModel := NewEditModel(&item)

	if editModel.apSelection == nil {
		t.Error("EditModel should have apSelection component for multiple APs")
	} else {
		// Verify options
		// Default + 2 APs = 3 options
		// Note: choice component does not expose options publicly, but we can verify via View() output or changing component to expose it.
		// For now let's assume if it's not nil, it's created.
		// We can check if View contains the BSSIDs
		view := editModel.View()
		// BSSIDs from mock.go: "11:11:11:11:11:11", "22:22:22:22:22:22"

		// The ChoiceComponent renders selected item bold.
		// We can't easily parse TUI output without being very brittle.
		// But we can check if the BSSIDs are present in the string.
		if !contains(view, "11:11:11:11:11:11") {
			t.Error("EditModel view should contain BSSID 11:11:11:11:11:11")
		}
		if !contains(view, "22:22:22:22:22:22") {
			t.Error("EditModel view should contain BSSID 22:22:22:22:22:22")
		}
	}

	// Test activation with specific BSSID
	targetBSSID := "22:22:22:22:22:22"
	err = mb.ActivateConnection("MultiAPTestNetwork", targetBSSID)
	if err != nil {
		t.Errorf("Failed to activate with valid BSSID: %v", err)
	}

	// Test activation with invalid BSSID
	err = mb.ActivateConnection("MultiAPTestNetwork", "FF:FF:FF:FF:FF:FF")
	if err == nil {
		t.Error("Expected error when activating with invalid BSSID, got nil")
	}
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
