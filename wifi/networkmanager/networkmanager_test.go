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
	activateWirelessConnectionFunc   func(gonetworkmanager.Connection, gonetworkmanager.Device, gonetworkmanager.AccessPoint) (gonetworkmanager.ActiveConnection, error)
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

func (m *mockNM) ActivateWirelessConnection(conn gonetworkmanager.Connection, device gonetworkmanager.Device, ap gonetworkmanager.AccessPoint) (gonetworkmanager.ActiveConnection, error) {
	if m.activateWirelessConnectionFunc != nil {
		return m.activateWirelessConnectionFunc(conn, device, ap)
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
	managed                  bool
	state                    gonetworkmanager.NmDeviceState
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

func (m *mockDeviceWireless) GetPropertyManaged() (bool, error) {
	if !m.managed && m.state == 0 {
		return true, nil
	}
	return m.managed, nil
}

func (m *mockDeviceWireless) GetPropertyState() (gonetworkmanager.NmDeviceState, error) {
	if m.state == 0 {
		return gonetworkmanager.NmDeviceStateDisconnected, nil
	}
	return m.state, nil
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
	path         dbus.ObjectPath
	settings     gonetworkmanager.ConnectionSettings
	saveCalled   bool
	deleteCalled bool
}

func newMockConnection(path, id, ssid string, security wifi.SecurityType) *mockConnection {
	settings := gonetworkmanager.ConnectionSettings{
		"connection": {
			"id":   id,
			"type": "802-11-wireless",
		},
		"802-11-wireless": {
			"ssid": []byte(ssid),
		},
	}
	switch security {
	case wifi.SecurityWEP:
		settings["802-11-wireless"]["security"] = "802-11-wireless-security"
		settings["802-11-wireless-security"] = map[string]interface{}{
			"key-mgmt": "none",
		}
	case wifi.SecurityWPA:
		settings["802-11-wireless"]["security"] = "802-11-wireless-security"
		settings["802-11-wireless-security"] = map[string]interface{}{
			"key-mgmt": "wpa-psk",
		}
	}
	return &mockConnection{
		path:     dbus.ObjectPath(path),
		settings: settings,
	}
}

func (m *mockConnection) GetPath() dbus.ObjectPath {
	if m.path == "" {
		return dbus.ObjectPath("/org/freedesktop/NetworkManager/Settings/1")
	}
	return m.path
}

func (m *mockConnection) GetSettings() (gonetworkmanager.ConnectionSettings, error) {
	return m.settings, nil
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
	mode      gonetworkmanager.Nm80211Mode
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
		mode:      gonetworkmanager.Nm80211ModeInfra,
		rsnFlags:  uint32(gonetworkmanager.Nm80211APSecKeyMgmtPSK),
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
func (m *mockAccessPoint) GetPropertyMode() (gonetworkmanager.Nm80211Mode, error) {
	return m.mode, nil
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
		connections:  make(map[networkKey]gonetworkmanager.Connection),
		accessPoints: make(map[networkKey]gonetworkmanager.AccessPoint),
	}
}

func TestGetWirelessDevice_Caching(t *testing.T) {
	callCount := 0
	mockDev := &mockDeviceWireless{managed: true, state: gonetworkmanager.NmDeviceStateDisconnected}

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
	connections := result.Networks
	if len(connections) != 1 {
		t.Fatalf("ListNetworks(ScanAuto) returned %d connections, want 1", len(connections))
	}
	if connections[0].SSID != "Cafe" {
		t.Fatalf("ListNetworks(ScanAuto) returned SSID %q, want Cafe", connections[0].SSID)
	}
	if result.ScanError == nil || result.ScanError.Error() != "scan not allowed" {
		t.Fatalf("ListNetworks(ScanAuto) returned scan error %v, want scan not allowed", result.ScanError)
	}
	if b.lastScan.IsZero() {
		t.Fatal("ListNetworks(ScanAuto) did not record lastScan after a scan failure")
	}
}

func TestListNetworks_ClearsCachedWhenScanSucceeds(t *testing.T) {
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{
			newMockAccessPoint("Cafe", "00:00:00:00:00:01", 67),
		},
	}
	b := newTestBackend(device, nil)
	b.scanError = errors.New("previous scan failed")
	b.scanFunc = func(gonetworkmanager.DeviceWireless, map[string]dbus.Variant) error {
		return nil
	}

	result, err := b.ListNetworks(wifi.ScanAuto)
	if err != nil {
		t.Fatalf("ListNetworks(ScanAuto) returned error: %v", err)
	}
	if result.ScanError != nil {
		t.Fatalf("ListNetworks(ScanAuto) retained scan error after successful scan: %v", result.ScanError)
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
	connections := result.Networks
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
	if len(result.Networks) != 1 || result.Networks[0].SSID != "Forced" {
		t.Fatalf("ListNetworks(ScanForce) returned %#v, want Forced network", result.Networks)
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
	connections := result.Networks
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
	connections := result.Networks
	if len(connections) != 1 {
		t.Fatalf("ListNetworks(ScanNever) returned %d connections, want 1", len(connections))
	}
	if got := len(connections[0].AccessPoints); got != 2 {
		t.Fatalf("merged connection has %d access points, want 2: %#v", got, connections[0].AccessPoints)
	}
	if connections[0].AccessPoints[0].BSSID != "00:00:00:00:00:05" {
		t.Fatalf("strongest access point was not first: %#v", connections[0].AccessPoints)
	}
	if ap, err := b.getAccessPoint("Mesh"); err != nil || ap != device.accessPoints[1] {
		t.Fatalf("strongest access point was not retained for activation")
	}
}

func TestListNetworks_DoesNotMarkOpenVariantKnownWhenWPAProfileExists(t *testing.T) {
	openAP := newMockAccessPoint("Cafe", "00:00:00:00:00:08", 90)
	openAP.rsnFlags = 0
	wpaAP := newMockAccessPoint("Cafe", "00:00:00:00:00:09", 40)
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{openAP, wpaAP},
	}
	knownWPA := newMockConnection("/org/freedesktop/NetworkManager/Settings/2", "Cafe WPA", "Cafe", wifi.SecurityWPA)
	b := newTestBackend(device, []gonetworkmanager.Connection{knownWPA})

	result, err := b.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("ListNetworks(ScanNever) returned error: %v", err)
	}

	var openFound, wpaFound bool
	for _, conn := range result.Networks {
		if conn.SSID != "Cafe" {
			continue
		}
		switch conn.Security {
		case wifi.SecurityOpen:
			openFound = true
			if conn.IsKnown {
				t.Fatal("open Cafe variant was marked known by a WPA saved profile")
			}
		case wifi.SecurityWPA:
			wpaFound = true
			if !conn.IsKnown {
				t.Fatal("WPA Cafe variant was not marked known")
			}
		}
	}
	if !openFound || !wpaFound {
		t.Fatalf("ListNetworks returned %#v, want separate open and WPA Cafe variants", result.Networks)
	}
}

func TestListNetworks_KeepsSameSSIDWithDifferentWPAFlagsSeparate(t *testing.T) {
	wpaAP := newMockAccessPoint("Corp", "00:00:00:00:00:10", 80)
	wpaAP.wpaFlags = 1
	wpaAP.rsnFlags = 0
	rsnAP := newMockAccessPoint("Corp", "00:00:00:00:00:11", 70)
	rsnAP.wpaFlags = 0
	rsnAP.rsnFlags = 1
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{wpaAP, rsnAP},
	}
	b := newTestBackend(device, nil)

	result, err := b.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("ListNetworks(ScanNever) returned error: %v", err)
	}

	var corpRows int
	for _, conn := range result.Networks {
		if conn.SSID == "Corp" {
			corpRows++
			if got := len(conn.AccessPoints); got != 1 {
				t.Fatalf("Corp row merged %d APs, want each flag variant separate: %#v", got, conn)
			}
		}
	}
	if corpRows != 2 {
		t.Fatalf("ListNetworks returned %d Corp rows, want 2 for different WPA/RSN flags: %#v", corpRows, result.Networks)
	}
}

