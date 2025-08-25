package main

import (
	"fmt"
	"time"
)

// formatDuration takes a time and returns a human-readable string like "2 hours ago"
func formatDuration(t time.Time) string {
	d := time.Since(t)
	var s string
	switch {
	case d < time.Minute*2:
		s = fmt.Sprintf("%0.f seconds", d.Seconds())
	case d < time.Hour*2:
		s = fmt.Sprintf("%0.f minutes", d.Minutes())
	case d < time.Hour*48:
		s = fmt.Sprintf("%0.1f hours", d.Hours())
	case d < time.Hour*24*9:
		s = fmt.Sprintf("%0.1f days", d.Hours() / 24)
	default:
		s = fmt.Sprintf("%0.f days", d.Hours() / 24)
	}
	return fmt.Sprintf("%s ago", s)
}
