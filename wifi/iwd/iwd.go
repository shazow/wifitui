//go:build linux

// WARNING: This implementation is untested.
package iwd

import (
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/shazow/wifitui/wifi"
)

const connectionTimeout = 30 * time.Second
const propertyChangeTimeout = 5 * time.Second

// IWD constants
const (
	iwdDest              = "net.connman.iwd"
	iwdPath              = "/"
	iwdIface             = "net.connman.iwd"
	iwdDeviceIface       = "net.connman.iwd.Device"
	iwdNetworkIface      = "net.connman.iwd.Network"
	iwdStationIface      = "net.connman.iwd.Station"
	iwdKnownNetworkIface = "net.connman.iwd.KnownNetwork"
)

// Backend implements the backend.Backend interface using iwd.
type Backend struct {
	// connectionDetails stores D-Bus specific info needed for operations.
	// We won't use this for iwd for now.
}

// New creates a new iwd.Backend.
func New() (wifi.Backend, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	// We don't defer conn.Close() here because the backend will use it.
	// Instead, we'll just use this connection to check for service availability.
	obj := conn.Object(iwdDest, iwdPath)
	if obj == nil {
		return nil, fmt.Errorf("failed to get dbus object for %s: %w", iwdDest, wifi.ErrNotAvailable)
	}
	// A simple way to check for availability is to try to get a property.
	_, err = obj.GetProperty(iwdIface + ".Version")
	if err != nil {
		return nil, fmt.Errorf("iwd is not available: %w", wifi.ErrNotAvailable)
	}

	return &Backend{}, nil
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

	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	if shouldScan {
		station, err := b.getStationDevice(conn)
		if err == nil {
			// Best effort scan
			_ = conn.Object(iwdDest, station).Call(iwdStationIface+".Scan", 0)
		}
	}

	devices, err := b.getDevices(conn)
	if err != nil {
		return nil, err
	}

	var connections []wifi.Connection
	visibleNetworks := make(map[string]wifi.Connection)

	for _, devicePath := range devices {
		deviceObj := conn.Object(iwdDest, devicePath)
		var networkPaths []dbus.ObjectPath
		err := deviceObj.Call(iwdStationIface+".GetOrderedNetworks", 0).Store(&networkPaths)
		if err != nil {
			continue
		}

		for _, networkPath := range networkPaths {
			networkObj := conn.Object(iwdDest, networkPath)
			nameVar, _ := networkObj.GetProperty(iwdNetworkIface + ".Name")
			ssid := nameVar.Value().(string)

			strengthVar, _ := networkObj.GetProperty(iwdNetworkIface + ".Strength")
			strength := strengthVar.Value().(byte)

			typeVar, _ := networkObj.GetProperty(iwdNetworkIface + ".Type")
			securityType := typeVar.Value().(string)
			var security wifi.SecurityType
			switch securityType {
			case "wpa-psk", "wpa2-psk", "wpa-eap", "wpa2-eap":
				security = wifi.SecurityWPA
			case "wep":
				security = wifi.SecurityWEP
			default:
				security = wifi.SecurityOpen
			}

			connectedVar, _ := networkObj.GetProperty(iwdNetworkIface + ".Connected")
			isActive := connectedVar.Value().(bool)

			if existing, exists := visibleNetworks[ssid]; exists {
				if strength > existing.Strength {
					visibleNetworks[ssid] = wifi.Connection{
						SSID:      ssid,
						IsActive:  isActive,
						IsSecure:  security != wifi.SecurityOpen,
						Security:  security,
						IsVisible: true,
						Strength:  strength,
					}
				}
			} else {
				visibleNetworks[ssid] = wifi.Connection{
					SSID:        ssid,
					IsActive:    isActive,
					IsSecure:    security != wifi.SecurityOpen,
					Security:    security,
					IsVisible:   true,
					Strength:    strength,
					AutoConnect: false, // Cannot autoconnect to unknown network
				}
			}
		}
	}

	// Get known networks
	knownPaths, err := b.getKnownNetworks(conn)
	if err != nil {
		// Don't fail if we can't get known networks
	} else {
		for _, path := range knownPaths {
			knownObj := conn.Object(iwdDest, path)
			nameVar, _ := knownObj.GetProperty(iwdKnownNetworkIface + ".Name")
			ssid := nameVar.Value().(string)
			hiddenVar, err := knownObj.GetProperty(iwdKnownNetworkIface + ".Hidden")
			isHidden := false
			if err == nil {
				if val, ok := hiddenVar.Value().(bool); ok {
					isHidden = val
				}
			}

			autoConnectVar, err := knownObj.GetProperty(iwdKnownNetworkIface + ".AutoConnect")
			autoConnect := false
			if err == nil {
				if val, ok := autoConnectVar.Value().(bool); ok {
					autoConnect = val
				}
			}

			if c, exists := visibleNetworks[ssid]; exists {
				c.IsKnown = true
				c.IsHidden = isHidden
				c.AutoConnect = autoConnect
				visibleNetworks[ssid] = c
			} else {
				// Add non-visible known network
				connections = append(connections, wifi.Connection{SSID: ssid, IsKnown: true, IsHidden: isHidden, AutoConnect: autoConnect})
			}
		}
	}

	for _, conn := range visibleNetworks {
		connections = append(connections, conn)
	}

	return connections, nil
}

