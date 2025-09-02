//go:build darwin
// WARNING: This implementation is untested.

package darwin

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/shazow/wifitui/backend"
)

// runWithOutput wraps exec.Command to capture stderr and wrap errors.
func runWithOutput(c *exec.Cmd) ([]byte, error) {
	var stderr strings.Builder
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		return out, fmt.Errorf("failed to run command: %s: %w: %s", c.String(), err, stderr.String())
	}
	return out, nil
}

// runOnly wraps exec.Command for commands where we don't care about stdout.
func runOnly(c *exec.Cmd) error {
	var stderr strings.Builder
	c.Stderr = &stderr
	err := c.Run()
	if err != nil {
		return fmt.Errorf("failed to run command: %s: %w: %s", c.String(), err, stderr.String())
	}
	return nil
}

// Backend implements the backend.Backend interface for macOS.
type Backend struct {
	WifiInterface string
}


// New creates a new darwin.Backend.
func New() (backend.Backend, error) {
	// Find the Wi-Fi interface name (e.g., en0)
	cmd := exec.Command("networksetup", "-listallhardwareports")
	out, err := runWithOutput(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to list hardware ports: %w", backend.ErrOperationFailed)
	}

	device, err := findWifiDevice(string(out))
	if err != nil {
		return nil, err
	}
	return &Backend{WifiInterface: device}, nil
}

// BuildNetworkList scans (if shouldScan is true) and returns all networks.
func (b *Backend) BuildNetworkList(shouldScan bool) ([]backend.Connection, error) {
	// Get current network
	cmd := exec.Command("networksetup", "-getairportnetwork", b.WifiInterface)
	out, err := runWithOutput(cmd)
	var currentSSID string
	if err == nil {
		re := regexp.MustCompile(`Current Wi-Fi Network: (.+)`)
		matches := re.FindStringSubmatch(string(out))
		if len(matches) > 1 {
			currentSSID = matches[1]
		}
	}

	// Get known networks
	cmd = exec.Command("networksetup", "-listpreferredwirelessnetworks", b.WifiInterface)
	out, err = runWithOutput(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to list preferred networks: %w: %s", backend.ErrOperationFailed, err)
	}
	knownSSIDs := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "Preferred") {
			knownSSIDs[line] = true
		}
	}

	// Scan for visible networks
	cmd = exec.Command("/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport", "-s")
	out, err = runWithOutput(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to scan for networks: %w", backend.ErrOperationFailed)
	}

	var conns []backend.Connection
	processedSSIDs := make(map[string]bool)
	scanner = bufio.NewScanner(strings.NewReader(string(out)))
	// The regex is to parse the output of the airport command.
	// It should handle SSIDs with spaces.
	re := regexp.MustCompile(`(.{1,32})\s+([0-9a-f:]+)\s+(-?\d+)\s+.*`)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "                            SSID BSSID") {
			continue
		}
		matches := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) > 3 {
			ssid := strings.TrimSpace(matches[1])
			if _, processed := processedSSIDs[ssid]; processed {
				continue
			}
			processedSSIDs[ssid] = true

			rssi, _ := strconv.Atoi(matches[3])
			strength := uint8(2 * (rssi + 100))
			if rssi >= 0 {
				strength = 0
			} else if rssi <= -100 {
				strength = 0
			} else {
				strength = uint8(2 * (rssi + 100))
			}
			if strength > 100 {
				strength = 100
			}

			var security backend.SecurityType
			if strings.Contains(line, "WPA") || strings.Contains(line, "WPA2") {
				security = backend.SecurityWPA
			} else if strings.Contains(line, "WEP") {
				security = backend.SecurityWEP
			} else {
				security = backend.SecurityOpen
			}
			isKnown := knownSSIDs[ssid]
			conns = append(conns, backend.Connection{
				SSID:        ssid,
				IsActive:    ssid == currentSSID,
				IsKnown:     isKnown,
				IsVisible:   true,
				Strength:    strength,
				IsSecure:    security != backend.SecurityOpen,
				Security:    security,
				AutoConnect: isKnown,
			})
		}
	}

	// Add known networks that are not visible
	for ssid := range knownSSIDs {
		if _, processed := processedSSIDs[ssid]; !processed {
			conns = append(conns, backend.Connection{
				SSID:        ssid,
				IsKnown:     true,
				AutoConnect: true,
			})
		}
	}

	return conns, nil
}

// ActivateConnection activates a known network.
func (b *Backend) ActivateConnection(ssid string) error {
	password, err := b.GetSecrets(ssid)
	if err != nil {
		// This will fail for open networks, but that's ok
		password = ""
	}

	cmd := exec.Command("networksetup", "-setairportnetwork", b.WifiInterface, ssid, password)
	return runOnly(cmd)
}

// ForgetNetwork removes a known network configuration.
func (b *Backend) ForgetNetwork(ssid string) error {
	cmd := exec.Command("networksetup", "-removepreferredwirelessnetwork", b.WifiInterface, ssid)
	return runOnly(cmd)
}

// JoinNetwork connects to a new network, potentially creating a new configuration.
func (b *Backend) JoinNetwork(ssid string, password string, security backend.SecurityType, isHidden bool) error {
	cmd := exec.Command("networksetup", "-setairportnetwork", b.WifiInterface, ssid, password)
	if err := runOnly(cmd); err != nil {
		return err
	}
	// Add to preferred networks so it becomes "known"
	var securityType string
	switch security {
	case backend.SecurityOpen:
		securityType = "OPEN"
	case backend.SecurityWEP:
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

// UpdateSecret changes the password for a known connection.
func (b *Backend) UpdateSecret(ssid string, newPassword string) error {
	// In macOS, we need to delete the old password and add a new one.
	// The -U flag in add-generic-password updates the item if it exists,
	// but it's safer to delete and add.
	cmd := exec.Command("security", "delete-generic-password", "-a", ssid, "-s", ssid)
	_ = runOnly(cmd) // Ignore error if it doesn't exist

	cmd = exec.Command("security", "add-generic-password", "-a", ssid, "-s", ssid, "-w", newPassword)
	return runOnly(cmd)
}

// SetAutoConnect sets the autoconnect property for a known connection.
func (b *Backend) SetAutoConnect(ssid string, autoConnect bool) error {
	if autoConnect {
		// FIXME: Re-adding a network to preferred requires security type, which we don't have here.
		return fmt.Errorf("enabling autoconnect is not yet supported on darwin: %w", backend.ErrNotSupported)
	}
	cmd := exec.Command("networksetup", "-removepreferredwirelessnetwork", b.WifiInterface, ssid)
	return runOnly(cmd)
}
