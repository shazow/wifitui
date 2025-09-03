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

	// Set the state to edit view, with focus on an input field
	m.state = stateEditView
	m.selectedItem = connectionItem{} // A default item
	m.editFocus = focusInput          // Start with focus on the password input

	// Create an escape key message
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}

	// The first press of 'esc' should move focus from the input to the buttons
	updatedModel, _ := m.updateEditView(escMsg)
	m = updatedModel.(model)

	if m.state != stateEditView {
		t.Fatalf("expected state to remain 'editView' after first escape, but got %v", m.state)
	}
	if m.editFocus != focusButtons {
		t.Fatalf("expected focus to move to 'focusButtons' after first escape, but got %v", m.editFocus)
	}

	// The second press of 'esc' should switch the view back to the list view
	updatedModel, _ = m.updateEditView(escMsg)
	m = updatedModel.(model)

	// Assert the state changed back to list view
	if m.state != stateListView {
		t.Errorf("expected state to be 'stateListView' after second escape, but got %v", m.state)
	}
}
