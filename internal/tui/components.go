package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Styles ---
var (
	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	blurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
)

// --- Checkbox ---

type Checkbox struct {
	label   string
	checked bool
	focused bool
}

func NewCheckbox(label string, checked bool) *Checkbox {
	return &Checkbox{
		label:   label,
		checked: checked,
	}
}

func (c *Checkbox) Focus() tea.Cmd {
	c.focused = true
	return nil
}

func (c *Checkbox) Blur() {
	c.focused = false
}

func (c *Checkbox) Update(msg tea.Msg) (Focusable, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			c.checked = !c.checked
		}
	}
	return c, nil
}

func (c *Checkbox) View() string {
	var checkbox string
	if c.checked {
		checkbox = "[x]"
	} else {
		checkbox = "[ ]"
	}
	label := " " + c.label
	if c.focused {
		return focusedStyle.Render(checkbox + label)
	}
	return blurredStyle.Render(checkbox + label)
}

func (c *Checkbox) Checked() bool {
	return c.checked
}

// --- RadioGroup ---

type RadioGroup struct {
	options []string
	selected int
	focused bool
}

func NewRadioGroup(options []string, selected int) *RadioGroup {
	return &RadioGroup{
		options: options,
		selected: selected,
	}
}

func (r *RadioGroup) Focus() tea.Cmd {
	r.focused = true
	return nil
}

func (r *RadioGroup) Blur() {
	r.focused = false
}

func (r *RadioGroup) Update(msg tea.Msg) (Focusable, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "right":
			r.selected = (r.selected + 1) % len(r.options)
		case "left":
			r.selected = (r.selected - 1 + len(r.options)) % len(r.options)
		}
	}
	return r, nil
}

func (r *RadioGroup) View() string {
	var s strings.Builder
	for i, option := range r.options {
		style := blurredStyle
		if r.focused && i == r.selected {
			style = focusedStyle
		}
		s.WriteString(style.Render("[ " + option + " ]"))
		s.WriteString("  ")
	}
	return s.String()
}

func (r *RadioGroup) Selected() int {
	return r.selected
}

// --- ButtonGroup ---

type ButtonGroup struct {
	buttons  []string
	selected int
	focused  bool
	action   func(int) tea.Cmd
}

func NewButtonGroup(buttons []string, action func(int) tea.Cmd) *ButtonGroup {
	return &ButtonGroup{
		buttons: buttons,
		action:  action,
	}
}

func (b *ButtonGroup) Focus() tea.Cmd {
	b.focused = true
	return nil
}

func (b *ButtonGroup) Blur() {
	b.focused = false
}

func (b *ButtonGroup) Update(msg tea.Msg) (Focusable, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "right":
			b.selected = (b.selected + 1) % len(b.buttons)
		case "left":
			b.selected = (b.selected - 1 + len(b.buttons)) % len(b.buttons)
		case "enter":
			if b.action != nil {
				return b, b.action(b.selected)
			}
		}
	}
	return b, nil
}

func (b *ButtonGroup) View() string {
	var s strings.Builder
	for i, label := range b.buttons {
		style := blurredStyle
		if b.focused && i == b.selected {
			style = focusedStyle
		}
		s.WriteString(style.Render("[ " + label + " ]"))
		s.WriteString("  ")
	}
	return s.String()
}
