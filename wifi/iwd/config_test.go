//go:build linux

package iwd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNetworkFileName(t *testing.T) {
	tests := []struct {
		ssid, networkType, want string
	}{
		{"Home_Network", "psk", "Home_Network.psk"},
		{"Cafe Free-WiFi", "open", "Cafe Free-WiFi.open"},
		{"Bob's", "psk", "=426f622773.psk"},
		{"café", "8021x", "=636166c3a9.8021x"},
	}
	for _, tt := range tests {
		if got := networkFileName(tt.ssid, tt.networkType); got != tt.want {
			t.Errorf("networkFileName(%q, %q) = %q, want %q", tt.ssid, tt.networkType, got, tt.want)
		}
	}
}

func TestSetKeyfileValue(t *testing.T) {
	tests := []struct {
		name  string
		data  string
		value string
		want  string
	}{
		{
			name:  "add group to empty file",
			data:  "",
			value: "true",
			want:  "[Settings]\nAlwaysRandomizeAddress=true\n",
		},
		{
			name:  "add group after other groups",
			data:  "[Security]\nPassphrase=secret\n",
			value: "true",
			want:  "[Security]\nPassphrase=secret\n\n[Settings]\nAlwaysRandomizeAddress=true\n",
		},
		{
			name:  "add key to existing group",
			data:  "[Settings]\nAutoConnect=false\n\n[Security]\nPassphrase=secret\n",
			value: "true",
			want:  "[Settings]\nAlwaysRandomizeAddress=true\nAutoConnect=false\n\n[Security]\nPassphrase=secret\n",
		},
		{
			name:  "replace existing key",
			data:  "[Settings]\nAlwaysRandomizeAddress = false\n",
			value: "true",
			want:  "[Settings]\nAlwaysRandomizeAddress=true\n",
		},
		{
			name:  "remove key",
			data:  "[Security]\nPassphrase=secret\n[Settings]\nAlwaysRandomizeAddress=true\nHidden=true\n",
			value: "",
			want:  "[Security]\nPassphrase=secret\n[Settings]\nHidden=true\n",
		},
		{
			name:  "remove missing key",
			data:  "# comment\n[Settings]\nHidden=true\n",
			value: "",
			want:  "# comment\n[Settings]\nHidden=true\n",
		},
		{
			name:  "same key in another group is untouched",
			data:  "[Other]\nAlwaysRandomizeAddress=false\n",
			value: "true",
			want:  "[Other]\nAlwaysRandomizeAddress=false\n\n[Settings]\nAlwaysRandomizeAddress=true\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(setKeyfileValue([]byte(tt.data), settingsGroup, alwaysRandomizeAddressKey, tt.value))
			if got != tt.want {
				t.Fatalf("setKeyfileValue() =\n%q\nwant\n%q", got, tt.want)
			}
			if tt.value != "" && !keyfileBool([]byte(got), settingsGroup, alwaysRandomizeAddressKey) {
				t.Fatalf("keyfileBool() = false after setting %q", tt.value)
			}
		})
	}
}

func TestEscapeKeyfileValue(t *testing.T) {
	if got, want := escapeKeyfileValue(" pass\\word\n\t"), `\spass\\word\n\t`; got != want {
		t.Fatalf("escapeKeyfileValue() = %q, want %q", got, want)
	}
}

func TestPerNetworkAddressRandomization(t *testing.T) {
	tests := []struct {
		name     string
		mainConf *string
		want     bool
	}{
		{"missing main.conf", nil, false},
		{"not set", ptr("[General]\nEnableNetworkConfiguration=true\n"), false},
		{"once", ptr("[General]\nAddressRandomization=once\n"), false},
		{"network", ptr("[General]\nAddressRandomization=network\n"), true},
		{"network in another group", ptr("[Scan]\nAddressRandomization=network\n"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.mainConf != nil {
				if err := os.WriteFile(filepath.Join(dir, "main.conf"), []byte(*tt.mainConf), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := PerNetworkAddressRandomization(dir)
			if err != nil {
				t.Fatalf("PerNetworkAddressRandomization() returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("PerNetworkAddressRandomization() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Home_Network.psk")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeFileAtomic(path, []byte("new")); err != nil {
		t.Fatalf("writeFileAtomic() returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil || string(data) != "new" {
		t.Fatalf("file contains %q (err %v), want %q", data, err, "new")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file permissions = %v, want 0600", perm)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("directory has %d entries, want only the written file", len(entries))
	}
}

func ptr(s string) *string {
	return &s
}
