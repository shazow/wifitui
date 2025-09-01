//go:build linux

// WARNING: This implementation is untested.
package iwd

import (
	"fmt"

	"github.com/godbus/dbus/v5"
	"github.com/shazow/wifitui/backend"
)

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
func New() (backend.Backend, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	// We don't defer conn.Close() here because the backend will use it.
	// Instead, we'll just use this connection to check for service availability.
	obj := conn.Object(iwdDest, iwdPath)
	if obj == nil {
		return nil, fmt.Errorf("failed to get dbus object for %s: %w", iwdDest, backend.ErrNotAvailable)
	}
	// A simple way to check for availability is to try to get a property.
	_, err = obj.GetProperty(iwdIface + ".Version")
	if err != nil {
		return nil, fmt.Errorf("iwd is not available: %w", backend.ErrNotAvailable)
	}

	return &Backend{}, nil
}

// BuildNetworkList scans (if shouldScan is true) and returns all networks.
func (b *Backend) BuildNetworkList(shouldScan bool) ([]backend.Connection, error) {
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

	var connections []backend.Connection
	visibleNetworks := make(map[string]backend.Connection)

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
			var security backend.SecurityType
			switch securityType {
			case "wpa-psk", "wpa2-psk", "wpa-eap", "wpa2-eap":
				security = backend.SecurityWPA
			case "wep":
				security = backend.SecurityWEP
			default:
				security = backend.SecurityOpen
			}

			connectedVar, _ := networkObj.GetProperty(iwdNetworkIface + ".Connected")
			isActive := connectedVar.Value().(bool)

			if existing, exists := visibleNetworks[ssid]; exists {
				if strength > existing.Strength {
					visibleNetworks[ssid] = backend.Connection{
						SSID:      ssid,
						IsActive:  isActive,
						IsSecure:  security != backend.SecurityOpen,
						Security:  security,
						IsVisible: true,
						Strength:  strength,
					}
				}
			} else {
				visibleNetworks[ssid] = backend.Connection{
					SSID:      ssid,
					IsActive:  isActive,
					IsSecure:  security != backend.SecurityOpen,
					Security:  security,
					IsVisible: true,
					Strength:  strength,
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

			if _, exists := visibleNetworks[ssid]; exists {
				c := visibleNetworks[ssid]
				c.IsKnown = true
				c.IsHidden = isHidden
				visibleNetworks[ssid] = c
			} else {
				// Add non-visible known network
				connections = append(connections, backend.Connection{SSID: ssid, IsKnown: true, IsHidden: isHidden})
			}
		}
	}

	for _, conn := range visibleNetworks {
		connections = append(connections, conn)
	}

	return connections, nil
}

func (b *Backend) ActivateConnection(ssid string) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	station, err := b.getStationDevice(conn)
	if err != nil {
		return err
	}
	return conn.Object(iwdDest, station).Call(iwdStationIface+".Connect", 0, ssid).Store()
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
		return fmt.Errorf("cannot forget: network %s is not known: %w", ssid, backend.ErrNotFound)
	}
	return conn.Object(iwdDest, iwdPath).Call(iwdIface+".ForgetNetwork", 0, path).Store()
}

func (b *Backend) JoinNetwork(ssid string, password string, security backend.SecurityType, isHidden bool) error {
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
		case backend.SecurityOpen:
			securityType = "open"
		case backend.SecurityWEP:
			securityType = "wep"
		default:
			securityType = "psk" // Default to WPA/WPA2 PSK
		}
		return conn.Object(iwdDest, station).Call(iwdStationIface+".ConnectHidden", 0, ssid, securityType, password).Store()
	}

	return conn.Object(iwdDest, station).Call(iwdStationIface+".Connect", 0, ssid, password).Store()
}

func (b *Backend) GetSecrets(ssid string) (string, error) {
	// The iwd API doesn't seem to expose a way to get the PSK directly for security reasons.
	// We can't implement this feature for iwd.
	return "", fmt.Errorf("getting secrets is not supported by the iwd backend: %w", backend.ErrNotSupported)
}

func (b *Backend) UpdateSecret(ssid string, newPassword string) error {
	// To "update" a secret, we have to forget the network and then re-join it.
	err := b.ForgetNetwork(ssid)
	if err != nil {
		return fmt.Errorf("failed to forget network before updating secret: %w", backend.ErrOperationFailed)
	}

	// We can't re-connect without a visible AP.
	// This is a limitation of this approach.
	return fmt.Errorf("updating secrets requires the network to be visible; try connecting to it again manually: %w", backend.ErrNotSupported)
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
	return "", fmt.Errorf("no station device found: %w", backend.ErrNotFound)
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
