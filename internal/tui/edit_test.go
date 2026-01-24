package tui

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/shazow/wifitui/wifi"
	"github.com/shazow/wifitui/wifi/mock"
)

func TestEditModel_TabKey(t *testing.T) {
	m := NewEditModel(nil) // New network

	initialFocus := m.focusManager.Focused()
	tabMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")}

	m.Update(tabMsg)

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
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	}

	if m.passwordAdapter.Model.EchoMode != textinput.EchoNormal {
		t.Errorf("expected password to be revealed on focus")
	}

	// Blur the password field
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})

	if m.passwordAdapter.Model.EchoMode != textinput.EchoPassword {
		t.Errorf("expected password to be hidden on blur")
	}
}

func TestEditModel_CancelButton(t *testing.T) {
	m := NewEditModel(nil) // New network

	// Focus the buttons
	for m.focusManager.Focused() != m.buttonGroup {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	}

	// Select the cancel button
	m.buttonGroup.selected = 1
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(enterMsg)

	// Check that the correct message was sent
	msg := cmd()
	if _, ok := msg.(popViewMsg); !ok {
		t.Errorf("expected a popViewMsg but got %T", msg)
	}
}

func TestEditModel_ForgetFlow(t *testing.T) {
	item := &connectionItem{
		Connection: wifi.Connection{SSID: "TestNetwork", IsKnown: true},
	}
	m := NewEditModel(item)

	// Focus the buttons
	for m.focusManager.Focused() != m.buttonGroup {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	}

	// Select the forget button
	m.buttonGroup.selected = 2
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(enterMsg)

	msg := cmd()
	m.Update(msg)

	if !m.isForgetting {
		t.Fatal("isForgetting was not set to true")
	}

	// Press 'n' to cancel
	nKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	m.Update(nKeyMsg)

	if m.isForgetting {
		t.Fatal("isForgetting was not set to false after pressing 'n'")
	}

	// Select the forget button again
	_, cmd = m.Update(enterMsg)

	msg = cmd()
	m.Update(msg)

	// Press 'y' to confirm
	yKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}
	_, cmd = m.Update(yKeyMsg)

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

func TestSecretLoadingLoop(t *testing.T) {
	// Create a mock backend that fails to get secrets with ErrMissingPermission
	b, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	mb := b.(*mock.MockBackend)
	// We wrap the error to simulate what networkmanager backend does
	mb.GetSecretsError = fmt.Errorf("need to be in the 'networkmanager' group: %w", wifi.ErrMissingPermission)
	mb.ActionSleep = 0

	// Create the main model
	m, err := NewModel(mb)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Create an EditModel for a known connection
	conn := &connectionItem{
		Connection: wifi.Connection{
			SSID:    "KnownNet",
			IsKnown: true,
		},
	}
	editModel := NewEditModel(conn)
	m.stack.Push(editModel)

	// Simulate flow starting from loadSecretsMsg
	msg := loadSecretsMsg{item: *conn}

	// 1. Update with loadSecretsMsg
	updatedM, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("Expected command from loadSecretsMsg")
	}

	cmdMsg := cmd()
	batch, ok := cmdMsg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("Expected tea.BatchMsg from loadSecretsMsg, got %T", cmdMsg)
	}

	var nextMsg tea.Msg
	foundErrorMsg := false

	for _, c := range batch {
		res := c()
		if _, ok := res.(errorMsg); ok {
			nextMsg = res
			foundErrorMsg = true
			break
		}
	}

	if !foundErrorMsg {
		t.Fatal("Expected errorMsg from loadSecretsMsg command execution")
	}

	// 2. Update with errorMsg
	updatedM, cmd = updatedM.Update(nextMsg)
	if cmd == nil {
		t.Fatal("Expected command from errorMsg")
	}

	// Expect connectionFailedMsg (wrapped in a Cmd)
	cmdMsg = cmd()
	// This might be direct msg or batch?
	// tui.go: return m, func() tea.Msg { return connectionFailedMsg{err: msg.err} }
	// So cmd() returns connectionFailedMsg directly.

	connFailedMsg, ok := cmdMsg.(connectionFailedMsg)
	if !ok {
		t.Fatalf("Expected connectionFailedMsg, got %T", cmdMsg)
	}

	// 3. Update with connectionFailedMsg
	updatedM, cmd = updatedM.Update(connFailedMsg)

	// EditModel returns tea.Batch(...)
	if cmd == nil {
		t.Fatal("Expected command from connectionFailedMsg")
	}

	cmdMsg = cmd()

	// Check if we got a batch or a single message (Bubble Tea optimization?)
	foundLoadSecretsMsg := false

	if batch, ok := cmdMsg.(tea.BatchMsg); ok {
		for _, c := range batch {
			res := c()
			if _, ok := res.(loadSecretsMsg); ok {
				foundLoadSecretsMsg = true
				break
			}
		}
	} else {
		// Single message result
		if _, ok := cmdMsg.(loadSecretsMsg); ok {
			foundLoadSecretsMsg = true
		}
	}

	if foundLoadSecretsMsg {
		t.Fatal("Loop detected: loadSecretsMsg was triggered after connectionFailedMsg")
	} else {
		t.Log("No loadSecretsMsg found, loop broken")
	}
}
