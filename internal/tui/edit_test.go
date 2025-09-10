package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/shazow/wifitui/wifi"
)

func TestEditModel_TabKey(t *testing.T) {
	m := NewEditModel(nil) // New network

	initialFocus := m.focusManager.Focused()
	tabMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")}

	updatedModel, _ := m.Update(tabMsg)
	m = updatedModel.(EditModel)

	newFocus := m.focusManager.Focused()
	if newFocus == initialFocus {
		t.Errorf("expected focus to change after pressing tab")
	}
}

func TestEditModel_PasswordReveal(t *testing.T) {
	item := &connectionItem{
		Connection: wifi.Connection{IsKnown: true, IsSecure: true},
	}
	m := NewEditModel(item)
	m.passwordAdapter.Model.SetValue("password")

	// Focus the password field
	for m.focusManager.Focused() != m.passwordAdapter {
		updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
		m = updatedModel.(EditModel)
	}

	if m.passwordAdapter.Model.EchoMode != textinput.EchoNormal {
		t.Errorf("expected password to be revealed on focus")
	}

	// Blur the password field
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	m = updatedModel.(EditModel)

	if m.passwordAdapter.Model.EchoMode != textinput.EchoPassword {
		t.Errorf("expected password to be hidden on blur")
	}
}

func TestEditModel_CancelButton(t *testing.T) {
	m := NewEditModel(nil) // New network

	// Focus the buttons
	for m.focusManager.Focused() != m.buttonGroup {
		updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
		m = updatedModel.(EditModel)
	}

	// Select the cancel button
	m.buttonGroup.selected = 1
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(enterMsg)

	// Check that the correct message was sent
	msg := cmd()
	if _, ok := msg.(changeViewMsg); !ok {
		t.Errorf("expected a changeViewMsg but got %T", msg)
	}
}

func TestEditModel_ForgetFlow(t *testing.T) {
	item := &connectionItem{
		Connection: wifi.Connection{SSID: "TestNetwork", IsKnown: true},
	}
	m := NewEditModel(item)

	// Focus the buttons
	for m.focusManager.Focused() != m.buttonGroup {
		updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
		m = updatedModel.(EditModel)
	}

	// Select the forget button
	m.buttonGroup.selected = 2
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, cmd := m.Update(enterMsg)
	m = updatedModel.(EditModel)

	msg := cmd()
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(EditModel)

	if !m.isForgetting {
		t.Fatal("isForgetting was not set to true")
	}

	// Press 'n' to cancel
	nKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	updatedModel, _ = m.Update(nKeyMsg)
	m = updatedModel.(EditModel)

	if m.isForgetting {
		t.Fatal("isForgetting was not set to false after pressing 'n'")
	}

	// Select the forget button again
	updatedModel, cmd = m.Update(enterMsg)
	m = updatedModel.(EditModel)

	msg = cmd()
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(EditModel)

	// Press 'y' to confirm
	yKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}
	updatedModel, cmd = m.Update(yKeyMsg)
	m = updatedModel.(EditModel)

	if m.isForgetting {
		t.Fatal("isForgetting was not set to false after pressing 'y'")
	}

	msg = cmd()
	if forgetMsg, ok := msg.(forgetNetworkMsg); !ok {
		t.Errorf("expected a forgetNetworkMsg but got %T", msg)
	} else {
		if forgetMsg.item.SSID != "TestNetwork" {
			t.Errorf("expected forgetNetworkMsg for 'TestNetwork' but got for '%s'", forgetMsg.item.SSID)
		}
	}
}
