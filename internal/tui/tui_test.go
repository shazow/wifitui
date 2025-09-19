package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shazow/wifitui/wifi"
	"github.com/shazow/wifitui/wifi/mock"
	"github.com/stretchr/testify/require"
)

func TestTUI_EnableWirelessFromDisabled(t *testing.T) {
	backend, err := mock.New()
	require.NoError(t, err)

	mockBackend := backend.(*mock.MockBackend)
	mockBackend.WirelessEnabled = false
	mockBackend.ConnectSleep = 0 // for faster tests

	m, err := NewModel(backend)
	require.NoError(t, err)

	// Initial load should result in wireless disabled error
	initCmd := m.Init()
	require.NotNil(t, initCmd)

	// The command is a batch, so we expect a BatchMsg from running it.
	batchMsg := initCmd()
	cmds, ok := batchMsg.(tea.BatchMsg) // tea.BatchMsg is []tea.Cmd
	require.True(t, ok, "expected a batch message")

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
	require.True(t, found, "expected to find an error message in batch")
	require.True(t, errors.Is(errMsg.err, wifi.ErrWirelessDisabled), "Expected wireless disabled error")

	// Update the model with the error, should push the disabled view
	mUpdated, _ := m.Update(errMsg)
	m = mUpdated.(*model)

	require.Len(t, m.componentStack, 2, "component stack should have 2 items: list, disabled")
	_, ok = m.componentStack[1].(*WirelessDisabledModel)
	require.True(t, ok, "Top of stack should be WirelessDisabledModel")


	// Press 'r' to re-enable wireless
	rKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}
	mUpdated, rCmd := m.Update(rKeyMsg)
	m = mUpdated.(*model)

	// After pressing 'r', we should get a batch command.
	require.NotNil(t, rCmd, "a command should be returned")

	// Execute the command, we should get a batch of commands
	batch := rCmd()
	cmds, ok = batch.(tea.BatchMsg)
	require.True(t, ok, "expected a batch message")

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
	require.True(t, hasPop, "batch should contain popViewMsg")
	require.True(t, hasScan, "batch should contain scanMsg")

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
	require.Len(t, m.componentStack, 1, "component stack should have 1 item")
	_, ok = m.componentStack[0].(*ListModel)
	require.True(t, ok, "Top of stack should be ListModel")
}
