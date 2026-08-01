package darwincorewlan

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/shazow/wifitui/wifi"
)

func TestDecodeCoreWLANScan(t *testing.T) {
	output := []byte(`[
		{"ssid":"Cafe","bssid":"00:11:22:33:44:55","security":"open","rssi":-65,"frequency":2412},
		{"ssid":"Home","bssid":"00:11:22:33:44:66","security":"wpa","rssi":-50,"frequency":5180}
	]`)
	networks, err := decodeCoreWLANScan(output)
	if err != nil {
		t.Fatalf("decodeCoreWLANScan returned error: %v", err)
	}
	if len(networks) != 2 || networks[0].ssid != "Cafe" || networks[0].security != wifi.SecurityOpen || networks[1].frequency != 5180 {
		t.Fatalf("decodeCoreWLANScan = %#v", networks)
	}
}

func TestDecodeCoreWLANScanAllowsEmptyResults(t *testing.T) {
	networks, err := decodeCoreWLANScan([]byte("[]"))
	if err != nil || len(networks) != 0 {
		t.Fatalf("decodeCoreWLANScan(empty set) = %#v, %v; want empty success", networks, err)
	}
}

func TestDecodeCoreWLANScanRejectsUnusableResults(t *testing.T) {
	for _, output := range []string{"", `[{"ssid":""}]`, "not json"} {
		t.Run(output, func(t *testing.T) {
			_, err := decodeCoreWLANScan([]byte(output))
			if !errors.Is(err, wifi.ErrScanProtocol) {
				t.Fatalf("decodeCoreWLANScan(%q) = %v, want ErrScanProtocol", output, err)
			}
		})
	}
}

func TestDecodeCoreWLANScanPreservesJSONError(t *testing.T) {
	_, err := decodeCoreWLANScan([]byte("["))
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("decodeCoreWLANScan error = %v, want wrapped *json.SyntaxError", err)
	}
}

func TestCoreWLANStatusErrorClassifiesKnownFailures(t *testing.T) {
	tests := []struct {
		status int
		want   error
	}{
		{coreWLANStatusDeviceUnavailable, wifi.ErrScanDeviceUnavailable},
		{coreWLANStatusProtocol, wifi.ErrScanProtocol},
		{coreWLANStatusPermissionDenied, wifi.ErrScanPermissionDenied},
		{coreWLANStatusTimeout, wifi.ErrScanTimeout},
		{coreWLANStatusUnsupported, wifi.ErrNotSupported},
	}
	for _, test := range tests {
		err := coreWLANStatusError(test.status, "native detail")
		if !errors.Is(err, test.want) || !strings.Contains(err.Error(), "native detail") {
			t.Fatalf("coreWLANStatusError(%d) = %v, want native detail wrapping %v", test.status, err, test.want)
		}
	}
}

func TestListNetworksScanReportsVisibleNetworks(t *testing.T) {
	backend := &Backend{
		WifiInterface: "en0",
		scanNetworks: func(device string) ([]scannedNetwork, error) {
			if device != "en0" {
				t.Fatalf("scan device = %q, want en0", device)
			}
			return []scannedNetwork{
				{ssid: "Cafe", bssid: "00:11:22:33:44:55", security: wifi.SecurityOpen, rssi: -60, frequency: 2412},
				{ssid: "Cafe", bssid: "00:11:22:33:44:66", security: wifi.SecurityWPA, rssi: -50, frequency: 5180},
				{ssid: "Cafe", bssid: "00:11:22:33:44:77", security: wifi.SecurityWPA, rssi: -70, frequency: 2437},
			}, nil
		},
	}

	result, err := backend.ListNetworks(wifi.ScanForce)
	if err != nil {
		t.Fatalf("ListNetworks returned an error: %v", err)
	}
	if len(result.Networks) != 2 {
		t.Fatalf("ListNetworks returned %d networks, want separate open and WPA entries: %#v", len(result.Networks), result.Networks)
	}
	seen := make(map[wifi.SecurityType]wifi.Network, len(result.Networks))
	for _, network := range result.Networks {
		if !network.IsVisible {
			t.Fatalf("scanned network not visible: %#v", network)
		}
		seen[network.Security] = network
	}
	if seen[wifi.SecurityOpen].IsSecure || !seen[wifi.SecurityWPA].IsSecure {
		t.Fatalf("security variants = %#v, want open and secure WPA entries", seen)
	}
	wpa := seen[wifi.SecurityWPA]
	if len(wpa.AccessPoints) != 2 || wpa.Strength() != 100 {
		t.Fatalf("WPA variant = %#v, want two access points with 100%% peak strength", wpa)
	}
}

func TestListNetworksScanFailureIsFatal(t *testing.T) {
	scanErr := coreWLANStatusError(coreWLANStatusPermissionDenied, "SSIDs unavailable")
	backend := &Backend{
		WifiInterface: "en0",
		scanNetworks: func(string) ([]scannedNetwork, error) {
			return nil, scanErr
		},
	}

	_, err := backend.ListNetworks(wifi.ScanForce)
	if !errors.Is(err, wifi.ErrScanPermissionDenied) {
		t.Fatalf("ListNetworks error = %v, want wrapped ErrScanPermissionDenied", err)
	}
}

func TestListNetworksScanNeverSkipsScanner(t *testing.T) {
	backend := &Backend{
		WifiInterface: "en0",
		scanNetworks: func(string) ([]scannedNetwork, error) {
			t.Fatal("ScanNever invoked the scanner")
			return nil, nil
		},
	}

	result, err := backend.ListNetworks(wifi.ScanNever)
	if err != nil || len(result.Networks) != 0 {
		t.Fatalf("ScanNever = %#v, %v; want empty success", result, err)
	}
}

func TestUnimplementedOperationsReturnNotSupported(t *testing.T) {
	backend := &Backend{WifiInterface: "en0"}
	autoConnect := true
	operations := map[string]func() error{
		"ActivateNetwork": func() error { return backend.ActivateNetwork("Cafe") },
		"ForgetNetwork":   func() error { return backend.ForgetNetwork("Cafe") },
		"JoinNetwork":     func() error { return backend.JoinNetwork("Cafe", "hunter2", wifi.SecurityWPA, false) },
		"GetSecrets": func() error {
			_, err := backend.GetSecrets("Cafe")
			return err
		},
		"UpdateNetwork": func() error {
			return backend.UpdateNetwork("Cafe", wifi.UpdateOptions{AutoConnect: &autoConnect})
		},
		"IsWirelessEnabled": func() error {
			_, err := backend.IsWirelessEnabled()
			return err
		},
		"SetWireless": func() error { return backend.SetWireless(true) },
	}
	for name, operation := range operations {
		if err := operation(); !errors.Is(err, wifi.ErrNotSupported) {
			t.Fatalf("%s error = %v, want ErrNotSupported", name, err)
		}
	}
}

func TestFindWifiDevice(t *testing.T) {
	mockedOutput := `Hardware Port: Wi-Fi
Device: en0
Ethernet Address: a1:b2:c3:d4:e5:f6

Hardware Port: Bluetooth PAN
Device: en8
Ethernet Address: a1:b2:c3:d4:e5:f7`

	device, err := findWifiDevice(mockedOutput)
	if err != nil {
		t.Fatalf("findWifiDevice returned an error: %v", err)
	}
	if device != "en0" {
		t.Fatalf(`findWifiDevice returned "%s", want "en0"`, device)
	}
}
