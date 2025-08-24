//go:build !linux && !darwin
package main

import (
	"fmt"
	"github.com/shazow/wifitui/backend"
)

// GetBackend picks a backend based on the system's environment and build flags.
func GetBackend() (backend.Backend, error) {
	// This is a placeholder and should be implemented in build-specific files.
	return nil, fmt.Errorf("no supported backend")
}
