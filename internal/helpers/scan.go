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

// FormatScanDiagnostic returns a complete user-facing scan diagnostic. The
// cached flag supports backends that implement the deprecated IsCached result
// field but cannot provide a failure cause.
func FormatScanDiagnostic(err error, cached bool) string {
	if err != nil {
		return "Scan failed: " + FormatScanFailure(err)
	}
	if cached {
		return "Scan failed: showing cached results; backend did not provide a failure reason"
	}
	return ""
}
