package log

import (
	"context"
	"log/slog"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// BufferedLogHandler is a slog.Handler that retains the last N log records in memory.
// It acts as a "middleware" handler, forwarding logs to a downstream handler
// after buffering them. It is safe for concurrent use.
type BufferedLogHandler struct {
	// The downstream handler to which logs are sent after being buffered.
	downstream slog.Handler

	// mu protects the buffer.
	mu sync.Mutex

	// capacity is the maximum number of records to store.
	capacity int

	// buffer holds the log records. It's a pointer to a slice so that
	// handlers created with WithAttrs or WithGroup share the same buffer.
	buffer *[]*slog.Record
}

// NewBufferedLogHandler creates a new BufferedLogHandler.
// It wraps a downstream handler and specifies the buffer capacity.
func NewBufferedLogHandler(downstream slog.Handler, capacity int) *BufferedLogHandler {
	// Initialize the buffer as a pointer to an empty slice with the given capacity.
	buffer := make([]*slog.Record, 0, capacity)
	return &BufferedLogHandler{
		downstream: downstream,
		capacity:   capacity,
		buffer:     &buffer,
	}
}

// Enabled reports whether the handler handles records at the given level.
// The decision is delegated to the downstream handler.
func (h *BufferedLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.downstream.Enabled(context.Background(), level)
}

// Handle stores a clone of the record in its buffer and then passes the original
// record to the downstream handler.
func (h *BufferedLogHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// To prevent the record from being modified by other handlers,
	// we make a clone of it before storing it in the buffer.
	recordClone := r.Clone()

	// Append the new record.
	*h.buffer = append(*h.buffer, &recordClone)

	// If the buffer has exceeded its capacity, trim the oldest record.
	// This slice operation is efficient for small, fixed-size buffers.
	if len(*h.buffer) > h.capacity {
		*h.buffer = (*h.buffer)[1:]
	}

	// Pass the original record to the downstream handler.
	return h.downstream.Handle(context.Background(), r)
}

// WithAttrs returns a new BufferedLogHandler that shares the same buffer but has
// the given attributes added, via the downstream handler.
func (h *BufferedLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &BufferedLogHandler{
		downstream: h.downstream.WithAttrs(attrs),
		capacity:   h.capacity,
		mu:         h.mu,     // Share the same mutex
		buffer:     h.buffer, // Share the same buffer pointer
	}
}

// WithGroup returns a new BufferedLogHandler that shares the same buffer but has
// the given group name, via the downstream handler.
func (h *BufferedLogHandler) WithGroup(name string) slog.Handler {
	return &BufferedLogHandler{
		downstream: h.downstream.WithGroup(name),
		capacity:   h.capacity,
		mu:         h.mu,     // Share the same mutex
		buffer:     h.buffer, // Share the same buffer pointer
	}
}

// Records returns a copy of the log records currently in the buffer.
// Returning a copy prevents race conditions if the caller iterates over
// the slice while new logs are being added concurrently.
func (h *BufferedLogHandler) Records() []*slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Create a new slice and copy the records into it to ensure thread safety.
	recordsCopy := make([]*slog.Record, len(*h.buffer))
	copy(recordsCopy, *h.buffer)
	return recordsCopy
}

// --- TUI Proxy Handler ---

// TUIProxyHandler is a slog.Handler that sends log messages to a tea.Program.
type TUIProxyHandler struct {
	slog.Handler
	ch chan<- tea.Msg
}

// NewTUIProxyHandler creates a new TUIProxyHandler.
func NewTUIProxyHandler(handler slog.Handler, ch chan<- tea.Msg) *TUIProxyHandler {
	return &TUIProxyHandler{
		Handler: handler,
		ch:      ch,
	}
}

// Handle sends the log message to the tea.Program.
func (h *TUIProxyHandler) Handle(ctx context.Context, r slog.Record) error {
	// Send the log message to the TUI
	if h.ch != nil {
		h.ch <- LogMsg(r)
	}

	return h.Handler.Handle(ctx, r)
}

// LogMsg is a tea.Msg that represents a log message.
type LogMsg slog.Record

var (
	bufferHandler *BufferedLogHandler
	tuiHandler    *TUIProxyHandler
)

// Init initializes the default logger.
func Init(handler slog.Handler) {
	bufferHandler = NewBufferedLogHandler(handler, 20)
	tuiHandler = NewTUIProxyHandler(bufferHandler, nil)
	slog.SetDefault(slog.New(tuiHandler))
}

// SetOutput sets the output channel for the default logger.
func SetOutput(ch chan<- tea.Msg) {
	tuiHandler.ch = ch
}

// Logs returns the stored log messages from the default logger.
func Logs() []*slog.Record {
	return bufferHandler.Records()
}
