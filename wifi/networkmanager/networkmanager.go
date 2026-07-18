//go:build linux

package networkmanager

import (
	"context"
	"errors"
	"fmt"
	"os/user"
	"strings"
	"sync"
	"time"

	gonetworkmanager "github.com/Wifx/gonetworkmanager/v3"
	"github.com/godbus/dbus/v5"
	"github.com/google/uuid"
	"github.com/shazow/wifitui/wifi"
)

const (
	connectionTimeout = 10 * time.Second
	pollingInterval   = time.Second
	scanTimeout       = 30 * time.Second
	scanInterval      = 30 * time.Second
	diagnosticTimeout = 2 * time.Second
	nmScanPermission  = "org.freedesktop.NetworkManager.wifi.scan"
)

const (
	dbusPropertiesInterface        = "org.freedesktop.DBus.Properties"
	nmWirelessDeviceInterface      = "org.freedesktop.NetworkManager.Device.Wireless"
	nmWirelessAccessPointInterface = "org.freedesktop.NetworkManager.AccessPoint"
)

// Backend implements the backend.Backend interface using D-Bus to communicate with NetworkManager.
type Backend struct {
	NM       gonetworkmanager.NetworkManager
	Settings gonetworkmanager.Settings
	Device   gonetworkmanager.DeviceWireless

	connections       map[networkKey]gonetworkmanager.Connection
	accessPoints      map[networkKey]gonetworkmanager.AccessPoint
	networkKeysBySSID map[string][]networkKey

	scanFunc func(gonetworkmanager.DeviceWireless, map[string]dbus.Variant) error
	// scanPermissionFunc is overridden by tests to avoid a real D-Bus call.
	scanPermissionFunc func() (string, error)
	// testHookScanWait is overridden by tests to observe scan coordination.
	testHookScanWait func()
	scanInterval     time.Duration
	lastScanAttempt  time.Time
	scanMu           sync.Mutex
	scanInFlight     *scanOperation
}

type scanOperation struct {
	key  string
	done chan struct{}
	err  error
}

type networkKey struct {
	ssid     string
	security wifi.SecurityType
	mode     uint32
	flags    uint32
	wpaFlags uint32
	rsnFlags uint32
}

type savedProfile struct {
	connection    gonetworkmanager.Connection
	path          dbus.ObjectPath
	id            string
	ssid          string
	security      wifi.SecurityType
	mode          gonetworkmanager.Nm80211Mode
	keyMgmt       gonetworkmanager.Nm80211APSec
	lastConnected *time.Time
	autoConnect   bool
	hidden        bool
}

// New creates a new dbus.Backend.
func New() (wifi.Backend, error) {
	nm, err := gonetworkmanager.NewNetworkManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create network manager client: %w", wifi.ErrNotAvailable)
	}

	// gonetworkmanager's constructor only builds a DBus proxy and can succeed even when
	// NetworkManager is not actually running. Force a lightweight property fetch so we can
	// fall back to the iwd backend on iwd-only systems.
	if _, err := nm.GetPropertyVersion(); err != nil {
		if isUnavailableDBusError(err) {
			return nil, fmt.Errorf("networkmanager dbus service unavailable: %w: %w", wifi.ErrNotAvailable, err)
		}
		return nil, fmt.Errorf("failed to query network manager version: %w: %w", wifi.ErrOperationFailed, err)
	}

	settings, err := gonetworkmanager.NewSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", wifi.ErrOperationFailed)
	}

	return &Backend{
		NM:           nm,
		Settings:     settings,
		connections:  make(map[networkKey]gonetworkmanager.Connection),
		accessPoints: make(map[networkKey]gonetworkmanager.AccessPoint),
	}, nil
}

func isUnavailableDBusError(err error) bool {
	if err == nil {
		return false
	}
	if name := dbusErrorName(err); name != "" {
		return name == "org.freedesktop.DBus.Error.ServiceUnknown" ||
			name == "org.freedesktop.DBus.Error.NameHasNoOwner"
	}
	msg := err.Error()
	return strings.Contains(msg, "org.freedesktop.DBus.Error.ServiceUnknown") ||
		strings.Contains(msg, "org.freedesktop.DBus.Error.NameHasNoOwner") ||
		strings.Contains(msg, "The name is not activatable")
}

func dbusErrorName(err error) string {
	var value dbus.Error
	if errors.As(err, &value) {
		return value.Name
	}
	var pointer *dbus.Error
	if errors.As(err, &pointer) && pointer != nil {
		return pointer.Name
	}
	return ""
}

func (b *Backend) getWirelessDevice() (gonetworkmanager.DeviceWireless, error) {
	if b.Device != nil {
		return b.Device, nil
	}

	devices, err := b.NM.GetDevices()
	if err != nil {
		return nil, err
	}

	for _, device := range devices {
		if dev, ok := device.(gonetworkmanager.DeviceWireless); ok {
			// In hwsim and other multi-radio setups, NetworkManager can report AP-side
			// radios that are intentionally unmanaged or not yet usable for client
			// scans. Prefer a managed station radio instead of caching the first
			// wireless device and failing later with "Scanning not allowed while
			// unavailable".
			managed, err := dev.GetPropertyManaged()
			if err == nil && !managed {
				continue
			}

			state, err := dev.GetPropertyState()
			if err == nil && (state == gonetworkmanager.NmDeviceStateUnmanaged || state == gonetworkmanager.NmDeviceStateUnavailable) {
				continue
			}

			b.Device = dev
			return dev, nil
		}
	}

	return nil, fmt.Errorf("no wireless device found: %w", wifi.ErrNotFound)
}

