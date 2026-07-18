package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	if err := runList(&buf, io.Discard, false, true, false, mockBackend); err != nil {
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
	if err := runList(&buf, io.Discard, false, false, false, mockBackend); err != nil {
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
	var errBuf bytes.Buffer

	backend := cachedBackend{
		Backend: mockBackend,
	}
	if err := runList(&buf, &errBuf, false, false, true, backend); err != nil {
		t.Fatalf("runList() failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Unencrypted_Honeypot") {
		t.Errorf("runList() output missing cached network. got=%q", output)
	}
	if strings.Contains(output, "Scan failed:") {
		t.Errorf("runList() mixed scan warning into stdout. got=%q", output)
	}
	if !strings.Contains(errBuf.String(), "Scan failed: scan not allowed") {
		t.Errorf("runList() stderr missing scan warning. got=%q", errBuf.String())
	}
}

func TestRunListShowsLegacyCachedWarning(t *testing.T) {
	mockBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	var diagnostics bytes.Buffer

	if err := runList(io.Discard, &diagnostics, false, false, true, legacyCachedBackend{Backend: mockBackend}); err != nil {
		t.Fatalf("runList() failed: %v", err)
	}
	if want := "Scan failed: showing cached results; backend did not provide a failure reason"; !strings.Contains(diagnostics.String(), want) {
		t.Fatalf("runList() stderr = %q, want %q", diagnostics.String(), want)
	}
}

func TestRunListReturnsScanWarningWriteError(t *testing.T) {
	mockBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	wantErr := errors.New("write failed")
	backend := cachedBackend{Backend: mockBackend}

	err = runList(io.Discard, errorWriter{err: wantErr}, false, false, true, backend)
	if !errors.Is(err, wantErr) {
		t.Fatalf("runList() error = %v, want an error wrapping %v", err, wantErr)
	}
}

func TestRunListScanForcesRefresh(t *testing.T) {
	mockBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	backend := scanRecordingBackend{
		Backend: mockBackend,
	}

	var buf bytes.Buffer
	if err := runList(&buf, io.Discard, false, false, true, &backend); err != nil {
		t.Fatalf("runList() failed: %v", err)
	}
	if len(backend.listScans) != 1 || backend.listScans[0] != wifi.ScanForce {
		t.Fatalf("runList(scan=true) used scans %#v, want only ScanForce", backend.listScans)
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

func TestRunShowDoesNotRequestScan(t *testing.T) {
	mockBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	backend := scanRecordingBackend{
		Backend: mockBackend,
	}

	var buf bytes.Buffer
	if err := runShow(&buf, false, "Password is password", &backend); err != nil {
		t.Fatalf("runShow() failed: %v", err)
	}
	if len(backend.listScans) != 1 || backend.listScans[0] != wifi.ScanNever {
		t.Fatalf("runShow() used scans %#v, want only ScanNever", backend.listScans)
	}
}

type cachedBackend struct {
	wifi.Backend
}

func (b cachedBackend) ListNetworks(scan wifi.ScanMode) (wifi.NetworksResult, error) {
	result, err := b.Backend.ListNetworks(scan)
	result.IsCached = true
	result.ScanError = errors.New("scan not allowed")
	return result, err
}

type legacyCachedBackend struct {
	wifi.Backend
}

func (b legacyCachedBackend) ListNetworks(scan wifi.ScanMode) (wifi.NetworksResult, error) {
	result, err := b.Backend.ListNetworks(scan)
	result.IsCached = true
	return result, err
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}

type scanRecordingBackend struct {
	wifi.Backend
	listScans []wifi.ScanMode
}

type scanAndConnectFailureBackend struct {
	wifi.Backend
	scanErr       error
	activationErr error
}

func (b scanAndConnectFailureBackend) ListNetworks(scan wifi.ScanMode) (wifi.NetworksResult, error) {
	result, err := b.Backend.ListNetworks(scan)
	result.ScanError = b.scanErr
	return result, err
}

func (b scanAndConnectFailureBackend) ActivateNetwork(string) error {
	return b.activationErr
}

func TestAttemptConnectIncludesScanFailureWhenActivationFails(t *testing.T) {
	mockBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	scanErr := &wifi.ScanFailure{
		Backend: "NetworkManager",
		Stage:   wifi.ScanStageRequest,
		Cause:   errors.New("scan rejected"),
	}
	activationErr := errors.New("activation rejected")
	backend := scanAndConnectFailureBackend{
		Backend:       mockBackend,
		scanErr:       scanErr,
		activationErr: activationErr,
	}

	err = attemptConnect("Cafe", "", wifi.SecurityWPA, false, true, backend)
	if !errors.Is(err, activationErr) {
		t.Fatalf("attemptConnect() error %v does not retain activation failure", err)
	}
	var gotScanFailure *wifi.ScanFailure
	if !errors.As(err, &gotScanFailure) {
		t.Fatalf("attemptConnect() error %v does not retain scan failure", err)
	}
}

func (b *scanRecordingBackend) ListNetworks(scan wifi.ScanMode) (wifi.NetworksResult, error) {
	b.listScans = append(b.listScans, scan)
	return b.Backend.ListNetworks(scan)
}

func TestRunListJSON(t *testing.T) {
	mockBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	var buf bytes.Buffer

	if err := runList(&buf, io.Discard, true, true, false, mockBackend); err != nil {
		t.Fatalf("runList() failed: %v", err)
	}

	var networks []wifi.Network
	if err := json.Unmarshal(buf.Bytes(), &networks); err != nil {
		t.Fatalf("runList() output is not valid JSON: %v. got=%q", err, buf.String())
	}

	if len(networks) == 0 {
		t.Fatalf("runList() output is empty")
	}

	// Just check for one of the SSIDs
	found := false
	for _, c := range networks {
		if c.SSID == "HideYoKidsHideYoWiFi" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("runList() JSON output missing expected network. got=%q", buf.String())
	}
}

func TestRunListJSONReportsScanFailureOnStderr(t *testing.T) {
	mockBackend, err := mock.New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}
	var output bytes.Buffer
	var diagnostics bytes.Buffer

	err = runList(&output, &diagnostics, true, true, true, cachedBackend{Backend: mockBackend})
	if err != nil {
		t.Fatalf("runList() failed: %v", err)
	}

	var networks []wifi.Network
	if err := json.Unmarshal(output.Bytes(), &networks); err != nil {
		t.Fatalf("runList() output is not valid JSON: %v. got=%q", err, output.String())
	}
	if !strings.Contains(diagnostics.String(), "Scan failed: scan not allowed") {
		t.Fatalf("runList() stderr missing scan failure. got=%q", diagnostics.String())
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

	type networkWithSecret struct {
		wifi.Network
		Passphrase string `json:"passphrase,omitempty"`
	}

	var connWithSecretData networkWithSecret
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
	connWithSecretData = networkWithSecret{}
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
	networks := result.Networks
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
	networks = result.Networks
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

func TestFilterVisibleNetworks(t *testing.T) {
	networks := []wifi.Network{
		{SSID: "visible1", IsVisible: true},
		{SSID: "hidden1", IsVisible: false},
		{SSID: "visible2", IsVisible: true},
		{SSID: "hidden2", IsVisible: false},
	}

	visible := filterVisibleNetworks(networks)
	if len(visible) != 2 {
		t.Fatalf("filterVisibleNetworks() returned %d networks, want 2", len(visible))
	}
	for _, c := range visible {
		if !c.IsVisible {
			t.Errorf("filterVisibleNetworks() returned non-visible network %q", c.SSID)
		}
	}

	// Empty input
	if got := filterVisibleNetworks(nil); got != nil {
		t.Errorf("filterVisibleNetworks(nil) = %v, want nil", got)
	}
}

func TestFindNetworkBySSID(t *testing.T) {
	networks := []wifi.Network{
		{SSID: "NetworkA"},
		{SSID: "NetworkB"},
		{SSID: "NetworkC"},
	}

	c, found := findNetworkBySSID(networks, "NetworkB")
	if !found {
		t.Fatal("findNetworkBySSID() did not find existing network")
	}
	if c.SSID != "NetworkB" {
		t.Errorf("findNetworkBySSID() returned wrong network: got %q, want %q", c.SSID, "NetworkB")
	}

	_, found = findNetworkBySSID(networks, "NotThere")
	if found {
		t.Error("findNetworkBySSID() returned true for missing network")
	}

	_, found = findNetworkBySSID(nil, "NetworkA")
	if found {
		t.Error("findNetworkBySSID() returned true for empty slice")
	}
}
