package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/shazow/wifitui/wifi"
)

func TestListModel_NewKey(t *testing.T) {
	m := NewListModel()
	nKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	_, cmd := m.Update(nKeyMsg)

	msg := cmd()
	if _, ok := msg.(showEditViewMsg); !ok {
		t.Errorf("expected a showEditViewMsg but got %T", msg)
	}
}

func TestListModel_ForgetFlow(t *testing.T) {
	m := NewListModel()
	item := connectionItem{
		Connection: wifi.Connection{SSID: "TestNetwork", IsKnown: true},
	}
	m.list.SetItems([]list.Item{item})

	// Press 'f' to start forgetting
	fKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")}
	updatedModel, _ := m.Update(fKeyMsg)
	m = updatedModel.(ListModel)

	if m.forgettingItem == nil || m.forgettingItem.SSID != "TestNetwork" {
		t.Fatal("forgettingItem was not set correctly")
	}

	// Press 'n' to cancel
	nKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	updatedModel, _ = m.Update(nKeyMsg)
	m = updatedModel.(ListModel)

	if m.forgettingItem != nil {
		t.Fatal("forgettingItem was not cleared after pressing 'n'")
	}

	// Press 'f' again
	updatedModel, _ = m.Update(fKeyMsg)
	m = updatedModel.(ListModel)

	// Press 'y' to confirm
	yKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}
	updatedModel, cmd := m.Update(yKeyMsg)
	m = updatedModel.(ListModel)

	if m.forgettingItem != nil {
		t.Fatal("forgettingItem was not cleared after pressing 'y'")
	}

	msg := cmd()
	if forgetMsg, ok := msg.(forgetNetworkMsg); !ok {
		t.Errorf("expected a forgetNetworkMsg but got %T", msg)
	} else {
		if forgetMsg.item.SSID != "TestNetwork" {
			t.Errorf("expected forgetNetworkMsg for 'TestNetwork' but got for '%s'", forgetMsg.item.SSID)
		}
	}
}
