package darwin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/shazow/wifitui/wifi"
)

func TestRunWithOutputPreservesExitErrorAndStderr(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=TestCommandHelperProcess")
	command.Env = append(os.Environ(), "WIFITUI_TEST_COMMAND_HELPER=1")

	out, err := runWithOutput(command)
	if string(out) != "partial output" {
		t.Fatalf("runWithOutput output = %q, want partial output", out)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
		t.Fatalf("runWithOutput error = %v, want wrapped exit status 7", err)
	}
	if !strings.Contains(err.Error(), "diagnostic detail") {
		t.Fatalf("runWithOutput error = %q, want captured stderr", err)
	}
}

func TestCommandHelperProcess(t *testing.T) {
	if os.Getenv("WIFITUI_TEST_COMMAND_HELPER") != "1" {
		return
	}
	_, _ = fmt.Fprint(os.Stdout, "partial output")
	_, _ = fmt.Fprint(os.Stderr, "diagnostic detail")
	os.Exit(7)
}

type commandResult struct {
	output string
	err    error
}

type fakeOutputRunner struct {
	t          *testing.T
	results    map[string]commandResult
	commands   []string
	unexpected bool
}

func (r *fakeOutputRunner) run(name string, args ...string) ([]byte, error) {
	r.t.Helper()
	command := strings.Join(append([]string{name}, args...), " ")
	r.commands = append(r.commands, command)
	result, ok := r.results[command]
	if !ok {
		r.unexpected = true
		r.t.Errorf("unexpected command: %s", command)
		return nil, fmt.Errorf("unexpected command: %s", command)
	}
	return []byte(result.output), result.err
}

func baseCommandResults() map[string]commandResult {
	return map[string]commandResult{
		"networksetup -getairportpower en0": {
			output: "Wi-Fi Power (en0): On\n",
		},
		"networksetup -getairportnetwork en0": {
			output: "Current Wi-Fi Network: Guest\n",
		},
		"networksetup -listpreferredwirelessnetworks en0": {
			output: "Preferred networks on en0:\n\tHome\n",
		},
	}
}

func TestListNetworksScanNeverSkipsScan(t *testing.T) {
	runner := &fakeOutputRunner{t: t, results: baseCommandResults()}
	backend := &Backend{
		WifiInterface: "en0",
		runOutput:     runner.run,
		scanNetworks: func(string) ([]scannedNetwork, error) {
			t.Fatal("ScanNever invoked the scanner")
			return nil, nil
		},
	}

	result, err := backend.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("ListNetworks returned an error: %v", err)
	}
	if result.ScanError != nil {
		t.Fatalf("ScanNever returned a scan error: %v", result.ScanError)
	}
	if result.IsCached {
		t.Fatal("ScanNever marked the current network list as cached")
	}
	if runner.unexpected {
		t.Fatal("ScanNever invoked an unexpected command")
	}
	if len(result.Networks) != 2 {
		t.Fatalf("listNetworks returned %d networks, want 2: %#v", len(result.Networks), result.Networks)
	}
	current := result.Networks[0]
	if current.SSID != "Guest" || !current.IsActive || !current.IsVisible || current.IsKnown {
		t.Fatalf("current non-preferred network = %#v, want active, visible, and unknown Guest", current)
	}
	known := result.Networks[1]
	if known.SSID != "Home" || known.IsActive || known.IsVisible || !known.IsKnown || !known.AutoConnect {
		t.Fatalf("preferred network = %#v, want non-visible known Home", known)
	}
}

func TestListNetworksScanModesRunScanner(t *testing.T) {
	for _, scan := range []wifi.ScanMode{wifi.ScanAuto, wifi.ScanForce} {
		t.Run(fmt.Sprintf("mode_%d", scan), func(t *testing.T) {
			runner := &fakeOutputRunner{t: t, results: baseCommandResults()}
			scanCalls := 0
			backend := &Backend{
				WifiInterface: "en0",
				runOutput:     runner.run,
				scanNetworks: func(device string) ([]scannedNetwork, error) {
					scanCalls++
					if device != "en0" {
						t.Fatalf("scan device = %q, want en0", device)
					}
					return []scannedNetwork{
						{ssid: "Guest", bssid: "00:11:22:33:44:55", security: wifi.SecurityWPA, rssi: -55},
						{ssid: "Home", bssid: "00:11:22:33:44:66", security: wifi.SecurityWPA, rssi: -70},
					}, nil
				},
			}

			result, err := backend.ListNetworks(scan)
			if err != nil {
				t.Fatalf("ListNetworks returned an error: %v", err)
			}
			if result.ScanError != nil || result.IsCached {
				t.Fatalf("successful report returned scan fallback metadata: %#v", result)
			}
			if scanCalls != 1 {
				t.Fatalf("scanner call count = %d, want 1", scanCalls)
			}
			if len(result.Networks) != 2 || result.Networks[0].SSID != "Guest" || !result.Networks[0].IsActive {
				t.Fatalf("listNetworks returned unexpected networks: %#v", result.Networks)
			}
		})
	}
}

