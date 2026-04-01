//go:build linux && !mock

package main

import (
	"errors"
	"testing"

	"github.com/shazow/wifitui/wifi"
	wifimock "github.com/shazow/wifitui/wifi/mock"
)

func TestGetBackend_FallsBackToIWDWhenNetworkManagerUnavailable(t *testing.T) {
	origBackends := backends
	t.Cleanup(func() {
		backends = origBackends
	})

	backends = []func() (wifi.Backend, error){
		func() (wifi.Backend, error) {
			return nil, errors.New("org.freedesktop.DBus.Error.ServiceUnknown: The name is not activatable")
		},
		wifimock.New,
	}

	got, err := GetBackend()
	if err != nil {
		t.Fatalf("GetBackend() unexpected error: %v", err)
	}
	if _, ok := got.(*wifimock.MockBackend); !ok {
		t.Fatalf("GetBackend() got %T, want *mock.MockBackend fallback backend", got)
	}
}
