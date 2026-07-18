package darwin

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
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

func TestListNetworksScanNeverSkipsProfiler(t *testing.T) {
	runner := &fakeOutputRunner{t: t, results: baseCommandResults()}

	result, err := listNetworks(runner.run, "en0", wifi.ScanNever)
	if err != nil {
		t.Fatalf("listNetworks returned an error: %v", err)
	}
	if result.ScanError != nil {
		t.Fatalf("ScanNever returned a scan error: %v", result.ScanError)
	}
	if result.IsCached {
		t.Fatal("ScanNever marked the current network list as cached")
	}
	if runner.unexpected || slices.Contains(runner.commands, "system_profiler SPAirPortDataType") {
		t.Fatal("ScanNever invoked system_profiler")
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

func TestListNetworksScanModesRunProfiler(t *testing.T) {
	const profilerOutput = `Wi-Fi:
      Interfaces:
        en0:
          Current Network Information:
            Guest:
              Security: WPA2 Personal
              Signal / Noise: -55 dBm / -95 dBm
          Other Local Wi-Fi Networks:
            Home:
              Security: WPA2 Personal
              Signal / Noise: -70 dBm / -90 dBm`

	for _, scan := range []wifi.ScanMode{wifi.ScanAuto, wifi.ScanForce} {
		t.Run(fmt.Sprintf("mode_%d", scan), func(t *testing.T) {
			results := baseCommandResults()
			results["system_profiler SPAirPortDataType"] = commandResult{output: profilerOutput}
			runner := &fakeOutputRunner{t: t, results: results}

			result, err := listNetworks(runner.run, "en0", scan)
			if err != nil {
				t.Fatalf("listNetworks returned an error: %v", err)
			}
			if result.ScanError != nil || result.IsCached {
				t.Fatalf("successful report returned scan fallback metadata: %#v", result)
			}
			if got := countCommand(runner.commands, "system_profiler SPAirPortDataType"); got != 1 {
				t.Fatalf("system_profiler call count = %d, want 1", got)
			}
			if len(result.Networks) != 2 || result.Networks[0].SSID != "Guest" || !result.Networks[0].IsActive {
				t.Fatalf("listNetworks returned unexpected networks: %#v", result.Networks)
			}
		})
	}
}

func TestListNetworksProfilerFailureReturnsVisibleCurrentNetwork(t *testing.T) {
	profilerErr := errors.New("profiler failed")
	results := baseCommandResults()
	results["system_profiler SPAirPortDataType"] = commandResult{err: profilerErr}
	runner := &fakeOutputRunner{t: t, results: results}

	result, err := listNetworks(runner.run, "en0", wifi.ScanForce)
	if err != nil {
		t.Fatalf("listNetworks returned a fatal error: %v", err)
	}
	if !result.IsCached {
		t.Fatal("scan fallback did not set IsCached")
	}
	if !errors.Is(result.ScanError, profilerErr) {
		t.Fatalf("ScanError = %v, want wrapped profiler error", result.ScanError)
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

func TestListNetworksProfilerAndCurrentNetworkFailuresArePreserved(t *testing.T) {
	currentErr := errors.New("current network failed")
	profilerErr := errors.New("profiler failed")
	results := baseCommandResults()
	results["networksetup -getairportnetwork en0"] = commandResult{err: currentErr}
	results["system_profiler SPAirPortDataType"] = commandResult{err: profilerErr}
	runner := &fakeOutputRunner{t: t, results: results}

	result, err := listNetworks(runner.run, "en0", wifi.ScanAuto)
	if err != nil {
		t.Fatalf("listNetworks returned a fatal error: %v", err)
	}
	if !errors.Is(result.ScanError, profilerErr) || !errors.Is(result.ScanError, currentErr) {
		t.Fatalf("ScanError = %v, want both command errors preserved", result.ScanError)
	}
	if len(result.Networks) != 1 || result.Networks[0].SSID != "Home" || result.Networks[0].IsVisible {
		t.Fatalf("fallback networks = %#v, want only non-visible preferred Home", result.Networks)
	}
}

func countCommand(commands []string, want string) int {
	count := 0
	for _, command := range commands {
		if command == want {
			count++
		}
	}
	return count
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

func TestParseSystemProfilerOutput(t *testing.T) {
	mockedOutput := `Wi-Fi:

      Software Versions:
          CoreWLAN: 16.0 (1657)
      Interfaces:
        en0:
          Card Type: Wi-Fi
          Status: Connected
          Current Network Information:
            MyHomeNetwork:
              PHY Mode: 802.11ac
              Channel: 36 (5GHz, 80MHz)
              Network Type: Infrastructure
              Security: WPA2 Personal
              Signal / Noise: -55 dBm / -95 dBm
              Transmit Rate: 866
          Other Local Wi-Fi Networks:
            NeighborWiFi:
              PHY Mode: 802.11n
              Channel: 6 (2GHz, 20MHz)
              Network Type: Infrastructure
              Security: WPA2 Personal
              Signal / Noise: -75 dBm / -90 dBm
            OpenCafe:
              PHY Mode: 802.11g
              Channel: 11 (2GHz, 20MHz)
              Network Type: Infrastructure
              Security: Open
        awdl0:
          MAC Address: 00:11:22:33:44:55`

	networks := parseSystemProfilerOutput(mockedOutput)

	if len(networks) != 3 {
		t.Fatalf("expected 3 networks, got %d", len(networks))
	}

	// Check active network
	found := false
	for _, n := range networks {
		if n.ssid == "MyHomeNetwork" {
			found = true
			if !n.isActive {
				t.Error("MyHomeNetwork should be marked as active")
			}
			if n.rssi != -55 {
				t.Errorf("MyHomeNetwork rssi should be -55, got %d", n.rssi)
			}
			if n.security != wifi.SecurityWPA {
				t.Errorf("MyHomeNetwork security should be WPA, got %v", n.security)
			}
		}
	}
	if !found {
		t.Error("MyHomeNetwork not found in parsed networks")
	}

	// Check neighbor network
	found = false
	for _, n := range networks {
		if n.ssid == "NeighborWiFi" {
			found = true
			if n.isActive {
				t.Error("NeighborWiFi should not be marked as active")
			}
			if n.rssi != -75 {
				t.Errorf("NeighborWiFi rssi should be -75, got %d", n.rssi)
			}
		}
	}
	if !found {
		t.Error("NeighborWiFi not found in parsed networks")
	}

	// Check open network
	found = false
	for _, n := range networks {
		if n.ssid == "OpenCafe" {
			found = true
			if n.security != wifi.SecurityOpen {
				t.Errorf("OpenCafe security should be Open, got %v", n.security)
			}
		}
	}
	if !found {
		t.Error("OpenCafe not found in parsed networks")
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
