package helpers

import (
	"errors"
	"fmt"
	"testing"

	"github.com/shazow/wifitui/wifi"
)

func TestFormatScanFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "unavailable device",
			err: &wifi.ScanFailure{
				Backend: "NetworkManager",
				Stage:   wifi.ScanStageRequest,
				Device:  "wlan0",
				Cause:   fmt.Errorf("%w: rejected", wifi.ErrScanDeviceUnavailable),
			},
			want: "wlan0 is unavailable",
		},
		{
			name: "permission denied through additional wrapping",
			err:  fmt.Errorf("refresh failed: %w", fmt.Errorf("%w: rejected", wifi.ErrScanPermissionDenied)),
			want: "permission denied by the network service",
		},
		{
			name: "unknown failure retains context",
			err: &wifi.ScanFailure{
				Backend: "iwd",
				Stage:   wifi.ScanStageRequest,
				Cause:   errors.New("station rejected request"),
			},
			want: "iwd request: station rejected request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatScanFailure(tt.err); got != tt.want {
				t.Fatalf("FormatScanFailure() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatScanDiagnostic(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		cached bool
		want   string
	}{
		{
			name: "detailed failure takes precedence",
			err:  errors.New("scan not allowed"),
			want: "Scan failed: scan not allowed",
		},
		{
			name:   "legacy cached result",
			cached: true,
			want:   "Scan failed: showing cached results; backend did not provide a failure reason",
		},
		{name: "successful result"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatScanDiagnostic(tt.err, tt.cached); got != tt.want {
				t.Fatalf("FormatScanDiagnostic() = %q, want %q", got, tt.want)
			}
		})
	}
}
