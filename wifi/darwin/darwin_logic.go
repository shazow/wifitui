package darwin

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/shazow/wifitui/wifi"
)

type scannedNetwork struct {
	ssid     string
	security wifi.SecurityType
	rssi     int
	isActive bool
}

// parseSystemProfilerOutput parses the output of `system_profiler SPAirPortDataType`
// to extract visible Wi-Fi networks with their signal strength and security.
func parseSystemProfilerOutput(output string) []scannedNetwork {
	var networks []scannedNetwork
	processedSSIDs := make(map[string]bool)

	scanner := bufio.NewScanner(strings.NewReader(output))

	inCurrentNetwork := false
	inOtherNetworks := false
	var currentNetwork *scannedNetwork

	signalRe := regexp.MustCompile(`Signal / Noise:\s*(-?\d+)\s*dBm`)
	securityRe := regexp.MustCompile(`Security:\s*(.+)`)

	for scanner.Scan() {
		line := scanner.Text()

		// Detect section headers
		if strings.Contains(line, "Current Network Information:") {
			inCurrentNetwork = true
			inOtherNetworks = false
			continue
		}
		if strings.Contains(line, "Other Local Wi-Fi Networks:") {
			inCurrentNetwork = false
			inOtherNetworks = true
			continue
		}

		// Stop parsing if we hit another interface (like awdl0)
		if strings.HasPrefix(strings.TrimSpace(line), "awdl") {
			break
		}

		if !inCurrentNetwork && !inOtherNetworks {
			continue
		}

		trimmed := strings.TrimSpace(line)

		// Network name detection: lines that end with ":" and have specific indentation
		// In system_profiler output, network names are at a specific indent level
		leadingSpaces := len(line) - len(strings.TrimLeft(line, " "))

		// Network names are at 12-space indent (under Current/Other sections)
		if leadingSpaces == 12 && strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, ": ") {
			// Save previous network if exists
			if currentNetwork != nil && currentNetwork.ssid != "" {
				if !processedSSIDs[currentNetwork.ssid] {
					networks = append(networks, *currentNetwork)
					processedSSIDs[currentNetwork.ssid] = true
				} else if currentNetwork.rssi != 0 {
					// Update existing entry if this one has signal strength
					for i := range networks {
						if networks[i].ssid == currentNetwork.ssid && networks[i].rssi == 0 {
							networks[i].rssi = currentNetwork.rssi
							break
						}
					}
				}
			}

			ssid := strings.TrimSuffix(trimmed, ":")
			currentNetwork = &scannedNetwork{
				ssid:     ssid,
				isActive: inCurrentNetwork,
				rssi:     0,
				security: wifi.SecurityOpen,
			}
			continue
		}

		// Parse properties of current network
		if currentNetwork != nil {
			if matches := signalRe.FindStringSubmatch(line); len(matches) > 1 {
				rssi, _ := strconv.Atoi(matches[1])
				currentNetwork.rssi = rssi
			}
			if matches := securityRe.FindStringSubmatch(line); len(matches) > 1 {
				secStr := strings.TrimSpace(matches[1])
				currentNetwork.security = parseSecurityType(secStr)
			}
		}
	}

	// Don't forget the last network
	if currentNetwork != nil && currentNetwork.ssid != "" {
		if !processedSSIDs[currentNetwork.ssid] {
			networks = append(networks, *currentNetwork)
		} else if currentNetwork.rssi != 0 {
			for i := range networks {
				if networks[i].ssid == currentNetwork.ssid && networks[i].rssi == 0 {
					networks[i].rssi = currentNetwork.rssi
					break
				}
			}
		}
	}

	return networks
}

func parseSecurityType(s string) wifi.SecurityType {
	s = strings.ToLower(s)
	if strings.Contains(s, "wpa3") || strings.Contains(s, "wpa2") || strings.Contains(s, "wpa") {
		return wifi.SecurityWPA
	}
	if strings.Contains(s, "wep") {
		return wifi.SecurityWEP
	}
	return wifi.SecurityOpen
}

func rssiToStrength(rssi int) uint8 {
	if rssi >= 0 || rssi <= -100 {
		return 0
	}
	strength := uint8(2 * (rssi + 100))
	if strength > 100 {
		strength = 100
	}
	return strength
}

// findWifiDevice parses the output of `networksetup -listallhardwareports` to find the Wi-Fi device.
func findWifiDevice(output string) (string, error) {
	// The output is a series of stanzas, separated by blank lines.
	// Each stanza describes a hardware port.
	stanzas := strings.Split(output, "\n\n")
	for _, stanza := range stanzas {
		var hardwarePort, device string
		isWifiPort := false
		lines := strings.Split(stanza, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Hardware Port: ") {
				hardwarePort = strings.TrimPrefix(line, "Hardware Port: ")
				if strings.Contains(hardwarePort, "Wi-Fi") || strings.Contains(hardwarePort, "AirPort") {
					isWifiPort = true
				}
			}
			if strings.HasPrefix(line, "Device: ") {
				device = strings.TrimPrefix(line, "Device: ")
			}
		}
		if isWifiPort && device != "" {
			return device, nil
		}
	}
	return "", fmt.Errorf("no Wi-Fi interface found: %w", wifi.ErrNotFound)
}
