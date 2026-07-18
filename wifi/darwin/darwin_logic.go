package darwin

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"

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
	ssid      string
	bssid     string
	security  wifi.SecurityType
	rssi      int
	frequency uint
}

type coreWLANNetwork struct {
	SSID      string `json:"ssid"`
	BSSID     string `json:"bssid"`
	Security  string `json:"security"`
	RSSI      int    `json:"rssi"`
	Frequency uint   `json:"frequency"`
}

const (
	coreWLANStatusSuccess = iota
	coreWLANStatusDeviceUnavailable
	coreWLANStatusFailed
	coreWLANStatusProtocol
	coreWLANStatusPermissionDenied
	coreWLANStatusTimeout
	coreWLANStatusUnsupported
)

func coreWLANStatusError(status int, message string) error {
	classification := error(nil)
	switch status {
	case coreWLANStatusDeviceUnavailable:
		classification = wifi.ErrScanDeviceUnavailable
	case coreWLANStatusProtocol:
		classification = wifi.ErrScanProtocol
	case coreWLANStatusPermissionDenied:
		classification = wifi.ErrScanPermissionDenied
	case coreWLANStatusTimeout:
		classification = wifi.ErrScanTimeout
	case coreWLANStatusUnsupported:
		classification = wifi.ErrNotSupported
	}
	if classification == nil {
		return errors.New(message)
	}
	return fmt.Errorf("%s: %w", message, classification)
}

func decodeCoreWLANScan(output []byte) ([]scannedNetwork, error) {
	var decoded []coreWLANNetwork
	if err := json.Unmarshal(output, &decoded); err != nil {
		return nil, fmt.Errorf("%w: decode CoreWLAN results: %w", wifi.ErrScanProtocol, err)
	}

	networks := make([]scannedNetwork, 0, len(decoded))
	for _, network := range decoded {
		if network.SSID == "" {
			continue
		}
		security := wifi.SecurityUnknown
		switch network.Security {
		case "open":
			security = wifi.SecurityOpen
		case "wep":
			security = wifi.SecurityWEP
		case "wpa":
			security = wifi.SecurityWPA
		}
		networks = append(networks, scannedNetwork{
			ssid:      network.SSID,
			bssid:     network.BSSID,
			security:  security,
			rssi:      network.RSSI,
			frequency: network.Frequency,
		})
	}
	if len(decoded) > 0 && len(networks) == 0 {
		return nil, fmt.Errorf("%w: CoreWLAN returned networks without an SSID", wifi.ErrScanProtocol)
	}
	return networks, nil
}

type outputRunner func(name string, args ...string) ([]byte, error)
type networkScanner func(device string) ([]scannedNetwork, error)

// Backend implements wifi.Backend for macOS. A Backend must not be copied after
// first use. The command and scan functions are injectable so orchestration and
// cache behavior can be tested on any OS.
type Backend struct {
	WifiInterface string

	runOutput    outputRunner
	scanNetworks networkScanner

	cacheMu     sync.RWMutex
	lastVisible []wifi.Network
}

var currentNetworkRE = regexp.MustCompile(`Current Wi-Fi Network: (.+)`)

// ListNetworks returns the current network list and optionally scans first.
func (b *Backend) ListNetworks(scan wifi.ScanMode) (wifi.NetworksResult, error) {
	run := b.runOutput
	if run == nil {
		run = func(name string, args ...string) ([]byte, error) {
			return runWithOutput(exec.Command(name, args...))
		}
	}

	out, err := run("networksetup", "-getairportpower", b.WifiInterface)
	if err != nil {
		return wifi.NetworksResult{}, fmt.Errorf("failed to get wireless power: %w", err)
	}
	if !strings.Contains(string(out), ": On") {
		return wifi.NetworksResult{}, wifi.ErrWirelessDisabled
	}

	// The association query is best effort: preferred networks and scan
	// results are still useful without it. If fallback is also needed, preserve
	// both failures in ScanError because the missing association degrades it.
	currentOut, currentErr := run("networksetup", "-getairportnetwork", b.WifiInterface)
	currentSSID := ""
	if currentErr == nil {
		matches := currentNetworkRE.FindStringSubmatch(string(currentOut))
		if len(matches) > 1 {
			currentSSID = strings.TrimSpace(matches[1])
		}
	}

	preferredOut, err := run("networksetup", "-listpreferredwirelessnetworks", b.WifiInterface)
	if err != nil {
		return wifi.NetworksResult{}, fmt.Errorf("failed to list preferred networks: %w: %w", wifi.ErrOperationFailed, err)
	}
	knownSSIDs := parsePreferredNetworks(string(preferredOut))

	if scan == wifi.ScanNever {
		return wifi.NetworksResult{Networks: mergeNetworks(b.cachedNetworks(), knownSSIDs, currentSSID)}, nil
	}

	scanner := b.scanNetworks
	if scanner == nil {
		scanner = scanVisibleNetworks
	}
	scanned, err := scanner(b.WifiInterface)
	if err != nil {
		cause := err
		stage := wifi.ScanStageRequest
		if errors.Is(err, wifi.ErrScanProtocol) || errors.Is(err, wifi.ErrScanTimeout) {
			stage = wifi.ScanStageCompletion
		}
		if currentErr != nil {
			cause = fmt.Errorf("scan failed: %w; current network query also failed: %w", err, currentErr)
		}
		return b.scanFallback(b.cachedNetworks(), knownSSIDs, currentSSID, stage, cause), nil
	}
	visible := visibleNetworks(scanned)
	if len(scanned) > 0 && len(visible) == 0 {
		cause := fmt.Errorf("%w: scan returned no networks with an SSID", wifi.ErrScanProtocol)
		return b.scanFallback(b.cachedNetworks(), knownSSIDs, currentSSID, wifi.ScanStageCompletion, cause), nil
	}
	b.storeNetworks(visible)
	return wifi.NetworksResult{Networks: mergeNetworks(visible, knownSSIDs, currentSSID)}, nil
}

