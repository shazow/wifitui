package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shazow/wifitui/wifi"
	"github.com/shazow/wifitui/wifi/mock"
)

func TestTUI_EnableWirelessFromDisabled(t *testing.T) {
	backend, err := mock.New()
	if err != nil {
		t.Fatalf("mock.New() failed: %v", err)
	}

	mockBackend := backend.(*mock.MockBackend)
	mockBackend.WirelessEnabled = false
	mockBackend.ConnectSleep = 0 // for faster tests

	m, err := NewModel(backend)
	if err != nil {
		t.Fatalf("NewModel() failed: %v", err)
	}

	// Initial load should result in wireless disabled error
	initCmd := m.Init()
	if initCmd == nil {
		t.Fatal("Init() returned nil command")
	}

	// The command is a batch, so we expect a BatchMsg from running it.
	batchMsg := initCmd()
	cmds, ok := batchMsg.(tea.BatchMsg) // tea.BatchMsg is []tea.Cmd
	if !ok {
		t.Fatalf("expected a batch message, got %T", batchMsg)
	}

	var errMsg errorMsg
	found := false
	for _, cmd := range cmds {
		msg := cmd()
		if e, ok := msg.(errorMsg); ok {
			errMsg = e
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected to find an error message in batch")
	}
	if !errors.Is(errMsg.err, wifi.ErrWirelessDisabled) {
		t.Fatal("Expected wireless disabled error")
	}

	// Update the model with the error, should push the disabled view
	mUpdated, _ := m.Update(errMsg)
	m = mUpdated.(*model)

	if len(m.componentStack) != 2 {
		t.Fatalf("expected component stack to have 2 items, got %d", len(m.componentStack))
	}
	if _, ok := m.componentStack[1].(*WirelessDisabledModel); !ok {
		t.Fatal("Top of stack should be WirelessDisabledModel")
	}


	// Press 'r' to re-enable wireless
	rKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}
	mUpdated, rCmd := m.Update(rKeyMsg)
	m = mUpdated.(*model)

	// After pressing 'r', we should get a batch command.
	if rCmd == nil {
		t.Fatal("expected a command to be returned")
	}

	// Execute the command, we should get a batch of commands
	batch := rCmd()
	cmds, ok = batch.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected a batch message, got %T", batch)
	}

	// The batch should contain a popViewMsg and a scanMsg.
	var hasPop, hasScan bool
	for _, cmd := range cmds {
        msg := cmd()
		if _, ok := msg.(popViewMsg); ok {
			hasPop = true
		}
		if _, ok := msg.(scanMsg); ok {
			hasScan = true
		}
	}
	if !hasPop {
		t.Fatal("batch should contain popViewMsg")
	}
	if !hasScan {
		t.Fatal("batch should contain scanMsg")
	}

	// Process the messages
	mUpdated, _ = m.Update(popViewMsg{})
	m = mUpdated.(*model)
	mUpdated, scanCmd := m.Update(scanMsg{})
	m = mUpdated.(*model)

	// Process the command from the scan
	scanResult := scanCmd()
	mUpdated, _ = m.Update(scanResult)
	m = mUpdated.(*model)

	// After all updates, the disabled view should be gone.
	if len(m.componentStack) != 1 {
		t.Fatalf("expected component stack to have 1 item, got %d", len(m.componentStack))
	}
	if _, ok := m.componentStack[0].(*ListModel); !ok {
		t.Fatal("Top of stack should be ListModel")
	}
}