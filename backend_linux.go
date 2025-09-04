//go:build linux && !mock

package main

import (
	"github.com/shazow/wifitui/wifi"
	"github.com/shazow/wifitui/wifi/iwd"
	"github.com/shazow/wifitui/wifi/networkmanager"
)

func GetBackend() (wifi.Backend, error) {
	b, err := networkmanager.New()
	if err == nil {
		return b, nil
	}
	// If networkmanager dbus backend failed to initialize, try the iwd backend
	return iwd.New()
}
