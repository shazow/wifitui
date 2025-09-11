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
	item1 := connectionItem{Connection: wifi.Connection{SSID: "TestNetwork1", IsKnown: true}}
	item2 := connectionItem{Connection: wifi.Connection{SSID: "TestNetwork2", IsKnown: true}}
	m.list.SetItems([]list.Item{item1, item2})

	// Press 'f' to start forgetting
	fKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")}
	updatedModel, _ := m.Update(fKeyMsg)
	m = updatedModel.(*ListModel)

	if !m.isForgetting {
		t.Fatal("isForgetting was not set to true")
	}

	// Press 'n' to cancel
	nKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	updatedModel, _ = m.Update(nKeyMsg)
	m = updatedModel.(*ListModel)

	if m.isForgetting {
		t.Fatal("isForgetting was not cleared after pressing 'n'")
	}

	// Press 'f' again
	updatedModel, _ = m.Update(fKeyMsg)
	m = updatedModel.(*ListModel)

	// Press 'j' to navigate, should be ignored
	downKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	updatedModel, _ = m.Update(downKeyMsg)
	m = updatedModel.(*ListModel)

	if !m.isForgetting {
		t.Fatal("isForgetting should not be cleared after pressing a navigation key")
	}

	// Press 'y' to confirm
	yKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}
	updatedModel, cmd := m.Update(yKeyMsg)
	m = updatedModel.(*ListModel)

	if m.isForgetting {
		t.Fatal("isForgetting was not cleared after pressing 'y'")
	}

	msg := cmd()
	if forgetMsg, ok := msg.(forgetNetworkMsg); !ok {
		t.Errorf("expected a forgetNetworkMsg but got %T", msg)
	} else {
		if forgetMsg.item.SSID != "TestNetwork1" {
			t.Errorf("expected forgetNetworkMsg for 'TestNetwork1' but got for '%s'", forgetMsg.item.SSID)
		}
	}
}