func (b *Backend) ActivateConnection(ssid, bssid string) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	station, err := b.getStationDevice(conn)
	if err != nil {
		return err
	}
	err = conn.Object(iwdDest, station).Call(iwdStationIface+".Connect", 0, ssid).Store()
	if err != nil {
		return err
	}
	return b.waitForConnection(ssid)
}

func (b *Backend) ForgetNetwork(ssid string) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	path, err := b.findKnownNetworkPath(conn, ssid)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("cannot forget: network %s is not known: %w", ssid, wifi.ErrNotFound)
	}
	return conn.Object(iwdDest, iwdPath).Call(iwdIface+".ForgetNetwork", 0, path).Store()
}

func (b *Backend) JoinNetwork(ssid string, password string, security wifi.SecurityType, isHidden bool, bssid string) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	station, err := b.getStationDevice(conn)
	if err != nil {
		return err
	}

	if isHidden {
		var securityType string
		switch security {
		case wifi.SecurityOpen:
			securityType = "open"
		case wifi.SecurityWEP:
			securityType = "wep"
		default:
			securityType = "psk" // Default to WPA/WPA2 PSK
		}
		err = conn.Object(iwdDest, station).Call(iwdStationIface+".ConnectHidden", 0, ssid, securityType, password).Store()
	} else {
		err = conn.Object(iwdDest, station).Call(iwdStationIface+".Connect", 0, ssid, password).Store()
	}
	if err != nil {
		return err
	}
	return b.waitForConnection(ssid)
}

func (b *Backend) GetSecrets(ssid string) (string, error) {
	// The iwd API doesn't seem to expose a way to get the PSK directly for security reasons.
	// We can't implement this feature for iwd.
	return "", fmt.Errorf("getting secrets is not supported by the iwd backend: %w", wifi.ErrNotSupported)
}

func (b *Backend) UpdateConnection(ssid string, opts wifi.UpdateOptions) error {
	if opts.Password != nil {
		// IWD doesn't support updating secrets directly. To "update" a secret,
		// we would have to forget the network and then re-join it, but that
		// requires the network to be visible.
		return fmt.Errorf("updating secrets is not supported by the iwd backend: %w", wifi.ErrNotSupported)
	}

	if opts.AutoConnect != nil {
		conn, err := dbus.SystemBus()
		if err != nil {
			return err
		}
		path, err := b.findKnownNetworkPath(conn, ssid)
		if err != nil {
			return err
		}
		if path == "" {
			return fmt.Errorf("cannot set autoconnect: network %s is not known: %w", ssid, wifi.ErrNotFound)
		}

		obj := conn.Object(iwdDest, path)
		variant := dbus.MakeVariant(*opts.AutoConnect)
		return obj.Call("org.freedesktop.DBus.Properties.Set", 0, iwdKnownNetworkIface, "AutoConnect", variant).Err
	}

	return nil
}

func (b *Backend) IsWirelessEnabled() (bool, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return false, err
	}
	station, err := b.getStationDevice(conn)
	if err != nil {
		return false, err
	}
	obj := conn.Object(iwdDest, station)
	poweredVar, err := obj.GetProperty(iwdDeviceIface + ".Powered")
	if err != nil {
		return false, err
	}
	return poweredVar.Value().(bool), nil
}

