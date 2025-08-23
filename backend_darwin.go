//go:build darwin
// WARNING: This implementation is untested.

package main

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// MacOSBackend implements the Backend interface for macOS.
type MacOSBackend struct {
	wifiInterface string
}

// NewMacOSBackend creates a new MacOSBackend.
func NewMacOSBackend() (Backend, error) {
	// Find the Wi-Fi interface name (e.g., en0)
	cmd := exec.Command("networksetup", "-listallhardwareports")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list hardware ports: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	var hardwarePort, device string
	for _, line := range lines {
		if strings.HasPrefix(line, "Hardware Port: ") {
			hardwarePort = strings.TrimPrefix(line, "Hardware Port: ")
		}
		if strings.HasPrefix(line, "Device: ") {
			device = strings.TrimPrefix(line, "Device: ")
		}
		if (strings.Contains(hardwarePort, "Wi-Fi") || strings.Contains(hardwarePort, "AirPort")) && device != "" {
			return &MacOSBackend{wifiInterface: device}, nil
		}
	}

	return nil, fmt.Errorf("no Wi-Fi interface found")
}

// NewBackend is the entry point for creating a backend on macOS.
func NewBackend() (Backend, error) {
	return NewMacOSBackend()
}

// BuildNetworkList scans (if shouldScan is true) and returns all networks.
func (b *MacOSBackend) BuildNetworkList(shouldScan bool) ([]Connection, error) {
	// Get current network
	cmd := exec.Command("networksetup", "-getairportnetwork", b.wifiInterface)
	out, err := cmd.Output()
	var currentSSID string
	if err == nil {
		re := regexp.MustCompile(`Current Wi-Fi Network: (.+)`)
		matches := re.FindStringSubmatch(string(out))
		if len(matches) > 1 {
			currentSSID = matches[1]
		}
	}

	// Get known networks
	cmd = exec.Command("networksetup", "-listpreferredwirelessnetworks", b.wifiInterface)
	out, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list preferred networks: %w", err)
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
	out, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to scan for networks: %w", err)
	}

	var conns []Connection
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

			conns = append(conns, Connection{
				SSID:      ssid,
				IsActive:  ssid == currentSSID,
				IsKnown:   knownSSIDs[ssid],
				IsVisible: true,
				Strength:  strength,
				IsSecure:  strings.Contains(line, "WPA") || strings.Contains(line, "WEP"),
			})
		}
	}

	// Add known networks that are not visible
	for ssid := range knownSSIDs {
		if _, processed := processedSSIDs[ssid]; !processed {
			conns = append(conns, Connection{
				SSID:    ssid,
				IsKnown: true,
			})
		}
	}

	sortConnections(conns)
	return conns, nil
}

// ActivateConnection activates a known network.
func (b *MacOSBackend) ActivateConnection(ssid string) error {
	password, err := b.GetSecrets(ssid)
	if err != nil {
		// This will fail for open networks, but that's ok
		password = ""
	}

	cmd := exec.Command("networksetup", "-setairportnetwork", b.wifiInterface, ssid, password)
	return cmd.Run()
}

// ForgetNetwork removes a known network configuration.
func (b *MacOSBackend) ForgetNetwork(ssid string) error {
	cmd := exec.Command("networksetup", "-removepreferredwirelessnetwork", b.wifiInterface, ssid)
	return cmd.Run()
}

// JoinNetwork connects to a new network, potentially creating a new configuration.
func (b *MacOSBackend) JoinNetwork(ssid string, password string) error {
	cmd := exec.Command("networksetup", "-setairportnetwork", b.wifiInterface, ssid, password)
	if err := cmd.Run(); err != nil {
		return err
	}
	// Add to preferred networks so it becomes "known"
	// The security type is not always WPA2E, but this is the best guess.
	cmd = exec.Command("networksetup", "-addpreferredwirelessnetworkatindex", b.wifiInterface, ssid, "0", "WPA2E")
	return cmd.Run()
}

// GetSecrets retrieves the password for a known connection.
func (b *MacOSBackend) GetSecrets(ssid string) (string, error) {
	cmd := exec.Command("security", "find-generic-password", "-wa", ssid)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// UpdateSecret changes the password for a known connection.
func (b *MacOSBackend) UpdateSecret(ssid string, newPassword string) error {
	// In macOS, we need to delete the old password and add a new one.
	// The -U flag in add-generic-password updates the item if it exists,
	// but it's safer to delete and add.
	cmd := exec.Command("security", "delete-generic-password", "-a", ssid, "-s", ssid)
	_ = cmd.Run() // Ignore error if it doesn't exist

	cmd = exec.Command("security", "add-generic-password", "-a", ssid, "-s", ssid, "-w", newPassword)
	return cmd.Run()
}
