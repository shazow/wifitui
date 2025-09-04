//go:build mock

package main

import (
	"github.com/shazow/wifitui/wifi"
	mockBackend "github.com/shazow/wifitui/wifi/mock"
)

func GetBackend() (wifi.Backend, error) {
	return mockBackend.New()
}
