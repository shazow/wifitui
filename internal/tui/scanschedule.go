package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	ScanOff  = 0
	ScanFast = 2 * time.Second
	ScanSlow = 8 * time.Second
)

// ScanSchedule is a component that triggers scans at a regular interval.
type ScanSchedule struct {
	callback func() tea.Msg
	interval time.Duration
}

// NewScanSchedule creates a new ScanSchedule.
func NewScanSchedule(callback func() tea.Msg) *ScanSchedule {
	return &ScanSchedule{
		callback: callback,
	}
}

// Toggle enables or disables the scan schedule.
func (s *ScanSchedule) Toggle() (bool, tea.Cmd) {
	var cmd tea.Cmd
	var enabled bool
	if s.interval == ScanOff {
		// It's off, turn it on
		cmd = s.SetSchedule(ScanFast)
		enabled = true
	} else {
		// It's on, turn it off
		cmd = s.SetSchedule(ScanOff)
		enabled = false
	}
	return enabled, cmd
}

// SetSchedule sets the scan interval.
func (s *ScanSchedule) SetSchedule(interval time.Duration) tea.Cmd {
	isStarting := s.interval == ScanOff && interval != ScanOff
	s.interval = interval

	if isStarting {
		// We were off, now we are on. Start the scan loop.
		return tea.Batch(s.callback, s.tick())
	}
	return nil
}

// Update handles messages for the ScanSchedule.
func (s *ScanSchedule) Update(msg tea.Msg) tea.Cmd {
	if s.interval == ScanOff {
		return nil
	}

	switch msg.(type) {
	case tickMsg:
		// When we get a tick, call the callback and then schedule the next tick.
		return tea.Batch(s.callback, s.tick())
	}
	return nil
}

// internal message to trigger a tick
type tickMsg struct{}

func (s *ScanSchedule) tick() tea.Cmd {
	if s.interval == ScanOff {
		return nil
	}
	return tea.Tick(s.interval, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}
