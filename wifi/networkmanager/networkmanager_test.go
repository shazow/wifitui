//go:build linux

package networkmanager

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gonetworkmanager "github.com/Wifx/gonetworkmanager/v3"
	"github.com/godbus/dbus/v5"
	"github.com/shazow/wifitui/wifi"
)

type mockNM struct {
	gonetworkmanager.NetworkManager
	getDevicesFunc                   func() ([]gonetworkmanager.Device, error)
	getPropertyWirelessEnabledFunc   func() (bool, error)
	getPropertyActiveConnectionsFunc func() ([]gonetworkmanager.ActiveConnection, error)
	activateConnectionFunc           func(gonetworkmanager.Connection, gonetworkmanager.Device, *dbus.Object) (gonetworkmanager.ActiveConnection, error)
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

func (m *mockNM) ActivateConnection(conn gonetworkmanager.Connection, device gonetworkmanager.Device, specificObject *dbus.Object) (gonetworkmanager.ActiveConnection, error) {
	if m.activateConnectionFunc != nil {
		return m.activateConnectionFunc(conn, device, specificObject)
	}
	return nil, nil
}

type mockDeviceWireless struct {
	gonetworkmanager.DeviceWireless
	path                     dbus.ObjectPath
	iface                    string
	accessPoints             []gonetworkmanager.AccessPoint
	allAccessPoints          []gonetworkmanager.AccessPoint
	getAccessPointsCalled    bool
	getAllAccessPointsCalled bool
}

func (m *mockDeviceWireless) GetPath() dbus.ObjectPath {
	if m.path == "" {
		return dbus.ObjectPath("/org/freedesktop/NetworkManager/Devices/1")
	}
	return m.path
}

func (m *mockDeviceWireless) GetPropertyInterface() (string, error) {
	if m.iface == "" {
		return "wlan0", nil
	}
	return m.iface, nil
}

func (m *mockDeviceWireless) GetAccessPoints() ([]gonetworkmanager.AccessPoint, error) {
	m.getAccessPointsCalled = true
	return m.accessPoints, nil
}

func (m *mockDeviceWireless) GetAllAccessPoints() ([]gonetworkmanager.AccessPoint, error) {
	m.getAllAccessPointsCalled = true
	if m.allAccessPoints != nil {
		return m.allAccessPoints, nil
	}
	return m.accessPoints, nil
}

type mockSettings struct {
	gonetworkmanager.Settings
	connections              []gonetworkmanager.Connection
	addConnectionUnsavedFunc func(gonetworkmanager.ConnectionSettings) (gonetworkmanager.Connection, error)
}

func (m *mockSettings) ListConnections() ([]gonetworkmanager.Connection, error) {
	return m.connections, nil
}

func (m *mockSettings) AddConnectionUnsaved(settings gonetworkmanager.ConnectionSettings) (gonetworkmanager.Connection, error) {
	if m.addConnectionUnsavedFunc != nil {
		return m.addConnectionUnsavedFunc(settings)
	}
	return &mockConnection{}, nil
}

type mockConnection struct {
	gonetworkmanager.Connection
	saveCalled   bool
	deleteCalled bool
}

func (m *mockConnection) Save() error {
	m.saveCalled = true
	return nil
}

func (m *mockConnection) Delete() error {
	m.deleteCalled = true
	return nil
}

type mockActiveConnection struct {
	gonetworkmanager.ActiveConnection
	state gonetworkmanager.NmActiveConnectionState
}

func (m *mockActiveConnection) SubscribeState(receiver chan gonetworkmanager.StateChange, exit chan struct{}) error {
	return nil
}

func (m *mockActiveConnection) GetPropertyState() (gonetworkmanager.NmActiveConnectionState, error) {
	if m.state == 0 {
		return gonetworkmanager.NmActiveConnectionStateActivated, nil
	}
	return m.state, nil
}

type mockAccessPoint struct {
	gonetworkmanager.AccessPoint
	path      dbus.ObjectPath
	ssid      string
	bssid     string
	strength  uint8
	frequency uint32
	flags     uint32
	wpaFlags  uint32
	rsnFlags  uint32
}

func newMockAccessPoint(ssid, bssid string, strength uint8) *mockAccessPoint {
	return &mockAccessPoint{
		path:      dbus.ObjectPath("/org/freedesktop/NetworkManager/AccessPoint/" + bssid),
		ssid:      ssid,
		bssid:     bssid,
		strength:  strength,
		frequency: 2412,
		rsnFlags:  1,
	}
}

func (m *mockAccessPoint) GetPath() dbus.ObjectPath         { return m.path }
func (m *mockAccessPoint) GetPropertySSID() (string, error) { return m.ssid, nil }
func (m *mockAccessPoint) GetPropertyHWAddress() (string, error) {
	return m.bssid, nil
}
func (m *mockAccessPoint) GetPropertyStrength() (uint8, error) { return m.strength, nil }
func (m *mockAccessPoint) GetPropertyFrequency() (uint32, error) {
	return m.frequency, nil
}
func (m *mockAccessPoint) GetPropertyFlags() (uint32, error)    { return m.flags, nil }
func (m *mockAccessPoint) GetPropertyWPAFlags() (uint32, error) { return m.wpaFlags, nil }
func (m *mockAccessPoint) GetPropertyRSNFlags() (uint32, error) { return m.rsnFlags, nil }
func (m *mockAccessPoint) MarshalJSON() ([]byte, error)         { return nil, nil }

func newTestBackend(device *mockDeviceWireless, connections []gonetworkmanager.Connection) *Backend {
	return &Backend{
		NM: &mockNM{
			getDevicesFunc: func() ([]gonetworkmanager.Device, error) {
				return []gonetworkmanager.Device{device}, nil
			},
		},
		Settings:     &mockSettings{connections: connections},
		Connections:  make(map[string]gonetworkmanager.Connection),
		AccessPoints: make(map[string]gonetworkmanager.AccessPoint),
	}
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

func TestListNetworks_ReturnsCachedListWhenScanFails(t *testing.T) {
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{
			newMockAccessPoint("Cafe", "00:00:00:00:00:01", 67),
		},
	}
	b := newTestBackend(device, nil)
	b.scanFunc = func(gonetworkmanager.DeviceWireless, map[string]dbus.Variant) error {
		return errors.New("scan not allowed")
	}

	result, err := b.ListNetworks(wifi.ScanAuto)
	if err != nil {
		t.Fatalf("ListNetworks(ScanAuto) returned fatal scan error: %v", err)
	}
	connections := result.Connections
	if len(connections) != 1 {
		t.Fatalf("ListNetworks(ScanAuto) returned %d connections, want 1", len(connections))
	}
	if connections[0].SSID != "Cafe" {
		t.Fatalf("ListNetworks(ScanAuto) returned SSID %q, want Cafe", connections[0].SSID)
	}
	if !result.IsCached {
		t.Fatal("ListNetworks(ScanAuto) did not mark cached results after scan failure")
	}
	if b.lastScan.IsZero() {
		t.Fatal("ListNetworks(ScanAuto) did not record lastScan after a scan failure")
	}
}

func TestListNetworks_MarksCachedWhenScanFails(t *testing.T) {
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{
			newMockAccessPoint("Cafe", "00:00:00:00:00:01", 67),
		},
	}
	b := newTestBackend(device, nil)
	b.scanFunc = func(gonetworkmanager.DeviceWireless, map[string]dbus.Variant) error {
		return errors.New("scan not allowed")
	}

	result, err := b.ListNetworks(wifi.ScanAuto)
	if err != nil {
		t.Fatalf("ListNetworks(ScanAuto) returned fatal scan error: %v", err)
	}
	if !result.IsCached {
		t.Fatal("ListNetworks(ScanAuto) did not mark cached results after scan failure")
	}
}

