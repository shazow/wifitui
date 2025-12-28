package tui

import (
	"testing"

	"github.com/shazow/wifitui/wifi"
	"github.com/shazow/wifitui/wifi/mock"
)

func TestDuplicateEntriesInList(t *testing.T) {
	// 1. Setup Backend with duplicates
	b, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	mb := b.(*mock.MockBackend)
	// Configure specific duplicates
	ssid := "DupNet"
	mb.VisibleConnections = []wifi.Connection{
		{SSID: ssid, Strength: 50, IsActive: true},
		{SSID: ssid, Strength: 80, IsActive: true},
	}
	// Ensure no delay for tests
	mb.ActionSleep = 0

	// 2. Init Model
	m, err := NewModel(mb)
	if err != nil {
		t.Fatalf("NewModel failed: %v", err)
	}

	// 3. Trigger Scan
	// Directly simulate the flow:
	// A. Backend returns a list (with duplicates due to the bug).
	conns, err := mb.BuildNetworkList(true)
	if err != nil {
		t.Fatalf("BuildNetworkList failed: %v", err)
	}

	// B. Main model receives scanFinishedMsg
	msg := scanFinishedMsg(conns)

	// C. Update the model
	_, _ = m.Update(msg)

	// 4. Inspect ListModel
	top := m.stack.Top()
	lm, ok := top.(*ListModel)
	if !ok {
		t.Fatalf("Top component is not ListModel, got %T", top)
	}

	// Access list items using the public Items() method of the internal list.Model
	// Note: lm.list is not exported, but we are in package tui.
	items := lm.list.Items()

	count := 0
	for _, item := range items {
		ci, ok := item.(connectionItem)
		if !ok {
			continue
		}
		if ci.SSID == ssid {
			count++
		}
	}

	// We expect duplicates now because of the simulated bug
	if count < 2 {
		t.Errorf("Expected at least 2 connections for SSID %q (reproducing bug), got %d", ssid, count)
	} else {
		t.Logf("Successfully reproduced bug: Found %d duplicates for SSID %q", count, ssid)
	}
}
