package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// CustomHelpList is a wrapper around list.Model that allows us to override
// the help text.
type CustomHelpList struct {
	list.Model
}

// Update is a wrapper for the embedded list.Model's Update method. It
// returns a CustomHelpList instead of a list.Model.
func (m CustomHelpList) Update(msg tea.Msg) (CustomHelpList, tea.Cmd) {
	var cmd tea.Cmd
	m.Model, cmd = m.Model.Update(msg)
	return m, cmd
}

// ShortHelp returns a slice of keybindings that are currently active and
// relevant to the user. It's used to display a short help view of keys. It
// satisfies the help.KeyMap interface.
func (m CustomHelpList) ShortHelp() []key.Binding {
	// Start with an empty slice, excluding the default up/down keys.
	kb := []key.Binding{}

	// Add pagination keys if needed.
	if m.Paginator.TotalPages > 1 {
		kb = append(kb, m.KeyMap.NextPage, m.KeyMap.PrevPage)
	}

	// Add other default keys.
	kb = append(kb, m.KeyMap.Filter, m.KeyMap.Quit)

	// Add the custom short help keys from the list model.
	if m.AdditionalShortHelpKeys != nil {
		kb = append(kb, m.AdditionalShortHelpKeys()...)
	}

	return kb
}

// FullHelp returns the full help for the list.
func (m CustomHelpList) FullHelp() [][]key.Binding {
	return m.Model.FullHelp()
}