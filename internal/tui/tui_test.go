package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shazow/wifitui/wifi"
	"github.com/shazow/wifitui/wifi/mock"
)

func TestTuiModel_ScanFinishedUpdatesList(t *testing.T) {
	backend, err := mock.New()
	if err != nil {
		t.Fatalf("mock.New() failed: %v", err)
	}
	connections := []wifi.Connection{
		{SSID: "TestNet1"},
		{SSID: "TestNet2"},
	}

	m, err := NewModel(backend)
	if err != nil {
		t.Fatalf("NewModel failed: %v", err)
	}

	// Set a size for the model, otherwise the list component won't have enough space to render.
	sizeMsg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updatedModel, _ := m.Update(sizeMsg)
	m = updatedModel.(*model)

	// Simulate a scan finishing
	scanMsg := scanFinishedMsg(connections)
	updatedModel, _ = m.Update(scanMsg)
	m = updatedModel.(*model)

	// Check the view
	view := m.View()

	if !strings.Contains(view, "TestNet1") {
		t.Errorf("View does not contain 'TestNet1' in\n%s", view)
	}
	if !strings.Contains(view, "TestNet2") {
		t.Errorf("View does not contain 'TestNet2' in\n%s", view)
	}
}

func TestTuiModel_EnableRadioSwitchesView(t *testing.T) {
	backend, err := mock.New()
	if err != nil {
		t.Fatalf("mock.New() failed: %v", err)
	}

	m, err := NewModel(backend)
	if err != nil {
		t.Fatalf("NewModel failed: %v", err)
	}

	// Manually trigger the disabled state
	disabledMsg := errorMsg{err: wifi.ErrWirelessDisabled}
	updatedModel, _ := m.Update(disabledMsg)
	m = updatedModel.(*model)

	// Verify we are in the disabled view
	view := m.View()
	if !strings.Contains(view, "Wi-Fi is disabled.") {
		t.Fatalf("View does not contain 'Wi-Fi is disabled.' in\n%s", view)
	}

	// Now, pop the view. This is what happens when the radio is enabled.
	// The OnLeave hook should take care of the rest.
	updatedModel, cmd := m.Update(popViewMsg{})
	m = updatedModel.(*model)

	// The batch command contains a command to start the scanner and a command to do an initial scan.
	// Let's execute the commands and process the resulting messages.
	batchCmd := cmd().(tea.BatchMsg)
	var scanCmd tea.Cmd
	for _, c := range batchCmd {
		msg := c()
		// We only care about the scanMsg for this test's purpose.
		if _, ok := msg.(scanMsg); ok {
			updatedModel, scanCmd = m.Update(msg)
			m = updatedModel.(*model)
			break
		}
	}

	if scanCmd == nil {
		t.Fatal("did not find a scan command in the batch")
	}

	// Now execute the command that builds the network list
	scanFinishedMsg := scanCmd()
	updatedModel, _ = m.Update(scanFinishedMsg)
	m = updatedModel.(*model)

	// And we need to give the list a size
	sizeMsg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updatedModel, _ = m.Update(sizeMsg)
	m = updatedModel.(*model)

	view = m.View()
	if strings.Contains(view, "Wi-Fi is disabled.") {
		t.Errorf("View still contains 'Wi-Fi is disabled.' after enabling radio in\n%s", view)
	}
	// The mock backend will return its default list of networks.
	if !strings.Contains(view, "WiFi Network") {
		t.Errorf("View does not contain network list title after enabling radio in\n%s", view)
	}
}

