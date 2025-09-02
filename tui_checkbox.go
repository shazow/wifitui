package main

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Checkbox is a simple checkbox component.
type Checkbox struct {
	Label   string
	Checked bool
	focused bool
}

// NewCheckbox creates a new checkbox.
func NewCheckbox(label string, checked bool) *Checkbox {
	return &Checkbox{
		Label:   label,
		Checked: checked,
	}
}

// Update handles messages for the checkbox.
func (c *Checkbox) Update(msg tea.Msg) (Focusable, tea.Cmd) {
	if !c.focused {
		return c, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return c, nil
	}

	switch keyMsg.String() {
	case "enter", " ":
		c.Checked = !c.Checked
	}

	return c, nil
}

// View renders the checkbox.
func (c *Checkbox) View() string {
	var s string
	if c.Checked {
		s = "[x] "
	} else {
		s = "[ ] "
	}
	s += c.Label

	if c.focused {
		return focusedInputStyle.Render(s)
	}
	return normalInputStyle.Render(s)
}

// Focus sets the focus on the checkbox.
func (c *Checkbox) Focus() {
	c.focused = true
}

// Blur removes the focus from the checkbox.
func (c *Checkbox) Blur() {
	c.focused = false
}

// IsFocused returns whether the checkbox is focused.
func (c *Checkbox) IsFocused() bool {
	return c.focused
}
