//go:build linux && !mock

package main

import (
	"log/slog"

	"github.com/shazow/wifitui/wifi"
	"github.com/shazow/wifitui/wifi/iwd"
	"github.com/shazow/wifitui/wifi/networkmanager"
)

func GetBackend(logger *slog.Logger) (wifi.Backend, error) {
	b, err := networkmanager.New(logger)
	if err == nil {
		return b, nil
	}
	logger.Warn("failed to initialize networkmanager backend, falling back to iwd", "error", err)
	// If networkmanager dbus backend failed to initialize, try the iwd backend
	return iwd.New(logger)
}
