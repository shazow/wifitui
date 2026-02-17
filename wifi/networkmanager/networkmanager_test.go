package networkmanager

import (
	"errors"
	"testing"

	gonetworkmanager "github.com/Wifx/gonetworkmanager/v3"
)

type mockNM struct {
	gonetworkmanager.NetworkManager
	getDevicesFunc                 func() ([]gonetworkmanager.Device, error)
	getPropertyWirelessEnabledFunc func() (bool, error)
}

func (m *mockNM) GetDevices() ([]gonetworkmanager.Device, error) {
	if m.getDevicesFunc != nil {
		return m.getDevicesFunc()
	}
	return nil, nil
}

func (m *mockNM) GetPropertyWirelessEnabled() (bool, error) {
	if m.getPropertyWirelessEnabledFunc != nil {
		return m.getPropertyWirelessEnabledFunc()
	}
	return true, nil
}

type mockDeviceWireless struct {
	gonetworkmanager.DeviceWireless
}

func TestGetWirelessDevice_Caching(t *testing.T) {
	callCount := 0
	mockDev := &mockDeviceWireless{}

	nm := &mockNM{
		getDevicesFunc: func() ([]gonetworkmanager.Device, error) {
			callCount++
			return []gonetworkmanager.Device{mockDev}, nil
		},
	}

	b := &Backend{
		NM: nm,
	}

	// First call
	dev, err := b.getWirelessDevice()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dev != mockDev {
		t.Errorf("expected device %v, got %v", mockDev, dev)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}

	// Second call (should be cached)
	dev2, err := b.getWirelessDevice()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dev2 != mockDev {
		t.Errorf("expected device %v, got %v", mockDev, dev2)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestActivateConnection_LazyLoad_Minimal(t *testing.T) {
	expectedErr := errors.New("lazy load check error")

	nm := &mockNM{
		getPropertyWirelessEnabledFunc: func() (bool, error) {
			return false, expectedErr
		},
	}

	b := &Backend{
		NM:          nm,
		Connections: make(map[string]gonetworkmanager.Connection),
	}

	// We call ActivateConnection with an empty Connections map.
	// This should trigger getConnection -> BuildNetworkList.
	// BuildNetworkList calls IsWirelessEnabled -> mockNM.GetPropertyWirelessEnabled.
	// If that returns our expectedErr, then we know BuildNetworkList was called.
	err := b.ActivateConnection("any-ssid")

	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}
