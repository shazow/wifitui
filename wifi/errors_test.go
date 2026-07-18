package wifi

import (
	"errors"
	"fmt"
	"testing"
)

type platformScanError struct {
	code string
}

func (e *platformScanError) Error() string { return "scan rejected" }

func TestScanFailurePreservesClassificationAndCause(t *testing.T) {
	platformErr := &platformScanError{code: "not-allowed"}
	cause := fmt.Errorf("%w: %w", ErrScanDeviceUnavailable, platformErr)
	err := &ScanFailure{
		Backend: "NetworkManager",
		Stage:   ScanStageRequest,
		Device:  "wlan0",
		Code:    platformErr.code,
		Cause:   cause,
	}

	if !errors.Is(err, ErrScanDeviceUnavailable) {
		t.Fatal("errors.Is did not find the scan classification")
	}

	var gotFailure *ScanFailure
	if !errors.As(err, &gotFailure) {
		t.Fatal("errors.As did not find ScanFailure")
	}
	if gotFailure.Code != platformErr.code {
		t.Fatalf("ScanFailure.Code = %q, want %q", gotFailure.Code, platformErr.code)
	}

	var gotPlatformErr *platformScanError
	if !errors.As(err, &gotPlatformErr) {
		t.Fatal("errors.As did not find the original platform error")
	}
	if gotPlatformErr != platformErr {
		t.Fatalf("errors.As returned platform error %p, want %p", gotPlatformErr, platformErr)
	}

	want := "NetworkManager request on wlan0: wireless device unavailable: scan rejected"
	if got := err.Error(); got != want {
		t.Fatalf("ScanFailure.Error() = %q, want %q", got, want)
	}
}
