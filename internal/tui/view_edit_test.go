package tui

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
	m, err := NewModel(b)
	if err != nil {
		t.Fatalf("failed to create initial model: %v", err)
	}

	// Set the state to edit view
	m.state = stateEditView
	m.selectedItem = connectionItem{} // A default item
	m.setupEditView()

	// Create an escape key message
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}

	// The press of 'esc' should switch the view back to the list view
	updatedModel, _ := m.updateEditView(escMsg)
	m = updatedModel.(model)

	// Assert the state changed back to list view
	if m.state != stateListView {
		t.Errorf("expected state to be 'stateListView' after escape, but got %v", m.state)
	}
}

func TestUpdateEditView_TabKey(t *testing.T) {
	// Initialize the model
	b, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	m, err := NewModel(b)
	if err != nil {
		t.Fatalf("failed to create initial model: %v", err)
	}

	// Set the state to edit view for a new network
	m.state = stateEditView
	m.selectedItem = connectionItem{} // A new network
	m.setupEditView()

	// Get the initial focused element
	initialFocus := m.editFocusManager.Focused()

	// Create a tab key message
	tabMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")}

	// Update the model with the tab key press
	updatedModel, _ := m.updateEditView(tabMsg)
	m = updatedModel.(model)

	// Get the new focused element
	newFocus := m.editFocusManager.Focused()

	// Assert that the focus has changed
	if newFocus == initialFocus {
		t.Errorf("expected focus to change after pressing tab, but it did not")
	}
}
