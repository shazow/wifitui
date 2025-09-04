package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Focusable defines the contract for a UI element that can be managed by the
// focus manager.
type Focusable interface {
	// Focus is called when the element gains focus. It can return a command.
	Focus() tea.Cmd
	// Blur is called when the element loses focus.
	Blur()
	// Update is called when the element is focused and a message is received.
	// It should return the updated element and any resulting command.
	Update(msg tea.Msg) (Focusable, tea.Cmd)
	// View renders the element's UI.
	View() string
}

// FocusGroup is an interface for a Focusable that contains other Focusables.
// This is the key to enabling recursive focus management.
type FocusGroup interface {
	Focusable
	// Next moves the focus to the next element in the group.
	Next() tea.Cmd
	// Prev moves the focus to the previous element in the group.
	Prev() tea.Cmd
}

// FocusManager manages focus for a group of Focusable elements. It also
// implements the FocusGroup interface, allowing for nested focus managers.
type FocusManager struct {
	items []Focusable
	focus int
}

// NewFocusManager creates a new focus manager with the given items.
func NewFocusManager(items ...Focusable) *FocusManager {
	return &FocusManager{
		items: items,
		focus: 0,
	}
}

// --- FocusManager implementation of FocusGroup ---

// Focus gives focus to the manager, which in turn focuses its first child.
func (m *FocusManager) Focus() tea.Cmd {
	if len(m.items) > 0 {
		m.focus = 0
		return m.items[m.focus].Focus()
	}
	return nil
}

// Blur removes focus from the manager, which in turn blurs its active child.
func (m *FocusManager) Blur() {
	if len(m.items) > 0 {
		m.items[m.focus].Blur()
	}
}

// Update passes the message to the currently focused child element.
func (m *FocusManager) Update(msg tea.Msg) (Focusable, tea.Cmd) {
	if len(m.items) == 0 {
		return m, nil
	}

	newItem, cmd := m.items[m.focus].Update(msg)
	m.items[m.focus] = newItem
	return m, cmd
}

// View renders the view of the currently focused child. A real implementation
// would likely render all children, highlighting the focused one.
func (m *FocusManager) View() string {
	if len(m.items) == 0 {
		return ""
	}
	// For simplicity, we'll just view the focused item.
	// A real app might iterate and render all items.
	return m.items[m.focus].View()
}

// Next moves focus to the next item, handling nested focus groups.
func (m *FocusManager) Next() tea.Cmd {
	if len(m.items) == 0 {
		return nil
	}

	// Try to advance focus within a focused subgroup.
	if subGroup, ok := m.items[m.focus].(FocusGroup); ok {
		// To check for wrapping without a return value, we must inspect
		// the subgroup's state. We can only do this if it's a FocusManager.
		if subManager, ok := subGroup.(*FocusManager); ok {
			oldFocus := subManager.focus
			cmd := subManager.Next() // This is the recursive call
			newFocus := subManager.focus

			// If focus advanced and didn't wrap, we're done propagating.
			if newFocus > oldFocus {
				return cmd
			}
		} else {
			// For other FocusGroup implementations, we can't know if they
			// wrapped, so we can't propagate focus automatically.
			return subGroup.Next()
		}
	}

	// If the child is not a group, or if the subgroup wrapped,
	// we advance the focus in the current group.
	m.items[m.focus].Blur()
	m.focus = (m.focus + 1) % len(m.items)
	return m.items[m.focus].Focus()
}

// Prev moves focus to the previous item, handling nested focus groups.
func (m *FocusManager) Prev() tea.Cmd {
	if len(m.items) == 0 {
		return nil
	}

	// Try to move focus backward within a focused subgroup.
	if subGroup, ok := m.items[m.focus].(FocusGroup); ok {
		if subManager, ok := subGroup.(*FocusManager); ok {
			oldFocus := subManager.focus
			cmd := subManager.Prev()
			newFocus := subManager.focus

			// If focus moved backward and didn't wrap, we're done.
			if newFocus < oldFocus {
				return cmd
			}
		} else {
			return subGroup.Prev()
		}
	}

	// If the child is not a group, or if the subgroup wrapped,
	// we move focus backward in the current group.
	m.items[m.focus].Blur()
	m.focus--
	if m.focus < 0 {
		m.focus = len(m.items) - 1
	}

	// If the new item is a group, we need to focus its last element.
	// We can do this by calling Prev() on it, which will cause it to wrap.
	if subGroup, ok := m.items[m.focus].(FocusGroup); ok {
		return subGroup.Prev()
	}

	return m.items[m.focus].Focus()
}

// Focused returns the currently focused element.
func (m *FocusManager) Focused() Focusable {
	if len(m.items) == 0 {
		return nil
	}
	return m.items[m.focus]
}

// SetFocus sets the focus to the item at the given index.
func (m *FocusManager) SetFocus(index int) tea.Cmd {
	if index < 0 || index >= len(m.items) {
		return nil
	}
	if len(m.items) > 0 {
		m.items[m.focus].Blur()
	}
	m.focus = index
	if len(m.items) > 0 {
		return m.items[m.focus].Focus()
	}
	return nil
}