func (b *Backend) scanAndWait(device gonetworkmanager.DeviceWireless) error {
	return b.scanAndWaitWithOptions(device, nil)
}

func (b *Backend) scanAndWaitWithOptions(device gonetworkmanager.DeviceWireless, options map[string]dbus.Variant) error {
	if b.scanFunc != nil {
		if err := b.scanFunc(device, options); err != nil {
			return b.scanRequestFailure(device, err)
		}
		return nil
	}

	var baseline time.Duration
	if value, err := device.GetPropertyLastScan(); err == nil && value > 0 {
		baseline = time.Duration(value) * time.Millisecond
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return newScanFailure(device, wifi.ScanStageSetup, fmt.Errorf("connect to system D-Bus: %w", err))
	}

	path := device.GetPath()
	rule := fmt.Sprintf("type='signal',interface='%s',member='PropertiesChanged',path='%s'", dbusPropertiesInterface, path)

	// We need to add the match rule to receiving signals matching this rule.
	call := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)
	if call.Err != nil {
		return newScanFailure(device, wifi.ScanStageSetup, fmt.Errorf("subscribe to scan completion: %w", call.Err))
	}
	defer conn.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0, rule)

	// Channel to receive signals
	c := make(chan *dbus.Signal, 1)
	conn.Signal(c)
	defer conn.RemoveSignal(c)

	// FIXME: Would be nice if we could detect whether a scan was already in progress (or if the device is ready to scan again),
	// otherwise it seems scan requests get dropped if they're requested too frequently.
	// Alternatively we can try to autodetect intervals that are too frequent by seeing how often scanTimeout is getting triggered, but this is not ideal.
	err = b.requestScan(device, options)
	if err != nil {
		return b.scanRequestFailure(device, err)
	}

	// Wait for the signal
	timer := time.NewTimer(scanTimeout)
	defer timer.Stop()

	for {
		select {
		case sig, ok := <-c:
			if !ok {
				cause := fmt.Errorf("%w: D-Bus signal channel closed", wifi.ErrScanProtocol)
				return newScanFailure(device, wifi.ScanStageCompletion, cause)
			}
			if sig == nil {
				cause := fmt.Errorf("%w: received an empty D-Bus signal", wifi.ErrScanProtocol)
				return newScanFailure(device, wifi.ScanStageCompletion, cause)
			}
			// Signal body for PropertiesChanged:
			// interface_name (string)
			// changed_properties (map[string]dbus.Variant)
			// invalidated_properties ([]string)
			if len(sig.Body) < 2 {
				continue
			}
			iface, ok := sig.Body[0].(string)
			if !ok || iface != nmWirelessDeviceInterface {
				continue
			}
			changed, ok := sig.Body[1].(map[string]dbus.Variant)
			if !ok {
				continue
			}
			variant, ok := changed["LastScan"]
			if !ok {
				continue
			}
			value, ok := variant.Value().(int64)
			if !ok {
				cause := fmt.Errorf("%w: LastScan has type %T", wifi.ErrScanProtocol, variant.Value())
				return newScanFailure(device, wifi.ScanStageCompletion, cause)
			}
			if value < 0 {
				continue
			}
			var nextScan time.Duration
			if value > 0 {
				nextScan = time.Duration(value) * time.Millisecond
			}
			if baseline == 0 || nextScan > baseline {
				return nil
			}
		case <-timer.C:
			cause := fmt.Errorf("%w after %s waiting for LastScan to change", wifi.ErrScanTimeout, scanTimeout)
			return newScanFailure(device, wifi.ScanStageCompletion, cause)
		}
	}
}

func newScanFailure(device gonetworkmanager.DeviceWireless, stage wifi.ScanStage, cause error) *wifi.ScanFailure {
	deviceInterface, _ := device.GetPropertyInterface()
	return &wifi.ScanFailure{
		Backend: "NetworkManager",
		Stage:   stage,
		Device:  deviceInterface,
		Code:    dbusErrorName(cause),
		Cause:   cause,
	}
}

func (b *Backend) scanRequestFailure(device gonetworkmanager.DeviceWireless, cause error) error {
	deviceInterface, _ := device.GetPropertyInterface()
	if isUnavailableDBusError(cause) {
		cause = fmt.Errorf("%w: %w", wifi.ErrNotAvailable, cause)
		return &wifi.ScanFailure{
			Backend: "NetworkManager",
			Stage:   wifi.ScanStageRequest,
			Device:  deviceInterface,
			Code:    dbusErrorName(cause),
			Cause:   cause,
		}
	}

	managed, managedErr := device.GetPropertyManaged()
	state, stateErr := device.GetPropertyState()

	if managedErr == nil && !managed {
		cause = fmt.Errorf("%w (%s is unmanaged): %w", wifi.ErrScanDeviceUnavailable, deviceInterface, cause)
	} else if stateErr == nil && (state == gonetworkmanager.NmDeviceStateUnmanaged || state == gonetworkmanager.NmDeviceStateUnavailable) {
		cause = fmt.Errorf("%w (%s is %s): %w", wifi.ErrScanDeviceUnavailable, deviceInterface, scanDeviceStateName(state), cause)
	} else if name := dbusErrorName(cause); name != "" {
		permission, permissionErr := b.getScanPermission()
		switch {
		case permissionErr == nil && permission == "no":
			cause = fmt.Errorf("%w: %w", wifi.ErrScanPermissionDenied, cause)
		case permissionErr == nil && permission == "auth":
			cause = fmt.Errorf("%w: %w", wifi.ErrScanAuthRequired, cause)
		case strings.HasSuffix(name, ".AccessDenied") || strings.HasSuffix(name, ".PermissionDenied"):
			cause = fmt.Errorf("%w: %w", wifi.ErrScanPermissionDenied, cause)
		}
	}

	return &wifi.ScanFailure{
		Backend: "NetworkManager",
		Stage:   wifi.ScanStageRequest,
		Device:  deviceInterface,
		Code:    dbusErrorName(cause),
		Cause:   cause,
	}
}

