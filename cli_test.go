package main

import (
	"bytes"
	"encoding/json"
	"errors"
	flags "github.com/jessevdk/go-flags"
	"strings"
	"testing"
	"time"

	"github.com/shazow/wifitui/wifi"
	"github.com/shazow/wifitui/wifi/mock"
)

func TestRunListAll(t *testing.T) {
	mockBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	var buf bytes.Buffer

	// Test with all=true (should list invisible known networks)
	if err := runList(&buf, false, true, false, mockBackend); err != nil {
		t.Fatalf("runList() failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "HideYoKidsHideYoWiFi") {
		t.Errorf("runList() output missing expected network. got=%q", output)
	}
	if !strings.Contains(output, "Unencrypted_Honeypot") {
		t.Errorf("runList() output missing expected network. got=%q", output)
	}
}

func TestRunListDefault(t *testing.T) {
	mockBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	var buf bytes.Buffer

	// Default behavior (all=false)
	if err := runList(&buf, false, false, false, mockBackend); err != nil {
		t.Fatalf("runList() failed: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "HideYoKidsHideYoWiFi") {
		t.Errorf("runList() output should NOT contain invisible network. got=%q", output)
	}
	if !strings.Contains(output, "Unencrypted_Honeypot") {
		t.Errorf("runList() output missing expected visible network. got=%q", output)
	}
}

func TestRunShow(t *testing.T) {
	mockBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	var buf bytes.Buffer

	// Test case: network found and known
	if err := runShow(&buf, false, "Password is password", mockBackend); err != nil {
		t.Fatalf("runShow() with found network failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "SSID: Password is password") {
		t.Errorf("runShow() output missing SSID. got=%q", output)
	}
	if !strings.Contains(output, "Passphrase: password") {
		t.Errorf("runShow() output missing passphrase. got=%q", output)
	}

	// Test case: network found, but not known (no secret)
	buf.Reset()
	if err := runShow(&buf, false, "GET off my LAN", mockBackend); err != nil {
		// This should not fail, just return no secret.
		t.Fatalf("runShow() with network without secret failed: %v", err)
	}
	output = buf.String()
	if !strings.Contains(output, "SSID: GET off my LAN") {
		t.Errorf("runShow() output missing SSID. got=%q", output)
	}
	if !strings.Contains(output, "Passphrase: ") {
		t.Errorf("runShow() output should have empty passphrase. got=%q", output)
	}

	// Test case: network not found
	buf.Reset()
	{
		const doesNotExist = "_DOES NOT EXIST_"
		err := runShow(&buf, false, doesNotExist, mockBackend)
		if err == nil {
			t.Fatalf("runShow() with not found network should have failed, but did not")
		}
		if !errors.Is(err, wifi.ErrNotFound) {
			t.Errorf("runShow() with not found network gave wrong error. got=%q", err)
		}
	}
}

func TestRunListJSON(t *testing.T) {
	mockBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	var buf bytes.Buffer

	if err := runList(&buf, true, true, false, mockBackend); err != nil {
		t.Fatalf("runList() failed: %v", err)
	}

	var connections []wifi.Connection
	if err := json.Unmarshal(buf.Bytes(), &connections); err != nil {
		t.Fatalf("runList() output is not valid JSON: %v. got=%q", err, buf.String())
	}

	if len(connections) == 0 {
		t.Fatalf("runList() output is empty")
	}

	// Just check for one of the SSIDs
	found := false
	for _, c := range connections {
		if c.SSID == "HideYoKidsHideYoWiFi" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("runList() JSON output missing expected network. got=%q", buf.String())
	}
}

func TestRunShowJSON(t *testing.T) {
	mockBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	var buf bytes.Buffer

	// Test case: network found and known
	if err := runShow(&buf, true, "Password is password", mockBackend); err != nil {
		t.Fatalf("runShow() with found network failed: %v", err)
	}

	type connectionWithSecret struct {
		wifi.Connection
		Passphrase string `json:"passphrase,omitempty"`
	}

	var connWithSecretData connectionWithSecret
	if err := json.Unmarshal(buf.Bytes(), &connWithSecretData); err != nil {
		t.Fatalf("runShow() output is not valid JSON: %v. got=%q", err, buf.String())
	}

	if connWithSecretData.SSID != "Password is password" {
		t.Errorf("runShow() JSON output has wrong SSID. got=%q", connWithSecretData.SSID)
	}
	if connWithSecretData.Passphrase != "password" {
		t.Errorf("runShow() JSON output has wrong passphrase. got=%q", connWithSecretData.Passphrase)
	}

	// Test case: network found, but not known (no secret)
	buf.Reset()
	if err := runShow(&buf, true, "GET off my LAN", mockBackend); err != nil {
		// This should not fail, just return no secret.
		t.Fatalf("runShow() with network without secret failed: %v", err)
	}

	// Re-initialize the struct to avoid carrying over the passphrase
	connWithSecretData = connectionWithSecret{}
	if err := json.Unmarshal(buf.Bytes(), &connWithSecretData); err != nil {
		t.Fatalf("runShow() output is not valid JSON: %v. got=%q", err, buf.String())
	}

	if connWithSecretData.SSID != "GET off my LAN" {
		t.Errorf("runShow() JSON output has wrong SSID. got=%q", connWithSecretData.SSID)
	}
	if connWithSecretData.Passphrase != "" {
		t.Errorf("runShow() JSON output should have empty passphrase. got=%q", connWithSecretData.Passphrase)
	}
}

func TestRunConnect(t *testing.T) {
	mockBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	var buf bytes.Buffer

	// Test case: connect to a new network with a passphrase
	if err := runConnect(&buf, "new-network", "new-password", wifi.SecurityWPA, false, RetryConfig{Interval: time.Second}, mockBackend); err != nil {
		t.Fatalf("runConnect() with passphrase failed: %v", err)
	}

	// Check if the network was added
	networks, err := mockBackend.BuildNetworkList(false)
	if err != nil {
		t.Fatalf("failed to get network list: %v", err)
	}
	found := false
	for _, n := range networks {
		if n.SSID == "new-network" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("runConnect() did not add the new network")
	}

	// Test case: connect to a known network without a passphrase
	buf.Reset()
	if err := runConnect(&buf, "Password is password", "", wifi.SecurityWPA, false, RetryConfig{Interval: time.Second}, mockBackend); err != nil {
		t.Fatalf("runConnect() without passphrase failed: %v", err)
	}

	// Check if the network is active
	networks, err = mockBackend.BuildNetworkList(false)
	if err != nil {
		t.Fatalf("failed to get network list: %v", err)
	}
	found = false
	for _, n := range networks {
		if n.SSID == "Password is password" {
			if n.IsActive {
				found = true
			}
			break
		}
	}
	if !found {
		t.Errorf("runConnect() did not activate the existing network")
	}
}

func TestRunRadio(t *testing.T) {
	mockBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	var buf bytes.Buffer

	if err := runRadio(&buf, "off", mockBackend); err != nil {
		t.Fatalf("runRadio(off) failed: %v", err)
	}
	enabled, err := mockBackend.IsWirelessEnabled()
	if err != nil {
		t.Fatalf("IsWirelessEnabled() failed: %v", err)
	}
	if enabled {
		t.Fatalf("expected wireless to be disabled")
	}

	buf.Reset()
	if err := runRadio(&buf, "on", mockBackend); err != nil {
		t.Fatalf("runRadio(on) failed: %v", err)
	}
	enabled, err = mockBackend.IsWirelessEnabled()
	if err != nil {
		t.Fatalf("IsWirelessEnabled() failed: %v", err)
	}
	if !enabled {
		t.Fatalf("expected wireless to be enabled")
	}

	buf.Reset()
	if err := runRadio(&buf, "toggle", mockBackend); err != nil {
		t.Fatalf("runRadio(toggle) failed: %v", err)
	}
	enabled, err = mockBackend.IsWirelessEnabled()
	if err != nil {
		t.Fatalf("IsWirelessEnabled() failed: %v", err)
	}
	if enabled {
		t.Fatalf("expected wireless to be disabled after toggle")
	}

	buf.Reset()
	if err := runRadio(&buf, "", mockBackend); err != nil {
		t.Fatalf("runRadio(default toggle) failed: %v", err)
	}
	enabled, err = mockBackend.IsWirelessEnabled()
	if err != nil {
		t.Fatalf("IsWirelessEnabled() failed: %v", err)
	}
	if !enabled {
		t.Fatalf("expected wireless to be enabled after default toggle")
	}
}

func TestRunRadioInvalidAction(t *testing.T) {
	mockBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	var buf bytes.Buffer

	err = runRadio(&buf, "wat", mockBackend)
	if err == nil {
		t.Fatal("expected invalid action error")
	}
	if !strings.Contains(err.Error(), "invalid radio action") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type flakyBackend struct {
	*mock.MockBackend
	failCount    int
	maxFails     int
	scannedSSIDs []string // SSIDs that were scanned
}

func (f *flakyBackend) BuildNetworkList(shouldScan bool) ([]wifi.Connection, error) {
	if shouldScan {
		f.scannedSSIDs = append(f.scannedSSIDs, "any")
	}
	return f.MockBackend.BuildNetworkList(shouldScan)
}

func (f *flakyBackend) JoinNetwork(ssid, passphrase string, security wifi.SecurityType, isHidden bool) error {
	if f.failCount < f.maxFails {
		f.failCount++
		return errors.New("transient failure")
	}
	return f.MockBackend.JoinNetwork(ssid, passphrase, security, isHidden)
}

func TestRunConnectRetry(t *testing.T) {
	baseBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	mb, ok := baseBackend.(*mock.MockBackend)
	if !ok {
		t.Fatalf("expected *mock.MockBackend, got %T", baseBackend)
	}

	// fail count = 2.
	// 1st try: fail (no scan).
	// 2nd try: fail (scan, immediate).
	// 3rd try: succeed (after sleep).
	// Total time ~5s.
	fb := &flakyBackend{
		MockBackend: mb,
		maxFails:    2,
	}

	var buf bytes.Buffer
	retryTotal := 7 * time.Second

	// We set ActionSleep to 0 to make the mock actions fast, only our loop sleep matters.
	fb.MockBackend.ActionSleep = 0

	start := time.Now()
	// Using passphrase triggers JoinNetwork which we overrode
	if err := runConnect(&buf, "retry-network", "password", wifi.SecurityWPA, false, RetryConfig{Total: retryTotal, Interval: 5 * time.Second}, fb); err != nil {
		t.Fatalf("runConnect() with retry failed: %v", err)
	}
	duration := time.Since(start)

	if duration < 5*time.Second {
		t.Errorf("runConnect() returned too quickly, expected at least 5s delay, got %v", duration)
	}

	// Check if scan was performed
	if len(fb.scannedSSIDs) == 0 {
		t.Errorf("expected at least one scan attempt, got 0")
	}

	// Check output for retry messages
	output := buf.String()
	// We expect 2 failures:
	// 1. "Quick connect failed..."
	// 2. "Connection failed: ... Retrying in 5s..."
	if !strings.Contains(output, "Quick connect failed") {
		t.Errorf("expected fast retry message, got:\n%s", output)
	}
	if !strings.Contains(output, "Connection failed: \"transient failure\"") {
		t.Errorf("expected regular retry message, got:\n%s", output)
	}
}

func TestRunConnectFastRetry(t *testing.T) {
	baseBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	mb, ok := baseBackend.(*mock.MockBackend)
	if !ok {
		t.Fatalf("expected *mock.MockBackend, got %T", baseBackend)
	}

	// fail count = 1.
	// 1st try: fail (no scan).
	// 2nd try: succeed (scan, immediate).
	// Total time < 1s.
	fb := &flakyBackend{
		MockBackend: mb,
		maxFails:    1,
	}

	var buf bytes.Buffer
	retryTotal := 7 * time.Second

	// We set ActionSleep to 0 to make the mock actions fast.
	fb.MockBackend.ActionSleep = 0

	start := time.Now()
	// Using passphrase triggers JoinNetwork which we overrode
	if err := runConnect(&buf, "retry-network", "password", wifi.SecurityWPA, false, RetryConfig{Total: retryTotal, Interval: 5 * time.Second}, fb); err != nil {
		t.Fatalf("runConnect() with fast retry failed: %v", err)
	}
	duration := time.Since(start)

	if duration > 2*time.Second {
		t.Errorf("runConnect() took too long for fast retry, got %v", duration)
	}

	// Check if scan was performed
	if len(fb.scannedSSIDs) == 0 {
		t.Errorf("expected at least one scan attempt, got 0")
	}

	output := buf.String()
	if !strings.Contains(output, "Quick connect failed") {
		t.Errorf("expected fast retry message, got:\n%s", output)
	}
}

func TestRunConnectCustomRetryInterval(t *testing.T) {
	baseBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	mb, ok := baseBackend.(*mock.MockBackend)
	if !ok {
		t.Fatalf("expected *mock.MockBackend, got %T", baseBackend)
	}

	// fail count = 2.
	// 1st try: fail (no scan).
	// 2nd try: fail (scan, immediate).
	// 3rd try: succeed (after sleep).
	fb := &flakyBackend{
		MockBackend: mb,
		maxFails:    2,
	}

	var buf bytes.Buffer
	retryTotal := 5 * time.Second
	retryInterval := 2 * time.Second

	fb.MockBackend.ActionSleep = 0

	start := time.Now()
	if err := runConnect(&buf, "retry-network", "password", wifi.SecurityWPA, false, RetryConfig{Total: retryTotal, Interval: retryInterval}, fb); err != nil {
		t.Fatalf("runConnect() failed: %v", err)
	}
	duration := time.Since(start)

	// Expected wait: 1st retry (fast) is immediate, 2nd retry waits for retryInterval (2s).
	// Total wait should be around 2s.
	if duration < 2*time.Second || duration > 3*time.Second {
		t.Errorf("expected duration around 2s, got %v", duration)
	}

	output := buf.String()
	if !strings.Contains(output, "Retrying in 2s") {
		t.Errorf("expected retry message with 2s interval, got:\n%s", output)
	}
}

func init() {
	mock.DefaultActionSleep = 0
}

func TestParseSecurityType(t *testing.T) {
	tests := []struct {
		input   string
		want    wifi.SecurityType
		wantErr bool
	}{
		{"open", wifi.SecurityOpen, false},
		{"wep", wifi.SecurityWEP, false},
		{"wpa", wifi.SecurityWPA, false},
		{"", wifi.SecurityUnknown, true},
		{"WPA", wifi.SecurityUnknown, true},
		{"invalid", wifi.SecurityUnknown, true},
	}
	for _, tt := range tests {
		got, err := parseSecurityType(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseSecurityType(%q) expected error, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("parseSecurityType(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("parseSecurityType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		}
	}
}

func TestParseRetryConfig(t *testing.T) {
	tests := []struct {
		input        string
		wantTotal    time.Duration
		wantInterval time.Duration
		wantErr      bool
	}{
		{"", 0, defaultRetryInterval, false},
		{"60s", 60 * time.Second, defaultRetryInterval, false},
		{"2m", 2 * time.Minute, defaultRetryInterval, false},
		{"2m:20s", 2 * time.Minute, 20 * time.Second, false},
		{"invalid", 0, 0, true},
		{"60s:invalid", 0, 0, true},
	}
	for _, tt := range tests {
		got, err := parseRetryConfig(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseRetryConfig(%q) expected error, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("parseRetryConfig(%q) unexpected error: %v", tt.input, err)
			}
			if got.Total != tt.wantTotal {
				t.Errorf("parseRetryConfig(%q).Total = %v, want %v", tt.input, got.Total, tt.wantTotal)
			}
			if got.Interval != tt.wantInterval {
				t.Errorf("parseRetryConfig(%q).Interval = %v, want %v", tt.input, got.Interval, tt.wantInterval)
			}
		}
	}
}

func TestFilterVisibleConnections(t *testing.T) {
	connections := []wifi.Connection{
		{SSID: "visible1", IsVisible: true},
		{SSID: "hidden1", IsVisible: false},
		{SSID: "visible2", IsVisible: true},
		{SSID: "hidden2", IsVisible: false},
	}

	visible := filterVisibleConnections(connections)
	if len(visible) != 2 {
		t.Fatalf("filterVisibleConnections() returned %d connections, want 2", len(visible))
	}
	for _, c := range visible {
		if !c.IsVisible {
			t.Errorf("filterVisibleConnections() returned non-visible connection %q", c.SSID)
		}
	}

	// Empty input
	if got := filterVisibleConnections(nil); got != nil {
		t.Errorf("filterVisibleConnections(nil) = %v, want nil", got)
	}
}

func TestFindConnectionBySSID(t *testing.T) {
	connections := []wifi.Connection{
		{SSID: "NetworkA"},
		{SSID: "NetworkB"},
		{SSID: "NetworkC"},
	}

	c, found := findConnectionBySSID(connections, "NetworkB")
	if !found {
		t.Fatal("findConnectionBySSID() did not find existing network")
	}
	if c.SSID != "NetworkB" {
		t.Errorf("findConnectionBySSID() returned wrong network: got %q, want %q", c.SSID, "NetworkB")
	}

	_, found = findConnectionBySSID(connections, "NotThere")
	if found {
		t.Error("findConnectionBySSID() returned true for missing network")
	}

	_, found = findConnectionBySSID(nil, "NetworkA")
	if found {
		t.Error("findConnectionBySSID() returned true for empty slice")
	}
}

type recordingBackend struct {
	*mock.MockBackend
	setDeviceCalls []string
}

func (r *recordingBackend) SetDevice(name string) error {
	r.setDeviceCalls = append(r.setDeviceCalls, name)
	return r.MockBackend.SetDevice(name)
}

func TestRunDevices(t *testing.T) {
	baseBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	mb := baseBackend.(*mock.MockBackend)
	mb.ActionSleep = 0

	var buf bytes.Buffer
	if err := runDevices(&buf, false, mb); err != nil {
		t.Fatalf("runDevices() failed: %v", err)
	}
	output := strings.TrimSpace(buf.String())
	if !strings.Contains(output, "wlan0") {
		t.Fatalf("expected devices output to contain wlan0, got %q", output)
	}

	buf.Reset()
	if err := runDevices(&buf, true, mb); err != nil {
		t.Fatalf("runDevices() json failed: %v", err)
	}
	var devices []wifi.Device
	if err := json.Unmarshal(buf.Bytes(), &devices); err != nil {
		t.Fatalf("invalid devices json: %v", err)
	}
	if len(devices) == 0 {
		t.Fatal("expected at least one device")
	}
}

func TestConfigureBackendDeviceCalled(t *testing.T) {
	baseBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	rec := &recordingBackend{MockBackend: baseBackend.(*mock.MockBackend)}
	rec.ActionSleep = 0

	originalBackend := b
	originalOpts := opts
	defer func() {
		b = originalBackend
		opts = originalOpts
	}()

	b = rec
	opts.Device = "wlan1"

	cmd := ListCommand{}
	var args []string
	if err := cmd.Execute(args); err != nil {
		t.Fatalf("ListCommand.Execute() failed: %v", err)
	}
	if len(rec.setDeviceCalls) != 1 || rec.setDeviceCalls[0] != "wlan1" {
		t.Fatalf("expected SetDevice to be called with wlan1, got %#v", rec.setDeviceCalls)
	}
}

func TestParseCLIWithDeviceAndWait(t *testing.T) {
	baseBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	mb := baseBackend.(*mock.MockBackend)
	mb.ActionSleep = 0
	originalBackend := b
	defer func() { b = originalBackend }()
	b = mb

	var localOpts Options
	parser := flags.NewParser(&localOpts, flags.HelpFlag)
	_, err = parser.ParseArgs([]string{"--device", "wlan1", "devices", "--json"})
	if err != nil {
		t.Fatalf("parse args failed: %v", err)
	}
	if localOpts.Device != "wlan1" {
		t.Fatalf("expected --device parsed, got %q", localOpts.Device)
	}
	if !localOpts.Devices.JSON {
		t.Fatalf("expected devices --json to be parsed")
	}
}

func TestRunConnectWaitsForVisibleSSID(t *testing.T) {
	baseBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	mb := baseBackend.(*mock.MockBackend)
	mb.ActionSleep = 0
	mb.VisibleConnections = nil

	attempts := 0
	mb.JoinError = nil

	origBuild := mb.VisibleConnections
	_ = origBuild

	backend := &waitVisibleBackend{MockBackend: mb, appearAfter: 2, ssid: "appears-later"}
	var buf bytes.Buffer
	err = runConnect(&buf, "appears-later", "pw", wifi.SecurityWPA, false, RetryConfig{Total: 3 * time.Second, Interval: 10 * time.Millisecond, RequireVisible: true}, backend)
	if err != nil {
		t.Fatalf("runConnect should eventually succeed, got %v", err)
	}
	if backend.calls < 2 {
		t.Fatalf("expected multiple scan attempts, got %d", backend.calls)
	}
	if !strings.Contains(buf.String(), "not visible yet") {
		t.Fatalf("expected visibility retry message, got %q", buf.String())
	}
	_ = attempts
}

type waitVisibleBackend struct {
	*mock.MockBackend
	calls       int
	appearAfter int
	ssid        string
}

func (w *waitVisibleBackend) BuildNetworkList(shouldScan bool) ([]wifi.Connection, error) {
	w.calls++
	if shouldScan && w.calls >= w.appearAfter {
		found := false
		for _, c := range w.MockBackend.VisibleConnections {
			if c.SSID == w.ssid {
				found = true
			}
		}
		if !found {
			w.MockBackend.VisibleConnections = append(w.MockBackend.VisibleConnections, wifi.Connection{SSID: w.ssid, IsVisible: true, Security: wifi.SecurityWPA})
		}
	}
	return w.MockBackend.BuildNetworkList(shouldScan)
}
