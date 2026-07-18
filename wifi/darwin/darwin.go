//go:build darwin

package darwin

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/shazow/wifitui/wifi"
)

// Backend implements the wifi.Backend interface for macOS.
type Backend struct {
	WifiInterface string
}

// New creates a new darwin.Backend.
func New() (wifi.Backend, error) {
	// Find the Wi-Fi interface name (e.g., en0)
	cmd := exec.Command("networksetup", "-listallhardwareports")
	out, err := runWithOutput(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to list hardware ports: %w: %w", wifi.ErrOperationFailed, err)
	}

	device, err := findWifiDevice(string(out))
	if err != nil {
		return nil, err
	}

	return &Backend{WifiInterface: device}, nil
}

// ListNetworks returns all networks.
func (b *Backend) ListNetworks(scan wifi.ScanMode) (wifi.NetworksResult, error) {
	return listNetworks(func(name string, args ...string) ([]byte, error) {
		return runWithOutput(exec.Command(name, args...))
	}, b.WifiInterface, scan)
}

// ActivateNetwork activates a known network.
func (b *Backend) ActivateNetwork(ssid string) error {
	// For known networks, networksetup uses stored credentials from the keychain
	// automatically - no need to fetch the password ourselves.
	cmd := exec.Command("networksetup", "-setairportnetwork", b.WifiInterface, ssid)
	return runOnly(cmd)
}

// ForgetNetwork removes a known network configuration.
func (b *Backend) ForgetNetwork(ssid string) error {
	cmd := exec.Command("networksetup", "-removepreferredwirelessnetwork", b.WifiInterface, ssid)
	return runOnly(cmd)
}

// IsWirelessEnabled checks if the wireless radio is enabled.
func (b *Backend) IsWirelessEnabled() (bool, error) {
	cmd := exec.Command("networksetup", "-getairportpower", b.WifiInterface)
	out, err := runWithOutput(cmd)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(out), ": On"), nil
}

// SetWireless enables or disables the wireless radio.
func (b *Backend) SetWireless(enabled bool) error {
	var state string
	if enabled {
		state = "on"
	} else {
		state = "off"
	}
	cmd := exec.Command("networksetup", "-setairportpower", b.WifiInterface, state)
	return runOnly(cmd)
}

// JoinNetwork connects to a new network, potentially creating a new configuration.
func (b *Backend) JoinNetwork(ssid string, password string, security wifi.SecurityType, isHidden bool) error {
	cmd := exec.Command("networksetup", "-setairportnetwork", b.WifiInterface, ssid, password)
	if err := runOnly(cmd); err != nil {
		return err
	}
	// Add to preferred networks so it becomes "known"
	var securityType string
	switch security {
	case wifi.SecurityOpen:
		securityType = "OPEN"
	case wifi.SecurityWEP:
		securityType = "WEP"
	default:
		securityType = "WPA2" // Default to WPA2 for WPA/WPA2
	}
	cmd = exec.Command("networksetup", "-addpreferredwirelessnetworkatindex", b.WifiInterface, ssid, "0", securityType)
	return runOnly(cmd)
}

// GetSecrets retrieves the password for a known connection.
func (b *Backend) GetSecrets(ssid string) (string, error) {
	cmd := exec.Command("security", "find-generic-password", "-wa", ssid)
	out, err := runWithOutput(cmd)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// UpdateNetwork updates a known network.
func (b *Backend) UpdateNetwork(ssid string, opts wifi.UpdateOptions) error {
	if opts.Password != nil {
		// In macOS, we need to delete the old password and add a new one.
		// The -U flag in add-generic-password updates the item if it exists,
		// but it's safer to delete and add.
		cmd := exec.Command("security", "delete-generic-password", "-a", ssid, "-s", ssid)
		_ = runOnly(cmd) // Ignore error if it doesn't exist

		cmd = exec.Command("security", "add-generic-password", "-a", ssid, "-s", ssid, "-w", *opts.Password)
		if err := runOnly(cmd); err != nil {
			return err
		}
	}

	if opts.AutoConnect != nil {
		if *opts.AutoConnect {
			// FIXME: Re-adding a network to preferred requires security type, which we don't have here.
			return fmt.Errorf("enabling autoconnect is not yet supported on darwin: %w", wifi.ErrNotSupported)
		}
		cmd := exec.Command("networksetup", "-removepreferredwirelessnetwork", b.WifiInterface, ssid)
		if err := runOnly(cmd); err != nil {
			return err
		}
	}

	return nil
}