func TestListNetworks_ClearsCachedWhenScanSucceeds(t *testing.T) {
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{
			newMockAccessPoint("Cafe", "00:00:00:00:00:01", 67),
		},
	}
	b := newTestBackend(device, nil)
	b.scanCached = true
	b.scanFunc = func(gonetworkmanager.DeviceWireless, map[string]dbus.Variant) error {
		return nil
	}

	result, err := b.ListNetworks(wifi.ScanAuto)
	if err != nil {
		t.Fatalf("ListNetworks(ScanAuto) returned error: %v", err)
	}
	if result.IsCached {
		t.Fatal("ListNetworks(ScanAuto) marked cached results after successful scan")
	}
}

func TestListNetworks_SkipsScanWhenLastScanFresh(t *testing.T) {
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{
			newMockAccessPoint("Fresh", "00:00:00:00:00:02", 75),
		},
	}
	b := newTestBackend(device, nil)
	b.lastScan = time.Now().Add(-10 * time.Second)
	b.scanInterval = 30 * time.Second
	scanCalled := false
	b.scanFunc = func(gonetworkmanager.DeviceWireless, map[string]dbus.Variant) error {
		scanCalled = true
		return nil
	}

	result, err := b.ListNetworks(wifi.ScanAuto)
	if err != nil {
		t.Fatalf("ListNetworks(ScanAuto) returned error: %v", err)
	}
	connections := result.Connections
	if scanCalled {
		t.Fatal("ListNetworks(ScanAuto) requested a scan even though LastScan was fresh")
	}
	if len(connections) != 1 || connections[0].SSID != "Fresh" {
		t.Fatalf("ListNetworks(ScanAuto) returned %#v, want Fresh network", connections)
	}
}

