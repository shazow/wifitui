//go:build linux

package networkmanager

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Wifx/gonetworkmanager"
	"github.com/google/uuid"
	"github.com/shazow/wifitui/wifi"
)

const connectionTimeout = 30 * time.Second

// Backend implements the backend.Backend interface using D-Bus to communicate with NetworkManager.
type Backend struct {
	NM           gonetworkmanager.NetworkManager
	Settings     gonetworkmanager.Settings
	Connections  map[string]gonetworkmanager.Connection
	AccessPoints map[string]gonetworkmanager.AccessPoint
	logger       *slog.Logger
}

// New creates a new dbus.Backend.
func New(logger *slog.Logger) (wifi.Backend, error) {
	logger = logger.With("backend", "networkmanager")
	logger.Debug("initializing networkmanager backend")

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
		logger:       logger,
	}, nil
}

// BuildNetworkList scans (if shouldScan is true) and returns all networks.
func (b *Backend) BuildNetworkList(shouldScan bool) ([]wifi.Connection, error) {
	b.Connections = make(map[string]gonetworkmanager.Connection)
	b.AccessPoints = make(map[string]gonetworkmanager.AccessPoint)

	b.logger.Debug("getting devices")
	devices, err := b.NM.GetDevices()
	if err != nil {
		return nil, err
	}

	var wirelessDevice gonetworkmanager.DeviceWireless
	for _, device := range devices {
		if dev, ok := device.(gonetworkmanager.DeviceWireless); ok {
			wirelessDevice = dev
			break
		}
	}
	if wirelessDevice == nil {
		return nil, fmt.Errorf("no wireless device found: %w", wifi.ErrNotFound)
	}

	if shouldScan {
		b.logger.Debug("requesting scan")
		err = wirelessDevice.RequestScan()
		if err != nil {
			return nil, err
		}
	}

	b.logger.Debug("listing known connections")
	knownConnections, err := b.Settings.ListConnections()
	if err != nil {
		return nil, err
	}

	b.logger.Debug("getting access points")
	accessPoints, err := wirelessDevice.GetAccessPoints()
	if err != nil {
		return nil, err
	}

	var conns []wifi.Connection
	processedSSIDs := make(map[string]bool)

	b.logger.Debug("getting active connections")
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
		if err != nil {
			b.logger.Warn("failed to get ap ssid", "error", err)
			continue
		}
		if ssid == "" {
			continue
		}

		strength, _ := ap.GetPropertyStrength()
		if existing, exists := b.AccessPoints[ssid]; exists {
			exStrength, _ := existing.GetPropertyStrength()
			if strength <= exStrength {
				b.logger.Debug("skipping ap with weaker signal", "ssid", ssid, "strength", strength, "existing", exStrength)
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
				b.logger.Warn("failed to get known connection settings", "error", err)
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
	logger := b.logger.With("ssid", ssid)
	logger.Info("activating connection")
	conn, ok := b.Connections[ssid]
	if !ok {
		return fmt.Errorf("connection not found for %s: %w", ssid, wifi.ErrNotFound)
	}

	ap, apOK := b.AccessPoints[ssid]
	if !apOK {
		return fmt.Errorf("access point not found for %s: %w", ssid, wifi.ErrNotFound)
	}

	logger.Debug("getting wireless device")
	devices, err := b.NM.GetDevices()
	if err != nil {
		return err
	}
	var wirelessDevice gonetworkmanager.Device
	for _, device := range devices {
		deviceType, err := device.GetPropertyDeviceType()
		if err != nil {
			continue
		}
		if deviceType == gonetworkmanager.NmDeviceTypeWifi {
			wirelessDevice = device
			break
		}
	}
	if wirelessDevice == nil {
		return fmt.Errorf("no wireless device found: %w", wifi.ErrNotFound)
	}

	logger.Debug("activating wireless connection")
	activeConn, err := b.NM.ActivateWirelessConnection(conn, wirelessDevice, ap)
	if err != nil {
		return err
	}

	logger.Debug("subscribing to state changes")
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
		logger.Debug("connection already activated")
		return nil
	}

	for {
		select {
		case change := <-stateChanges:
			logger.Debug("connection state changed", "state", change.State)
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
	b.logger.Info("forgetting network", "ssid", ssid)
	conn, ok := b.Connections[ssid]
	if !ok {
		return fmt.Errorf("connection not found for %s: %w", ssid, wifi.ErrNotFound)
	}
	return conn.Delete()
}

func (b *Backend) JoinNetwork(ssid string, password string, security wifi.SecurityType, isHidden bool) error {
	logger := b.logger.With("ssid", ssid, "hidden", isHidden, "security", security)
	logger.Info("joining network")
	devices, err := b.NM.GetDevices()
	if err != nil {
		return err
	}
	var wirelessDevice gonetworkmanager.DeviceWireless
	for _, device := range devices {
		if dev, ok := device.(gonetworkmanager.DeviceWireless); ok {
			wirelessDevice = dev
			break
		}
	}
	if wirelessDevice == nil {
		return fmt.Errorf("no wireless device found: %w", wifi.ErrNotFound)
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
		logger.Debug("joining hidden network")
		activeConn, err = b.NM.AddAndActivateConnection(connection, wirelessDevice)
	} else {
		logger.Debug("joining visible network")
		ap, ok := b.AccessPoints[ssid]
		if !ok {
			return fmt.Errorf("access point not found for %s: %w", ssid, wifi.ErrNotFound)
		}
		activeConn, err = b.NM.AddAndActivateWirelessConnection(connection, wirelessDevice, ap)
	}
	if err != nil {
		return err
	}

	logger.Debug("subscribing to state changes")
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
		logger.Debug("connection already activated")
		return nil
	}

	for {
		select {
		case change := <-stateChanges:
			logger.Debug("connection state changed", "state", change.State)
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
	b.logger.Info("updating secret", "ssid", ssid)
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

func (b *Backend) SetAutoConnect(ssid string, autoConnect bool) error {
	b.logger.Info("setting autoconnect", "ssid", ssid, "autoconnect", autoConnect)
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
