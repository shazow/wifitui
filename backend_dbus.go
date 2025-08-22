package main

import (
	"fmt"
	"sort"

	"github.com/godbus/dbus/v5"
	"github.com/google/uuid"
)

// D-Bus constants for NetworkManager
const (
	nmDest             = "org.freedesktop.NetworkManager"
	nmPath             = "/org/freedesktop/NetworkManager"
	nmIface            = "org.freedesktop.NetworkManager"
	nmSettingsPath     = "/org/freedesktop/NetworkManager/Settings"
	nmSettingsIface    = "org.freedesktop.NetworkManager.Settings"
	nmConnIface        = "org.freedesktop.NetworkManager.Settings.Connection"
	nmActiveConnIface  = "org.freedesktop.NetworkManager.Connection.Active"
	nmDeviceIface      = "org.freedesktop.NetworkManager.Device"
	nmWirelessIface    = "org.freedesktop.NetworkManager.Device.Wireless"
	nmAccessPointIface = "org.freedesktop.NetworkManager.AccessPoint"
)

// DBusBackend implements the Backend interface using D-Bus to communicate with NetworkManager.
type DBusBackend struct {
	// connectionDetails stores D-Bus specific info needed for operations.
	// It's populated by BuildNetworkList and used by other methods.
	// The key is the SSID of the network.
	connectionDetails map[string]dbusDetails
}

// dbusDetails holds the D-Bus specific information for a connection.
type dbusDetails struct {
	path     dbus.ObjectPath // Path to the connection settings
	apPath   dbus.ObjectPath // Path to the access point
	settings map[string]map[string]dbus.Variant
}

// internalKnownConnection is a temporary struct used during list building.
type internalKnownConnection struct {
	ssid     string
	path     dbus.ObjectPath
	settings map[string]map[string]dbus.Variant
}

// internalAccessPoint holds the information for a single visible Wi-Fi access point.
type internalAccessPoint struct {
	ssid     string
	path     dbus.ObjectPath
	strength uint8
	isSecure bool
}

// NewDBusBackend creates a new DBusBackend.
func NewDBusBackend() (Backend, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	// We don't defer conn.Close() here because the backend will use it.
	// Instead, we'll just use this connection to check for service availability.
	// A better implementation might pool or reuse the connection.
	obj := conn.Object(nmDest, nmPath)
	if obj == nil {
		return nil, fmt.Errorf("failed to get dbus object for %s", nmDest)
	}

	// This is a simple way to check if the service is available.
	// A more robust check might involve trying to call a method.
	var devices []dbus.ObjectPath
	err = obj.Call(nmIface+".GetDevices", 0).Store(&devices)
	if err != nil {
		return nil, fmt.Errorf("networkmanager is not available: %w", err)
	}

	return &DBusBackend{
		connectionDetails: make(map[string]dbusDetails),
	}, nil
}

