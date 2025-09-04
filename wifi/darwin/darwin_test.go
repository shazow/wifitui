package darwin

import "testing"

func TestFindWifiDevice(t *testing.T) {
	mockedOutput := `Hardware Port: Wi-Fi
Device: en0
Ethernet Address: a1:b2:c3:d4:e5:f6

Hardware Port: Bluetooth PAN
Device: en8
Ethernet Address: a1:b2:c3:d4:e5:f7

Hardware Port: Thunderbolt Bridge
Device: bridge0
Ethernet Address: a1:b2:c3:d4:e5:f8`

	device, err := findWifiDevice(mockedOutput)
	if err != nil {
		t.Fatalf("findWifiDevice returned an error: %v", err)
	}
	if device != "en0" {
		t.Fatalf(`findWifiDevice returned "%s", want "en0"`, device)
	}
}
