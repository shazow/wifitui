package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shazow/wifitui/wifi"
)

func TestForgetModel_YesKey(t *testing.T) {
	item := connectionItem{Connection: wifi.Connection{SSID: "test"}}
	m := NewForgetModel(item, 80, 24)
	yKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}
	_, cmd := m.Update(yKeyMsg)

	msg := cmd()
	if _, ok := msg.(forgetNetworkMsg); !ok {
		t.Errorf("expected a forgetNetworkMsg but got %T", msg)
	}
}

func TestForgetModel_NoKey(t *testing.T) {
	item := connectionItem{Connection: wifi.Connection{SSID: "test"}}
	m := NewForgetModel(item, 80, 24)
	nKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	_, cmd := m.Update(nKeyMsg)

	msg := cmd()
	if _, ok := msg.(changeViewMsg); !ok {
		t.Errorf("expected a changeViewMsg but got %T", msg)
	}
}
