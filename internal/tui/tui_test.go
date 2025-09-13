package tui

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	wifilog "github.com/shazow/wifitui/internal/log"
	"github.com/shazow/wifitui/wifi/mock"
)

func TestTUIReceivesLog(t *testing.T) {
	b, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	m, err := NewModel(b)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	record := slog.Record{
		Time:    time.Now(),
		Level:   slog.LevelInfo,
		Message: "hello world",
	}
	msg := wifilog.LogMsg(record)

	newModel, _ := m.Update(msg)
	m = newModel.(*model)

	if m.latestLog.Message != "hello world" {
		t.Errorf("expected latest log message to be 'hello world', got '%s'", m.latestLog.Message)
	}
	if m.latestLog.Level != slog.LevelInfo {
		t.Errorf("expected latest log level to be INFO, got '%s'", m.latestLog.Level)
	}
}

func TestTUILogView(t *testing.T) {
	wifilog.Init(slog.NewTextHandler(io.Discard, nil))
	slog.Info("log message 1")
	slog.Error("log message 2")

	m := NewLogViewModel()
	view := m.View()

	if !strings.Contains(view, "log message 1") {
		t.Errorf("expected view to contain 'log message 1', but it didn't")
	}
	if !strings.Contains(view, "log message 2") {
		t.Errorf("expected view to contain 'log message 2', but it didn't")
	}
}

func TestTUIKeybindingForLogView(t *testing.T) {
	b, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	m, err := NewModel(b)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")}
	newModel, _ := m.Update(msg)
	m = newModel.(*model)

	if len(m.componentStack) != 2 {
		t.Fatalf("expected component stack to have 2 items, got %d", len(m.componentStack))
	}

	topComponent := m.componentStack[len(m.componentStack)-1]
	if _, ok := topComponent.(*LogViewModel); !ok {
		t.Errorf("expected top component to be a *LogViewModel, but it wasn't")
	}
}
