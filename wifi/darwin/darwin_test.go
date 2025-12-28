package darwin

import (
	"testing"

	"github.com/shazow/wifitui/wifi"
)

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
		{-50, 100},  // Strong signal
		{-70, 60},   // Medium signal
		{-90, 20},   // Weak signal
		{-100, 0},   // Minimum
		{-110, 0},   // Below minimum
		{0, 0},      // Invalid
		{10, 0},     // Invalid positive
	}

	for _, tt := range tests {
		result := rssiToStrength(tt.rssi)
		if result != tt.expected {
			t.Errorf("rssiToStrength(%d) = %d, want %d", tt.rssi, result, tt.expected)
		}
	}
}
