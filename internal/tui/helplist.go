package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// CustomHelpList is a wrapper around list.Model that allows us to override
// the help text.
type CustomHelpList struct {
	list list.Model
}

// Update is a wrapper for the list.Model's Update method. It
// returns a CustomHelpList instead of a list.Model.
func (m CustomHelpList) Update(msg tea.Msg) (CustomHelpList, tea.Cmd) {
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// ShortHelp returns a slice of keybindings that are currently active and
// relevant to the user. It's used to display a short help view of keys. It
// satisfies the help.KeyMap interface.
func (m CustomHelpList) ShortHelp() []key.Binding {
	// Start with an empty slice, excluding the default up/down keys.
	kb := []key.Binding{}

	// Add pagination keys if needed.
	if m.list.Paginator.TotalPages > 1 {
		kb = append(kb, m.list.KeyMap.NextPage, m.list.KeyMap.PrevPage)
	}

	// Add other default keys.
	kb = append(kb, m.list.KeyMap.Filter, m.list.KeyMap.Quit)

	// Add the custom short help keys from the list model.
	if m.list.AdditionalShortHelpKeys != nil {
		kb = append(kb, m.list.AdditionalShortHelpKeys()...)
	}

	return kb
}

// FullHelp returns the full help for the list.
func (m CustomHelpList) FullHelp() [][]key.Binding {
	return m.list.FullHelp()
}

// ---- Proxy methods to list.Model ----

// SetSize sets the size of the list.
func (m *CustomHelpList) SetSize(w, h int) {
	m.list.SetSize(w, h)
}

// SetItems sets the items in the list.
func (m *CustomHelpList) SetItems(items []list.Item) {
	m.list.SetItems(items)
}

// FilterState returns the current filter state.
func (m CustomHelpList) FilterState() list.FilterState {
	return m.list.FilterState()
}

// SelectedItem returns the selected item.
func (m CustomHelpList) SelectedItem() list.Item {
	return m.list.SelectedItem()
}

// Index returns the index of the selected item.
func (m CustomHelpList) Index() int {
	return m.list.Index()
}

// Items returns the items in the list.
func (m CustomHelpList) Items() []list.Item {
	return m.list.Items()
}

// View renders the list.
func (m CustomHelpList) View() string {
	return m.list.View()
}