package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