func parsePreferredNetworks(output string) map[string]bool {
	knownSSIDs := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(output))
	firstNonEmpty := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if firstNonEmpty {
			firstNonEmpty = false
			if strings.HasPrefix(line, "Preferred networks on ") && strings.HasSuffix(line, ":") {
				continue
			}
		}
		knownSSIDs[line] = true
	}
	return knownSSIDs
}

func (b *Backend) scanFallback(cached []wifi.Network, knownSSIDs map[string]bool, currentSSID string, stage wifi.ScanStage, cause error) wifi.NetworksResult {
	return wifi.NetworksResult{
		Networks: mergeNetworks(cached, knownSSIDs, currentSSID),
		IsCached: true,
		ScanError: &wifi.ScanFailure{
			Backend: "macOS",
			Stage:   stage,
			Device:  b.WifiInterface,
			Cause:   cause,
		},
	}
}

func visibleNetworks(scanned []scannedNetwork) []wifi.Network {
	networksByKey := make(map[darwinNetworkKey]wifi.Network, len(scanned))
	for _, network := range scanned {
		if network.ssid == "" {
			continue
		}
		accessPoint := wifi.AccessPoint{
			SSID:      network.ssid,
			BSSID:     network.bssid,
			Strength:  rssiToStrength(network.rssi),
			Frequency: network.frequency,
		}
		key := darwinNetworkKey{ssid: network.ssid, security: network.security}
		if existing, ok := networksByKey[key]; ok {
			existing.AccessPoints = append(existing.AccessPoints, accessPoint)
			networksByKey[key] = existing
			continue
		}
		networksByKey[key] = wifi.Network{
			SSID:         network.ssid,
			IsVisible:    true,
			AccessPoints: []wifi.AccessPoint{accessPoint},
			IsSecure:     network.security != wifi.SecurityOpen,
			Security:     network.security,
		}
	}
	return sortedNetworks(networksByKey)
}

func mergeNetworks(visible []wifi.Network, knownSSIDs map[string]bool, currentSSID string) []wifi.Network {
	networksByKey := make(map[darwinNetworkKey]wifi.Network, len(visible)+len(knownSSIDs)+1)
	variantCount := make(map[string]int, len(visible))
	for _, network := range visible {
		variantCount[network.SSID]++
	}
	currentSSIDVisible := false
	visibleSSIDs := make(map[string]bool, len(visible))
	for _, network := range cloneNetworks(visible) {
		// networksetup reports current and preferred networks by SSID only.
		// Do not apply that metadata to an arbitrary security variant when the
		// scan found more than one; doing so could mark an open evil twin known.
		unambiguous := variantCount[network.SSID] == 1
		network.IsActive = unambiguous && network.SSID == currentSSID
		network.IsKnown = unambiguous && knownSSIDs[network.SSID]
		network.AutoConnect = network.IsKnown
		key := darwinNetworkKey{ssid: network.SSID, security: network.Security}
		networksByKey[key] = network
		visibleSSIDs[network.SSID] = true
		currentSSIDVisible = currentSSIDVisible || network.SSID == currentSSID
	}
	if currentSSID != "" && !currentSSIDVisible {
		key := darwinNetworkKey{ssid: currentSSID, security: wifi.SecurityUnknown}
		networksByKey[key] = wifi.Network{
			SSID:        currentSSID,
			IsActive:    true,
			IsVisible:   true,
			IsKnown:     knownSSIDs[currentSSID],
			AutoConnect: knownSSIDs[currentSSID],
			Security:    wifi.SecurityUnknown,
		}
		visibleSSIDs[currentSSID] = true
	}

	for ssid := range knownSSIDs {
		if !visibleSSIDs[ssid] {
			key := darwinNetworkKey{ssid: ssid, security: wifi.SecurityUnknown}
			networksByKey[key] = wifi.Network{
				SSID:        ssid,
				IsKnown:     true,
				AutoConnect: true,
				Security:    wifi.SecurityUnknown,
			}
		}
	}
	return sortedNetworks(networksByKey)
}

type darwinNetworkKey struct {
	ssid     string
	security wifi.SecurityType
}

func (b *Backend) cachedNetworks() []wifi.Network {
	b.cacheMu.RLock()
	defer b.cacheMu.RUnlock()
	return cloneNetworks(b.lastVisible)
}

func (b *Backend) storeNetworks(networks []wifi.Network) {
	b.cacheMu.Lock()
	b.lastVisible = cloneNetworks(networks)
	b.cacheMu.Unlock()
}

func cloneNetworks(networks []wifi.Network) []wifi.Network {
	cloned := make([]wifi.Network, len(networks))
	for i, network := range networks {
		cloned[i] = network
		cloned[i].AccessPoints = append([]wifi.AccessPoint(nil), network.AccessPoints...)
	}
	return cloned
}

func sortedNetworks(networksByKey map[darwinNetworkKey]wifi.Network) []wifi.Network {
	networks := make([]wifi.Network, 0, len(networksByKey))
	for _, network := range networksByKey {
		networks = append(networks, network)
	}
	wifi.SortNetworks(networks)
	return networks
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