func TestListNetworks_ForceScanWhenLastScanFresh(t *testing.T) {
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{
			newMockAccessPoint("Forced", "00:00:00:00:00:02", 75),
		},
	}
	b := newTestBackend(device, nil)
	b.lastScan = time.Now().Add(-10 * time.Second)
	b.scanInterval = 30 * time.Second
	scanCalled := false
	b.scanFunc = func(gonetworkmanager.DeviceWireless, map[string]dbus.Variant) error {
		scanCalled = true
		return nil
	}

	result, err := b.ListNetworks(wifi.ScanForce)
	if err != nil {
		t.Fatalf("ListNetworks(ScanForce) returned error: %v", err)
	}
	if !scanCalled {
		t.Fatal("ListNetworks(ScanForce) skipped scan even though force was requested")
	}
	if len(result.Connections) != 1 || result.Connections[0].SSID != "Forced" {
		t.Fatalf("ListNetworks(ScanForce) returned %#v, want Forced network", result.Connections)
	}
}

func TestListNetworks_RequestsScanWhenLastScanUnset(t *testing.T) {
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{
			newMockAccessPoint("Unset", "00:00:00:00:00:03", 75),
		},
	}
	b := newTestBackend(device, nil)
	scanCalled := false
	b.scanFunc = func(gonetworkmanager.DeviceWireless, map[string]dbus.Variant) error {
		scanCalled = true
		return nil
	}

	_, err := b.ListNetworks(wifi.ScanAuto)
	if err != nil {
		t.Fatalf("ListNetworks(ScanAuto) returned error: %v", err)
	}
	if !scanCalled {
		t.Fatal("ListNetworks(ScanAuto) skipped scan even though LastScan was unset")
	}
	if b.lastScan.IsZero() {
		t.Fatal("ListNetworks(ScanAuto) did not record lastScan after requesting a scan")
	}
}

func TestScanIfStale_CoalescesConcurrentScans(t *testing.T) {
	device := &mockDeviceWireless{}
	b := newTestBackend(device, nil)

	var scanCalls atomic.Int32
	enteredScan := make(chan struct{}, 2)
	releaseScan := make(chan struct{})
	b.scanFunc = func(gonetworkmanager.DeviceWireless, map[string]dbus.Variant) error {
		scanCalls.Add(1)
		enteredScan <- struct{}{}
		<-releaseScan
		return nil
	}

	var wg sync.WaitGroup
	scan := func() {
		defer wg.Done()
		b.scanIfStale(device)
	}

	wg.Add(1)
	go scan()
	<-enteredScan

	wg.Add(1)
	go scan()

	select {
	case <-enteredScan:
	case <-time.After(20 * time.Millisecond):
	}
	close(releaseScan)
	wg.Wait()
	if got := scanCalls.Load(); got != 1 {
		t.Fatalf("scanIfStale made %d scan attempts, want 1", got)
	}
}

func TestListNetworks_DefersRetryAfterScanFailure(t *testing.T) {
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{
			newMockAccessPoint("Deferred", "00:00:00:00:00:05", 64),
		},
	}
	b := newTestBackend(device, nil)
	b.scanInterval = 30 * time.Second

	var scanCalls atomic.Int32
	b.scanFunc = func(gonetworkmanager.DeviceWireless, map[string]dbus.Variant) error {
		scanCalls.Add(1)
		return errors.New("scan not allowed")
	}

	for range 2 {
		_, err := b.ListNetworks(wifi.ScanAuto)
		if err != nil {
			t.Fatalf("ListNetworks(ScanAuto) returned error: %v", err)
		}
	}

	if got := scanCalls.Load(); got != 1 {
		t.Fatalf("ListNetworks(ScanAuto) made %d scan attempts, want 1", got)
	}
}

