package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shazow/wifitui/backend/mock"
)

func TestRunList(t *testing.T) {
	mockBackend := mock.New()
	var buf bytes.Buffer

	err := runList(&buf, false, mockBackend)
	if err != nil {
		t.Fatalf("runList() failed: %v", err)
	}

	output := buf.String()
	expectedLines := []string{
		"TestNet 1\t80%, visible",
		"TestNet 2\t50%, visible, secure",
		"TestNet 3\tknown",
		"TestNet 4\tknown",
		"VisibleOnly\t0%, visible, secure",
	}

	// Normalize the output to handle variations in line endings and extra spaces
	normalizedOutput := strings.TrimSpace(strings.ReplaceAll(output, "\r\n", "\n"))
	lines := strings.Split(normalizedOutput, "\n")

	if len(lines) != len(expectedLines) {
		t.Fatalf("runList() output has wrong number of lines. got=%d, want=%d\n---\n%s\n---", len(lines), len(expectedLines), output)
	}

	for i, expectedLine := range expectedLines {
		if lines[i] != expectedLine {
			t.Errorf("runList() output line %d wrong. got=%q, want=%q", i, lines[i], expectedLine)
		}
	}
}

func TestRunShow(t *testing.T) {
	mockBackend := mock.New()
	var buf bytes.Buffer

	// Test case: network found and known
	err := runShow(&buf, false, "TestNet 2", mockBackend)
	if err != nil {
		t.Fatalf("runShow() with found network failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "SSID: TestNet 2") {
		t.Errorf("runShow() output missing SSID. got=%q", output)
	}
	if !strings.Contains(output, "Passphrase: password123") {
		t.Errorf("runShow() output missing passphrase. got=%q", output)
	}

	// Test case: network found, but not known (no secret)
	buf.Reset()
	err = runShow(&buf, false, "VisibleOnly", mockBackend)
	if err != nil {
		t.Fatalf("runShow() with visible-only network failed: %v", err)
	}
	output = buf.String()
	if !strings.Contains(output, "SSID: VisibleOnly") {
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
