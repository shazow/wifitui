package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shazow/wifitui/backend/mock"
)

func TestRunList(t *testing.T) {
	mockBackend := mock.NewBackend()
	var buf bytes.Buffer

	err := runList(&buf, false, mockBackend)
	if err != nil {
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

func TestRunShow(t *testing.T) {
	mockBackend := mock.NewBackend()
	var buf bytes.Buffer

	// Test case: network found and known
	err := runShow(&buf, false, "Password is password", mockBackend)
	if err != nil {
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
	err = runShow(&buf, false, "GET off my LAN", mockBackend)
	if err != nil {
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
	err = runShow(&buf, false, "NotFound", mockBackend)
	if err == nil {
		t.Fatalf("runShow() with not found network should have failed, but did not")
	}
	if !strings.Contains(err.Error(), "network not found: NotFound") {
		t.Errorf("runShow() with not found network gave wrong error. got=%q", err)
	}
}
