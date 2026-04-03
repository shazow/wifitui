//go:build linux && !mock

package main

import (
	"github.com/shazow/wifitui/wifi"
	"github.com/shazow/wifitui/wifi/iwd"
	"github.com/shazow/wifitui/wifi/networkmanager"
)

var backends = []func() (wifi.Backend, error){
	networkmanager.New,
	iwd.New,
}

func GetBackend() (wifi.Backend, error) {
	var lastErr error
	for _, newBackend := range backends {
		b, err := newBackend()
		if err == nil {
			return b, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