func TestListNetworks_UsesAllAccessPoints(t *testing.T) {
	device := &mockDeviceWireless{
		allAccessPoints: []gonetworkmanager.AccessPoint{
			newMockAccessPoint("AllAP", "00:00:00:00:00:03", 54),
		},
	}
	b := newTestBackend(device, nil)

	result, err := b.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("ListNetworks(ScanNever) returned error: %v", err)
	}
	connections := result.Connections
	if !device.getAllAccessPointsCalled {
		t.Fatal("ListNetworks(ScanNever) did not call GetAllAccessPoints")
	}
	if len(connections) != 1 || connections[0].SSID != "AllAP" {
		t.Fatalf("ListNetworks(ScanNever) returned %#v, want AllAP network", connections)
	}
}

func TestListNetworks_MergesDuplicateAccessPointsOnce(t *testing.T) {
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{
			newMockAccessPoint("Mesh", "00:00:00:00:00:04", 35),
			newMockAccessPoint("Mesh", "00:00:00:00:00:05", 82),
		},
	}
	b := newTestBackend(device, nil)

	result, err := b.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("ListNetworks(ScanNever) returned error: %v", err)
	}
	connections := result.Connections
	if len(connections) != 1 {
		t.Fatalf("ListNetworks(ScanNever) returned %d connections, want 1", len(connections))
	}
	if got := len(connections[0].AccessPoints); got != 2 {
		t.Fatalf("merged connection has %d access points, want 2: %#v", got, connections[0].AccessPoints)
	}
	if connections[0].AccessPoints[0].BSSID != "00:00:00:00:00:05" {
		t.Fatalf("strongest access point was not first: %#v", connections[0].AccessPoints)
	}
}

func TestListNetworks_PreservesWeakerDuplicateAccessPoint(t *testing.T) {
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{
			newMockAccessPoint("Mesh", "00:00:00:00:00:06", 90),
			newMockAccessPoint("Mesh", "00:00:00:00:00:07", 20),
		},
	}
	b := newTestBackend(device, nil)

	result, err := b.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("ListNetworks(ScanNever) returned error: %v", err)
	}
	connections := result.Connections
	if len(connections) != 1 {
		t.Fatalf("ListNetworks(ScanNever) returned %d connections, want 1", len(connections))
	}
	if got := len(connections[0].AccessPoints); got != 2 {
		t.Fatalf("merged connection has %d access points, want 2: %#v", got, connections[0].AccessPoints)
	}
	if ap := b.AccessPoints["Mesh"]; ap != device.accessPoints[0] {
		t.Fatalf("strongest access point was not retained for activation")
	}
}

func TestHiddenSSIDScanOptions(t *testing.T) {
	options := hiddenSSIDScanOptions("HiddenNet")

	variant, ok := options["ssids"]
	if !ok {
		t.Fatal("hiddenSSIDScanOptions did not set ssids")
	}
	ssids, ok := variant.Value().([][]byte)
	if !ok {
		t.Fatalf("ssids option has type %T, want [][]byte", variant.Value())
	}
	if len(ssids) != 1 || string(ssids[0]) != "HiddenNet" {
		t.Fatalf("ssids option = %#v, want HiddenNet", ssids)
	}
}

func TestJoinNetwork_HiddenRequestsTargetedScan(t *testing.T) {
	device := &mockDeviceWireless{}
	connection := &mockConnection{}
	var events []string
	var scanOptions map[string]dbus.Variant

	b := newTestBackend(device, nil)
	b.Settings = &mockSettings{
		addConnectionUnsavedFunc: func(settings gonetworkmanager.ConnectionSettings) (gonetworkmanager.Connection, error) {
			events = append(events, "add")
			return connection, nil
		},
	}
	b.NM = &mockNM{
		getDevicesFunc: func() ([]gonetworkmanager.Device, error) {
			return []gonetworkmanager.Device{device}, nil
		},
		activateConnectionFunc: func(conn gonetworkmanager.Connection, device gonetworkmanager.Device, specificObject *dbus.Object) (gonetworkmanager.ActiveConnection, error) {
			events = append(events, "activate")
			return &mockActiveConnection{}, nil
		},
	}
	b.scanFunc = func(device gonetworkmanager.DeviceWireless, options map[string]dbus.Variant) error {
		events = append(events, "scan")
		scanOptions = options
		return nil
	}

	err := b.JoinNetwork("HiddenNet", "password", wifi.SecurityWPA, true)
	if err != nil {
		t.Fatalf("JoinNetwork(hidden) returned error: %v", err)
	}
	if len(events) < 3 || events[0] != "scan" || events[1] != "add" || events[2] != "activate" {
		t.Fatalf("events = %#v, want scan before add and activate", events)
	}
	variant, ok := scanOptions["ssids"]
	if !ok {
		t.Fatal("hidden join did not request ssids scan option")
	}
	ssids, ok := variant.Value().([][]byte)
	if !ok || len(ssids) != 1 || string(ssids[0]) != "HiddenNet" {
		t.Fatalf("hidden join ssids option = %#v, want HiddenNet", variant.Value())
	}
	if !connection.saveCalled {
		t.Fatal("hidden join did not save activated connection")
	}
	if connection.deleteCalled {
		t.Fatal("hidden join deleted connection after successful activation")
	}
}

