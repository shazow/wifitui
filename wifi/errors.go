package wifi

import (
	"errors"
	"fmt"
)

// ErrIncorrectPassphrase is returned when a connection fails due to an
// incorrect passphrase.
var ErrIncorrectPassphrase = errors.New("incorrect passphrase")

// ErrWirelessDisabled is returned when the wireless radio is disabled.
var ErrWirelessDisabled = errors.New("wireless disabled")

// ErrNotFound is returned when a network is not found.
var ErrNotFound = errors.New("not found")

// ErrNotAvailable is returned when a backend is not available.
var ErrNotAvailable = errors.New("not available")

// ErrOperationFailed is returned when an operation fails.
var ErrOperationFailed = errors.New("operation failed")

// ErrNotSupported is returned when a feature is not supported.
var ErrNotSupported = errors.New("not supported")

// ErrAccessPointMismatch is returned when trying to merge connections with different SSID or security.
var ErrAccessPointMismatch = errors.New("SSID or security mismatch")

// ErrMissingPermission is returned when the user lacks necessary permissions.
var ErrMissingPermission = errors.New("missing permission")

// Scan failure classifications. Backends wrap these errors together with the
// original platform error so callers can use errors.Is without losing the
// underlying diagnostic information.
var (
	ErrScanDeviceUnavailable = errors.New("wireless device unavailable")
	ErrScanPermissionDenied  = errors.New("Wi-Fi scan permission denied")
	ErrScanAuthRequired      = errors.New("Wi-Fi scan authorization required")
	ErrScanTimeout           = errors.New("scan timed out")
	ErrScanProtocol          = errors.New("invalid scan response")
)

// ScanStage identifies the part of a scan operation that failed.
type ScanStage string

const (
	ScanStageSetup      ScanStage = "setup"
	ScanStageRequest    ScanStage = "request"
	ScanStageCompletion ScanStage = "completion"
)

// ScanFailure adds backend and operation context to a non-fatal scan error.
// Cause retains the original platform error and any wrapped classification.
type ScanFailure struct {
	Backend string
	Stage   ScanStage
	Device  string
	Code    string
	Cause   error
}

func (e *ScanFailure) Error() string {
	operation := e.Backend
	if e.Stage != "" {
		if operation != "" {
			operation += " "
		}
		operation += string(e.Stage)
	}
	if e.Device != "" {
		if operation != "" {
			operation += " "
		}
		operation += fmt.Sprintf("on %s", e.Device)
	}

	if e.Cause == nil {
		return operation
	}
	if operation == "" {
		return e.Cause.Error()
	}
	return fmt.Sprintf("%s: %v", operation, e.Cause)
}

// Unwrap exposes the classified platform error to errors.Is and errors.As.
func (e *ScanFailure) Unwrap() error {
	return e.Cause
}
