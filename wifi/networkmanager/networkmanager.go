//go:build linux

package networkmanager

import (
	"fmt"
	"time"

	gonetworkmanager "github.com/Wifx/gonetworkmanager/v3"
	"github.com/google/uuid"
	"github.com/shazow/wifitui/wifi"
)

const connectionTimeout = 10 * time.Second

// Backend implements the backend.Backend interface using D-Bus to communicate with NetworkManager.
type Backend struct {
	NM           gonetworkmanager.NetworkManager
	Settings     gonetworkmanager.Settings
	Connections  map[string]gonetworkmanager.Connection
	AccessPoints map[string]gonetworkmanager.AccessPoint
}

// New creates a new dbus.Backend.
func New() (wifi.Backend, error) {
	nm, err := gonetworkmanager.NewNetworkManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create network manager client: %w", wifi.ErrNotAvailable)
	}

	settings, err := gonetworkmanager.NewSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", wifi.ErrOperationFailed)
	}

	return &Backend{
		NM:           nm,
		Settings:     settings,
		Connections:  make(map[string]gonetworkmanager.Connection),
		AccessPoints: make(map[string]gonetworkmanager.AccessPoint),
	}, nil
}

func (b *Backend) getWirelessDevice() (gonetworkmanager.DeviceWireless, error) {
	// TODO: Cache result in struct?
	devices, err := b.NM.GetDevices()
	if err != nil {
		return nil, err
	}

	for _, device := range devices {
		if dev, ok := device.(gonetworkmanager.DeviceWireless); ok {
			return dev, nil
		}
	}

	return nil, fmt.Errorf("no wireless device found: %w", wifi.ErrNotFound)
}

