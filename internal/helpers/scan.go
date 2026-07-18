package helpers

import (
	"errors"
	"fmt"

	"github.com/shazow/wifitui/wifi"
)

// FormatScanFailure returns a concise user-facing explanation while preserving
// the complete wrapped error for callers that need diagnostic details.
func FormatScanFailure(err error) string {
	if err == nil {
		return ""
	}

	var failure *wifi.ScanFailure
	_ = errors.As(err, &failure)

	switch {
	case errors.Is(err, wifi.ErrScanDeviceUnavailable):
		if failure != nil && failure.Device != "" {
			return fmt.Sprintf("%s is unavailable", failure.Device)
		}
		return "Wi-Fi device is unavailable"
	case errors.Is(err, wifi.ErrScanPermissionDenied):
		return "permission denied by the network service"
	case errors.Is(err, wifi.ErrScanAuthRequired):
		return "authorization required by the network service"
	default:
		return err.Error()
	}
}
