package tui

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
)

// newSafeRenderer creates a lipgloss renderer that skips terminal
// auto-detection (OSC queries) when the writer is not a TTY.
//
// Workaround reference: https://github.com/muesli/termenv/issues/205
func newSafeRenderer(w io.Writer) *lipgloss.Renderer {
	if f, ok := w.(*os.File); ok && !isatty.IsTerminal(f.Fd()) {
		return lipgloss.NewRenderer(w, termenv.WithProfile(termenv.Ascii))
	}

	return lipgloss.NewRenderer(w)
}

func init() {
	lipgloss.SetDefaultRenderer(newSafeRenderer(os.Stdout))
}
