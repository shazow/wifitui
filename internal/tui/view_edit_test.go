package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/shazow/wifitui/backend"
	"github.com/shazow/wifitui/backend/mock"
)

func TestUpdateEditView_TabKey(t *testing.T) {
	b, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	m, err := NewModel(b)
	if err != nil {
		t.Fatalf("failed to create initial model: %v", err)
	}

	m.state = stateEditView
	m.selectedItem = connectionItem{} // New network
	m.setupEditView()

	initialFocus := m.editFocusManager.Focused()
	tabMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")}

	updatedModel, _ := m.updateEditView(tabMsg)
	m = updatedModel.(*model)

	newFocus := m.editFocusManager.Focused()
	if newFocus == initialFocus {
		t.Errorf("expected focus to change after pressing tab")
	}
}

func TestUpdateEditView_PasswordReveal(t *testing.T) {
	b, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	m, err := NewModel(b)
	if err != nil {
		t.Fatalf("failed to create initial model: %v", err)
	}

	m.state = stateEditView
	m.selectedItem = connectionItem{
		Connection: backend.Connection{IsKnown: true, IsSecure: true},
	}
	m.passwordInput.SetValue("password")
	m.setupEditView()

	// Focus the password field
	for m.editFocusManager.Focused() != m.passwordAdapter {
		updatedModel, _ := m.updateEditView(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
		m = updatedModel.(*model)
	}

	if m.passwordInput.EchoMode != textinput.EchoNormal {
		t.Errorf("expected password to be revealed on focus")
	}

	// Blur the password field
	updatedModel, _ := m.updateEditView(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	m = updatedModel.(*model)

	if m.passwordInput.EchoMode != textinput.EchoPassword {
		t.Errorf("expected password to be hidden on blur")
	}
}

func TestUpdateEditView_CancelButton(t *testing.T) {
	b, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	m, err := NewModel(b)
	if err != nil {
		t.Fatalf("failed to create initial model: %v", err)
	}

	m.state = stateEditView
	m.selectedItem = connectionItem{} // New network
	m.setupEditView()

	// Focus the buttons
	for m.editFocusManager.Focused() != m.buttonGroup {
		updatedModel, _ := m.updateEditView(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
		m = updatedModel.(*model)
	}

	// Select the cancel button
	m.buttonGroup.selected = 1
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, cmd := m.updateEditView(enterMsg)
	m = updatedModel.(*model)

	// Check that the correct message was sent
	msg := cmd()
	if _, ok := msg.(changeViewMsg); !ok {
		t.Errorf("expected a changeViewMsg but got %T", msg)
	}
}
