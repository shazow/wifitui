package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

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
	if err := runConnect(&buf, "new-network", "new-password", wifi.SecurityWPA, false, mockBackend); err != nil {
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
	if err := runConnect(&buf, "Password is password", "", wifi.SecurityWPA, false, mockBackend); err != nil {
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

func init() {
	mock.DefaultActionSleep = 0
}
