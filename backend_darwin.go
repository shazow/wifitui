//go:build darwin && !mock

package main

import (
	"github.com/shazow/wifitui/wifi"
	"github.com/shazow/wifitui/wifi/darwin"
)

func GetBackend() (wifi.Backend, error) {
	return darwin.New()
}