func TestListNetworksScanFailureReturnsVisibleCurrentNetwork(t *testing.T) {
	scanErr := errors.New("CoreWLAN failed")
	runner := &fakeOutputRunner{t: t, results: baseCommandResults()}
	backend := &Backend{
		WifiInterface: "en0",
		runOutput:     runner.run,
		scanNetworks: func(string) ([]scannedNetwork, error) {
			return nil, scanErr
		},
	}

	result, err := backend.ListNetworks(wifi.ScanForce)
	if err != nil {
		t.Fatalf("listNetworks returned a fatal error: %v", err)
	}
	if !result.IsCached {
		t.Fatal("scan fallback did not set IsCached")
	}
	if !errors.Is(result.ScanError, scanErr) {
		t.Fatalf("ScanError = %v, want wrapped CoreWLAN error", result.ScanError)
	}
	var failure *wifi.ScanFailure
	if !errors.As(result.ScanError, &failure) {
		t.Fatalf("ScanError type = %T, want *wifi.ScanFailure", result.ScanError)
	}
	if failure.Backend != "macOS" || failure.Stage != wifi.ScanStageRequest || failure.Device != "en0" {
		t.Fatalf("ScanFailure = %#v, want macOS request failure on en0", failure)
	}
	if len(result.Networks) != 2 || result.Networks[0].SSID != "Guest" || !result.Networks[0].IsVisible {
		t.Fatalf("fallback networks = %#v, want visible current Guest first", result.Networks)
	}
}

func TestListNetworksScanAndCurrentNetworkFailuresArePreserved(t *testing.T) {
	currentErr := errors.New("current network failed")
	scanErr := errors.New("CoreWLAN failed")
	results := baseCommandResults()
	results["networksetup -getairportnetwork en0"] = commandResult{err: currentErr}
	runner := &fakeOutputRunner{t: t, results: results}
	backend := &Backend{
		WifiInterface: "en0",
		runOutput:     runner.run,
		scanNetworks: func(string) ([]scannedNetwork, error) {
			return nil, scanErr
		},
	}

	result, err := backend.ListNetworks(wifi.ScanAuto)
	if err != nil {
		t.Fatalf("listNetworks returned a fatal error: %v", err)
	}
	if !errors.Is(result.ScanError, scanErr) || !errors.Is(result.ScanError, currentErr) {
		t.Fatalf("ScanError = %v, want both command errors preserved", result.ScanError)
	}
	if len(result.Networks) != 1 || result.Networks[0].SSID != "Home" || result.Networks[0].IsVisible {
		t.Fatalf("fallback networks = %#v, want only non-visible preferred Home", result.Networks)
	}
}

func TestListNetworksRetainsSuccessfulVisibleSnapshot(t *testing.T) {
	runner := &fakeOutputRunner{t: t, results: baseCommandResults()}
	backend := &Backend{
		WifiInterface: "en0",
		runOutput:     runner.run,
		scanNetworks: func(string) ([]scannedNetwork, error) {
			return []scannedNetwork{
				{ssid: "Guest", security: wifi.SecurityWPA, rssi: -50},
				{ssid: "Cafe", security: wifi.SecurityOpen, rssi: -65},
			}, nil
		},
	}

	scanned, err := backend.ListNetworks(wifi.ScanForce)
	if err != nil || scanned.ScanError != nil {
		t.Fatalf("initial scan = %#v, %v", scanned, err)
	}
	for i := range scanned.Networks {
		if scanned.Networks[i].SSID == "Cafe" {
			scanned.Networks[i].AccessPoints[0].Strength = 1
		}
	}

	cached, err := backend.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("ScanNever returned an error: %v", err)
	}
	cafe, ok := networkBySSID(cached.Networks, "Cafe")
	if !ok || !cafe.IsVisible || cafe.IsKnown || cafe.Strength() != 70 {
		t.Fatalf("cached Cafe = %#v, %t; want independent visible snapshot at 70%%", cafe, ok)
	}
}

