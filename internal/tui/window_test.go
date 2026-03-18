package tui

import "testing"

func TestWindowStateContentWidthFallback(t *testing.T) {
	var w *WindowState
	if got, want := w.ContentWidth(4, 50, 20), 46; got != want {
		t.Fatalf("expected %d, got %d", want, got)
	}
}

func TestWindowStateContentWidthUsesWindowWidth(t *testing.T) {
	w := &WindowState{Width: 100}
	if got, want := w.ContentWidth(4, 50, 20), 96; got != want {
		t.Fatalf("expected %d, got %d", want, got)
	}
}

func TestWindowStateContentWidthClampsToMinimum(t *testing.T) {
	w := &WindowState{Width: 10}
	if got, want := w.ContentWidth(4, 50, 20), 20; got != want {
		t.Fatalf("expected %d, got %d", want, got)
	}
}
