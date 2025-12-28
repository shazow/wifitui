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
	// Disable signal strength randomization to ensure deterministic bug reproduction
	mb.DisableRandomization = true

	// 2. Init Model
	m, err := NewModel(mb)
	if err != nil {
		t.Fatalf("NewModel failed: %v", err)
	}

	// 3. Trigger Scan (Initial)
	conns, err := mb.BuildNetworkList(true)
	if err != nil {
		t.Fatalf("BuildNetworkList failed: %v", err)
	}
	msg := scanFinishedMsg(conns)
	_, _ = m.Update(msg)

	// Helper function to check for duplicates
	checkForDuplicates := func(label string) {
		top := m.stack.Top()
		lm, ok := top.(*ListModel)
		if !ok {
			t.Fatalf("[%s] Top component is not ListModel, got %T", label, top)
		}
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
		if count < 2 {
			t.Errorf("[%s] Expected at least 2 connections for SSID %q, got %d", label, ssid, count)
		} else {
			t.Logf("[%s] Found %d duplicates for SSID %q", label, count, ssid)
		}
	}

	// Verify duplicates after first scan
	checkForDuplicates("First Scan")

	// 4. Trigger Rescan
	// In the real app, this happens via a timer or keypress. Here we simulate the result.
	// Since we disabled randomization, the strengths should remain 50 and 80.
	conns2, err := mb.BuildNetworkList(true)
	if err != nil {
		t.Fatalf("BuildNetworkList (Rescan) failed: %v", err)
	}
	msg2 := scanFinishedMsg(conns2)
	_, _ = m.Update(msg2)

	// Verify duplicates persist after rescan
	checkForDuplicates("Rescan")
}
