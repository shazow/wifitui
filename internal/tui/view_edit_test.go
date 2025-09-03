package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shazow/wifitui/backend/mock"
)

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
	m = updatedModel.(*model)

	// Get the new focused element
	newFocus := m.editFocusManager.Focused()

	// Assert that the focus has changed
	if newFocus == initialFocus {
		t.Errorf("expected focus to change after pressing tab, but it did not")
	}
}

func TestUpdateEditView_CancelButton(t *testing.T) {
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

	// Focus the button group
	for m.editFocusManager.Focused() != m.buttonGroup {
		m.editFocusManager.Next()
	}

	// Select the cancel button (always the last one for a new network)
	m.buttonGroup.selected = 1

	// Create an enter key message
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}

	// Update the model with the enter key press
	updatedModel, _ := m.updateEditView(enterMsg)
	m = updatedModel.(*model)

	// Assert that the state has changed to list view
	if m.state != stateListView {
		t.Errorf("expected state to change to stateListView after pressing cancel, but got %v", m.state)
	}
}
