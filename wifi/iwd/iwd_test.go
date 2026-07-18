//go:build linux

package iwd

import (
	"errors"
	"fmt"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestDBusErrorNameThroughWrapping(t *testing.T) {
	rawErr := dbus.Error{Name: "net.connman.iwd.Error.Busy", Body: []any{"busy"}}
	err := fmt.Errorf("request scan: %w", rawErr)

	if got := dbusErrorName(err); got != rawErr.Name {
		t.Fatalf("dbusErrorName() = %q, want %q", got, rawErr.Name)
	}
}

func TestNewScanFailurePreservesDBusError(t *testing.T) {
	rawErr := dbus.Error{Name: "net.connman.iwd.Error.Busy", Body: []any{"busy"}}
	err := newScanFailure(rawErr)

	if err.Backend != "iwd" || err.Stage != "request" || err.Code != rawErr.Name {
		t.Fatalf("newScanFailure() = %#v", err)
	}
	var gotDBusError dbus.Error
	if !errors.As(err, &gotDBusError) {
		t.Fatalf("newScanFailure() did not retain D-Bus error: %v", err)
	}
}