// BuildNetworkList scans (if shouldScan is true) and returns all networks.
func (b *DBusBackend) BuildNetworkList(shouldScan bool) ([]Connection, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Clear previous details
	b.connectionDetails = make(map[string]dbusDetails)

	wirelessDevice, err := b.getWirelessDevice(conn)
	if err != nil {
		// No wireless device, can't scan or get APs.
		// Fallback to just listing known connections.
		return b.buildListOfKnownConnectionsOnly(conn)
	}

	if shouldScan {
		devObj := conn.Object(nmDest, wirelessDevice)
		err = devObj.Call(nmWirelessIface+".RequestScan", 0, map[string]dbus.Variant{}).Store()
		if err != nil {
			return nil, err
		}
	}

	knowns, err := b.getKnownConnections(conn)
	if err != nil {
		return nil, err
	}

	aps, err := b.getVisibleAccessPoints(conn, wirelessDevice)
	if err != nil {
		return nil, err
	}

	var connections []Connection
	processedSSIDs := make(map[string]bool)
	activeWifiPath := b.getActiveWifiConnectionPath(conn)

	// Process visible APs first
	for ssid, ap := range aps {
		processedSSIDs[ssid] = true
		var connInfo Connection
		if known, ok := knowns[ssid]; ok {
			isHidden := false
			if wirelessSettings, ok := known.settings["802-11-wireless"]; ok {
				if hidden, ok := wirelessSettings["hidden"]; ok {
					if hiddenValue, ok := hidden.Value().(bool); ok {
						isHidden = hiddenValue
					}
				}
			}
			connInfo = Connection{
				SSID:      ssid,
				IsActive:  (activeWifiPath != "" && known.path == activeWifiPath),
				IsKnown:   true,
				IsSecure:  ap.isSecure,
				IsVisible: true,
				IsHidden:  isHidden,
				Strength:  ap.strength,
			}
			b.connectionDetails[ssid] = dbusDetails{path: known.path, apPath: ap.path, settings: known.settings}
		} else {
			connInfo = Connection{
				SSID:      ssid,
				IsKnown:   false,
				IsSecure:  ap.isSecure,
				IsVisible: true,
				Strength:  ap.strength,
			}
			b.connectionDetails[ssid] = dbusDetails{apPath: ap.path}
		}
		connections = append(connections, connInfo)
	}

	// Add known connections that are not visible
	for ssid, known := range knowns {
		if _, processed := processedSSIDs[ssid]; !processed {
			isHidden := false
			if wirelessSettings, ok := known.settings["802-11-wireless"]; ok {
				if hidden, ok := wirelessSettings["hidden"]; ok {
					if hiddenValue, ok := hidden.Value().(bool); ok {
						isHidden = hiddenValue
					}
				}
			}
			connections = append(connections, Connection{SSID: ssid, IsKnown: true, IsHidden: isHidden})
			b.connectionDetails[ssid] = dbusDetails{path: known.path, settings: known.settings}
		}
	}

	// Sort by active, then visible, then by SSID
	sort.SliceStable(connections, func(i, j int) bool {
		if connections[i].IsActive != connections[j].IsActive {
			return connections[i].IsActive
		}
		if connections[i].IsVisible != connections[j].IsVisible {
			return connections[i].IsVisible
		}
		// If both are visible, sort by strength. Otherwise, sort by name.
		if connections[i].IsVisible {
			if connections[i].Strength != connections[j].Strength {
				return connections[i].Strength > connections[j].Strength
			}
		}
		return connections[i].SSID < connections[j].SSID
	})

	return connections, nil
}