func scanDeviceStateName(state gonetworkmanager.NmDeviceState) string {
	switch state {
	case gonetworkmanager.NmDeviceStateUnmanaged:
		return "unmanaged"
	case gonetworkmanager.NmDeviceStateUnavailable:
		return "unavailable"
	default:
		return state.String()
	}
}

func (b *Backend) getScanPermission() (string, error) {
	if b.scanPermissionFunc != nil {
		return b.scanPermissionFunc()
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}
	var permissions map[string]string
	ctx, cancel := context.WithTimeout(context.Background(), diagnosticTimeout)
	defer cancel()
	err = conn.Object(gonetworkmanager.NetworkManagerInterface, gonetworkmanager.NetworkManagerObjectPath).
		CallWithContext(ctx, gonetworkmanager.NetworkManagerGetPermissions, 0).
		Store(&permissions)
	if err != nil {
		return "", err
	}
	return permissions[nmScanPermission], nil
}

func (b *Backend) requestScan(device gonetworkmanager.DeviceWireless, options map[string]dbus.Variant) error {
	if len(options) == 0 {
		return device.RequestScan()
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("failed to connect to dbus: %w", err)
	}
	obj := conn.Object(gonetworkmanager.NetworkManagerInterface, device.GetPath())
	return obj.Call(gonetworkmanager.DeviceWirelessRequestScan, 0, options).Store()
}

func hiddenSSIDScanOptions(ssid string) map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"ssids": dbus.MakeVariant([][]byte{[]byte(ssid)}),
	}
}

func (b *Backend) scanHiddenSSID(device gonetworkmanager.DeviceWireless, ssid string) error {
	if ssid == "" {
		return nil
	}
	return b.scanWithOptions(device, true, "ssid:"+ssid, hiddenSSIDScanOptions(ssid))
}

// WatchNetworkChanges returns a channel that receives a value when NetworkManager
// reports wireless device or access point changes.
func (b *Backend) WatchNetworkChanges(ctx context.Context) (<-chan struct{}, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	device, err := b.getWirelessDevice()
	if err != nil {
		return nil, err
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to dbus: %w", err)
	}

	devicePath := device.GetPath()
	rules := networkChangeMatchRules(devicePath)
	addedRules := make([]string, 0, len(rules))
	for _, rule := range rules {
		call := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)
		if call.Err != nil {
			for _, added := range addedRules {
				conn.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0, added)
			}
			return nil, fmt.Errorf("failed to add match rule: %w", call.Err)
		}
		addedRules = append(addedRules, rule)
	}

	signals := make(chan *dbus.Signal, 16)
	changes := make(chan struct{}, 1)
	conn.Signal(signals)

	go func() {
		defer close(changes)
		defer conn.RemoveSignal(signals)
		defer func() {
			for _, rule := range addedRules {
				conn.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0, rule)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case sig, ok := <-signals:
				if !ok {
					return
				}
				if !isNetworkChangeSignal(sig, devicePath) {
					continue
				}
				select {
				case changes <- struct{}{}:
				default:
				}
			}
		}
	}()

	return changes, nil
}

func networkChangeMatchRules(devicePath dbus.ObjectPath) []string {
	path := string(devicePath)
	return []string{
		fmt.Sprintf("type='signal',interface='%s',member='PropertiesChanged',path='%s',arg0='%s'", dbusPropertiesInterface, path, nmWirelessDeviceInterface),
		fmt.Sprintf("type='signal',interface='%s',member='AccessPointAdded',path='%s'", nmWirelessDeviceInterface, path),
		fmt.Sprintf("type='signal',interface='%s',member='AccessPointRemoved',path='%s'", nmWirelessDeviceInterface, path),
		fmt.Sprintf("type='signal',interface='%s',member='PropertiesChanged',arg0='%s'", dbusPropertiesInterface, nmWirelessAccessPointInterface),
	}
}

func isNetworkChangeSignal(sig *dbus.Signal, devicePath dbus.ObjectPath) bool {
	if sig == nil {
		return false
	}

	switch sig.Name {
	case "org.freedesktop.DBus.Properties.PropertiesChanged":
		if len(sig.Body) == 0 {
			return false
		}
		iface, ok := sig.Body[0].(string)
		if !ok {
			return false
		}
		if iface == nmWirelessDeviceInterface {
			return sig.Path == devicePath
		}
		return iface == nmWirelessAccessPointInterface
	case "org.freedesktop.NetworkManager.Device.Wireless.AccessPointAdded",
		"org.freedesktop.NetworkManager.Device.Wireless.AccessPointRemoved":
		return sig.Path == devicePath
	default:
		return false
	}
}

func (b *Backend) scanIfStale(device gonetworkmanager.DeviceWireless) error {
	return b.scan(device, false)
}

func (b *Backend) scanNow(device gonetworkmanager.DeviceWireless) error {
	return b.scan(device, true)
}

func (b *Backend) scan(device gonetworkmanager.DeviceWireless, force bool) error {
	return b.scanWithOptions(device, force, "", nil)
}

