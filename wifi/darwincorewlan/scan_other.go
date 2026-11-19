//go:build !darwin

package darwincorewlan

import (
	"fmt"

	"github.com/shazow/wifitui/wifi"
)

func scanVisibleNetworks(string) ([]scannedNetwork, error) {
	return nil, fmt.Errorf("CoreWLAN scanning is only available on macOS: %w", wifi.ErrNotSupported)
}
