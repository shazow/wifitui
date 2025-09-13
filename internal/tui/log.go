package tui

import (
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	wifilog "github.com/shazow/wifitui/internal/log"
)

// LogViewModel is the model for the log view.
type LogViewModel struct {
}

// NewLogViewModel creates a new LogViewModel.
func NewLogViewModel() *LogViewModel {
	return &LogViewModel{}
}

// Init is the first command that is run when the program starts.
func (m *LogViewModel) Init() tea.Cmd {
	return nil
}

// Update handles all incoming messages and updates the model accordingly.
func (m *LogViewModel) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return popViewMsg{}
			}
		}
	}
	return m, nil
}

// View renders the UI based on the current model state.
func (m *LogViewModel) View() string {
	var s strings.Builder
	s.WriteString("Latest logs (press 'q' to return):\n\n")

	logs := wifilog.Logs()
	for _, log := range logs {
		var style lipgloss.Style
		switch log.Level {
		case slog.LevelError:
			style = lipgloss.NewStyle().Foreground(CurrentTheme.Error)
		default:
			style = lipgloss.NewStyle().Foreground(CurrentTheme.Normal)
		}
		s.WriteString(style.Render(fmt.Sprintf("[%s] %s", log.Level, log.Message)))
		log.Attrs(func(a slog.Attr) bool {
			s.WriteString(style.Render(fmt.Sprintf(" %s=%v", a.Key, a.Value.Any())))
			return true
		})
		s.WriteString("\n")
	}

	return s.String()
}

// --- Component interface ---

// Init is the first command that is run when the program starts.
func (c *LogViewModel) InitComponent() tea.Cmd {
	return c.Init()
}

// Update handles all incoming messages and updates the model accordingly.
func (c *LogViewModel) UpdateComponent(msg tea.Msg) (Component, tea.Cmd) {
	return c.Update(msg)
}

// View renders the UI based on the current model state.
func (c *LogViewModel) ViewComponent() string {
	return c.View()
}