// scanWithOptions serializes scan requests so a LastScan update from an older
// request cannot satisfy a later request with different options. Requests with
// the same key share the in-flight result; requests with different keys wait
// and then establish their own LastScan baseline in scanAndWaitWithOptions.
func (b *Backend) scanWithOptions(device gonetworkmanager.DeviceWireless, force bool, key string, options map[string]dbus.Variant) error {
	interval := scanInterval
	if b.scanInterval > 0 {
		interval = b.scanInterval
	}

	for {
		b.scanMu.Lock()
		if current := b.scanInFlight; current != nil {
			sameRequest := current.key == key
			b.scanMu.Unlock()
			if b.testHookScanWait != nil {
				b.testHookScanWait()
			}
			<-current.done
			if sameRequest {
				return current.err
			}
			continue
		}

		// Only ordinary automatic scans use the retry interval. A skipped call
		// did not attempt or join the previous scan, so it must not inherit that
		// scan's error.
		if key == "" && !force && !b.lastScanAttempt.IsZero() && time.Since(b.lastScanAttempt) < interval {
			b.scanMu.Unlock()
			return nil
		}

		operation := &scanOperation{
			key:  key,
			done: make(chan struct{}),
		}
		b.scanInFlight = operation
		b.scanMu.Unlock()

		operation.err = b.scanAndWaitWithOptions(device, options)
		b.scanMu.Lock()
		if key == "" {
			b.lastScanAttempt = time.Now()
		}
		close(operation.done)
		b.scanInFlight = nil
		b.scanMu.Unlock()
		return operation.err
	}
}

func securityFromAccessPoint(flags, wpaFlags, rsnFlags uint32) (wifi.SecurityType, bool) {
	isSecure := (flags&uint32(gonetworkmanager.Nm80211APFlagsPrivacy) != 0) || (wpaFlags > 0) || (rsnFlags > 0)
	if wpaFlags > 0 || rsnFlags > 0 {
		return wifi.SecurityWPA, isSecure
	}
	if isSecure {
		return wifi.SecurityWEP, isSecure
	}
	return wifi.SecurityOpen, isSecure
}

func securityFromSettings(settings gonetworkmanager.ConnectionSettings) wifi.SecurityType {
	wireless, ok := settings["802-11-wireless"]
	if !ok {
		return wifi.SecurityUnknown
	}
	securitySetting, ok := wireless["security"].(string)
	if !ok || securitySetting == "" {
		return wifi.SecurityOpen
	}
	securityValues, ok := settings[securitySetting]
	if !ok {
		return wifi.SecurityUnknown
	}
	keyMgmt, _ := securityValues["key-mgmt"].(string)
	switch {
	case keyMgmt == "none":
		return wifi.SecurityWEP
	case strings.Contains(keyMgmt, "wpa"),
		strings.Contains(keyMgmt, "sae"),
		strings.Contains(keyMgmt, "802.1x"):
		return wifi.SecurityWPA
	default:
		return wifi.SecurityWPA
	}
}

func modeFromSettings(settings gonetworkmanager.ConnectionSettings) gonetworkmanager.Nm80211Mode {
	wireless, ok := settings["802-11-wireless"]
	if !ok {
		return gonetworkmanager.Nm80211ModeUnknown
	}

	mode, _ := wireless["mode"].(string)
	switch strings.ToLower(mode) {
	case "infrastructure", "infra":
		return gonetworkmanager.Nm80211ModeInfra
	case "adhoc", "ad-hoc":
		return gonetworkmanager.Nm80211ModeAdhoc
	case "ap":
		return gonetworkmanager.Nm80211ModeAp
	default:
		return gonetworkmanager.Nm80211ModeUnknown
	}
}

func keyManagementFromSettings(settings gonetworkmanager.ConnectionSettings) gonetworkmanager.Nm80211APSec {
	wireless, ok := settings["802-11-wireless"]
	if !ok {
		return gonetworkmanager.Nm80211APSecNone
	}
	securitySetting, ok := wireless["security"].(string)
	if !ok || securitySetting == "" {
		return gonetworkmanager.Nm80211APSecNone
	}
	securityValues, ok := settings[securitySetting]
	if !ok {
		return gonetworkmanager.Nm80211APSecNone
	}

	keyMgmt, _ := securityValues["key-mgmt"].(string)
	keyMgmt = strings.ToLower(keyMgmt)
	switch {
	case strings.HasPrefix(keyMgmt, "wpa-psk"):
		return gonetworkmanager.Nm80211APSecKeyMgmtPSK
	case keyMgmt == "sae":
		return gonetworkmanager.Nm80211APSecKeyMgmtSAE
	case strings.HasPrefix(keyMgmt, "wpa-eap"),
		keyMgmt == "802.1x",
		keyMgmt == "ieee8021x":
		return gonetworkmanager.Nm80211APSecKeyMgmt8021X
	case keyMgmt == "owe":
		return gonetworkmanager.Nm80211APSecKeyMgmtOWE
	default:
		return gonetworkmanager.Nm80211APSecNone
	}
}

func parseSavedProfile(conn gonetworkmanager.Connection) (savedProfile, bool) {
	settings, err := conn.GetSettings()
	if err != nil {
		return savedProfile{}, false
	}
	connectionSettings, ok := settings["connection"]
	if !ok {
		return savedProfile{}, false
	}
	connType, _ := connectionSettings["type"].(string)
	if connType != "802-11-wireless" {
		return savedProfile{}, false
	}
	wireless, ok := settings["802-11-wireless"]
	if !ok {
		return savedProfile{}, false
	}
	ssidBytes, ok := wireless["ssid"].([]byte)
	if !ok || len(ssidBytes) == 0 {
		return savedProfile{}, false
	}

	profile := savedProfile{
		connection:  conn,
		path:        conn.GetPath(),
		id:          "",
		ssid:        string(ssidBytes),
		security:    securityFromSettings(settings),
		mode:        modeFromSettings(settings),
		keyMgmt:     keyManagementFromSettings(settings),
		autoConnect: true,
	}
	if id, ok := connectionSettings["id"].(string); ok {
		profile.id = id
	}
	if ts, ok := connectionSettings["timestamp"].(uint64); ok && ts > 0 {
		t := time.Unix(int64(ts), 0)
		profile.lastConnected = &t
	}
	if autoConnect, ok := connectionSettings["autoconnect"].(bool); ok {
		profile.autoConnect = autoConnect
	}
	if hidden, ok := wireless["hidden"].(bool); ok {
		profile.hidden = hidden
	}
	return profile, true
}

