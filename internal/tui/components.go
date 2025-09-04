package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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

// --- ChoiceComponent ---

type ChoiceComponent struct {
	label    string
	options  []string
	selected int
	focused  bool
}

func NewChoiceComponent(label string, options []string) *ChoiceComponent {
	return &ChoiceComponent{
		label:   label,
		options: options,
	}
}

func (c *ChoiceComponent) Focus() tea.Cmd {
	c.focused = true
	return nil
}
func (c *ChoiceComponent) Blur() {
	c.focused = false
}

func (c *ChoiceComponent) Update(msg tea.Msg) (Focusable, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "right":
			c.selected = (c.selected + 1) % len(c.options)
		case "left":
			c.selected = (c.selected - 1 + len(c.options)) % len(c.options)
		}
	}
	return c, nil
}

func (c *ChoiceComponent) View() string {
	var s strings.Builder
	s.WriteString(c.label + "\n")
	for i, option := range c.options {
		style := blurredStyle
		if c.focused && i == c.selected {
			style = focusedStyle
		}
		s.WriteString(style.Render("[ " + option + " ]"))
		s.WriteString("  ")
	}
	return s.String()
}

func (c *ChoiceComponent) Selected() int {
	return c.selected
}

// --- TextInput ---

// TextInput wraps a textinput.Model to make it conform to the Focusable interface.
type TextInput struct {
	textinput.Model
	label   string
	focused bool
	OnFocus func(*textinput.Model) tea.Cmd
	OnBlur  func(*textinput.Model)
}

// Update wraps the textinput.Model's Update method.
func (a *TextInput) Update(msg tea.Msg) (Focusable, tea.Cmd) {
	newModel, cmd := a.Model.Update(msg)
	a.Model = newModel
	return a, cmd
}

// Focus delegates to the underlying textinput.Model.
func (a *TextInput) Focus() tea.Cmd {
	a.focused = true
	a.Model.Focus()
	if a.OnFocus != nil {
		return a.OnFocus(&a.Model)
	}
	return nil
}

// Blur delegates to the underlying textinput.Model.
func (a *TextInput) Blur() {
	if a.OnBlur != nil {
		a.OnBlur(&a.Model)
	}
	a.focused = false
	a.Model.Blur()
}

// View delegates to the underlying textinput.Model.
func (a *TextInput) View() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true).
		Padding(0, 1)
	if a.focused {
		style = style.BorderForeground(lipgloss.Color("205"))
	}
	return a.label + "\n" + style.Render(a.Model.View())
}

// --- MultiButtonComponent ---

type MultiButtonComponent struct {
	buttons  []string
	selected int
	focused  bool
	action   func(int) tea.Cmd
}

func NewMultiButtonComponent(buttons []string, action func(int) tea.Cmd) *MultiButtonComponent {
	return &MultiButtonComponent{
		buttons: buttons,
		action:  action,
	}
}

func (b *MultiButtonComponent) Focus() tea.Cmd {
	b.focused = true
	return nil
}
func (b *MultiButtonComponent) Blur() {
	b.focused = false
}

func (b *MultiButtonComponent) Update(msg tea.Msg) (Focusable, tea.Cmd) {
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

func (b *MultiButtonComponent) View() string {
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
