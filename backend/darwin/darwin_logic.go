package darwin

import (
	"fmt"
	"strings"

	"github.com/shazow/wifitui/backend"
)

// findWifiDevice parses the output of `networksetup -listallhardwareports` to find the Wi-Fi device.
func findWifiDevice(output string) (string, error) {
	// The output is a series of stanzas, separated by blank lines.
	// Each stanza describes a hardware port.
	stanzas := strings.Split(output, "\n\n")
	for _, stanza := range stanzas {
		var hardwarePort, device string
		isWifiPort := false
		lines := strings.Split(stanza, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Hardware Port: ") {
				hardwarePort = strings.TrimPrefix(line, "Hardware Port: ")
				if strings.Contains(hardwarePort, "Wi-Fi") || strings.Contains(hardwarePort, "AirPort") {
					isWifiPort = true
				}
			}
			if strings.HasPrefix(line, "Device: ") {
				device = strings.TrimPrefix(line, "Device: ")
			}
		}
		if isWifiPort && device != "" {
			return device, nil
		}
	}
	return "", fmt.Errorf("no Wi-Fi interface found: %w", backend.ErrNotFound)
}