func TestJoinNetwork_HiddenScanFailureDoesNotAbortActivation(t *testing.T) {
	device := &mockDeviceWireless{}
	var activated bool

	b := newTestBackend(device, nil)
	b.Settings = &mockSettings{}
	b.NM = &mockNM{
		getDevicesFunc: func() ([]gonetworkmanager.Device, error) {
			return []gonetworkmanager.Device{device}, nil
		},
		activateConnectionFunc: func(conn gonetworkmanager.Connection, device gonetworkmanager.Device, specificObject *dbus.Object) (gonetworkmanager.ActiveConnection, error) {
			activated = true
			return &mockActiveConnection{}, nil
		},
	}
	b.scanFunc = func(gonetworkmanager.DeviceWireless, map[string]dbus.Variant) error {
		return errors.New("scan not allowed")
	}

	err := b.JoinNetwork("HiddenNet", "password", wifi.SecurityWPA, true)
	if err != nil {
		t.Fatalf("JoinNetwork(hidden) returned error after targeted scan failure: %v", err)
	}
	if !activated {
		t.Fatal("hidden join did not continue to activation after targeted scan failure")
	}
}

func TestIsNetworkChangeSignal(t *testing.T) {
	devicePath := dbus.ObjectPath("/org/freedesktop/NetworkManager/Devices/1")
	tests := []struct {
		name string
		sig  *dbus.Signal
		want bool
	}{
		{
			name: "wireless device properties",
			sig: &dbus.Signal{
				Path: devicePath,
				Name: "org.freedesktop.DBus.Properties.PropertiesChanged",
				Body: []interface{}{"org.freedesktop.NetworkManager.Device.Wireless"},
			},
			want: true,
		},
		{
			name: "access point added",
			sig: &dbus.Signal{
				Path: devicePath,
				Name: "org.freedesktop.NetworkManager.Device.Wireless.AccessPointAdded",
			},
			want: true,
		},
		{
			name: "access point removed",
			sig: &dbus.Signal{
				Path: devicePath,
				Name: "org.freedesktop.NetworkManager.Device.Wireless.AccessPointRemoved",
			},
			want: true,
		},
		{
			name: "access point properties",
			sig: &dbus.Signal{
				Path: dbus.ObjectPath("/org/freedesktop/NetworkManager/AccessPoint/1"),
				Name: "org.freedesktop.DBus.Properties.PropertiesChanged",
				Body: []interface{}{"org.freedesktop.NetworkManager.AccessPoint"},
			},
			want: true,
		},
		{
			name: "other device properties",
			sig: &dbus.Signal{
				Path: dbus.ObjectPath("/org/freedesktop/NetworkManager/Devices/2"),
				Name: "org.freedesktop.DBus.Properties.PropertiesChanged",
				Body: []interface{}{"org.freedesktop.NetworkManager.Device.Wireless"},
			},
			want: false,
		},
		{
			name: "other interface",
			sig: &dbus.Signal{
				Path: devicePath,
				Name: "org.freedesktop.DBus.Properties.PropertiesChanged",
				Body: []interface{}{"org.freedesktop.NetworkManager.Device.Wired"},
			},
			want: false,
		},
		{
			name: "nil signal",
			sig:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNetworkChangeSignal(tt.sig, devicePath); got != tt.want {
				t.Fatalf("isNetworkChangeSignal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsUnavailableDBusError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "service unknown",
			err:  testError("org.freedesktop.DBus.Error.ServiceUnknown: The name is not activatable"),
			want: true,
		},
		{
			name: "name has no owner",
			err:  testError("org.freedesktop.DBus.Error.NameHasNoOwner: Could not get owner"),
			want: true,
		},
		{
			name: "other error",
			err:  testError("some other error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUnavailableDBusError(tt.err); got != tt.want {
				t.Fatalf("isUnavailableDBusError() = %v, want %v", got, tt.want)
			}
		})
	}
}

type testError string

func (e testError) Error() string { return string(e) }
