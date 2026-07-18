//go:build darwin && !cgo

package darwin

import (
	"fmt"

	"github.com/shazow/wifitui/wifi"
)

func scanVisibleNetworks(string) ([]scannedNetwork, error) {
	return nil, fmt.Errorf("CoreWLAN scanning requires cgo: %w", wifi.ErrNotSupported)
}