func TestListNetworksFailedScanUsesRetainedSnapshot(t *testing.T) {
	runner := &fakeOutputRunner{t: t, results: baseCommandResults()}
	fail := false
	backend := &Backend{
		WifiInterface: "en0",
		runOutput:     runner.run,
		scanNetworks: func(string) ([]scannedNetwork, error) {
			if fail {
				return nil, errors.New("later scan failed")
			}
			return []scannedNetwork{{ssid: "Cafe", security: wifi.SecurityOpen, rssi: -65}}, nil
		},
	}
	if _, err := backend.ListNetworks(wifi.ScanAuto); err != nil {
		t.Fatalf("initial scan failed: %v", err)
	}
	fail = true

	result, err := backend.ListNetworks(wifi.ScanForce)
	if err != nil {
		t.Fatalf("failed scan returned fatal error: %v", err)
	}
	if result.ScanError == nil || !result.IsCached {
		t.Fatalf("failed scan metadata = %#v", result)
	}
	if cafe, ok := networkBySSID(result.Networks, "Cafe"); !ok || !cafe.IsVisible {
		t.Fatalf("failed scan discarded cached Cafe: %#v", result.Networks)
	}
}

func TestBackendVisibleSnapshotIsConcurrentSafe(t *testing.T) {
	backend := &Backend{}
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			for iteration := 0; iteration < 100; iteration++ {
				backend.storeNetworks([]wifi.Network{{
					SSID:         fmt.Sprintf("network-%d", worker),
					IsVisible:    true,
					AccessPoints: []wifi.AccessPoint{{Strength: uint8(iteration)}},
				}})
				cached := backend.cachedNetworks()
				if len(cached) != 1 {
					t.Errorf("cached network count = %d, want 1", len(cached))
					return
				}
			}
		}(worker)
	}
	workers.Wait()
}

func TestListNetworksEmptyScanClearsVisibleSnapshot(t *testing.T) {
	runner := &fakeOutputRunner{t: t, results: baseCommandResults()}
	backend := &Backend{
		WifiInterface: "en0",
		runOutput:     runner.run,
		lastVisible: []wifi.Network{{
			SSID:      "Stale Cafe",
			IsVisible: true,
		}},
		scanNetworks: func(string) ([]scannedNetwork, error) {
			return nil, nil
		},
	}

	result, err := backend.ListNetworks(wifi.ScanForce)
	if err != nil {
		t.Fatalf("ListNetworks returned fatal error: %v", err)
	}
	if result.ScanError != nil || result.IsCached {
		t.Fatalf("empty successful scan returned fallback metadata: %#v", result)
	}
	if _, ok := networkBySSID(result.Networks, "Stale Cafe"); ok {
		t.Fatalf("empty successful scan retained stale visible network: %#v", result.Networks)
	}
}

func TestListNetworksUnsupportedScanIsTypedFailure(t *testing.T) {
	runner := &fakeOutputRunner{t: t, results: baseCommandResults()}
	backend := &Backend{
		WifiInterface: "en0",
		runOutput:     runner.run,
		scanNetworks: func(string) ([]scannedNetwork, error) {
			return nil, wifi.ErrNotSupported
		},
	}

	result, err := backend.ListNetworks(wifi.ScanForce)
	if err != nil {
		t.Fatalf("ListNetworks returned fatal error: %v", err)
	}
	var failure *wifi.ScanFailure
	if !errors.As(result.ScanError, &failure) || !errors.Is(result.ScanError, wifi.ErrNotSupported) {
		t.Fatalf("ScanError = %#v, want typed unsupported ScanFailure", result.ScanError)
	}
}

func TestListNetworksTimeoutIsCompletionFailure(t *testing.T) {
	runner := &fakeOutputRunner{t: t, results: baseCommandResults()}
	backend := &Backend{
		WifiInterface: "en0",
		runOutput:     runner.run,
		scanNetworks: func(string) ([]scannedNetwork, error) {
			return nil, wifi.ErrScanTimeout
		},
	}

	result, err := backend.ListNetworks(wifi.ScanForce)
	if err != nil {
		t.Fatalf("ListNetworks returned fatal error: %v", err)
	}
	var failure *wifi.ScanFailure
	if !errors.As(result.ScanError, &failure) || failure.Stage != wifi.ScanStageCompletion {
		t.Fatalf("ScanError = %#v, want completion-stage timeout", result.ScanError)
	}
}