func (b *Backend) SetWireless(enabled bool) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	station, err := b.getStationDevice(conn)
	if err != nil {
		return err
	}
	obj := conn.Object(iwdDest, station)
	variant := dbus.MakeVariant(enabled)
	err = obj.Call("org.freedesktop.DBus.Properties.Set", 0, iwdDeviceIface, "Powered", variant).Err
	if err != nil {
		return err
	}

	// Now, block until the property is updated.
	signals := make(chan *dbus.Signal, 10)
	matchPath := dbus.WithMatchObjectPath(station)
	matchInterface := dbus.WithMatchInterface("org.freedesktop.DBus.Properties")
	conn.Signal(signals)
	conn.AddMatchSignal(matchInterface, matchPath)
	defer conn.RemoveMatchSignal(matchInterface, matchPath)

	for {
		select {
		case signal := <-signals:
			if signal.Name == "org.freedesktop.DBus.Properties.PropertiesChanged" {
				if len(signal.Body) < 2 {
					continue
				}
				iface, ok := signal.Body[0].(string)
				if !ok || iface != iwdDeviceIface {
					continue
				}
				props, ok := signal.Body[1].(map[string]dbus.Variant)
				if !ok {
					continue
				}
				if val, ok := props["Powered"]; ok {
					if val.Value().(bool) == enabled {
						return nil
					}
				}
			}
		case <-time.After(propertyChangeTimeout):
			return fmt.Errorf("timed out waiting for wireless state change")
		}
	}
}

func (b *Backend) waitForConnection(ssid string) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	timeout := time.After(connectionTimeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("connection timed out")
		case <-ticker.C:
			devices, err := b.getDevices(conn)
			if err != nil {
				return err
			}

			for _, devicePath := range devices {
				deviceObj := conn.Object(iwdDest, devicePath)
				var networkPaths []dbus.ObjectPath
				err := deviceObj.Call(iwdStationIface+".GetOrderedNetworks", 0).Store(&networkPaths)
				if err != nil {
					continue
				}

				for _, networkPath := range networkPaths {
					networkObj := conn.Object(iwdDest, networkPath)
					nameVar, _ := networkObj.GetProperty(iwdNetworkIface + ".Name")
					if name, ok := nameVar.Value().(string); ok && name == ssid {
						connectedVar, _ := networkObj.GetProperty(iwdNetworkIface + ".Connected")
						if connected, ok := connectedVar.Value().(bool); ok && connected {
							return nil // Connected!
						}
					}
				}
			}
		}
	}
}

// --- iwd Helper Functions ---

func (b *Backend) getDevices(conn *dbus.Conn) ([]dbus.ObjectPath, error) {
	var devices []dbus.ObjectPath
	obj := conn.Object(iwdDest, iwdPath)
	err := obj.Call(iwdIface+".GetDevices", 0).Store(&devices)
	return devices, err
}

func (b *Backend) getStationDevice(conn *dbus.Conn) (dbus.ObjectPath, error) {
	devices, err := b.getDevices(conn)
	if err != nil {
		return "", err
	}
	for _, devicePath := range devices {
		deviceObj := conn.Object(iwdDest, devicePath)
		typeVar, err := deviceObj.GetProperty(iwdDeviceIface + ".Type")
		if err != nil {
			continue
		}
		if typeStr, ok := typeVar.Value().(string); ok && typeStr == "station" {
			return devicePath, nil
		}
	}
	return "", fmt.Errorf("no station device found: %w", wifi.ErrNotFound)
}

func (b *Backend) getKnownNetworks(conn *dbus.Conn) ([]dbus.ObjectPath, error) {
	var networks []dbus.ObjectPath
	obj := conn.Object(iwdDest, iwdPath)
	err := obj.Call(iwdIface+".GetKnownNetworks", 0).Store(&networks)
	return networks, err
}

func (b *Backend) findKnownNetworkPath(conn *dbus.Conn, ssid string) (dbus.ObjectPath, error) {
	paths, err := b.getKnownNetworks(conn)
	if err != nil {
		return "", err
	}
	for _, path := range paths {
		obj := conn.Object(iwdDest, path)
		nameVar, err := obj.GetProperty(iwdKnownNetworkIface + ".Name")
		if err != nil {
			continue
		}
		if name, ok := nameVar.Value().(string); ok && name == ssid {
			return path, nil
		}
	}
	return "", nil
}
