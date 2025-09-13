package log

import (
	"context"
	"log/slog"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// TUIHandler is a slog.Handler that sends log messages to a tea.Program.
type TUIHandler struct {
	slog.Handler
	mu   sync.Mutex
	ch   chan<- tea.Msg
	logs []slog.Record
}

// NewTUIHandler creates a new TUIHandler.
func NewTUIHandler(handler slog.Handler, ch chan<- tea.Msg) *TUIHandler {
	return &TUIHandler{
		Handler: handler,
		ch:      ch,
	}
}

// Handle sends the log message to the tea.Program.
func (h *TUIHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Store the log message
	h.logs = append(h.logs, r)
	if len(h.logs) > 20 {
		h.logs = h.logs[1:]
	}

	// Send the log message to the TUI
	if h.ch != nil {
		h.ch <- LogMsg(r)
	}

	return h.Handler.Handle(ctx, r)
}

// Logs returns the stored log messages.
func (h *TUIHandler) Logs() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.logs
}

// LogMsg is a tea.Msg that represents a log message.
type LogMsg slog.Record

// SetOutput sets the output channel for the handler.
func (h *TUIHandler) SetOutput(ch chan<- tea.Msg) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ch = ch
}

var defaultHandler *TUIHandler

// Init initializes the default logger.
func Init(handler slog.Handler) {
	defaultHandler = NewTUIHandler(handler, nil)
	slog.SetDefault(slog.New(defaultHandler))
}

// SetOutput sets the output channel for the default logger.
func SetOutput(ch chan<- tea.Msg) {
	defaultHandler.SetOutput(ch)
}

// Logs returns the stored log messages from the default logger.
func Logs() []slog.Record {
	return defaultHandler.Logs()
}
