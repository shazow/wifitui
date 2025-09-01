//go:build linux

package networkmanager

import (
	"fmt"
	"time"

	"github.com/Wifx/gonetworkmanager"
	"github.com/google/uuid"
	"github.com/shazow/wifitui/backend"
)

// Backend implements the backend.Backend interface using D-Bus to communicate with NetworkManager.
type Backend struct {
	NM           gonetworkmanager.NetworkManager
	Settings     gonetworkmanager.Settings
	Connections  map[string]gonetworkmanager.Connection
	AccessPoints map[string]gonetworkmanager.AccessPoint
}

// New creates a new dbus.Backend.
func New() (backend.Backend, error) {
	nm, err := gonetworkmanager.NewNetworkManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create network manager client: %w", backend.ErrNotAvailable)
	}

	settings, err := gonetworkmanager.NewSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", backend.ErrOperationFailed)
	}

	return &Backend{
		NM:           nm,
		Settings:     settings,
		Connections:  make(map[string]gonetworkmanager.Connection),
		AccessPoints: make(map[string]gonetworkmanager.AccessPoint),
	}, nil
}

// BuildNetworkList scans (if shouldScan is true) and returns all networks.
func (b *Backend) BuildNetworkList(shouldScan bool) ([]backend.Connection, error) {
	b.Connections = make(map[string]gonetworkmanager.Connection)
	b.AccessPoints = make(map[string]gonetworkmanager.AccessPoint)

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
		return nil, fmt.Errorf("no wireless device found: %w", backend.ErrNotFound)
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

	var conns []backend.Connection
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
		var security backend.SecurityType
		if wpaFlags > 0 || rsnFlags > 0 {
			security = backend.SecurityWPA
		} else if isSecure {
			security = backend.SecurityWEP
		} else {
			security = backend.SecurityOpen
		}

		var connInfo backend.Connection
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
			connInfo = backend.Connection{
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
			connInfo = backend.Connection{
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
			conns = append(conns, backend.Connection{SSID: ssid, IsKnown: true, LastConnected: lastConnected})
		}
	}

	backend.SortConnections(conns)
	return conns, nil
}

func (b *Backend) ActivateConnection(ssid string) error {
	conn, ok := b.Connections[ssid]
	if !ok {
		return fmt.Errorf("connection not found for %s: %w", ssid, backend.ErrNotFound)
	}

	ap, apOK := b.AccessPoints[ssid]
	if !apOK {
		return fmt.Errorf("access point not found for %s: %w", ssid, backend.ErrNotFound)
	}

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
		return fmt.Errorf("no wireless device found: %w", backend.ErrNotFound)
	}

	_, err = b.NM.ActivateWirelessConnection(conn, wirelessDevice, ap)
	return err
}

func (b *Backend) ForgetNetwork(ssid string) error {
	conn, ok := b.Connections[ssid]
	if !ok {
		return fmt.Errorf("connection not found for %s: %w", ssid, backend.ErrNotFound)
	}
	return conn.Delete()
}

func (b *Backend) JoinNetwork(ssid string, password string, security backend.SecurityType, isHidden bool) error {
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
		return fmt.Errorf("no wireless device found: %w", backend.ErrNotFound)
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
	case backend.SecurityOpen:
		// No security settings needed
	case backend.SecurityWEP:
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

	if isHidden {
		_, err = b.NM.AddAndActivateConnection(connection, wirelessDevice)
	} else {
		ap, ok := b.AccessPoints[ssid]
		if !ok {
			return fmt.Errorf("access point not found for %s: %w", ssid, backend.ErrNotFound)
		}
		_, err = b.NM.AddAndActivateWirelessConnection(connection, wirelessDevice, ap)
	}
	return err
}

func (b *Backend) GetSecrets(ssid string) (string, error) {
	conn, ok := b.Connections[ssid]
	if !ok {
		return "", fmt.Errorf("connection not found for %s: %w", ssid, backend.ErrNotFound)
	}

	s, err := conn.GetSettings()
	if err != nil {
		return "", fmt.Errorf("failed to get settings: %w", backend.ErrOperationFailed)
	}

	if _, ok := s["802-11-wireless-security"]; !ok {
		return "", nil
	}

	settings, err := conn.GetSecrets("802-11-wireless-security")
	if err != nil {
		return "", fmt.Errorf("failed to get secrets: %w", backend.ErrOperationFailed)
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

func (b *Backend) UpdateSecret(ssid string, newPassword string) error {
	conn, ok := b.Connections[ssid]
	if !ok {
		return fmt.Errorf("connection not found for %s: %w", ssid, backend.ErrNotFound)
	}

	settings, err := conn.GetSettings()
	if err != nil {
		return err
	}

	if _, ok := settings["802-11-wireless-security"]; !ok {
		settings["802-11-wireless-security"] = make(map[string]interface{})
	}
	settings["802-11-wireless-security"]["psk"] = newPassword

	return conn.Update(settings)
}

func (b *Backend) SetAutoConnect(ssid string, autoConnect bool) error {
	conn, ok := b.Connections[ssid]
	if !ok {
		return fmt.Errorf("connection not found for %s: %w", ssid, backend.ErrNotFound)
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

	return conn.Update(settings)
}