func findCompatibleProfile(profiles []savedProfile, key networkKey) (savedProfile, bool) {
	for _, profile := range profiles {
		if profileMatchesNetwork(profile, key) {
			return profile, true
		}
	}
	return savedProfile{}, false
}

func profileMatchesNetwork(profile savedProfile, key networkKey) bool {
	if profile.ssid != key.ssid || profile.security != key.security {
		return false
	}
	if profile.mode != gonetworkmanager.Nm80211ModeUnknown && uint32(profile.mode) != key.mode {
		return false
	}
	if profile.security != wifi.SecurityWPA {
		return true
	}

	keyMgmt := uint32(profile.keyMgmt)
	if keyMgmt == 0 {
		return false
	}
	return keyMgmt&(key.wpaFlags|key.rsnFlags) != 0
}

func addNetworkKey(keys map[string][]networkKey, key networkKey) {
	for _, existing := range keys[key.ssid] {
		if existing == key {
			return
		}
	}
	keys[key.ssid] = append(keys[key.ssid], key)
}

// ListNetworks scans according to scan mode and returns all networks.
func (b *Backend) ListNetworks(scan wifi.ScanMode) (wifi.NetworksResult, error) {
	enabled, err := b.IsWirelessEnabled()
	if err != nil {
		return wifi.NetworksResult{}, err
	}
	if !enabled {
		return wifi.NetworksResult{}, wifi.ErrWirelessDisabled
	}
	newConnections := make(map[networkKey]gonetworkmanager.Connection)
	newAccessPoints := make(map[networkKey]gonetworkmanager.AccessPoint)
	newNetworkKeysBySSID := make(map[string][]networkKey)

	wirelessDevice, err := b.getWirelessDevice()
	if err != nil {
		return wifi.NetworksResult{}, err
	}

	var scanErr error
	switch scan {
	case wifi.ScanAuto:
		scanErr = b.scanIfStale(wirelessDevice)
	case wifi.ScanForce:
		scanErr = b.scanNow(wirelessDevice)
	}

	knownConnections, err := b.Settings.ListConnections()
	if err != nil {
		return wifi.NetworksResult{}, err
	}

	accessPoints, err := wirelessDevice.GetAllAccessPoints()
	if err != nil {
		accessPoints, err = wirelessDevice.GetAccessPoints()
	}
	if err != nil {
		return wifi.NetworksResult{}, err
	}

	var knownProfiles []savedProfile
	for _, knownConn := range knownConnections {
		profile, ok := parseSavedProfile(knownConn)
		if !ok {
			continue
		}
		knownProfiles = append(knownProfiles, profile)
	}

	activeConnections, err := b.NM.GetPropertyActiveConnections()
	if err != nil {
		return wifi.NetworksResult{}, err
	}

	var activeConnectionID string
	var activeConnectionPath dbus.ObjectPath
	for _, activeConn := range activeConnections {
		typ, err := activeConn.GetPropertyType()
		if err != nil {
			continue
		}
		if typ == "802-11-wireless" {
			if id, err := activeConn.GetPropertyID(); err == nil {
				activeConnectionID = id
			}
			if conn, err := activeConn.GetPropertyConnection(); err == nil {
				activeConnectionPath = conn.GetPath()
			}
			break
		}
	}

	applyProfile := func(conn *wifi.Network, profile savedProfile) {
		conn.IsKnown = true
		conn.LastConnected = profile.lastConnected
		conn.AutoConnect = profile.autoConnect
		if activeConnectionPath != "" {
			conn.IsActive = profile.path == activeConnectionPath
		} else if activeConnectionID != "" {
			conn.IsActive = profile.id == activeConnectionID
		}
	}

	uniqueConns := make(map[networkKey]wifi.Network)
	processedProfiles := make(map[dbus.ObjectPath]bool)
	for _, ap := range accessPoints {
		ssid, err := ap.GetPropertySSID()
		if err != nil || ssid == "" {
			continue
		}

		strength, _ := ap.GetPropertyStrength()
		hwAddress, _ := ap.GetPropertyHWAddress()
		frequency, _ := ap.GetPropertyFrequency()
		flags, _ := ap.GetPropertyFlags()
		wpaFlags, _ := ap.GetPropertyWPAFlags()
		rsnFlags, _ := ap.GetPropertyRSNFlags()
		mode, _ := ap.GetPropertyMode()
		security, isSecure := securityFromAccessPoint(flags, wpaFlags, rsnFlags)

		wifiAP := wifi.AccessPoint{
			SSID:      ssid,
			BSSID:     hwAddress,
			Strength:  strength,
			Frequency: uint(frequency),
		}

		key := networkKey{
			ssid:     ssid,
			security: security,
			mode:     uint32(mode),
			flags:    flags,
			wpaFlags: wpaFlags,
			rsnFlags: rsnFlags,
		}
		profile, known := findCompatibleProfile(knownProfiles, key)
		if known {
			newConnections[key] = profile.connection
			processedProfiles[profile.path] = true
		}
		addNetworkKey(newNetworkKeysBySSID, key)

		if existing, exists := newAccessPoints[key]; exists {
			exStrength, _ := existing.GetPropertyStrength()
			if strength > exStrength {
				newAccessPoints[key] = ap
			}
		} else {
			newAccessPoints[key] = ap
		}

		// Check if we already have this network processed.
		if conn, exists := uniqueConns[key]; exists {
			tempConn := wifi.Network{
				SSID:         ssid,
				Security:     security,
				IsSecure:     isSecure,
				IsVisible:    true,
				AccessPoints: []wifi.AccessPoint{wifiAP},
			}

			if known {
				applyProfile(&tempConn, profile)
			}

			// Now merge
			_ = conn.AddAccessPoint(tempConn)
			uniqueConns[key] = conn
			continue
		}

		connInfo := wifi.Network{
			SSID:         ssid,
			IsKnown:      false,
			IsSecure:     isSecure,
			IsVisible:    true,
			Security:     security,
			AutoConnect:  false, // Can't autoconnect to a network we don't know
			AccessPoints: []wifi.AccessPoint{wifiAP},
		}

		if known {
			applyProfile(&connInfo, profile)
		}
		uniqueConns[key] = connInfo
	}

	// Now build the final list from uniqueConns
	var conns []wifi.Network
	for _, c := range uniqueConns {
		conns = append(conns, c)
	}

	appendedInvisible := make(map[dbus.ObjectPath]bool)
	for _, profile := range knownProfiles {
		if processedProfiles[profile.path] || appendedInvisible[profile.path] {
			continue
		}
		key := networkKey{ssid: profile.ssid, security: profile.security, mode: uint32(profile.mode)}
		newConnections[key] = profile.connection
		addNetworkKey(newNetworkKeysBySSID, key)
		conns = append(conns, wifi.Network{
			SSID:          profile.ssid,
			IsKnown:       true,
			IsHidden:      profile.hidden,
			Security:      profile.security,
			LastConnected: profile.lastConnected,
			AutoConnect:   profile.autoConnect,
		})
		appendedInvisible[profile.path] = true
	}

	b.connections = newConnections
	b.accessPoints = newAccessPoints
	b.networkKeysBySSID = newNetworkKeysBySSID

	wifi.SortNetworks(conns)
	return wifi.NetworksResult{Networks: conns, ScanError: scanErr}, nil
}

