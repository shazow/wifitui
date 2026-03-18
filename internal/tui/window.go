package tui

import tea "github.com/charmbracelet/bubbletea"

// WindowState stores the latest terminal dimensions shared by components.
type WindowState struct {
	Width  int
	Height int
}

func (w *WindowState) Update(msg tea.WindowSizeMsg) {
	w.Width = msg.Width
	w.Height = msg.Height
}
