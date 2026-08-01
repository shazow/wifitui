//go:build darwin && !cgo

package darwincorewlan

import (
	"fmt"

	"github.com/shazow/wifitui/wifi"
)

func scanVisibleNetworks(string) ([]scannedNetwork, error) {
	return nil, fmt.Errorf("CoreWLAN scanning requires cgo: %w", wifi.ErrNotSupported)
}