func (b *Backend) getConnection(ssid string) (gonetworkmanager.Connection, error) {
	if b.connections == nil {
		b.connections = make(map[networkKey]gonetworkmanager.Connection)
	}

	if len(b.connections) == 0 {
		_, err := b.ListNetworks(wifi.ScanNever)
		if err != nil {
			return nil, err
		}
	}

	for _, key := range b.networkKeysBySSID[ssid] {
		if conn, ok := b.connections[key]; ok {
			return conn, nil
		}
	}
	return nil, fmt.Errorf("connection not found for %s: %w", ssid, wifi.ErrNotFound)
}

func (b *Backend) getAccessPoint(ssid string) (gonetworkmanager.AccessPoint, error) {
	if b.accessPoints == nil {
		b.accessPoints = make(map[networkKey]gonetworkmanager.AccessPoint)
	}

	if len(b.accessPoints) == 0 {
		_, err := b.ListNetworks(wifi.ScanNever)
		if err != nil {
			return nil, err
		}
	}

	var best gonetworkmanager.AccessPoint
	var bestStrength uint8
	for _, key := range b.networkKeysBySSID[ssid] {
		ap, ok := b.accessPoints[key]
		if !ok {
			continue
		}
		strength, _ := ap.GetPropertyStrength()
		if best == nil || strength > bestStrength {
			best = ap
			bestStrength = strength
		}
	}
	if best != nil {
		return best, nil
	}
	return nil, fmt.Errorf("access point not found for %s: %w", ssid, wifi.ErrNotFound)
}

func (b *Backend) getActivationTarget(ssid string) (gonetworkmanager.Connection, gonetworkmanager.AccessPoint, error) {
	if len(b.connections) == 0 || len(b.accessPoints) == 0 {
		_, err := b.ListNetworks(wifi.ScanNever)
		if err != nil {
			return nil, nil, err
		}
	}

	var bestConn gonetworkmanager.Connection
	var bestAP gonetworkmanager.AccessPoint
	var bestStrength uint8
	for _, key := range b.networkKeysBySSID[ssid] {
		conn, connOK := b.connections[key]
		ap, apOK := b.accessPoints[key]
		if connOK && apOK {
			strength, _ := ap.GetPropertyStrength()
			if bestAP == nil || strength > bestStrength {
				bestConn = conn
				bestAP = ap
				bestStrength = strength
			}
		}
	}
	if bestAP != nil {
		return bestConn, bestAP, nil
	}

	for _, key := range b.networkKeysBySSID[ssid] {
		if _, ok := b.accessPoints[key]; ok {
			return nil, nil, fmt.Errorf("connection not found for compatible access point for %s: %w", ssid, wifi.ErrNotFound)
		}
	}

	conn, err := b.getConnection(ssid)
	if err != nil {
		return nil, nil, err
	}
	return conn, nil, nil
}

func (b *Backend) ActivateNetwork(ssid string) error {
	conn, ap, err := b.getActivationTarget(ssid)
	if err != nil {
		return err
	}

	wirelessDevice, err := b.getWirelessDevice()
	if err != nil {
		return err
	}

	var activeConn gonetworkmanager.ActiveConnection
	if ap == nil {
		activeConn, err = b.NM.ActivateConnection(conn, wirelessDevice, nil)
	} else {
		activeConn, err = b.NM.ActivateWirelessConnection(conn, wirelessDevice, ap)
	}
	if err != nil {
		return err
	}

	return waitForActiveConnection(activeConn)
}

