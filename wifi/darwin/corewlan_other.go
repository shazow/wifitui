//go:build !darwin

package darwin

import (
	"fmt"

	"github.com/shazow/wifitui/wifi"
)

func scanVisibleNetworks(string) ([]scannedNetwork, error) {
	return nil, fmt.Errorf("CoreWLAN scanning is only available on macOS: %w", wifi.ErrNotSupported)
}