// BuildNetworkList scans (if shouldScan is true) and returns all networks.
func (b *Backend) BuildNetworkList(shouldScan bool) ([]wifi.Connection, error) {
	enabled, err := b.IsWirelessEnabled()
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, wifi.ErrWirelessDisabled
	}
	b.Connections = make(map[string]gonetworkmanager.Connection)
	b.AccessPoints = make(map[string]gonetworkmanager.AccessPoint)

	wirelessDevice, err := b.getWirelessDevice()
	if err != nil {
		return nil, err
	}

	if shouldScan {
		err = wirelessDevice.RequestScan()
		if err != nil {
			return nil, err
		}
	}

	knownConnections, err := b.Settings.ListConnections()
	if err != nil {
		return nil, err
	}

	accessPoints, err := wirelessDevice.GetAccessPoints()
	if err != nil {
		return nil, err
	}

	var conns []wifi.Connection
	processedSSIDs := make(map[string]bool)

	activeConnections, err := b.NM.GetPropertyActiveConnections()
	if err != nil {
		return nil, err
	}

	var activeConnectionID string
	for _, activeConn := range activeConnections {
		id, err := activeConn.GetPropertyID()
		if err != nil {
			continue
		}
		typ, err := activeConn.GetPropertyType()
		if err != nil {
			continue
		}
		if typ == "802-11-wireless" {
			activeConnectionID = id
			break
		}
	}

	for _, ap := range accessPoints {
		ssid, err := ap.GetPropertySSID()
		if err != nil || ssid == "" {
			continue
		}

		strength, _ := ap.GetPropertyStrength()
		if existing, exists := b.AccessPoints[ssid]; exists {
			exStrength, _ := existing.GetPropertyStrength()
			if strength <= exStrength {
				continue
			}
		}

		processedSSIDs[ssid] = true
		b.AccessPoints[ssid] = ap

		flags, _ := ap.GetPropertyFlags()
		wpaFlags, _ := ap.GetPropertyWPAFlags()
		rsnFlags, _ := ap.GetPropertyRSNFlags()
		isSecure := (uint32(flags)&uint32(gonetworkmanager.Nm80211APFlagsPrivacy) != 0) || (wpaFlags > 0) || (rsnFlags > 0)
		var security wifi.SecurityType
		if wpaFlags > 0 || rsnFlags > 0 {
			security = wifi.SecurityWPA
		} else if isSecure {
			security = wifi.SecurityWEP
		} else {
			security = wifi.SecurityOpen
		}

		var connInfo wifi.Connection
		var knownConn gonetworkmanager.Connection
		for _, kc := range knownConnections {
			s, err := kc.GetSettings()
			if err != nil {
				continue
			}
			if wireless, ok := s["802-11-wireless"]; ok {
				if ssidBytes, ok := wireless["ssid"].([]byte); ok {
					if string(ssidBytes) == ssid {
						knownConn = kc
						break
					}
				}
			}
		}

		if knownConn != nil {
			b.Connections[ssid] = knownConn
			s, _ := knownConn.GetSettings()
			var id string
			var lastConnected *time.Time
			if c, ok := s["connection"]; ok {
				if i, ok := c["id"].(string); ok {
					id = i
				}
				if ts, ok := c["timestamp"].(uint64); ok && ts > 0 {
					t := time.Unix(int64(ts), 0)
					lastConnected = &t
				}
			}
			autoConnect := true
			if c, ok := s["connection"]; ok {
				if ac, ok := c["autoconnect"].(bool); ok {
					autoConnect = ac
				}
			}
			connInfo = wifi.Connection{
				SSID:          ssid,
				IsActive:      activeConnectionID != "" && id == activeConnectionID,
				IsKnown:       true,
				IsSecure:      isSecure,
				IsVisible:     true,
				Strength:      strength,
				Security:      security,
				LastConnected: lastConnected,
				AutoConnect:   autoConnect,
			}
		} else {
			connInfo = wifi.Connection{
				SSID:        ssid,
				IsKnown:     false,
				IsSecure:    isSecure,
				IsVisible:   true,
				Strength:    strength,
				Security:    security,
				AutoConnect: false, // Can't autoconnect to a network we don't know
			}
		}
		conns = append(conns, connInfo)
	}

	for _, knownConn := range knownConnections {
		s, _ := knownConn.GetSettings()
		var connType string
		if ct, ok := s["connection"]["type"]; ok {
			if t, ok := ct.(string); ok {
				connType = t
			}
		}
		if connType != "802-11-wireless" {
			continue
		}
		var ssid string
		if wireless, ok := s["802-11-wireless"]; ok {
			if ssidBytes, ok := wireless["ssid"].([]byte); ok {
				ssid = string(ssidBytes)
			}
		}
		if ssid == "" {
			continue
		}

		if _, processed := processedSSIDs[ssid]; !processed {
			b.Connections[ssid] = knownConn
			var lastConnected *time.Time
			if c, ok := s["connection"]; ok {
				if ts, ok := c["timestamp"].(uint64); ok && ts > 0 {
					t := time.Unix(int64(ts), 0)
					lastConnected = &t
				}
			}
			conns = append(conns, wifi.Connection{SSID: ssid, IsKnown: true, LastConnected: lastConnected})
		}
	}

	wifi.SortConnections(conns)
	return conns, nil
}

func (b *Backend) ActivateConnection(ssid string) error {
	conn, ok := b.Connections[ssid]
	if !ok {
		return fmt.Errorf("connection not found for %s: %w", ssid, wifi.ErrNotFound)
	}

	ap, apOK := b.AccessPoints[ssid]
	if !apOK {
		return fmt.Errorf("access point not found for %s: %w", ssid, wifi.ErrNotFound)
	}

	wirelessDevice, err := b.getWirelessDevice()
	if err != nil {
		return err
	}

	activeConn, err := b.NM.ActivateWirelessConnection(conn, wirelessDevice, ap)
	if err != nil {
		return err
	}

	// Now, block until the connection is fully activated.
	stateChanges := make(chan gonetworkmanager.StateChange, 1)
	done := make(chan struct{})
	defer close(done)
	err = activeConn.SubscribeState(stateChanges, done)
	if err != nil {
		return err
	}

	// Check the initial state first
	initialState, err := activeConn.GetPropertyState()
	if err != nil {
		return err
	}
	if initialState == gonetworkmanager.NmActiveConnectionStateActivated {
		return nil
	}

	for {
		select {
		case change := <-stateChanges:
			if change.State == gonetworkmanager.NmActiveConnectionStateActivated {
				return nil
			}
			if change.State == gonetworkmanager.NmActiveConnectionStateDeactivated {
				return fmt.Errorf("connection failed")
			}
		case <-time.After(connectionTimeout):
			return fmt.Errorf("connection timed out")
		}
	}
}

