package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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

func TestRunListShowsScanWarningWithCachedResults(t *testing.T) {
	mockBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	var buf bytes.Buffer

	backend := cachedBackend{
		Backend: mockBackend,
	}
	if err := runList(&buf, false, false, true, backend); err != nil {
		t.Fatalf("runList() failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Unencrypted_Honeypot") {
		t.Errorf("runList() output missing cached network. got=%q", output)
	}
	if !strings.Contains(output, "Warning: scan failed; showing cached results") {
		t.Errorf("runList() output missing scan warning. got=%q", output)
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

type cachedBackend struct {
	wifi.Backend
}

func (b cachedBackend) ListNetworks(scan wifi.ScanMode) (wifi.NetworksResult, error) {
	result, err := b.Backend.ListNetworks(scan)
	result.IsCached = true
	return result, err
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
	result, err := mockBackend.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("failed to get network list: %v", err)
	}
	networks := result.Connections
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
	result, err = mockBackend.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("failed to get network list: %v", err)
	}
	networks = result.Connections
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

func (f *flakyBackend) ListNetworks(scan wifi.ScanMode) (wifi.NetworksResult, error) {
	if scan != wifi.ScanNever {
		f.scannedSSIDs = append(f.scannedSSIDs, "any")
	}
	return f.MockBackend.ListNetworks(scan)
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
	retryInterval := longDuration
	retryTotal := retryInterval * 3

	// We set ActionSleep to 0 to make the mock actions fast, only our loop sleep matters.
	fb.MockBackend.ActionSleep = 0

	start := time.Now()
	// Using passphrase triggers JoinNetwork which we overrode
	if err := runConnect(&buf, "retry-network", "password", wifi.SecurityWPA, false, RetryConfig{Total: retryTotal, Interval: retryInterval}, fb); err != nil {
		t.Fatalf("runConnect() with retry failed: %v", err)
	}
	duration := time.Since(start)

	if duration < retryInterval {
		t.Fatalf("expected at least one retry sleep (%v), got %v", retryInterval, duration)
	}
	if duration > retryInterval*4 {
		t.Fatalf("retry path took unexpectedly long: %v", duration)
	}

	// Check if scan was performed
	if len(fb.scannedSSIDs) == 0 {
		t.Errorf("expected at least one scan attempt, got 0")
	}

	// Check output for retry messages
	output := buf.String()
	// We expect 2 failures:
	// 1. "Quick connect failed..."
	// 2. "Connection failed: ... Retrying in <interval>..."
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
	retryTotal := longDuration * 3
	retryInterval := longDuration

	// We set ActionSleep to 0 to make the mock actions fast.
	fb.MockBackend.ActionSleep = 0

	start := time.Now()
	// Using passphrase triggers JoinNetwork which we overrode
	if err := runConnect(&buf, "retry-network", "password", wifi.SecurityWPA, false, RetryConfig{Total: retryTotal, Interval: retryInterval}, fb); err != nil {
		t.Fatalf("runConnect() with fast retry failed: %v", err)
	}
	duration := time.Since(start)

	if duration > retryInterval {
		t.Errorf("expected fast retry path to avoid sleeping, took %v", duration)
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
	retryInterval := shortDuration
	retryTotal := retryInterval * 3

	fb.MockBackend.ActionSleep = 0

	start := time.Now()
	if err := runConnect(&buf, "retry-network", "password", wifi.SecurityWPA, false, RetryConfig{Total: retryTotal, Interval: retryInterval}, fb); err != nil {
		t.Fatalf("runConnect() failed: %v", err)
	}
	duration := time.Since(start)

	if duration < retryInterval {
		t.Fatalf("expected at least one sleep of %v, got %v", retryInterval, duration)
	}
	if duration > retryInterval*4 {
		t.Fatalf("custom retry interval path took unexpectedly long: %v", duration)
	}

	output := buf.String()
	if !strings.Contains(output, fmt.Sprintf("Retrying in %v", retryInterval)) {
		t.Errorf("expected retry message with configured interval, got:\n%s", output)
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
