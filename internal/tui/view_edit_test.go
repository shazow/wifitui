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

	if m.passwordAdapter.Model.EchoMode != textinput.EchoPassword {
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

func TestFullWorkflow_JoinAndEdit(t *testing.T) {
	// This is a more comprehensive test that simulates a fuller user workflow
	// to catch subtle state management bugs.

	// 1. Setup
	b, _ := mock.New()
	m, _ := NewModel(b)
	ssid := "GET off my LAN"
	password := "password123"

	// 2. Simulate joining the network by calling the command directly.
	joinCmd := joinNetwork(b, ssid, password, backend.SecurityWPA, false)
	msg := joinCmd()
	if _, ok := msg.(errorMsg); ok {
		t.Fatal("joinNetwork command failed")
	}

	// 3. Process the connectionSavedMsg. This should return a refreshNetworks command.
	newModel, refreshCmd := m.Update(msg)
	if refreshCmd == nil {
		t.Fatal("expected a refresh command after saving a connection, but got nil")
	}

	// 4. Execute the refreshNetworks command. This returns a connectionsLoadedMsg.
	msg = refreshCmd()
	if _, ok := msg.(errorMsg); ok {
		t.Fatal("refreshNetworks command failed")
	}
	newModel, _ = newModel.Update(msg) // Now the model's list should be updated.

	// 5. Find and select the newly joined network in the model's list.
	m = newModel.(*model)
	itemIndex := -1
	for i, item := range m.list.Items() {
		if item.(connectionItem).SSID == ssid {
			itemIndex = i
			break
		}
	}
	if itemIndex == -1 {
		t.Fatalf("could not find joined network '%s' in the list", ssid)
	}
	m.list.Select(itemIndex)
	if !m.list.SelectedItem().(connectionItem).IsKnown {
		t.Fatal("network in list should be marked as known, but it is not")
	}

	// 6. Simulate the user pressing 'enter' by calling updateListView.
	// This should return a getSecrets command.
	newModel, getSecretsCmd := m.updateListView(tea.KeyMsg{Type: tea.KeyEnter})
	if getSecretsCmd == nil {
		t.Fatal("expected a command to get secrets, but got nil")
	}

	// 7. Execute the getSecrets command. This returns a secretsLoadedMsg.
	msg = getSecretsCmd()
	if _, ok := msg.(errorMsg); ok {
		t.Fatalf("getSecrets command failed: %v", msg.(errorMsg).err)
	}

	// 8. Process the secretsLoadedMsg. This should update the password field.
	finalModel, _ := newModel.Update(msg)
	m = finalModel.(*model)

	// 9. Check the final state.
	if m.passwordInput.Value() != password {
		t.Errorf("expected password input to have value '%s', but got '%s'",
			password, m.passwordInput.Value())
	}
	if m.state != stateEditView {
		t.Errorf("expected final state to be stateEditView, but got %v", m.state)
	}
}

func TestTypingInPasswordField_IsSaved(t *testing.T) {
	// 1. Setup
	b, _ := mock.New()
	m, _ := NewModel(b)
	ssid := "GET off my LAN"
	password := "typed-password"

	// 2. Navigate to the edit view for an unknown network
	m.state = stateEditView
	m.selectedItem = connectionItem{Connection: backend.Connection{SSID: ssid, Security: backend.SecurityWPA}}
	m.setupEditView()

	// 3. Simulate focusing the password field by tabbing to it
	// In this view, the password field is the first focusable element.
	if m.editFocusManager.Focused() != m.passwordAdapter {
		t.Fatalf("Password field was not focused by default")
	}

	// 4. Simulate typing a password
	var newModel tea.Model = m
	var cmd tea.Cmd
	for _, char := range password {
		newModel, cmd = newModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{char}})
		if cmd != nil {
			// handle commands if any, though typing shouldn't produce them
			newModel.Update(cmd())
		}
	}
	m = newModel.(*model)

	// Assert that the adapter's model has the value
	if m.passwordAdapter.Model.Value() != password {
		t.Fatalf("password was not typed into adapter correctly, got: %q", m.passwordAdapter.Model.Value())
	}

	// 5. Simulate focusing the "Join" button
	for m.editFocusManager.Focused() != m.buttonGroup {
		newModel, _ = m.updateEditView(tea.KeyMsg{Type: tea.KeyTab})
		m = newModel.(*model)
	}
	// Make sure "Join" is selected (it's the first button)
	m.buttonGroup.selected = 0

	// 6. Simulate clicking "Join"
	newModel, joinCmd := m.updateEditView(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(*model)
	if joinCmd == nil {
		t.Fatal("expected a join command")
	}

	// 7. Execute the command. It should return a `connectionSavedMsg`.
	msg := joinCmd()
	if _, ok := msg.(connectionSavedMsg); !ok {
		t.Fatalf("expected connectionSavedMsg, got %T", msg)
	}

	// 8. Now, check the backend state directly
	secret, err := b.GetSecrets(ssid)
	if err != nil {
		t.Fatalf("GetSecrets failed: %v", err)
	}
	if secret != password {
		t.Errorf("expected secret in backend to be '%s', but got '%s'", password, secret)
	}
}