func TestListNetworks_DoesNotMark8021XVariantKnownWhenPSKProfileExists(t *testing.T) {
	pskAP := newMockAccessPoint("Corp", "00:00:00:00:00:12", 40)
	pskAP.rsnFlags = uint32(gonetworkmanager.Nm80211APSecKeyMgmtPSK)
	eapAP := newMockAccessPoint("Corp", "00:00:00:00:00:13", 90)
	eapAP.rsnFlags = uint32(gonetworkmanager.Nm80211APSecKeyMgmt8021X)
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{eapAP, pskAP},
	}
	knownPSK := newMockConnection("/org/freedesktop/NetworkManager/Settings/3", "Corp PSK", "Corp", wifi.SecurityWPA)
	b := newTestBackend(device, []gonetworkmanager.Connection{knownPSK})

	result, err := b.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("ListNetworks(ScanNever) returned error: %v", err)
	}

	var pskFound, eapFound bool
	for _, conn := range result.Networks {
		if conn.SSID != "Corp" || len(conn.AccessPoints) != 1 {
			continue
		}
		switch conn.AccessPoints[0].BSSID {
		case pskAP.bssid:
			pskFound = true
			if !conn.IsKnown {
				t.Fatal("PSK Corp variant was not marked known by a PSK saved profile")
			}
		case eapAP.bssid:
			eapFound = true
			if conn.IsKnown {
				t.Fatal("802.1x Corp variant was marked known by a PSK saved profile")
			}
		}
	}
	if !pskFound || !eapFound {
		t.Fatalf("ListNetworks returned %#v, want separate PSK and 802.1x Corp variants", result.Networks)
	}
}

