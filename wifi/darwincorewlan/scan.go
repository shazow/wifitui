package darwincorewlan

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/shazow/wifitui/wifi"
)

// scannedNetwork is one access point observation from a CoreWLAN scan.
type scannedNetwork struct {
	ssid      string
	bssid     string
	security  wifi.SecurityType
	rssi      int
	frequency uint
}

// coreWLANNetwork mirrors the JSON records emitted by the cgo bridge.
type coreWLANNetwork struct {
	SSID      string `json:"ssid"`
	BSSID     string `json:"bssid"`
	Security  string `json:"security"`
	RSSI      int    `json:"rssi"`
	Frequency uint   `json:"frequency"`
}

// Status codes returned by wifitui_corewlan_scan in the cgo bridge.
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

type networkKey struct {
	ssid     string
	security wifi.SecurityType
}

// visibleNetworks aggregates scan observations into networks keyed by SSID and
// security so same-named networks with different security remain separate.
func visibleNetworks(scanned []scannedNetwork) []wifi.Network {
	networksByKey := make(map[networkKey]wifi.Network, len(scanned))
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
		key := networkKey{ssid: network.ssid, security: network.security}
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
