package wifi

import "testing"

func TestNetworksResultCarriesConnectionsAndCachedState(t *testing.T) {
	result := NetworksResult{
		Connections: []Connection{{SSID: "Cafe"}},
		IsCached:    true,
	}

	if len(result.Connections) != 1 || result.Connections[0].SSID != "Cafe" {
		t.Fatalf("NetworksResult.Connections = %#v, want Cafe connection", result.Connections)
	}
	if !result.IsCached {
		t.Fatal("NetworksResult.IsCached = false, want true")
	}
}

func TestScanModeValues(t *testing.T) {
	if ScanNever == ScanAuto || ScanNever == ScanForce || ScanAuto == ScanForce {
		t.Fatalf("ScanMode values must be distinct: never=%d auto=%d force=%d", ScanNever, ScanAuto, ScanForce)
	}
}