func TestListNetworks_DoesNotMarkDifferentModeVariantKnown(t *testing.T) {
	adhocAP := newMockAccessPoint("Direct", "00:00:00:00:00:14", 60)
	adhocAP.mode = gonetworkmanager.Nm80211ModeAdhoc
	adhocAP.rsnFlags = uint32(gonetworkmanager.Nm80211APSecKeyMgmtPSK)
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{adhocAP},
	}
	knownInfra := newMockConnection("/org/freedesktop/NetworkManager/Settings/4", "Direct Infra", "Direct", wifi.SecurityWPA)
	knownInfra.settings["802-11-wireless"]["mode"] = "infrastructure"
	b := newTestBackend(device, []gonetworkmanager.Connection{knownInfra})

	result, err := b.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("ListNetworks(ScanNever) returned error: %v", err)
	}

	for _, conn := range result.Networks {
		if len(conn.AccessPoints) == 1 && conn.AccessPoints[0].BSSID == adhocAP.bssid {
			if conn.IsKnown {
				t.Fatal("ad-hoc Direct variant was marked known by an infrastructure saved profile")
			}
			return
		}
	}
	t.Fatalf("ListNetworks returned %#v, want visible ad-hoc Direct variant", result.Networks)
}

func TestActivateNetwork_UsesAccessPointMatchingKnownSecurity(t *testing.T) {
	openAP := newMockAccessPoint("Cafe", "00:00:00:00:00:12", 90)
	openAP.rsnFlags = 0
	wpaAP := newMockAccessPoint("Cafe", "00:00:00:00:00:13", 40)
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{openAP, wpaAP},
	}
	knownWPA := newMockConnection("/org/freedesktop/NetworkManager/Settings/3", "Cafe WPA", "Cafe", wifi.SecurityWPA)
	b := newTestBackend(device, []gonetworkmanager.Connection{knownWPA})

	mockManager := b.NM.(*mockNM)
	var activatedAP gonetworkmanager.AccessPoint
	mockManager.activateWirelessConnectionFunc = func(conn gonetworkmanager.Connection, device gonetworkmanager.Device, ap gonetworkmanager.AccessPoint) (gonetworkmanager.ActiveConnection, error) {
		activatedAP = ap
		return &mockActiveConnection{}, nil
	}

	_, err := b.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("ListNetworks(ScanNever) returned error: %v", err)
	}
	if err := b.ActivateNetwork("Cafe"); err != nil {
		t.Fatalf("ActivateNetwork(Cafe) returned error: %v", err)
	}
	if activatedAP != wpaAP {
		t.Fatalf("ActivateNetwork used AP %#v, want WPA AP %#v", activatedAP, wpaAP)
	}
}