// waitForActiveConnection monitors NetworkManager's activation state until the
// connection activates, fails, or times out. It keeps the state-change
// subscription path from the previous inline loop, with a slow poll as a safety
// net for fast hwsim state transitions whose Activated signal can be missed.
func waitForActiveConnection(activeConn gonetworkmanager.ActiveConnection) error {
	stateChanges := make(chan gonetworkmanager.StateChange, 1)
	done := make(chan struct{})
	defer close(done)
	err := activeConn.SubscribeState(stateChanges, done)
	if err != nil {
		state, stateErr := activeConn.GetPropertyState()
		if stateErr == nil && state == gonetworkmanager.NmActiveConnectionStateDeactivated {
			return fmt.Errorf("connection failed before state subscription: %w", err)
		}
		if stateErr == nil && state == gonetworkmanager.NmActiveConnectionStateActivated {
			return nil
		}
		return err
	}

	initialState, err := activeConn.GetPropertyState()
	if err != nil {
		return err
	}
	switch initialState {
	case gonetworkmanager.NmActiveConnectionStateActivated:
		return nil
	case gonetworkmanager.NmActiveConnectionStateDeactivated:
		return fmt.Errorf("connection failed before activation wait")
	}

	// NetworkManager can occasionally miss or coalesce the Activated signal for
	// fast hwsim connections. Poll slowly as a safety net while primarily relying
	// on the state-change subscription for prompt completion and failure reasons.
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()
	timeout := time.NewTimer(connectionTimeout)
	defer timeout.Stop()

	for {
		select {
		case change := <-stateChanges:
			if change.State == gonetworkmanager.NmActiveConnectionStateActivated {
				return nil
			}
			if change.State == gonetworkmanager.NmActiveConnectionStateDeactivated {
				switch change.Reason {
				case gonetworkmanager.NmActiveConnectionStateReasonNoSecrets,
					gonetworkmanager.NmActiveConnectionStateReasonLoginFailed:
					return wifi.ErrIncorrectPassphrase
				default:
					return fmt.Errorf("connection failed: %s", change.Reason)
				}
			}
		case <-ticker.C:
			state, err := activeConn.GetPropertyState()
			if err != nil {
				continue
			}
			switch state {
			case gonetworkmanager.NmActiveConnectionStateActivated:
				return nil
			case gonetworkmanager.NmActiveConnectionStateDeactivated:
				return fmt.Errorf("connection failed while polling activation state")
			}
		case <-timeout.C:
			return fmt.Errorf("connection timed out")
		}
	}
}

func isUserInGroup(group string) (bool, error) {
	u, err := user.Current()
	if err != nil {
		return false, err
	}

	g, err := user.LookupGroup(group)
	if err != nil {
		return false, err
	}

	gids, err := u.GroupIds()
	if err != nil {
		return false, err
	}

	for _, gid := range gids {
		if gid == g.Gid {
			return true, nil
		}
	}

	return false, nil
}

func (b *Backend) ForgetNetwork(ssid string) error {
	conn, err := b.getConnection(ssid)
	if err != nil {
		return err
	}
	return conn.Delete()
}

func (b *Backend) JoinNetwork(ssid string, password string, security wifi.SecurityType, isHidden bool) error {
	wirelessDevice, err := b.getWirelessDevice()
	if err != nil {
		return err
	}
	deviceInterface, _ := wirelessDevice.GetPropertyInterface()
	var hiddenScanErr error
	if isHidden {
		hiddenScanErr = b.scanHiddenSSID(wirelessDevice, ssid)
	}

	connection := map[string]map[string]interface{}{
		"connection": {
			"id":             ssid,
			"uuid":           uuid.New().String(),
			"type":           "802-11-wireless",
			"interface-name": deviceInterface,
			"autoconnect":    true,
		},
		"802-11-wireless": {
			"mode": "infrastructure",
			"ssid": []byte(ssid),
		},
		"ipv4": {"method": "auto"},
		"ipv6": {"method": "auto"},
	}
	if isHidden {
		connection["802-11-wireless"]["hidden"] = true
	}

	switch security {
	case wifi.SecurityOpen:
		// No security settings needed
	case wifi.SecurityWEP:
		connection["802-11-wireless"]["security"] = "802-11-wireless-security"
		connection["802-11-wireless-security"] = map[string]interface{}{
			"key-mgmt": "none",
			"wep-key0": password,
		}
	default: // WPA/WPA2
		connection["802-11-wireless"]["security"] = "802-11-wireless-security"
		connection["802-11-wireless-security"] = map[string]interface{}{
			"key-mgmt": "wpa-psk",
			"psk":      password,
		}
	}

	conn, err := b.Settings.AddConnectionUnsaved(connection)
	if err != nil {
		return fmt.Errorf("failed to add unsaved connection: %w", err)
	}
	shouldDelete := true
	defer func() {
		if shouldDelete {
			_ = conn.Delete()
		}
	}()

	var activeConn gonetworkmanager.ActiveConnection
	if isHidden {
		// Use NetworkManager's generic ActivateConnection for hidden networks as there is no specific object.
		activeConn, err = b.NM.ActivateConnection(conn, wirelessDevice, nil)
	} else {
		ap, apErr := b.getAccessPoint(ssid)
		if apErr != nil {
			// It's possible for the access point to disappear between the scan and the join attempt.
			// In this case, we can try to join without the AP object.
			activeConn, err = b.NM.ActivateConnection(conn, wirelessDevice, nil)
		} else {
			activeConn, err = b.NM.ActivateWirelessConnection(conn, wirelessDevice, ap)
		}
	}
	if err != nil {
		if hiddenScanErr != nil {
			return fmt.Errorf("failed to activate hidden network after targeted scan failure: %w; scan failure: %w", err, hiddenScanErr)
		}
		return err
	}

	if err := waitForActiveConnection(activeConn); err != nil {
		if hiddenScanErr != nil {
			return fmt.Errorf("hidden network activation failed after targeted scan failure: %w; scan failure: %w", err, hiddenScanErr)
		}
		return err
	}

	err = conn.Save()
	if err != nil {
		return fmt.Errorf("failed to save connection: %w", err)
	}
	shouldDelete = false
	return nil
}

