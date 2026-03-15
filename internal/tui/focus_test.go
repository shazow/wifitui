package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// mockFocusable is a simple implementation of the Focusable interface for testing.
type mockFocusable struct {
	id      string
	focused bool
}

func (m *mockFocusable) Focus() tea.Cmd {
	m.focused = true
	return nil
}

func (m *mockFocusable) Blur() {
	m.focused = false
}

func (m *mockFocusable) Update(msg tea.Msg) (Focusable, tea.Cmd) {
	return m, nil
}

func (m *mockFocusable) View() string {
	focusedMarker := " "
	if m.focused {
		focusedMarker = "*"
	}
	return fmt.Sprintf("[%s%s]", focusedMarker, m.id)
}

// viewFocusManager is a helper to visualize the state of a focus manager.
func viewFocusManager(fm *FocusManager) string {
	var views []string
	for _, item := range fm.items {
		views = append(views, item.View())
	}
	return strings.Join(views, " ")
}

func TestFocusManager_Simple(t *testing.T) {
	item1 := &mockFocusable{id: "1"}
	item2 := &mockFocusable{id: "2"}
	item3 := &mockFocusable{id: "3"}

	fm := NewFocusManager(item1, item2, item3)
	fm.Focus()

	if fm.Focused() != item1 {
		t.Fatalf("Expected item1 to be focused initially, but got %v", fm.Focused())
	}
	if viewFocusManager(fm) != "[*1] [ 2] [ 3]" {
		t.Errorf("Unexpected view state: %s", viewFocusManager(fm))
	}

	// Test Next()
	fm.Next()
	if fm.Focused() != item2 {
		t.Fatalf("Expected item2 to be focused after Next(), but got %v", fm.Focused())
	}
	if viewFocusManager(fm) != "[ 1] [*2] [ 3]" {
		t.Errorf("Unexpected view state: %s", viewFocusManager(fm))
	}

	fm.Next()
	fm.Next()
	if fm.Focused() != item1 {
		t.Fatalf("Expected item1 to be focused after wrapping, but got %v", fm.Focused())
	}
	if viewFocusManager(fm) != "[*1] [ 2] [ 3]" {
		t.Errorf("Unexpected view state: %s", viewFocusManager(fm))
	}

	// Test Prev()
	fm.Prev()
	if fm.Focused() != item3 {
		t.Fatalf("Expected item3 to be focused after Prev() from start, but got %v", fm.Focused())
	}
	if viewFocusManager(fm) != "[ 1] [ 2] [*3]" {
		t.Errorf("Unexpected view state: %s", viewFocusManager(fm))
	}
}

func TestFocusManager_Recursive(t *testing.T) {
	// Setup: fm1 contains [item1, fm2, item4]
	//        fm2 contains [item2, item3]
	item1 := &mockFocusable{id: "1"}
	item2 := &mockFocusable{id: "2"}
	item3 := &mockFocusable{id: "3"}
	item4 := &mockFocusable{id: "4"}

	fm2 := NewFocusManager(item2, item3)
	fm1 := NewFocusManager(item1, fm2, item4)

	fm1.Focus()

	// Initial state: item1 is focused
	if fm1.Focused() != item1 {
		t.Fatalf("Expected item1 to be focused, got %v", fm1.Focused())
	}

	// fm1.Next() should focus fm2, which focuses its first child (item2)
	fm1.Next()
	if fm1.Focused() != fm2 {
		t.Fatalf("Expected fm2 to be focused in fm1, got %v", fm1.Focused())
	}
	if fm2.Focused() != item2 {
		t.Fatalf("Expected item2 to be focused in fm2, got %v", fm2.Focused())
	}
	if !item2.focused {
		t.Error("item2 should be marked as focused")
	}

	// fm1.Next() should now advance focus inside fm2 to item3
	fm1.Next()
	if fm1.Focused() != fm2 {
		t.Fatalf("Expected fm2 to still be focused in fm1, got %v", fm1.Focused())
	}
	if fm2.Focused() != item3 {
		t.Fatalf("Expected item3 to be focused in fm2, got %v", fm2.Focused())
	}
	if item2.focused {
		t.Error("item2 should have been blurred")
	}

	// fm1.Next() should now wrap inside fm2, and focus should move to item4 in fm1
	fm1.Next()
	if fm1.Focused() != item4 {
		t.Fatalf("Expected item4 to be focused in fm1, got %v", fm1.Focused())
	}
	if fm2.Focused() != item2 {
		// fm2's internal focus should have wrapped back to its start
		t.Fatalf("Expected fm2's focus to wrap to item2, got %v", fm2.Focused())
	}
	if item3.focused {
		t.Error("item3 should have been blurred")
	}

	// Now test Prev()
	fm1.Prev() // Should go from item4 to item3 (inside fm2)
	if fm1.Focused() != fm2 {
		t.Fatalf("Expected fm2 to be focused in fm1, got %v", fm1.Focused())
	}
	// When moving Prev() into a group, it should focus the last element.
	if fm2.Focused() != item3 {
		t.Fatalf("Expected item3 to be focused in fm2 after Prev() from item4, got %v", fm2.Focused())
	}

	fm1.Prev() // Should go from item3 to item2 (inside fm2)
	if fm1.Focused() != fm2 {
		t.Fatalf("Expected fm2 to still be focused in fm1, got %v", fm1.Focused())
	}
	if fm2.Focused() != item2 {
		t.Fatalf("Expected item2 to be focused in fm2, got %v", fm2.Focused())
	}
}
