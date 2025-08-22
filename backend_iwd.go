package main

import (
	"fmt"
	"sort"

	"github.com/godbus/dbus/v5"
)

// IWD constants
const (
	iwdDest           = "net.connman.iwd"
	iwdPath           = "/"
	iwdIface          = "net.connman.iwd"
	iwdDeviceIface    = "net.connman.iwd.Device"
	iwdNetworkIface   = "net.connman.iwd.Network"
	iwdStationIface   = "net.connman.iwd.Station"
	iwdKnownNetworkIface = "net.connman.iwd.KnownNetwork"
)

// IwdBackend implements the Backend interface using iwd.
type IwdBackend struct {
	// connectionDetails stores D-Bus specific info needed for operations.
	// We won't use this for iwd for now.
}

// NewIwdBackend creates a new IwdBackend.
func NewIwdBackend() (Backend, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	// We don't defer conn.Close() here because the backend will use it.
	// Instead, we'll just use this connection to check for service availability.
	obj := conn.Object(iwdDest, iwdPath)
	if obj == nil {
		return nil, fmt.Errorf("failed to get dbus object for %s", iwdDest)
	}
	// A simple way to check for availability is to try to get a property.
	_, err = obj.GetProperty(iwdIface + ".Version")
	if err != nil {
		return nil, fmt.Errorf("iwd is not available: %w", err)
	}

	return &IwdBackend{}, nil
}

// BuildNetworkList scans (if shouldScan is true) and returns all networks.
func (b *IwdBackend) BuildNetworkList(shouldScan bool) ([]Connection, error) {
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

	var connections []Connection
	visibleNetworks := make(map[string]Connection)

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
			isSecure := typeVar.Value().(string) != "open"

			connectedVar, _ := networkObj.GetProperty(iwdNetworkIface + ".Connected")
			isActive := connectedVar.Value().(bool)

			if existing, exists := visibleNetworks[ssid]; exists {
				if strength > existing.Strength {
					visibleNetworks[ssid] = Connection{
						SSID:      ssid,
						IsActive:  isActive,
						IsSecure:  isSecure,
						IsVisible: true,
						Strength:  strength,
					}
				}
			} else {
				visibleNetworks[ssid] = Connection{
					SSID:      ssid,
					IsActive:  isActive,
					IsSecure:  isSecure,
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
				connections = append(connections, Connection{SSID: ssid, IsKnown: true, IsHidden: isHidden})
			}
		}
	}

	for _, conn := range visibleNetworks {
		connections = append(connections, conn)
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

func (b *IwdBackend) ActivateConnection(c Connection) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	station, err := b.getStationDevice(conn)
	if err != nil {
		return err
	}
	return conn.Object(iwdDest, station).Call(iwdStationIface+".Connect", 0, c.SSID).Store()
}

func (b *IwdBackend) ForgetNetwork(c Connection) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	path, err := b.findKnownNetworkPath(conn, c.SSID)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("cannot forget: network %s is not known", c.SSID)
	}
	return conn.Object(iwdDest, iwdPath).Call(iwdIface+".ForgetNetwork", 0, path).Store()
}

func (b *IwdBackend) JoinNetwork(c Connection, password string) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	station, err := b.getStationDevice(conn)
	if err != nil {
		return err
	}
	return conn.Object(iwdDest, station).Call(iwdStationIface+".Connect", 0, c.SSID, password).Store()
}

func (b *IwdBackend) GetSecrets(c Connection) (string, error) {
	// The iwd API doesn't seem to expose a way to get the PSK directly for security reasons.
	// We can't implement this feature for iwd.
	return "", fmt.Errorf("getting secrets is not supported by the iwd backend")
}

func (b *IwdBackend) UpdateSecret(c Connection, newPassword string) error {
	// To "update" a secret, we have to forget the network and then re-join it.
	err := b.ForgetNetwork(c)
	if err != nil {
		return fmt.Errorf("failed to forget network before updating secret: %w", err)
	}

	// We can't re-connect without a visible AP.
	// This is a limitation of this approach.
	return fmt.Errorf("updating secrets requires the network to be visible; try connecting to it again manually")
}

// --- iwd Helper Functions ---

func (b *IwdBackend) getDevices(conn *dbus.Conn) ([]dbus.ObjectPath, error) {
	var devices []dbus.ObjectPath
	obj := conn.Object(iwdDest, iwdPath)
	err := obj.Call(iwdIface+".GetDevices", 0).Store(&devices)
	return devices, err
}

func (b *IwdBackend) getStationDevice(conn *dbus.Conn) (dbus.ObjectPath, error) {
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
	return "", fmt.Errorf("no station device found")
}

func (b *IwdBackend) getKnownNetworks(conn *dbus.Conn) ([]dbus.ObjectPath, error) {
	var networks []dbus.ObjectPath
	obj := conn.Object(iwdDest, iwdPath)
	err := obj.Call(iwdIface+".GetKnownNetworks", 0).Store(&networks)
	return networks, err
}

func (b *IwdBackend) findKnownNetworkPath(conn *dbus.Conn, ssid string) (dbus.ObjectPath, error) {
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