func (b *Backend) GetSecrets(ssid string) (string, error) {
	conn, err := b.getConnection(ssid)
	if err != nil {
		return "", err
	}

	s, err := conn.GetSettings()
	if err != nil {
		return "", fmt.Errorf("failed to get settings: %w", wifi.ErrOperationFailed)
	}

	if _, ok := s["802-11-wireless-security"]; !ok {
		return "", nil
	}

	settings, err := conn.GetSecrets("802-11-wireless-security")
	if err != nil {
		// Is the failure because we're not in the networkmanager group?
		if inGroup, errCheck := isUserInGroup("networkmanager"); errCheck == nil && !inGroup {
			return "", fmt.Errorf("need to be in the 'networkmanager' group to edit connections: %w: %w", wifi.ErrMissingPermission, err)
		}
		return "", fmt.Errorf("failed to get secrets: %w: %w", wifi.ErrOperationFailed, err)
	}

	if s, ok := settings["802-11-wireless-security"]; ok {
		if psk, ok := s["psk"]; ok {
			if p, ok := psk.(string); ok {
				return p, nil
			}
		}
	}

	return "", nil
}

// applyUpdateWorkaround modifies the settings map to workaround D-Bus type errors.
//
// NetworkManager's D-Bus API can return ipv6.addresses and ipv6.routes as an
// array of array of variants ('aav'), but expects them as an array of structs
// on update ('a(ayuay)' for addresses and 'a(ayuayu)' for routes). This causes
// a type mismatch error when calling the Update method with settings that
// were previously fetched from the API.
//
// To avoid this, we remove these properties from the settings map before
// updating the connection. This is safe because the operations that use this
// workaround are only intended to modify other properties of the connection.
//
// See: https://github.com/Wifx/gonetworkmanager/issues/13 and https://github.com/godbus/dbus/issues/400
func applyUpdateWorkaround(settings map[string]map[string]interface{}) {
	if ipv6Settings, ok := settings["ipv6"]; ok {
		delete(ipv6Settings, "addresses")
		delete(ipv6Settings, "routes")
	}
}

func (b *Backend) UpdateNetwork(ssid string, opts wifi.UpdateOptions) error {
	conn, err := b.getConnection(ssid)
	if err != nil {
		return err
	}

	settings, err := conn.GetSettings()
	if err != nil {
		return err
	}

	if opts.Password != nil {
		if _, ok := settings["802-11-wireless-security"]; !ok {
			settings["802-11-wireless-security"] = make(map[string]interface{})
		}
		settings["802-11-wireless-security"]["psk"] = *opts.Password
	}

	if opts.AutoConnect != nil {
		if _, ok := settings["connection"]; !ok {
			// This should not happen for a valid connection
			settings["connection"] = make(map[string]interface{})
		}
		settings["connection"]["autoconnect"] = *opts.AutoConnect
	}

	applyUpdateWorkaround(settings)
	return conn.Update(settings)
}

func (b *Backend) IsWirelessEnabled() (bool, error) {
	return b.NM.GetPropertyWirelessEnabled()
}

// SetWireless enables or disables the wireless radio. This function blocks until the radio is in the desired state.
func (b *Backend) SetWireless(enabled bool) error {
	// First, check if we're already in the desired state.
	if currentState, err := b.NM.GetPropertyWirelessEnabled(); err == nil && currentState == enabled {
		return nil
	}

	wirelessDevice, err := b.getWirelessDevice()
	if err != nil {
		return err
	}

	stateChanges := make(chan gonetworkmanager.DeviceStateChange, 1)
	exit := make(chan struct{})
	defer close(exit)

	if err := wirelessDevice.SubscribeState(stateChanges, exit); err != nil {
		return fmt.Errorf("failed to subscribe to state changes: %w", err)
	}

	// Now, change the state.
	if err := b.NM.SetPropertyWirelessEnabled(enabled); err != nil {
		return fmt.Errorf("failed to set wireless enabled property: %w", err)
	}

	var expectedState gonetworkmanager.NmDeviceState
	if enabled {
		// When enabling, the device becomes available and disconnected.
		expectedState = gonetworkmanager.NmDeviceStateDisconnected
	} else {
		// When disabling, the device becomes unavailable.
		expectedState = gonetworkmanager.NmDeviceStateUnavailable
	}

	// Check the current state of the device, in case the state changed before we started listening.
	if currentState, err := wirelessDevice.GetPropertyState(); err == nil {
		if enabled && currentState >= gonetworkmanager.NmDeviceStateDisconnected {
			return nil
		}
		if !enabled && currentState == expectedState {
			return nil
		}
	}

	for {
		select {
		case change := <-stateChanges:
			if enabled && change.State >= gonetworkmanager.NmDeviceStateDisconnected {
				return nil // Success!
			}
			if !enabled && change.State == expectedState {
				return nil // Success!
			}
		case <-time.After(connectionTimeout):
			s, _ := wirelessDevice.GetPropertyState()
			return fmt.Errorf("timed out waiting for wireless state change to %v, current state: %v", expectedState, s)
		}
	}
}
