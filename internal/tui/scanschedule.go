package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	fastScanInterval = 2 * time.Second
	slowScanInterval = 10 * time.Second
)

// ScanSchedule is a component that triggers scans at a regular interval.
type ScanSchedule struct {
	callback  func() tea.Msg
	isRunning bool
	interval  time.Duration
}

// NewScanSchedule creates a new ScanSchedule.
func NewScanSchedule(callback func() tea.Msg) *ScanSchedule {
	return &ScanSchedule{
		callback: callback,
		interval: fastScanInterval, // Start with a fast scan
	}
}

// Start begins the scan schedule.
func (s *ScanSchedule) Start() tea.Cmd {
	s.isRunning = true
	return s.tick()
}

// Stop halts the scan schedule.
func (s *ScanSchedule) Stop() {
	s.isRunning = false
}

// Update handles messages for the ScanSchedule.
func (s *ScanSchedule) Update(msg tea.Msg) tea.Cmd {
	if !s.isRunning {
		return nil
	}
	switch msg.(type) {
	case tickMsg:
		// When we get a tick, call the callback and then schedule the next tick.
		return tea.Batch(s.callback, s.tick())
	}
	return nil
}

// HasResults should be called when a scan has returned results.
func (s *ScanSchedule) HasResults(hasResults bool) {
	if hasResults {
		s.interval = slowScanInterval
	} else {
		s.interval = fastScanInterval
	}
}

// internal message to trigger a tick
type tickMsg struct{}

func (s *ScanSchedule) tick() tea.Cmd {
	if !s.isRunning {
		return nil
	}
	return tea.Tick(s.interval, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}