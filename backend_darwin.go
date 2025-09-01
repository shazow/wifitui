//go:build darwin && !mock

package main

import (
	"github.com/shazow/wifitui/backend"
	"github.com/shazow/wifitui/backend/darwin"
)

func GetBackend() (backend.Backend, error) {
	return darwin.New()
}
