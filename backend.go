//go:build !linux && !darwin && !mock

package main

import (
	"fmt"
	"github.com/shazow/wifitui/wifi"
)

// GetBackend picks a backend based on the system's environment and build flags.
func GetBackend() (wifi.Backend, error) {
	// This is a placeholder and should be implemented in build-specific files.
	return nil, fmt.Errorf("no supported backend")
}
