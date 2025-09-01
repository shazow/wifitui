//go:build linux && !mock

package main

import (
	"github.com/shazow/wifitui/backend"
	"github.com/shazow/wifitui/backend/iwd"
	"github.com/shazow/wifitui/backend/networkmanager"
)

func GetBackend() (backend.Backend, error) {
	b, err := networkmanager.New()
	if err == nil {
		return b, nil
	}
	// If networkmanager dbus backend failed to initialize, try the iwd backend
	return iwd.New()
}