func (b *Backend) ForgetNetwork(ssid string) error {
	conn, ok := b.Connections[ssid]
	if !ok {
		return fmt.Errorf("connection not found for %s: %w", ssid, wifi.ErrNotFound)
	}
	return conn.Delete()
}

func (b *Backend) JoinNetwork(ssid string, password string, security wifi.SecurityType, isHidden bool) error {
	wirelessDevice, err := b.getWirelessDevice()
	if err != nil {
		return err
	}
	deviceInterface, _ := wirelessDevice.GetPropertyInterface()

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

	var activeConn gonetworkmanager.ActiveConnection
	if isHidden {
		activeConn, err = b.NM.AddAndActivateConnection(connection, wirelessDevice)
	} else {
		ap, ok := b.AccessPoints[ssid]
		if !ok {
			return fmt.Errorf("access point not found for %s: %w", ssid, wifi.ErrNotFound)
		}
		activeConn, err = b.NM.AddAndActivateWirelessConnection(connection, wirelessDevice, ap)
	}
	if err != nil {
		return err
	}

	// Now, block until the connection is fully activated.
	stateChanges := make(chan gonetworkmanager.StateChange, 1)
	done := make(chan struct{})
	defer close(done)
	err = activeConn.SubscribeState(stateChanges, done)
	if err != nil {
		return err
	}

	// Check the initial state first
	initialState, err := activeConn.GetPropertyState()
	if err != nil {
		return err
	}
	if initialState == gonetworkmanager.NmActiveConnectionStateActivated {
		return nil
	}

	for {
		select {
		case change := <-stateChanges:
			if change.State == gonetworkmanager.NmActiveConnectionStateActivated {
				return nil
			}
			if change.State == gonetworkmanager.NmActiveConnectionStateDeactivated {
				return fmt.Errorf("connection failed")
			}
		case <-time.After(connectionTimeout):
			return fmt.Errorf("connection timed out")
		}
	}
}

func (b *Backend) GetSecrets(ssid string) (string, error) {
	conn, ok := b.Connections[ssid]
	if !ok {
		return "", fmt.Errorf("connection not found for %s: %w", ssid, wifi.ErrNotFound)
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
		return "", fmt.Errorf("failed to get secrets: %w", wifi.ErrOperationFailed)
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

func (b *Backend) UpdateSecret(ssid string, newPassword string) error {
	conn, ok := b.Connections[ssid]
	if !ok {
		return fmt.Errorf("connection not found for %s: %w", ssid, wifi.ErrNotFound)
	}

	settings, err := conn.GetSettings()
	if err != nil {
		return err
	}

	if _, ok := settings["802-11-wireless-security"]; !ok {
		settings["802-11-wireless-security"] = make(map[string]interface{})
	}
	settings["802-11-wireless-security"]["psk"] = newPassword

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

func (b *Backend) SetAutoConnect(ssid string, autoConnect bool) error {
	conn, ok := b.Connections[ssid]
	if !ok {
		return fmt.Errorf("connection not found for %s: %w", ssid, wifi.ErrNotFound)
	}

	settings, err := conn.GetSettings()
	if err != nil {
		return err
	}

	if _, ok := settings["connection"]; !ok {
		// This should not happen for a valid connection
		settings["connection"] = make(map[string]interface{})
	}
	settings["connection"]["autoconnect"] = autoConnect

	applyUpdateWorkaround(settings)
	return conn.Update(settings)
}
