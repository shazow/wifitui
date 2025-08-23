package main

import (
	"fmt"

	"github.com/Wifx/gonetworkmanager"
	"github.com/google/uuid"
)

// DBusBackend implements the Backend interface using D-Bus to communicate with NetworkManager.
type DBusBackend struct {
	nm           gonetworkmanager.NetworkManager
	settings     gonetworkmanager.Settings
	connections  map[string]gonetworkmanager.Connection
	accessPoints map[string]gonetworkmanager.AccessPoint
}

// NewDBusBackend creates a new DBusBackend.
func NewDBusBackend() (Backend, error) {
	nm, err := gonetworkmanager.NewNetworkManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create network manager client: %w", err)
	}

	settings, err := gonetworkmanager.NewSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}

	return &DBusBackend{
		nm:           nm,
		settings:     settings,
		connections:  make(map[string]gonetworkmanager.Connection),
		accessPoints: make(map[string]gonetworkmanager.AccessPoint),
	}, nil
}

// BuildNetworkList scans (if shouldScan is true) and returns all networks.
func (b *DBusBackend) BuildNetworkList(shouldScan bool) ([]Connection, error) {
	b.connections = make(map[string]gonetworkmanager.Connection)
	b.accessPoints = make(map[string]gonetworkmanager.AccessPoint)

	devices, err := b.nm.GetDevices()
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
		return nil, fmt.Errorf("no wireless device found")
	}

	if shouldScan {
		err = wirelessDevice.RequestScan()
		if err != nil {
			return nil, err
		}
	}

	knownConnections, err := b.settings.ListConnections()
	if err != nil {
		return nil, err
	}

	accessPoints, err := wirelessDevice.GetAccessPoints()
	if err != nil {
		return nil, err
	}

	var conns []Connection
	processedSSIDs := make(map[string]bool)

	activeConnections, err := b.nm.GetPropertyActiveConnections()
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
		if existing, exists := b.accessPoints[ssid]; exists {
			exStrength, _ := existing.GetPropertyStrength()
			if strength <= exStrength {
				continue
			}
		}

		processedSSIDs[ssid] = true
		b.accessPoints[ssid] = ap

		flags, _ := ap.GetPropertyFlags()
		isSecure := uint32(flags)&uint32(gonetworkmanager.Nm80211APFlagsPrivacy) != 0

		var connInfo Connection
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
			b.connections[ssid] = knownConn
			s, _ := knownConn.GetSettings()
			var id string
			if c, ok := s["connection"]; ok {
				if i, ok := c["id"].(string); ok {
					id = i
				}
			}
			connInfo = Connection{
				SSID:      ssid,
				IsActive:  activeConnectionID != "" && id == activeConnectionID,
				IsKnown:   true,
				IsSecure:  isSecure,
				IsVisible: true,
				Strength:  strength,
			}
		} else {
			connInfo = Connection{
				SSID:      ssid,
				IsKnown:   false,
				IsSecure:  isSecure,
				IsVisible: true,
				Strength:  strength,
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
			b.connections[ssid] = knownConn
			conns = append(conns, Connection{SSID: ssid, IsKnown: true})
		}
	}

	sortConnections(conns)
	return conns, nil
}

func (b *DBusBackend) ActivateConnection(c Connection) error {
	conn, ok := b.connections[c.SSID]
	if !ok {
		return fmt.Errorf("connection not found for %s", c.SSID)
	}

	ap, apOK := b.accessPoints[c.SSID]
	if !apOK {
		return fmt.Errorf("access point not found for %s", c.SSID)
	}

	devices, err := b.nm.GetDevices()
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
		return fmt.Errorf("no wireless device found")
	}

	_, err = b.nm.ActivateWirelessConnection(conn, wirelessDevice, ap)
	return err
}

func (b *DBusBackend) ForgetNetwork(c Connection) error {
	conn, ok := b.connections[c.SSID]
	if !ok {
		return fmt.Errorf("connection not found for %s", c.SSID)
	}
	return conn.Delete()
}

func (b *DBusBackend) JoinNetwork(c Connection, password string) error {
	ap, ok := b.accessPoints[c.SSID]
	if !ok {
		return fmt.Errorf("access point not found for %s", c.SSID)
	}

	devices, err := b.nm.GetDevices()
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
		return fmt.Errorf("no wireless device found")
	}

	connection := make(map[string]map[string]interface{})
	connection["802-11-wireless"] = make(map[string]interface{})
	connection["802-11-wireless"]["security"] = "802-11-wireless-security"
	connection["802-11-wireless-security"] = make(map[string]interface{})
	connection["802-11-wireless-security"]["key-mgmt"] = "wpa-psk"
	connection["802-11-wireless-security"]["psk"] = password
	connection["connection"] = make(map[string]interface{})
	connection["connection"]["id"] = c.SSID
	connection["connection"]["uuid"] = uuid.New().String()
	connection["connection"]["type"] = "802-11-wireless"

	_, err = b.nm.AddAndActivateWirelessConnection(connection, wirelessDevice, ap)
	return err
}

func (b *DBusBackend) GetSecrets(c Connection) (string, error) {
	conn, ok := b.connections[c.SSID]
	if !ok {
		return "", fmt.Errorf("connection not found for %s", c.SSID)
	}

	settings, err := conn.GetSecrets("802-11-wireless-security")
	if err != nil {
		return "", err
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

func (b *DBusBackend) UpdateSecret(c Connection, newPassword string) error {
	conn, ok := b.connections[c.SSID]
	if !ok {
		return fmt.Errorf("connection not found for %s", c.SSID)
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
