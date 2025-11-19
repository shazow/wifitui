package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shazow/wifitui/internal/helpers"
	"github.com/shazow/wifitui/wifi"
)

// Component is the interface for a TUI component.
type Component interface {
	Update(msg tea.Msg) (Component, tea.Cmd)
	View() string
	IsConsumingInput() bool
}

// Leavable is an optional interface for components that need to perform
// an action when they are popped from the stack.
type Leavable interface {
	OnLeave() tea.Cmd
}

// Enterable is an optional interface for components that need to perform
// an action when they are pushed onto the stack.
type Enterable interface {
	OnEnter() tea.Cmd
}

// popViewMsg is a message to pop the current view from the stack.
type popViewMsg struct{}

type statusMsg struct {
	status  string
	loading bool
}

// connectionItem holds the information for a single WiFi connection in our list
type connectionItem struct {
	wifi.Connection
}

func (i connectionItem) Title() string { return i.SSID }
func (i connectionItem) Description() string {
	if i.Strength > 0 {
		return fmt.Sprintf("%d%%", i.Strength)
	}
	if !i.IsVisible && i.LastConnected != nil {
		return helpers.FormatDuration(*i.LastConnected)
	}
	return ""
}
func (i connectionItem) FilterValue() string { return i.Title() }

// Bubbletea messages are used to communicate between the main loop and commands
type (
	// From backend
	connectionsLoadedMsg []wifi.Connection // Sent when connections are fetched
	scanFinishedMsg      []wifi.Connection // Sent when a scan is finished
	secretsLoadedMsg     struct {
		item   connectionItem
		secret string
	}
	connectionSavedMsg struct {
		forgottenSSID string
	}
	errorMsg struct{ err error }

	// To main model
	scanMsg    struct{}
	connectMsg struct {
		item        connectionItem
		autoConnect bool
	}
	joinNetworkMsg struct {
		ssid     string
		password string
		security wifi.SecurityType
		isHidden bool
	}
	loadSecretsMsg  struct{ item connectionItem }
	updateSecretMsg struct {
		item        connectionItem
		newPassword string
		autoConnect bool
	}
	forgetNetworkMsg struct{ item connectionItem }
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
		return lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Bold(true).Render(checkbox + label)
	}
	return lipgloss.NewStyle().Foreground(CurrentTheme.Subtle).Render(checkbox + label)
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
		style := lipgloss.NewStyle().Foreground(CurrentTheme.Subtle)
		if c.focused && i == c.selected {
			style = lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Bold(true)
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
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	if a.focused {
		style = style.BorderForeground(CurrentTheme.Primary)
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
		style := lipgloss.NewStyle().Foreground(CurrentTheme.Subtle)
		if b.focused && i == b.selected {
			style = lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Bold(true)
		}
		s.WriteString(style.Render("[ " + label + " ]"))
		s.WriteString("  ")
	}
	return s.String()
}
