package networkmanager

import (
	"testing"

	gonetworkmanager "github.com/Wifx/gonetworkmanager/v3"
)

type mockNM struct {
	gonetworkmanager.NetworkManager
	getDevicesFunc func() ([]gonetworkmanager.Device, error)
}

func (m *mockNM) GetDevices() ([]gonetworkmanager.Device, error) {
	if m.getDevicesFunc != nil {
		return m.getDevicesFunc()
	}
	return nil, nil
}

type mockDeviceWireless struct {
	gonetworkmanager.DeviceWireless
	scanFunc            func() error
	getAccessPointsFunc func() ([]gonetworkmanager.AccessPoint, error)
}

func (m *mockDeviceWireless) RequestScan() error {
	if m.scanFunc != nil {
		return m.scanFunc()
	}
	return nil
}

func (m *mockDeviceWireless) GetAccessPoints() ([]gonetworkmanager.AccessPoint, error) {
	if m.getAccessPointsFunc != nil {
		return m.getAccessPointsFunc()
	}
	return nil, nil
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

type mockNMForList struct {
	gonetworkmanager.NetworkManager
	getDevicesFunc func() ([]gonetworkmanager.Device, error)
}

func (m *mockNMForList) GetDevices() ([]gonetworkmanager.Device, error) {
	if m.getDevicesFunc != nil {
		return m.getDevicesFunc()
	}
	return nil, nil
}

func (m *mockNMForList) GetPropertyWirelessEnabled() (bool, error) {
	return true, nil
}

func (m *mockNMForList) GetPropertyActiveConnections() ([]gonetworkmanager.ActiveConnection, error) {
	return []gonetworkmanager.ActiveConnection{}, nil
}

type mockSettings struct {
	gonetworkmanager.Settings
}

func (m *mockSettings) ListConnections() ([]gonetworkmanager.Connection, error) {
	return []gonetworkmanager.Connection{}, nil
}

type mockAccessPoint struct {
	gonetworkmanager.AccessPoint
	ssid      string
	strength  uint8
	frequency uint32
	bssid     string
}

func (m *mockAccessPoint) GetPropertySSID() (string, error) {
	return m.ssid, nil
}

func (m *mockAccessPoint) GetPropertyStrength() (uint8, error) {
	return m.strength, nil
}

func (m *mockAccessPoint) GetPropertyFrequency() (uint32, error) {
	return m.frequency, nil
}

func (m *mockAccessPoint) GetPropertyHWAddress() (string, error) {
	return m.bssid, nil
}

func (m *mockAccessPoint) GetPropertyFlags() (uint32, error) {
	return 0, nil
}

func (m *mockAccessPoint) GetPropertyWPAFlags() (uint32, error) {
	return 0, nil
}

func (m *mockAccessPoint) GetPropertyRSNFlags() (uint32, error) {
	return 0, nil
}

func TestBuildNetworkList_AggregatesAPs(t *testing.T) {
	// AP1: Stronger
	ap1 := &mockAccessPoint{
		ssid:      "TestSSID",
		strength:  80,
		frequency: 2412,
		bssid:     "00:11:22:33:44:55",
	}
	// AP2: Weaker
	ap2 := &mockAccessPoint{
		ssid:      "TestSSID",
		strength:  70,
		frequency: 5180,
		bssid:     "66:77:88:99:AA:BB",
	}

	mockDev := &mockDeviceWireless{
		getAccessPointsFunc: func() ([]gonetworkmanager.AccessPoint, error) {
			// Order matters for reproduction: Stronger first to trigger the 'continue' skip logic
			return []gonetworkmanager.AccessPoint{ap1, ap2}, nil
		},
	}

	nm := &mockNMForList{
		getDevicesFunc: func() ([]gonetworkmanager.Device, error) {
			return []gonetworkmanager.Device{mockDev}, nil
		},
	}

	b := &Backend{
		NM:       nm,
		Settings: &mockSettings{},
	}

	// shouldScan = false to skip RequestScan call logic, focusing on GetAccessPoints
	conns, err := b.BuildNetworkList(false)
	if err != nil {
		t.Fatalf("BuildNetworkList failed: %v", err)
	}

	if len(conns) != 1 {
		t.Fatalf("Expected 1 connection, got %d", len(conns))
	}

	conn := conns[0]
	if conn.SSID != "TestSSID" {
		t.Errorf("Expected SSID TestSSID, got %s", conn.SSID)
	}

	if len(conn.AccessPoints) != 2 {
		t.Errorf("Expected 2 AccessPoints, got %d", len(conn.AccessPoints))
	} else {
		// Verify we have both frequencies
		freqs := make(map[uint]bool)
		for _, ap := range conn.AccessPoints {
			freqs[ap.Frequency] = true
		}
		if !freqs[2412] {
			t.Error("Missing 2412 MHz AP")
		}
		if !freqs[5180] {
			t.Error("Missing 5180 MHz AP")
		}
	}
}
