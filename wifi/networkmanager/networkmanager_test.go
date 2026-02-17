package networkmanager

import (
	"fmt"
	"testing"

	gonetworkmanager "github.com/Wifx/gonetworkmanager/v3"
)

type mockNM struct {
	gonetworkmanager.NetworkManager
	getDevicesFunc                   func() ([]gonetworkmanager.Device, error)
	getPropertyWirelessEnabledFunc   func() (bool, error)
	getPropertyActiveConnectionsFunc func() ([]gonetworkmanager.ActiveConnection, error)
	activateWirelessConnectionFunc   func(connection gonetworkmanager.Connection, device gonetworkmanager.Device, specificObject gonetworkmanager.AccessPoint) (gonetworkmanager.ActiveConnection, error)
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

func (m *mockNM) GetPropertyActiveConnections() ([]gonetworkmanager.ActiveConnection, error) {
	if m.getPropertyActiveConnectionsFunc != nil {
		return m.getPropertyActiveConnectionsFunc()
	}
	return nil, nil
}

func (m *mockNM) ActivateWirelessConnection(connection gonetworkmanager.Connection, device gonetworkmanager.Device, specificObject gonetworkmanager.AccessPoint) (gonetworkmanager.ActiveConnection, error) {
	if m.activateWirelessConnectionFunc != nil {
		return m.activateWirelessConnectionFunc(connection, device, specificObject)
	}
	return nil, fmt.Errorf("not implemented")
}

type mockDeviceWireless struct {
	gonetworkmanager.DeviceWireless
	getAccessPointsFunc func() ([]gonetworkmanager.AccessPoint, error)
}

func (m *mockDeviceWireless) GetAccessPoints() ([]gonetworkmanager.AccessPoint, error) {
	if m.getAccessPointsFunc != nil {
		return m.getAccessPointsFunc()
	}
	return nil, nil
}

type mockSettings struct {
	gonetworkmanager.Settings
	listConnectionsFunc func() ([]gonetworkmanager.Connection, error)
}

func (m *mockSettings) ListConnections() ([]gonetworkmanager.Connection, error) {
	if m.listConnectionsFunc != nil {
		return m.listConnectionsFunc()
	}
	return nil, nil
}

type mockConnection struct {
	gonetworkmanager.Connection
	getSettingsFunc func() (gonetworkmanager.ConnectionSettings, error)
}

func (m *mockConnection) GetSettings() (gonetworkmanager.ConnectionSettings, error) {
	if m.getSettingsFunc != nil {
		return m.getSettingsFunc()
	}
	return nil, nil
}

type mockAccessPoint struct {
	gonetworkmanager.AccessPoint
	getPropertySSIDFunc      func() (string, error)
	getPropertyStrengthFunc  func() (uint8, error)
	getPropertyHWAddressFunc func() (string, error)
	getPropertyFrequencyFunc func() (uint32, error)
	getPropertyFlagsFunc     func() (uint32, error)
	getPropertyWPAFlagsFunc  func() (uint32, error)
	getPropertyRSNFlagsFunc  func() (uint32, error)
}

func (m *mockAccessPoint) GetPropertySSID() (string, error) {
	if m.getPropertySSIDFunc != nil {
		return m.getPropertySSIDFunc()
	}
	return "", nil
}

func (m *mockAccessPoint) GetPropertyStrength() (uint8, error) {
	if m.getPropertyStrengthFunc != nil {
		return m.getPropertyStrengthFunc()
	}
	return 0, nil
}

func (m *mockAccessPoint) GetPropertyHWAddress() (string, error) {
	if m.getPropertyHWAddressFunc != nil {
		return m.getPropertyHWAddressFunc()
	}
	return "", nil
}

func (m *mockAccessPoint) GetPropertyFrequency() (uint32, error) {
	if m.getPropertyFrequencyFunc != nil {
		return m.getPropertyFrequencyFunc()
	}
	return 0, nil
}

func (m *mockAccessPoint) GetPropertyFlags() (uint32, error) {
	if m.getPropertyFlagsFunc != nil {
		return m.getPropertyFlagsFunc()
	}
	return 0, nil
}

func (m *mockAccessPoint) GetPropertyWPAFlags() (uint32, error) {
	if m.getPropertyWPAFlagsFunc != nil {
		return m.getPropertyWPAFlagsFunc()
	}
	return 0, nil
}

func (m *mockAccessPoint) GetPropertyRSNFlags() (uint32, error) {
	if m.getPropertyRSNFlagsFunc != nil {
		return m.getPropertyRSNFlagsFunc()
	}
	return 0, nil
}

type mockActiveConnection struct {
	gonetworkmanager.ActiveConnection
	subscribeStateFunc   func(chan gonetworkmanager.StateChange, chan struct{}) error
	getPropertyStateFunc func() (gonetworkmanager.NmActiveConnectionState, error)
}

func (m *mockActiveConnection) SubscribeState(ch chan gonetworkmanager.StateChange, done chan struct{}) error {
	if m.subscribeStateFunc != nil {
		return m.subscribeStateFunc(ch, done)
	}
	return nil
}

func (m *mockActiveConnection) GetPropertyState() (gonetworkmanager.NmActiveConnectionState, error) {
	if m.getPropertyStateFunc != nil {
		return m.getPropertyStateFunc()
	}
	return gonetworkmanager.NmActiveConnectionStateActivated, nil
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

func TestActivateConnection_LazyLoad(t *testing.T) {
	ssid := "test-ssid"
	calledListConnections := false
	calledGetAccessPoints := false
	calledActivate := false

	mockConn := &mockConnection{
		getSettingsFunc: func() (gonetworkmanager.ConnectionSettings, error) {
			return gonetworkmanager.ConnectionSettings{
				"802-11-wireless": {
					"ssid": []byte(ssid),
				},
				"connection": {
					"id": ssid,
				},
			}, nil
		},
	}

	mockAP := &mockAccessPoint{
		getPropertySSIDFunc: func() (string, error) {
			return ssid, nil
		},
		getPropertyStrengthFunc: func() (uint8, error) {
			return 100, nil
		},
	}

	mockDev := &mockDeviceWireless{
		getAccessPointsFunc: func() ([]gonetworkmanager.AccessPoint, error) {
			calledGetAccessPoints = true
			return []gonetworkmanager.AccessPoint{mockAP}, nil
		},
	}

	mockNM := &mockNM{
		getDevicesFunc: func() ([]gonetworkmanager.Device, error) {
			return []gonetworkmanager.Device{mockDev}, nil
		},
		activateWirelessConnectionFunc: func(connection gonetworkmanager.Connection, device gonetworkmanager.Device, specificObject gonetworkmanager.AccessPoint) (gonetworkmanager.ActiveConnection, error) {
			calledActivate = true
			return &mockActiveConnection{
				subscribeStateFunc: func(ch chan gonetworkmanager.StateChange, done chan struct{}) error {
					// Simulate immediate activation
					ch <- gonetworkmanager.StateChange{State: gonetworkmanager.NmActiveConnectionStateActivated}
					return nil
				},
				getPropertyStateFunc: func() (gonetworkmanager.NmActiveConnectionState, error) {
					return gonetworkmanager.NmActiveConnectionStateActivated, nil
				},
			}, nil
		},
	}

	mockSet := &mockSettings{
		listConnectionsFunc: func() ([]gonetworkmanager.Connection, error) {
			calledListConnections = true
			return []gonetworkmanager.Connection{mockConn}, nil
		},
	}

	b := &Backend{
		NM:           mockNM,
		Settings:     mockSet,
		Connections:  make(map[string]gonetworkmanager.Connection),
		AccessPoints: make(map[string]gonetworkmanager.AccessPoint),
	}

	err := b.ActivateConnection(ssid)

	if !calledListConnections {
		t.Errorf("expected ListConnections to be called (lazy loading)")
	}
	if !calledGetAccessPoints {
		t.Errorf("expected GetAccessPoints to be called (lazy loading)")
	}
	if err != nil {
		t.Errorf("ActivateConnection failed: %v", err)
	}
	if !calledActivate {
		t.Errorf("expected ActivateWirelessConnection to be called")
	}
}
