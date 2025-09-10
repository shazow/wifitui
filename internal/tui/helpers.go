package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// forgetHandler handles the key presses for the forget confirmation.
// It returns whether the forget flow is finished, and a command to execute.
func forgetHandler(msg tea.Msg, item connectionItem) (finished bool, cmd tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "enter":
			return true, func() tea.Msg {
				return forgetNetworkMsg{item: item}
			}
		case "n", "esc":
			return true, nil
		}
	}
	return false, nil
}
