package darwin

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/shazow/wifitui/wifi"
)

// runWithOutput wraps exec.Cmd to capture stderr and preserve its execution error.
func runWithOutput(command *exec.Cmd) ([]byte, error) {
	var stderr strings.Builder
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		return out, fmt.Errorf("failed to run command: %s: %w: %s", command.String(), err, stderr.String())
	}
	return out, nil
}

// runOnly wraps exec.Cmd for commands where stdout is not used.
func runOnly(command *exec.Cmd) error {
	var stderr strings.Builder
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("failed to run command: %s: %w: %s", command.String(), err, stderr.String())
	}
	return nil
}

type scannedNetwork struct {
	ssid     string
	security wifi.SecurityType
	rssi     int
	isActive bool
}

type outputRunner func(name string, args ...string) ([]byte, error)

var currentNetworkRE = regexp.MustCompile(`Current Wi-Fi Network: (.+)`)

// listNetworks contains the command orchestration separately from the
// Darwin-specific exec implementation so its behavior can be tested on any OS.
func listNetworks(run outputRunner, device string, scan wifi.ScanMode) (wifi.NetworksResult, error) {
	out, err := run("networksetup", "-getairportpower", device)
	if err != nil {
		return wifi.NetworksResult{}, fmt.Errorf("failed to get wireless power: %w", err)
	}
	if !strings.Contains(string(out), ": On") {
		return wifi.NetworksResult{}, wifi.ErrWirelessDisabled
	}

	// The association query is best effort: preferred networks and profiler
	// results are still useful without it. If fallback is also needed, preserve
	// both failures in ScanError because the missing association degrades it.
	currentOut, currentErr := run("networksetup", "-getairportnetwork", device)
	currentSSID := ""
	if currentErr == nil {
		matches := currentNetworkRE.FindStringSubmatch(string(currentOut))
		if len(matches) > 1 {
			currentSSID = strings.TrimSpace(matches[1])
		}
	}

	preferredOut, err := run("networksetup", "-listpreferredwirelessnetworks", device)
	if err != nil {
		return wifi.NetworksResult{}, fmt.Errorf("failed to list preferred networks: %w: %w", wifi.ErrOperationFailed, err)
	}
	knownSSIDs := parsePreferredNetworks(string(preferredOut))

	if scan == wifi.ScanNever {
		return wifi.NetworksResult{Networks: fallbackNetworks(knownSSIDs, currentSSID)}, nil
	}

	// TODO: Use CoreWLAN for a true scan and retain its last successful snapshot.
	// system_profiler cannot distinguish ScanAuto from ScanForce and does not
	// guarantee that collecting its report initiates a new wireless scan.
	profilerOut, err := run("system_profiler", "SPAirPortDataType")
	if err != nil {
		cause := err
		if currentErr != nil {
			cause = fmt.Errorf("system report failed: %w; current network query also failed: %w", err, currentErr)
		}
		return wifi.NetworksResult{
			Networks: fallbackNetworks(knownSSIDs, currentSSID),
			IsCached: true,
			ScanError: &wifi.ScanFailure{
				Backend: "macOS",
				Stage:   wifi.ScanStageRequest,
				Device:  device,
				Cause:   cause,
			},
		}, nil
	}

	return wifi.NetworksResult{Networks: aggregateNetworks(
		parseSystemProfilerOutput(string(profilerOut)), knownSSIDs, currentSSID,
	)}, nil
}

func parsePreferredNetworks(output string) map[string]bool {
	knownSSIDs := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "Preferred") {
			knownSSIDs[line] = true
		}
	}
	return knownSSIDs
}

func fallbackNetworks(knownSSIDs map[string]bool, currentSSID string) []wifi.Network {
	networksBySSID := make(map[string]wifi.Network, len(knownSSIDs)+1)
	for ssid := range knownSSIDs {
		networksBySSID[ssid] = wifi.Network{
			SSID:        ssid,
			IsKnown:     true,
			AutoConnect: true,
		}
	}
	if currentSSID != "" {
		current := networksBySSID[currentSSID]
		current.SSID = currentSSID
		current.IsActive = true
		current.IsVisible = true
		networksBySSID[currentSSID] = current
	}
	return sortedNetworks(networksBySSID)
}

func aggregateNetworks(scanned []scannedNetwork, knownSSIDs map[string]bool, currentSSID string) []wifi.Network {
	networksBySSID := make(map[string]wifi.Network, len(scanned)+len(knownSSIDs)+1)
	for _, network := range scanned {
		known := knownSSIDs[network.ssid]
		active := network.isActive || network.ssid == currentSSID
		accessPoint := wifi.AccessPoint{Strength: rssiToStrength(network.rssi)}
		if existing, ok := networksBySSID[network.ssid]; ok {
			existing.AccessPoints = append(existing.AccessPoints, accessPoint)
			existing.IsActive = existing.IsActive || active
			networksBySSID[network.ssid] = existing
			continue
		}
		networksBySSID[network.ssid] = wifi.Network{
			SSID:         network.ssid,
			IsActive:     active,
			IsKnown:      known,
			IsVisible:    true,
			AccessPoints: []wifi.AccessPoint{accessPoint},
			IsSecure:     network.security != wifi.SecurityOpen,
			Security:     network.security,
			AutoConnect:  known,
		}
	}

	if currentSSID != "" {
		current := networksBySSID[currentSSID]
		current.SSID = currentSSID
		current.IsActive = true
		current.IsVisible = true
		current.IsKnown = knownSSIDs[currentSSID]
		current.AutoConnect = current.IsKnown
		networksBySSID[currentSSID] = current
	}

	for ssid := range knownSSIDs {
		if _, ok := networksBySSID[ssid]; !ok {
			networksBySSID[ssid] = wifi.Network{
				SSID:        ssid,
				IsKnown:     true,
				AutoConnect: true,
			}
		}
	}
	return sortedNetworks(networksBySSID)
}

func sortedNetworks(networksBySSID map[string]wifi.Network) []wifi.Network {
	networks := make([]wifi.Network, 0, len(networksBySSID))
	for _, network := range networksBySSID {
		networks = append(networks, network)
	}
	wifi.SortNetworks(networks)
	return networks
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
