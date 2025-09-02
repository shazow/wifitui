package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shazow/wifitui/backend/mock"
)

func TestUpdateEditView_EscapeKey(t *testing.T) {
	// Initialize the model
	b, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	m, err := initialModel(b)
	if err != nil {
		t.Fatalf("failed to create initial model: %v", err)
	}

	// Set the state to edit view, with focus on an input field
	m.state = stateEditView
	m.selectedItem = connectionItem{} // A default item
	m.buttons = []*Button{
		NewButton("Join", 0, nil),
		NewButton("Cancel", 1, nil),
	}
	m.buttonFocusManager.SetComponents(Focusables(m.buttons)...)
	m.focusManager.SetComponents(m.passwordInput, m.buttonFocusManager)
	m.focusManager.Focus()

	// Create an escape key message
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}

	// The first press of 'esc' should blur the focus manager
	updatedModel, _ := m.updateEditView(escMsg)
	m, _ = updatedModel.(model)

	if m.state != stateEditView {
		t.Fatalf("expected state to remain 'editView' after first escape, but got %v", m.state)
	}
	if focused := m.focusManager.FocusedComponent(); focused != nil {
		t.Fatalf("expected focus manager to be blurred, but component %v is focused", focused)
	}

	// The second press of 'esc' should switch the view back to the list view
	updatedModel, _ = m.updateEditView(escMsg)
	m = updatedModel.(model)

	// Assert the state changed back to list view
	if m.state != stateListView {
		t.Errorf("expected state to be 'stateListView' after second escape, but got %v", m.state)
	}
}

func TestUpdateEditView_TabKey(t *testing.T) {
	// Initialize the model
	b, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	m, err := initialModel(b)
	if err != nil {
		t.Fatalf("failed to create initial model: %v", err)
	}

	// Set the state to edit view for a new network
	m.state = stateEditView
	m.selectedItem = connectionItem{} // New network
	m.buttons = []*Button{
		NewButton("Join", 0, nil),
		NewButton("Cancel", 1, nil),
	}
	m.buttonFocusManager.SetComponents(Focusables(m.buttons)...)
	m.focusManager.SetComponents(m.ssidInput, m.passwordInput, m.securityGroup, m.buttonFocusManager)
	m.focusManager.Focus()

	tabMsg := tea.KeyMsg{Type: tea.KeyTab}

	// Tab from SSID to Password
	updatedModel, _ := m.updateEditView(tabMsg)
	m, _ = updatedModel.(model)
	if focused := m.focusManager.FocusedComponent(); focused != m.passwordInput {
		t.Fatalf("expected focus to move to password input, but got %v", focused)
	}

	// Tab from Password to Security
	updatedModel, _ = m.updateEditView(tabMsg)
	m, _ = updatedModel.(model)
	if focused := m.focusManager.FocusedComponent(); focused != m.securityGroup {
		t.Fatalf("expected focus to move to security group, but got %v", focused)
	}

	// Tab from Security to Buttons
	updatedModel, _ = m.updateEditView(tabMsg)
	m, _ = updatedModel.(model)
	if focused := m.focusManager.FocusedComponent(); focused != m.buttonFocusManager {
		t.Fatalf("expected focus to move to button focus manager, but got %v", focused)
	}

	// Tab from Buttons back to SSID
	updatedModel, _ = m.updateEditView(tabMsg)
	m, _ = updatedModel.(model)
	if focused := m.focusManager.FocusedComponent(); focused != m.ssidInput {
		t.Fatalf("expected focus to move to ssid input, but got %v", focused)
	}
}
