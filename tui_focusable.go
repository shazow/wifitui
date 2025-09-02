package main

import tea "github.com/charmbracelet/bubbletea"

// Focusable defines the interface for components that can be focused.
type Focusable interface {
	Focus()
	Blur()
	IsFocused() bool
	Update(msg tea.Msg) (Focusable, tea.Cmd)
}

// FocusManager manages the focus state of a list of focusable components.
type FocusManager struct {
	components []Focusable
	focused    int
}

// NewFocusManager creates a new FocusManager.
func NewFocusManager(components ...Focusable) *FocusManager {
	fm := &FocusManager{}
	fm.SetComponents(components...)
	return fm
}

// SetComponents sets the focusable components for the manager.
func (fm *FocusManager) SetComponents(components ...Focusable) {
	fm.components = components
	fm.focused = -1
}

// Focus sets the focus on the first component.
func (fm *FocusManager) Focus() {
	if len(fm.components) > 0 {
		fm.focused = 0
		fm.components[fm.focused].Focus()
	}
}

// Blur removes the focus from all components.
func (fm *FocusManager) Blur() {
	if fm.focused != -1 && fm.focused < len(fm.components) {
		fm.components[fm.focused].Blur()
	}
	fm.focused = -1
}

// Update passes the message to the focused component.
func (fm *FocusManager) Update(msg tea.Msg) (Focusable, tea.Cmd) {
	if !fm.IsFocused() {
		return fm, nil
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "right", "tab":
			fm.Next()
			return fm, nil
		case "left", "shift+tab":
			fm.Prev()
			return fm, nil
		}
	}

	// Pass the message to the focused component
	var cmd tea.Cmd
	var updated Focusable
	updated, cmd = fm.components[fm.focused].Update(msg)
	fm.components[fm.focused] = updated
	return fm, cmd
}

// Next moves the focus to the next component.
func (fm *FocusManager) Next() {
	if len(fm.components) == 0 {
		return
	}
	if fm.focused != -1 {
		fm.components[fm.focused].Blur()
	}
	fm.focused = (fm.focused + 1) % len(fm.components)
	fm.components[fm.focused].Focus()
}

// Prev moves the focus to the previous component.
func (fm *FocusManager) Prev() {
	if len(fm.components) == 0 {
		return
	}
	if fm.focused != -1 {
		fm.components[fm.focused].Blur()
	}
	fm.focused = (fm.focused - 1 + len(fm.components)) % len(fm.components)
	fm.components[fm.focused].Focus()
}

// Focused returns the currently focused component.
func (fm *FocusManager) FocusedComponent() Focusable {
	if fm.focused == -1 || fm.focused >= len(fm.components) {
		return nil
	}
	return fm.components[fm.focused]
}

// IsFocused returns whether the focus manager has focus.
func (fm *FocusManager) IsFocused() bool {
	return fm.focused != -1
}

func Focusables(buttons []*Button) []Focusable {
	focusables := make([]Focusable, len(buttons))
	for i, b := range buttons {
		focusables[i] = b
	}
	return focusables
}
