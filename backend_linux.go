//go:build linux && !mock

package main

import (
	"log/slog"

	"github.com/shazow/wifitui/wifi"
	"github.com/shazow/wifitui/wifi/iwd"
	"github.com/shazow/wifitui/wifi/networkmanager"
)

func GetBackend(logger *slog.Logger) (wifi.Backend, error) {
	networkmanager.SetLogger(logger)
	b, err := networkmanager.New()
	if err == nil {
		return b, nil
	}
	logger.Warn("failed to initialize networkmanager backend, falling back to iwd", "error", err)

	iwd.SetLogger(logger)
	// If networkmanager dbus backend failed to initialize, try the iwd backend
	return iwd.New()
}
