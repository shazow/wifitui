//go:build darwin && !mock

package main

import (
	"log/slog"

	"github.com/shazow/wifitui/wifi"
	"github.com/shazow/wifitui/wifi/darwin"
)

func GetBackend(logger *slog.Logger) (wifi.Backend, error) {
	return darwin.New(logger)
}
