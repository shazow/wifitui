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