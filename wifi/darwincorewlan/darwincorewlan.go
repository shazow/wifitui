// Package darwincorewlan is a work-in-progress macOS backend built on the
// native CoreWLAN framework. It is NOT wired into wifitui's backend selection
// and is not part of any release; the shipped macOS backend is wifi/darwin,
// which shells out to networksetup and system_profiler.
//
// CoreWLAN is the supported API for Wi-Fi scanning and returns structured
// per-access-point data (BSSID, RSSI, channel/frequency, security) with typed
// error classification, but modern macOS treats SSID/BSSID scan results as
// location-sensitive: without Location Services authorization for the
// responsible process, scans fail or return redacted SSIDs. Making this
// backend generally useful requires an authorization story (a stable signing
// identity and, for the terminal case, Location Services access for the
// terminal emulator).
//
// Current state:
//   - Scanning works when the calling context is authorized, and reports
//     typed errors (wifi.ErrScanPermissionDenied and friends) when it is not.
//   - Everything else returns wifi.ErrNotSupported.
//
// The eventual plan is to grow native coverage method by method (association
// via CWInterface, profiles via CWConfiguration) and compose this backend
// with wifi/darwin through a fallback combinator, so authorized callers get
// the fast native path and everyone else keeps the shell-out behavior.
package darwincorewlan

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/shazow/wifitui/wifi"
)

type networkScanner func(device string) ([]scannedNetwork, error)

// Backend is a partial wifi.Backend implementation backed by CoreWLAN.
type Backend struct {
	WifiInterface string

	// scanNetworks is injectable so orchestration can be tested on any OS.
	scanNetworks networkScanner
}

var _ wifi.Backend = (*Backend)(nil)

// New creates a new darwincorewlan.Backend.
func New() (wifi.Backend, error) {
	out, err := runWithOutput(exec.Command("networksetup", "-listallhardwareports"))
	if err != nil {
		return nil, fmt.Errorf("failed to list hardware ports: %w: %w", wifi.ErrOperationFailed, err)
	}
	device, err := findWifiDevice(string(out))
	if err != nil {
		return nil, err
	}
	return &Backend{WifiInterface: device}, nil
}

// ListNetworks scans for visible networks via CoreWLAN. Known-network,
// association, and cached-snapshot support are not implemented yet, so
// ScanNever returns an empty result and scan failures are fatal rather than
// degrading into NetworksResult.ScanError.
func (b *Backend) ListNetworks(scan wifi.ScanMode) (wifi.NetworksResult, error) {
	if scan == wifi.ScanNever {
		return wifi.NetworksResult{}, nil
	}
	scanner := b.scanNetworks
	if scanner == nil {
		scanner = scanVisibleNetworks
	}
	scanned, err := scanner(b.WifiInterface)
	if err != nil {
		return wifi.NetworksResult{}, fmt.Errorf("CoreWLAN scan failed: %w", err)
	}
	return wifi.NetworksResult{Networks: visibleNetworks(scanned)}, nil
}

// ActivateNetwork is not implemented yet.
func (b *Backend) ActivateNetwork(ssid string) error {
	return fmt.Errorf("darwincorewlan does not support activating networks yet: %w", wifi.ErrNotSupported)
}

// ForgetNetwork is not implemented yet.
func (b *Backend) ForgetNetwork(ssid string) error {
	return fmt.Errorf("darwincorewlan does not support forgetting networks yet: %w", wifi.ErrNotSupported)
}

// JoinNetwork is not implemented yet.
func (b *Backend) JoinNetwork(ssid string, password string, security wifi.SecurityType, isHidden bool) error {
	return fmt.Errorf("darwincorewlan does not support joining networks yet: %w", wifi.ErrNotSupported)
}

// GetSecrets is not implemented yet.
func (b *Backend) GetSecrets(ssid string) (string, error) {
	return "", fmt.Errorf("darwincorewlan does not support reading secrets yet: %w", wifi.ErrNotSupported)
}

// UpdateNetwork is not implemented yet.
func (b *Backend) UpdateNetwork(ssid string, opts wifi.UpdateOptions) error {
	return fmt.Errorf("darwincorewlan does not support updating networks yet: %w", wifi.ErrNotSupported)
}

// IsWirelessEnabled is not implemented yet.
func (b *Backend) IsWirelessEnabled() (bool, error) {
	return false, fmt.Errorf("darwincorewlan does not support radio state yet: %w", wifi.ErrNotSupported)
}

// SetWireless is not implemented yet.
func (b *Backend) SetWireless(enabled bool) error {
	return fmt.Errorf("darwincorewlan does not support radio control yet: %w", wifi.ErrNotSupported)
}

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

// findWifiDevice parses `networksetup -listallhardwareports` output to find
// the Wi-Fi device. Duplicated from wifi/darwin until the planned backend
// fusion extracts shared helpers.
func findWifiDevice(output string) (string, error) {
	stanzas := strings.Split(output, "\n\n")
	for _, stanza := range stanzas {
		var hardwarePort, device string
		isWifiPort := false
		for _, line := range strings.Split(stanza, "\n") {
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
