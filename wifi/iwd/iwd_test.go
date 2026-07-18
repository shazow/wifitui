//go:build linux

package iwd

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/shazow/wifitui/wifi"
)

const testStationPath dbus.ObjectPath = "/net/connman/iwd/0/1"

func TestDBusErrorNameThroughWrapping(t *testing.T) {
	rawErr := dbus.Error{Name: "net.connman.iwd.Error.Busy", Body: []any{"busy"}}
	err := fmt.Errorf("request scan: %w", rawErr)

	if got := dbusErrorName(err); got != rawErr.Name {
		t.Fatalf("dbusErrorName() = %q, want %q", got, rawErr.Name)
	}
}

func TestNewScanFailurePreservesDBusError(t *testing.T) {
	rawErr := dbus.Error{Name: "net.connman.iwd.Error.Busy", Body: []any{"busy"}}
	err := newScanFailure(wifi.ScanStageRequest, rawErr)

	if err.Backend != "iwd" || err.Stage != wifi.ScanStageRequest || err.Code != rawErr.Name {
		t.Fatalf("newScanFailure() = %#v", err)
	}
	var gotDBusError dbus.Error
	if !errors.As(err, &gotDBusError) {
		t.Fatalf("newScanFailure() did not retain D-Bus error: %v", err)
	}
}

func TestWaitForScanCompletion(t *testing.T) {
	signals := make(chan *dbus.Signal, 4)
	signals <- scanningSignal(false)
	signals <- &dbus.Signal{
		Name: dbusPropertiesIface + ".PropertiesChanged",
		Path: testStationPath,
		Body: []any{iwdStationIface, map[string]dbus.Variant{"State": dbus.MakeVariant("connected")}},
	}
	signals <- scanningSignal(true)
	signals <- scanningSignal(false)

	if err := waitForScanCompletion(signals, testStationPath, time.Second); err != nil {
		t.Fatalf("waitForScanCompletion() = %v", err)
	}
}

func TestWaitForScanCompletionTimesOut(t *testing.T) {
	err := waitForScanCompletion(make(chan *dbus.Signal), testStationPath, 0)

	if !errors.Is(err, wifi.ErrScanTimeout) {
		t.Fatalf("waitForScanCompletion() = %v, want ErrScanTimeout", err)
	}
	var failure *wifi.ScanFailure
	if !errors.As(err, &failure) || failure.Stage != wifi.ScanStageCompletion {
		t.Fatalf("waitForScanCompletion() = %#v, want completion-stage ScanFailure", err)
	}
}

func TestWaitForScanCompletionRejectsMalformedScanning(t *testing.T) {
	signals := make(chan *dbus.Signal, 1)
	signals <- &dbus.Signal{
		Name: dbusPropertiesIface + ".PropertiesChanged",
		Path: testStationPath,
		Body: []any{iwdStationIface, map[string]dbus.Variant{"Scanning": dbus.MakeVariant("yes")}},
	}

	err := waitForScanCompletion(signals, testStationPath, time.Second)
	if !errors.Is(err, wifi.ErrScanProtocol) {
		t.Fatalf("waitForScanCompletion() = %v, want ErrScanProtocol", err)
	}
	var failure *wifi.ScanFailure
	if !errors.As(err, &failure) || failure.Stage != wifi.ScanStageCompletion {
		t.Fatalf("waitForScanCompletion() = %#v, want completion-stage ScanFailure", err)
	}
}

func scanningSignal(scanning bool) *dbus.Signal {
	return &dbus.Signal{
		Name: dbusPropertiesIface + ".PropertiesChanged",
		Path: testStationPath,
		Body: []any{iwdStationIface, map[string]dbus.Variant{"Scanning": dbus.MakeVariant(scanning)}},
	}
}