func TestActivateNetwork_UsesAccessPointMatchingKnownKeyManagement(t *testing.T) {
	eapAP := newMockAccessPoint("Corp", "00:00:00:00:00:15", 90)
	eapAP.rsnFlags = uint32(gonetworkmanager.Nm80211APSecKeyMgmt8021X)
	pskAP := newMockAccessPoint("Corp", "00:00:00:00:00:16", 40)
	pskAP.rsnFlags = uint32(gonetworkmanager.Nm80211APSecKeyMgmtPSK)
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{eapAP, pskAP},
	}
	knownPSK := newMockConnection("/org/freedesktop/NetworkManager/Settings/5", "Corp PSK", "Corp", wifi.SecurityWPA)
	b := newTestBackend(device, []gonetworkmanager.Connection{knownPSK})

	mockManager := b.NM.(*mockNM)
	var activatedAP gonetworkmanager.AccessPoint
	mockManager.activateWirelessConnectionFunc = func(conn gonetworkmanager.Connection, device gonetworkmanager.Device, ap gonetworkmanager.AccessPoint) (gonetworkmanager.ActiveConnection, error) {
		activatedAP = ap
		return &mockActiveConnection{}, nil
	}

	_, err := b.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("ListNetworks(ScanNever) returned error: %v", err)
	}
	if err := b.ActivateNetwork("Corp"); err != nil {
		t.Fatalf("ActivateNetwork(Corp) returned error: %v", err)
	}
	if activatedAP != pskAP {
		t.Fatalf("ActivateNetwork used AP %#v, want PSK AP %#v", activatedAP, pskAP)
	}
}

func TestActivateNetwork_UsesStrongestCompatibleAccessPoint(t *testing.T) {
	weakAP := newMockAccessPoint("Corp", "00:00:00:00:00:17", 20)
	weakAP.rsnFlags = uint32(gonetworkmanager.Nm80211APSecKeyMgmtPSK)
	strongAP := newMockAccessPoint("Corp", "00:00:00:00:00:18", 90)
	strongAP.wpaFlags = uint32(gonetworkmanager.Nm80211APSecKeyMgmtPSK)
	strongAP.rsnFlags = 0
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{weakAP, strongAP},
	}
	knownPSK := newMockConnection("/org/freedesktop/NetworkManager/Settings/6", "Corp PSK", "Corp", wifi.SecurityWPA)
	b := newTestBackend(device, []gonetworkmanager.Connection{knownPSK})

	mockManager := b.NM.(*mockNM)
	var activatedAP gonetworkmanager.AccessPoint
	mockManager.activateWirelessConnectionFunc = func(conn gonetworkmanager.Connection, device gonetworkmanager.Device, ap gonetworkmanager.AccessPoint) (gonetworkmanager.ActiveConnection, error) {
		activatedAP = ap
		return &mockActiveConnection{}, nil
	}

	_, err := b.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("ListNetworks(ScanNever) returned error: %v", err)
	}
	if err := b.ActivateNetwork("Corp"); err != nil {
		t.Fatalf("ActivateNetwork(Corp) returned error: %v", err)
	}
	if activatedAP != strongAP {
		t.Fatalf("ActivateNetwork used AP %#v, want strongest compatible AP %#v", activatedAP, strongAP)
	}
}

