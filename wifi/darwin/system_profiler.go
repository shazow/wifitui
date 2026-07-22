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

// scanSystemProfilerNetworks is a compatibility fallback for macOS processes
// that cannot read SSIDs through CoreWLAN because Location Services has not
// authorized the calling binary. system_profiler is slower, but its Apple-
// managed process can still provide the same visible-network snapshot used by
// wifitui before the native CoreWLAN scanner was introduced.
func scanSystemProfilerNetworks(_ string) ([]scannedNetwork, error) {
	out, err := runWithOutput(exec.Command("system_profiler", "SPAirPortDataType"))
	if err != nil {
		return nil, fmt.Errorf("system_profiler Wi-Fi scan failed: %w", err)
	}
	return parseSystemProfilerOutput(string(out)), nil
}

// parseSystemProfilerOutput extracts current and nearby networks from the
// human-readable SPAirPortDataType report. It intentionally remains isolated
// to the permission fallback; CoreWLAN is still authoritative when available.
func parseSystemProfilerOutput(output string) []scannedNetwork {
	var networks []scannedNetwork
	scanner := bufio.NewScanner(strings.NewReader(output))

	inCurrentNetwork := false
	inOtherNetworks := false
	var currentNetwork *scannedNetwork

	signalRE := regexp.MustCompile(`Signal / Noise:\s*(-?\d+)\s*dBm`)
	securityRE := regexp.MustCompile(`Security:\s*(.+)`)

	store := func(network *scannedNetwork) {
		if network == nil || network.ssid == "" {
			return
		}
		networks = append(networks, *network)
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		leadingSpaces := len(line) - len(strings.TrimLeft(line, " "))

		// Interface records are eight spaces deep, while network records are
		// twelve. Match structure rather than interface/SSID text so names such
		// as "awdlCafe" remain valid networks.
		if (inCurrentNetwork || inOtherNetworks) && leadingSpaces == 8 && strings.HasSuffix(trimmed, ":") {
			store(currentNetwork)
			return networks
		}
		switch {
		case leadingSpaces == 10 && trimmed == "Current Network Information:":
			inCurrentNetwork = true
			inOtherNetworks = false
			continue
		case leadingSpaces == 10 && trimmed == "Other Local Wi-Fi Networks:":
			inCurrentNetwork = false
			inOtherNetworks = true
			continue
		}
		if !inCurrentNetwork && !inOtherNetworks {
			continue
		}

		if leadingSpaces == 12 && strings.HasSuffix(trimmed, ":") {
			store(currentNetwork)
			currentNetwork = &scannedNetwork{
				ssid:     strings.TrimSuffix(trimmed, ":"),
				security: wifi.SecurityUnknown,
				isActive: inCurrentNetwork,
			}
			continue
		}

		if currentNetwork != nil {
			if matches := signalRE.FindStringSubmatch(line); len(matches) > 1 {
				currentNetwork.rssi, _ = strconv.Atoi(matches[1])
			}
			if matches := securityRE.FindStringSubmatch(line); len(matches) > 1 {
				currentNetwork.security = parseSystemProfilerSecurity(matches[1])
			}
		}
	}
	store(currentNetwork)
	return networks
}

func parseSystemProfilerSecurity(value string) wifi.SecurityType {
	value = strings.ToLower(value)
	switch {
	case strings.Contains(value, "wpa"):
		return wifi.SecurityWPA
	case strings.Contains(value, "wep"):
		return wifi.SecurityWEP
	case strings.Contains(value, "open") || strings.TrimSpace(value) == "none":
		return wifi.SecurityOpen
	default:
		return wifi.SecurityUnknown
	}
}
