//go:build mock

package main

import (
	"log/slog"

	"github.com/shazow/wifitui/wifi"
	mockBackend "github.com/shazow/wifitui/wifi/mock"
)

func GetBackend(logger *slog.Logger) (wifi.Backend, error) {
	mockBackend.SetLogger(logger)
	return mockBackend.New()
}
