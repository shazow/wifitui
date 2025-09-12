package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestErrorModel_AnyKey(t *testing.T) {
	err := errors.New("test error")
	m := NewErrorModel(err)
	anyKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	_, cmd := m.Update(anyKeyMsg)

	msg := cmd()
	if _, ok := msg.(popViewMsg); !ok {
		t.Errorf("expected a popViewMsg but got %T", msg)
	}
}