func TestActivateNetwork_UsesSavedProfileWithoutCachedAccessPoint(t *testing.T) {
	device := &mockDeviceWireless{}
	knownHidden := newMockConnection("/org/freedesktop/NetworkManager/Settings/7", "HiddenNet", "HiddenNet", wifi.SecurityWPA)
	knownHidden.settings["802-11-wireless"]["hidden"] = true
	b := newTestBackend(device, []gonetworkmanager.Connection{knownHidden})

	mockManager := b.NM.(*mockNM)
	var activatedConn gonetworkmanager.Connection
	var activatedDevice gonetworkmanager.Device
	var activatedObject *dbus.Object
	mockManager.activateConnectionFunc = func(conn gonetworkmanager.Connection, device gonetworkmanager.Device, specificObject *dbus.Object) (gonetworkmanager.ActiveConnection, error) {
		activatedConn = conn
		activatedDevice = device
		activatedObject = specificObject
		return &mockActiveConnection{}, nil
	}
	mockManager.activateWirelessConnectionFunc = func(conn gonetworkmanager.Connection, device gonetworkmanager.Device, ap gonetworkmanager.AccessPoint) (gonetworkmanager.ActiveConnection, error) {
		t.Fatalf("ActivateNetwork used ActivateWirelessConnection with AP %#v, want generic ActivateConnection", ap)
		return nil, nil
	}

	_, err := b.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("ListNetworks(ScanNever) returned error: %v", err)
	}
	if err := b.ActivateNetwork("HiddenNet"); err != nil {
		t.Fatalf("ActivateNetwork(HiddenNet) returned error: %v", err)
	}
	if activatedConn != knownHidden {
		t.Fatalf("ActivateNetwork used connection %#v, want saved hidden profile %#v", activatedConn, knownHidden)
	}
	if activatedDevice != device {
		t.Fatalf("ActivateNetwork used device %#v, want wireless device %#v", activatedDevice, device)
	}
	if activatedObject != nil {
		t.Fatalf("ActivateNetwork used specific object %#v, want nil", activatedObject)
	}
}

func TestActivateNetwork_DoesNotPairSavedProfileWithIncompatibleAccessPoint(t *testing.T) {
	eapAP := newMockAccessPoint("Corp", "00:00:00:00:00:17", 90)
	eapAP.rsnFlags = uint32(gonetworkmanager.Nm80211APSecKeyMgmt8021X)
	device := &mockDeviceWireless{
		accessPoints: []gonetworkmanager.AccessPoint{eapAP},
	}
	knownPSK := newMockConnection("/org/freedesktop/NetworkManager/Settings/6", "Corp PSK", "Corp", wifi.SecurityWPA)
	b := newTestBackend(device, []gonetworkmanager.Connection{knownPSK})

	mockManager := b.NM.(*mockNM)
	var activatedAP gonetworkmanager.AccessPoint
	mockManager.activateWirelessConnectionFunc = func(conn gonetworkmanager.Connection, device gonetworkmanager.Device, ap gonetworkmanager.AccessPoint) (gonetworkmanager.ActiveConnection, error) {
		activatedAP = ap
		return &mockActiveConnection{}, nil
	}

	_, err := b.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("ListNetworks(ScanNever) returned error: %v", err)
	}
	err = b.ActivateNetwork("Corp")
	if !errors.Is(err, wifi.ErrNotFound) {
		t.Fatalf("ActivateNetwork(Corp) error = %v, want ErrNotFound", err)
	}
	if activatedAP != nil {
		t.Fatalf("ActivateNetwork used incompatible AP %#v", activatedAP)
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

func TestGetWirelessDevice_SkipsUnavailableDevices(t *testing.T) {
	unavailable := &mockDeviceWireless{managed: true, state: gonetworkmanager.NmDeviceStateUnavailable}
	unmanaged := &mockDeviceWireless{managed: false, state: gonetworkmanager.NmDeviceStateDisconnected}
	available := &mockDeviceWireless{managed: true, state: gonetworkmanager.NmDeviceStateDisconnected}

	nm := &mockNM{
		getDevicesFunc: func() ([]gonetworkmanager.Device, error) {
			return []gonetworkmanager.Device{unavailable, unmanaged, available}, nil
		},
	}

	b := &Backend{NM: nm}
	dev, err := b.getWirelessDevice()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dev != available {
		t.Errorf("expected available device %v, got %v", available, dev)
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
