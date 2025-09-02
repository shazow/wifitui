package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// RadioGroup is a component for a group of radio buttons.
type RadioGroup struct {
	Options  []string
	Selected int
	focused  bool
}

// NewRadioGroup creates a new radio group.
func NewRadioGroup(options []string, selected int) *RadioGroup {
	return &RadioGroup{
		Options:  options,
		Selected: selected,
	}
}

// Update handles messages for the radio group.
func (rg *RadioGroup) Update(msg tea.Msg) (Focusable, tea.Cmd) {
	if !rg.focused {
		return rg, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return rg, nil
	}

	switch keyMsg.String() {
	case "right", "tab":
		rg.Selected = (rg.Selected + 1) % len(rg.Options)
	case "left":
		rg.Selected = (rg.Selected - 1 + len(rg.Options)) % len(rg.Options)
	}

	return rg, nil
}

// View renders the radio group.
func (rg *RadioGroup) View() string {
	var s strings.Builder
	for i, option := range rg.Options {
		style := normalButtonStyle
		if rg.focused && i == rg.Selected {
			style = focusedButtonStyle
		}
		s.WriteString(style.Render("[ " + option + " ]"))
		s.WriteString("  ")
	}
	return s.String()
}

// Focus sets the focus on the radio group.
func (rg *RadioGroup) Focus() {
	rg.focused = true
}

// Blur removes the focus from the radio group.
func (rg *RadioGroup) Blur() {
	rg.focused = false
}

// IsFocused returns whether the radio group is focused.
func (rg *RadioGroup) IsFocused() bool {
	return rg.focused
}
