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

func (w *WindowState) BaseWidth(fallback int) int {
	if w != nil && w.Width > 0 {
		return w.Width
	}
	return fallback
}

func (w *WindowState) BaseHeight(fallback int) int {
	if w != nil && w.Height > 0 {
		return w.Height
	}
	return fallback
}

func (w *WindowState) ContentWidth(totalFrame, fallbackWidth, minWidth int) int {
	contentWidth := w.BaseWidth(fallbackWidth) - totalFrame
	if contentWidth < minWidth {
		return minWidth
	}
	return contentWidth
}