func TestVisibleNetworksKeepsSecurityVariantsSeparate(t *testing.T) {
	networks := visibleNetworks([]scannedNetwork{
		{ssid: "Cafe", bssid: "00:11:22:33:44:55", security: wifi.SecurityOpen, rssi: -60},
		{ssid: "Cafe", bssid: "00:11:22:33:44:66", security: wifi.SecurityWPA, rssi: -50},
	})

	if len(networks) != 2 {
		t.Fatalf("visibleNetworks returned %d networks, want separate open and WPA entries: %#v", len(networks), networks)
	}
	seen := make(map[wifi.SecurityType]wifi.Network, len(networks))
	for _, network := range networks {
		seen[network.Security] = network
	}
	if seen[wifi.SecurityOpen].IsSecure || !seen[wifi.SecurityWPA].IsSecure {
		t.Fatalf("security variants = %#v, want open and secure WPA entries", seen)
	}
	if len(seen[wifi.SecurityOpen].AccessPoints) != 1 || len(seen[wifi.SecurityWPA].AccessPoints) != 1 {
		t.Fatalf("security variants merged access points: %#v", seen)
	}
}

func TestMergeNetworksDoesNotTrustAmbiguousSSIDMetadata(t *testing.T) {
	visible := visibleNetworks([]scannedNetwork{
		{ssid: "Cafe", bssid: "00:11:22:33:44:55", security: wifi.SecurityOpen, rssi: -60},
		{ssid: "Cafe", bssid: "00:11:22:33:44:66", security: wifi.SecurityWPA, rssi: -50},
	})
	networks := mergeNetworks(visible, map[string]bool{"Cafe": true}, "Cafe")

	if len(networks) != 2 {
		t.Fatalf("mergeNetworks returned %d networks, want 2 security variants: %#v", len(networks), networks)
	}
	for _, network := range networks {
		if network.IsKnown || network.IsActive || network.AutoConnect {
			t.Fatalf("ambiguous security variant received SSID-only metadata: %#v", network)
		}
	}
}

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

func TestParsePreferredNetworksKeepsSSIDStartingWithPreferred(t *testing.T) {
	known := parsePreferredNetworks("Preferred networks on en0:\n\tHome\n\tPreferred Cafe\n")
	if !known["Home"] || !known["Preferred Cafe"] || len(known) != 2 {
		t.Fatalf("parsePreferredNetworks = %#v", known)
	}
}

func networkBySSID(networks []wifi.Network, ssid string) (wifi.Network, bool) {
	for _, network := range networks {
		if network.SSID == ssid {
			return network, true
		}
	}
	return wifi.Network{}, false
}

func TestFindWifiDevice(t *testing.T) {
	mockedOutput := `Hardware Port: Wi-Fi
Device: en0
Ethernet Address: a1:b2:c3:d4:e5:f6

Hardware Port: Bluetooth PAN
Device: en8
Ethernet Address: a1:b2:c3:d4:e5:f7

Hardware Port: Thunderbolt Bridge
Device: bridge0
Ethernet Address: a1:b2:c3:d4:e5:f8`

	device, err := findWifiDevice(mockedOutput)
	if err != nil {
		t.Fatalf("findWifiDevice returned an error: %v", err)
	}
	if device != "en0" {
		t.Fatalf(`findWifiDevice returned "%s", want "en0"`, device)
	}
}

func TestRssiToStrength(t *testing.T) {
	tests := []struct {
		rssi     int
		expected uint8
	}{
		{-50, 100}, // Strong signal
		{-70, 60},  // Medium signal
		{-90, 20},  // Weak signal
		{-100, 0},  // Minimum
		{-110, 0},  // Below minimum
		{0, 0},     // Invalid
		{10, 0},    // Invalid positive
	}

	for _, tt := range tests {
		result := rssiToStrength(tt.rssi)
		if result != tt.expected {
			t.Errorf("rssiToStrength(%d) = %d, want %d", tt.rssi, result, tt.expected)
		}
	}
}