func (b *DBusBackend) ActivateConnection(c Connection) error {
	details, ok := b.connectionDetails[c.SSID]
	if !ok || details.path == "" {
		return fmt.Errorf("connection details not found for %s", c.SSID)
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	nmObj := conn.Object(nmDest, nmPath)
	wirelessDevice, err := b.getWirelessDevice(conn)
	if err != nil {
		return err
	}

	apPath := details.apPath
	if apPath == "" {
		// If not visible, we might not have an AP path.
		// Try activating without a specific AP. This may or may not work.
		apPath = "/"
	}

	var activeConnectionPath dbus.ObjectPath
	err = nmObj.Call(
		nmIface+".ActivateConnection", 0,
		details.path,   // connection
		wirelessDevice, // device
		apPath,         // specific_object
	).Store(&activeConnectionPath)

	if err != nil {
		return fmt.Errorf("failed to activate connection: %w", err)
	}
	return nil
}

func (b *DBusBackend) ForgetNetwork(c Connection) error {
	details, ok := b.connectionDetails[c.SSID]
	if !ok || details.path == "" {
		return fmt.Errorf("connection details not found for %s", c.SSID)
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	obj := conn.Object(nmDest, details.path)
	call := obj.Call(nmConnIface+".Delete", 0)
	if call.Err != nil {
		return fmt.Errorf("failed to forget connection: %w", call.Err)
	}
	return nil
}

func (b *DBusBackend) JoinNetwork(c Connection, password string) error {
	details, ok := b.connectionDetails[c.SSID]
	if !ok || details.apPath == "" {
		return fmt.Errorf("access point details not found for %s", c.SSID)
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	wirelessDevice, err := b.getWirelessDevice(conn)
	if err != nil {
		return err
	}

	connection := map[string]map[string]dbus.Variant{
		"connection": {
			"id":   dbus.MakeVariant(c.SSID),
			"uuid": dbus.MakeVariant(uuid.New().String()),
			"type": dbus.MakeVariant("802-11-wireless"),
		},
		"802-11-wireless": {
			"mode": dbus.MakeVariant("infrastructure"),
			"ssid": dbus.MakeVariant([]byte(c.SSID)),
		},
		"ipv4": {"method": dbus.MakeVariant("auto")},
		"ipv6": {"method": dbus.MakeVariant("auto")},
	}

	if c.IsSecure {
		connection["802-11-wireless-security"] = map[string]dbus.Variant{
			"key-mgmt": dbus.MakeVariant("wpa-psk"),
			"psk":      dbus.MakeVariant(password),
		}
	}

	nmObj := conn.Object(nmDest, nmPath)
	var activeConnectionPath dbus.ObjectPath
	var newConnectionPath dbus.ObjectPath
	err = nmObj.Call(
		nmIface+".AddAndActivateConnection", 0,
		connection,
		wirelessDevice,
		details.apPath,
	).Store(&newConnectionPath, &activeConnectionPath)

	if err != nil {
		return fmt.Errorf("failed to add/activate connection: %w", err)
	}
	return nil
}

func (b *DBusBackend) GetSecrets(c Connection) (string, error) {
	details, ok := b.connectionDetails[c.SSID]
	if !ok || details.path == "" {
		return "", fmt.Errorf("connection details not found for %s", c.SSID)
	}
	if _, ok := details.settings["802-11-wireless-security"]; !ok {
		return "", nil // No security settings, so no secret
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}
	defer conn.Close()

	obj := conn.Object(nmDest, details.path)
	var secrets map[string]map[string]dbus.Variant
	err = obj.Call(nmConnIface+".GetSecrets", 0, "802-11-wireless-security").Store(&secrets)
	if err != nil {
		return "", fmt.Errorf("failed to get secrets (did you authenticate?): %w", err)
	}

	psk, ok := secrets["802-11-wireless-security"]["psk"]
	if !ok {
		return "", nil // No PSK found
	}
	return psk.Value().(string), nil
}

func (b *DBusBackend) UpdateSecret(c Connection, newPassword string) error {
	details, ok := b.connectionDetails[c.SSID]
	if !ok || details.path == "" {
		return fmt.Errorf("connection details not found for %s", c.SSID)
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	obj := conn.Object(nmDest, details.path)
	currentSettings, err := b.getSettings(obj)
	if err != nil {
		return err
	}

	if _, ok := currentSettings["802-11-wireless-security"]; !ok {
		currentSettings["802-11-wireless-security"] = make(map[string]dbus.Variant)
	}
	currentSettings["802-11-wireless-security"]["psk"] = dbus.MakeVariant(newPassword)

	err = obj.Call(nmConnIface+".Update", 0, currentSettings).Store()
	if err != nil {
		return fmt.Errorf("failed to update connection: %w", err)
	}
	return nil
}

// --- D-Bus Helper Functions ---

func (b *DBusBackend) getWirelessDevice(conn *dbus.Conn) (dbus.ObjectPath, error) {
	nmObj := conn.Object(nmDest, nmPath)
	var devices []dbus.ObjectPath
	err := nmObj.Call(nmIface+".GetDevices", 0).Store(&devices)
	if err != nil {
		return "", err
	}
	for _, devicePath := range devices {
		deviceObj := conn.Object(nmDest, devicePath)
		deviceTypeVar, err := deviceObj.GetProperty(nmDeviceIface + ".DeviceType")
		if err != nil {
			continue
		}
		if deviceType, ok := deviceTypeVar.Value().(uint32); ok && deviceType == 2 { // NM_DEVICE_TYPE_WIFI
			return devicePath, nil
		}
	}
	return "", fmt.Errorf("no wireless device found")
}

func (b *DBusBackend) getKnownConnections(conn *dbus.Conn) (map[string]internalKnownConnection, error) {
	knowns := make(map[string]internalKnownConnection)
	settingsObj := conn.Object(nmDest, nmSettingsPath)
	var connPaths []dbus.ObjectPath
	err := settingsObj.Call(nmSettingsIface+".ListConnections", 0).Store(&connPaths)
	if err != nil {
		return nil, err
	}
	for _, path := range connPaths {
		connObj := conn.Object(nmDest, path)
		settings, err := b.getSettings(connObj)
		if err != nil {
			continue
		}
		if connType, ok := settings["connection"]["type"]; ok && connType.Value() == "802-11-wireless" {
			if ssidBytes, ok := settings["802-11-wireless"]["ssid"].Value().([]byte); ok {
				ssid := string(ssidBytes)
				knowns[ssid] = internalKnownConnection{ssid: ssid, path: path, settings: settings}
			}
		}
	}
	return knowns, nil
}

func (b *DBusBackend) getVisibleAccessPoints(conn *dbus.Conn, wirelessDevice dbus.ObjectPath) (map[string]internalAccessPoint, error) {
	aps := make(map[string]internalAccessPoint)
	devObj := conn.Object(nmDest, wirelessDevice)
	var apPaths []dbus.ObjectPath
	err := devObj.Call(nmWirelessIface+".GetAllAccessPoints", 0).Store(&apPaths)
	if err != nil {
		return nil, err
	}
	for _, path := range apPaths {
		apObj := conn.Object(nmDest, path)
		ssidVar, err := apObj.GetProperty(nmAccessPointIface + ".Ssid")
		if err != nil || ssidVar.Value() == nil {
			continue
		}
		ssidBytes, ok := ssidVar.Value().([]byte)
		if !ok || len(ssidBytes) == 0 {
			continue
		}
		ssid := string(ssidBytes)

		strengthVar, _ := apObj.GetProperty(nmAccessPointIface + ".Strength")
		strength := strengthVar.Value().(byte)

		if existing, exists := aps[ssid]; exists && strength <= existing.strength {
			continue
		}

		flagsVar, _ := apObj.GetProperty(nmAccessPointIface + ".Flags")
		aps[ssid] = internalAccessPoint{
			ssid:     ssid,
			path:     path,
			strength: strength,
			isSecure: (flagsVar.Value().(uint32) & 0x1) != 0, // NM_802_11_AP_FLAGS_PRIVACY
		}
	}
	return aps, nil
}

func (b *DBusBackend) getActiveWifiConnectionPath(conn *dbus.Conn) dbus.ObjectPath {
	nmObj := conn.Object(nmDest, nmPath)
	var activeConnPaths []dbus.ObjectPath
	variant, err := nmObj.GetProperty(nmIface + ".ActiveConnections")
	if err != nil {
		return ""
	}
	activeConnPaths, _ = variant.Value().([]dbus.ObjectPath)

	for _, path := range activeConnPaths {
		activeConnObj := conn.Object(nmDest, path)
		connTypeVar, err := activeConnObj.GetProperty(nmActiveConnIface + ".Type")
		if err != nil {
			continue
		}
		if connType, ok := connTypeVar.Value().(string); ok && connType == "802-11-wireless" {
			settingsPathVar, err := activeConnObj.GetProperty(nmActiveConnIface + ".Connection")
			if err != nil {
				continue
			}
			if settingsPath, ok := settingsPathVar.Value().(dbus.ObjectPath); ok {
				return settingsPath
			}
		}
	}
	return ""
}

func (b *DBusBackend) buildListOfKnownConnectionsOnly(conn *dbus.Conn) ([]Connection, error) {
	knowns, err := b.getKnownConnections(conn)
	if err != nil {
		return nil, err
	}
	var connections []Connection
	for ssid, known := range knowns {
		isHidden := false
		if wirelessSettings, ok := known.settings["802-11-wireless"]; ok {
			if hidden, ok := wirelessSettings["hidden"]; ok {
				if hiddenValue, ok := hidden.Value().(bool); ok {
					isHidden = hiddenValue
				}
			}
		}
		connections = append(connections, Connection{SSID: ssid, IsKnown: true, IsHidden: isHidden})
		b.connectionDetails[ssid] = dbusDetails{path: known.path, settings: known.settings}
	}
	return connections, nil
}

func (b *DBusBackend) getSettings(obj dbus.BusObject) (map[string]map[string]dbus.Variant, error) {
	var settings map[string]map[string]dbus.Variant
	err := obj.Call(nmConnIface+".GetSettings", 0).Store(&settings)
	if err != nil {
		return nil, err
	}
	return settings, nil
}
