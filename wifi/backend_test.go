package wifi

import "testing"

func TestNetworksResultCarriesNetworksAndCachedState(t *testing.T) {
	result := NetworksResult{
		Networks: []Network{{SSID: "Cafe"}},
		IsCached: true,
	}

	if len(result.Networks) != 1 || result.Networks[0].SSID != "Cafe" {
		t.Fatalf("NetworksResult.Networks = %#v, want Cafe network", result.Networks)
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
