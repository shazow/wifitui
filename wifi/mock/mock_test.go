package mock

import (
	"testing"

	"github.com/shazow/wifitui/wifi"
)

// Helper to find a connection in a slice
func findConnection(connections []wifi.Network, ssid string) *wifi.Network {
	for i := range connections {
		if connections[i].SSID == ssid {
			return &connections[i]
		}
	}
	return nil
}

func TestNew(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if b == nil {
		t.Fatal("New() returned nil backend")
	}
	mock := b.(*MockBackend)
	if len(mock.KnownNetworks) == 0 {
		t.Fatal("New() returned no known networks")
	}
	if mock.ActiveNetworkIndex != -1 {
		t.Errorf("expected ActiveNetworkIndex to be -1, got %d", mock.ActiveNetworkIndex)
	}
}

func TestListNetworks(t *testing.T) {
	b, _ := New()
	mock := b.(*MockBackend)
	knownSSID := "Password is password"

	// Activate a connection to test IsActive flag
	err := mock.ActivateNetwork("HideYoKidsHideYoWiFi")
	if err != nil {
		t.Fatalf("ActivateNetwork failed: %v", err)
	}

	result, err := b.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("ListNetworks() failed: %v", err)
	}
	networks := result.Networks

	conn := findConnection(networks, knownSSID)
	if conn == nil {
		t.Fatalf("did not find known network %s in list", knownSSID)
	}
	if !conn.IsKnown {
		t.Errorf("expected network %s to be known, but it was not", knownSSID)
	}

	conn = findConnection(networks, "HideYoKidsHideYoWiFi")
	if conn == nil {
		t.Fatalf("did not find active network in list")
	}
	if !conn.IsActive {
		t.Errorf("expected network to be active, but it was not")
	}
}

func TestActivateNetwork(t *testing.T) {
	b, _ := New()
	mockBackend := b.(*MockBackend)
	ssid := "Password is password"

	err := b.ActivateNetwork(ssid)
	if err != nil {
		t.Fatalf("ActivateNetwork() failed: %v", err)
	}

	if mockBackend.ActiveNetworkIndex == -1 {
		t.Fatal("ActiveNetworkIndex was not set")
	}
	if mockBackend.KnownNetworks[mockBackend.ActiveNetworkIndex].SSID != ssid {
		t.Errorf("expected active network to be %s, but got %s", ssid, mockBackend.KnownNetworks[mockBackend.ActiveNetworkIndex].SSID)
	}

	err = b.ActivateNetwork("non-existent-network")
	if err == nil {
		t.Fatal("ActivateNetwork() with non-existent network should have failed, but did not")
	}
}

func TestForgetNetwork(t *testing.T) {
	b, _ := New()
	mockBackend := b.(*MockBackend)

	// --- Test index shifting ---
	// Activate a network that is not the first one.
	ssidToActivate := "Password is password"
	err := b.ActivateNetwork(ssidToActivate)
	if err != nil {
		t.Fatalf("ActivateNetwork() failed: %v", err)
	}

	initialActiveIndex := mockBackend.ActiveNetworkIndex
	if initialActiveIndex <= 0 {
		t.Fatalf("Test setup failed: expected active index > 0, got %d", initialActiveIndex)
	}

	// Forget a network that appears *before* the active one.
	ssidToForget := mockBackend.KnownNetworks[0].SSID
	if ssidToForget == ssidToActivate {
		t.Fatalf("Test setup failed: network to forget is the same as the active one")
	}

	err = b.ForgetNetwork(ssidToForget)
	if err != nil {
		t.Fatalf("ForgetNetwork() failed: %v", err)
	}

	// Check that the active index was shifted correctly.
	expectedIndex := initialActiveIndex - 1
	if mockBackend.ActiveNetworkIndex != expectedIndex {
		t.Errorf("active index should have shifted to %d, but got %d", expectedIndex, mockBackend.ActiveNetworkIndex)
	}
	if mockBackend.KnownNetworks[mockBackend.ActiveNetworkIndex].SSID != ssidToActivate {
		t.Errorf("active network SSID is incorrect after forgetting another network")
	}

	// --- Test forgetting the active network ---
	err = b.ForgetNetwork(ssidToActivate)
	if err != nil {
		t.Fatalf("ForgetNetwork() of active network failed: %v", err)
	}
	if mockBackend.ActiveNetworkIndex != -1 {
		t.Errorf("ActiveNetworkIndex should be -1 after forgetting active network, got %d", mockBackend.ActiveNetworkIndex)
	}
}

func TestJoinNetwork(t *testing.T) {
	b, _ := New()
	mockBackend := b.(*MockBackend)

	newSSID := "new-network"
	password := "password"
	err := b.JoinNetwork(newSSID, password, wifi.SecurityWPA, false)
	if err != nil {
		t.Fatalf("JoinNetwork() failed: %v", err)
	}

	lastIndex := len(mockBackend.KnownNetworks) - 1
	if mockBackend.KnownNetworks[lastIndex].SSID != newSSID {
		t.Fatalf("JoinNetwork() did not add the new network to known networks")
	}
	if mockBackend.ActiveNetworkIndex != lastIndex {
		t.Errorf("newly joined network should be active, expected index %d but got %d", lastIndex, mockBackend.ActiveNetworkIndex)
	}
}

func TestGetSecretsForDuplicateSSID(t *testing.T) {
	b, _ := New()
	ssid := "HideYoKidsHideYoWiFi" // This one has duplicates
	expectedSecret := "hidden"     // This is the secret of the first one in the list

	secret, err := b.GetSecrets(ssid)
	if err != nil {
		t.Fatalf("GetSecrets() failed: %v", err)
	}
	if secret != expectedSecret {
		t.Errorf("expected secret '%s' for first matching network, got '%s'", expectedSecret, secret)
	}
}

func TestUpdateNetworkForDuplicateSSID(t *testing.T) {
	b, _ := New()
	mockBackend := b.(*MockBackend)
	ssid := "HideYoKidsHideYoWiFi"
	newPassword := "new-password"

	opts := wifi.UpdateOptions{Password: &newPassword}
	err := b.UpdateNetwork(ssid, opts)
	if err != nil {
		t.Fatalf("UpdateNetwork() failed: %v", err)
	}

	// Verify that only the first entry was updated
	if mockBackend.KnownNetworks[0].SSID != ssid || mockBackend.KnownNetworks[0].Secret != newPassword {
		t.Errorf("first instance of duplicate SSID was not updated correctly")
	}
	if mockBackend.KnownNetworks[1].SSID == ssid && mockBackend.KnownNetworks[1].Secret == newPassword {
		t.Errorf("second instance of duplicate SSID should not have been updated")
	}
}

func TestGetSecretsForKnownNetworkWithoutSecret(t *testing.T) {
	b, _ := New()
	ssid := "Unencrypted_Honeypot"

	err := b.JoinNetwork(ssid, "", wifi.SecurityOpen, false)
	if err != nil {
		t.Fatalf("JoinNetwork() failed: %v", err)
	}

	secret, err := b.GetSecrets(ssid)
	if err != nil {
		t.Fatalf("GetSecrets() failed: %v", err)
	}
	if secret != "" {
		t.Errorf("expected empty secret, got '%s'", secret)
	}
}

func TestUpdateNetwork(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}

	ssid := "Password is password"
	autoConnect := false
	opts := wifi.UpdateOptions{AutoConnect: &autoConnect}
	err = b.UpdateNetwork(ssid, opts)
	if err != nil {
		t.Fatalf("failed to set autoconnect to false: %v", err)
	}

	result, err := b.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("failed to build network list: %v", err)
	}
	conns := result.Networks

	conn := findConnection(conns, ssid)
	if conn == nil {
		t.Fatalf("did not find network %s", ssid)
	}

	if conn.AutoConnect {
		t.Errorf("expected autoconnect to be false, but it is true")
	}

	autoConnect = true
	opts = wifi.UpdateOptions{AutoConnect: &autoConnect}
	err = b.UpdateNetwork(ssid, opts)
	if err != nil {
		t.Fatalf("failed to set autoconnect to true: %v", err)
	}

	result, err = b.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("failed to build network list: %v", err)
	}
	conns = result.Networks

	conn = findConnection(conns, ssid)
	if conn == nil {
		t.Fatalf("did not find network %s", ssid)
	}

	if !conn.AutoConnect {
		t.Errorf("expected autoconnect to be true, but it is false")
	}

	newPassword := "new-password"
	opts = wifi.UpdateOptions{Password: &newPassword}
	err = b.UpdateNetwork(ssid, opts)
	if err != nil {
		t.Fatalf("failed to update password: %v", err)
	}

	secret, err := b.GetSecrets(ssid)
	if err != nil {
		t.Fatalf("failed to get secrets: %v", err)
	}
	if secret != newPassword {
		t.Errorf("expected secret to be '%s', but got '%s'", newPassword, secret)
	}
}

func TestListNetworks_WirelessDisabled(t *testing.T) {
	b, _ := New()
	mockBackend := b.(*MockBackend)
	mockBackend.WirelessEnabled = false

	_, err := b.ListNetworks(wifi.ScanNever)
	if err == nil {
		t.Fatal("ListNetworks() should have failed, but did not")
	}
	if err != wifi.ErrWirelessDisabled {
		t.Errorf("expected error %v, but got %v", wifi.ErrWirelessDisabled, err)
	}
}

func TestJoinNetwork_UpdatePassword(t *testing.T) {
	b, _ := New()
	ssid := "GET off my LAN" // A network that is visible but not known initially
	password := "password123"

	// 1. Join the network for the first time
	err := b.JoinNetwork(ssid, password, wifi.SecurityWPA, false)
	if err != nil {
		t.Fatalf("JoinNetwork() failed on first join: %v", err)
	}

	// 2. Check if the secret was saved correctly
	secret, err := b.GetSecrets(ssid)
	if err != nil {
		t.Fatalf("GetSecrets() failed after first join: %v", err)
	}
	if secret != password {
		t.Errorf("expected secret '%s', got '%s'", password, secret)
	}

	// 2a. Check ListNetworks output
	result, err := b.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("ListNetworks() failed: %v", err)
	}
	networks := result.Networks
	conn := findConnection(networks, ssid)
	if conn == nil {
		t.Fatalf("did not find network %s in list after first join", ssid)
	}
	if !conn.IsKnown {
		t.Errorf("expected network %s to be known in list, but it was not", ssid)
	}

	// 3. Join the same network again with a new password
	newPassword := "newPassword456"
	err = b.JoinNetwork(ssid, newPassword, wifi.SecurityWPA, false)
	if err != nil {
		t.Fatalf("JoinNetwork() failed on second join: %v", err)
	}

	// 4. Check if the secret was updated
	secret, err = b.GetSecrets(ssid)
	if err != nil {
		t.Fatalf("GetSecrets() failed after second join: %v", err)
	}
	if secret != newPassword {
		t.Errorf("expected secret to be updated to '%s', but got '%s'", newPassword, secret)
	}

	// 4a. Check ListNetworks output again
	result, err = b.ListNetworks(wifi.ScanNever)
	if err != nil {
		t.Fatalf("ListNetworks() failed after second join: %v", err)
	}
	networks = result.Networks
	conn = findConnection(networks, ssid)
	if conn == nil {
		t.Fatalf("did not find network %s in in list after second join", ssid)
	}
	if !conn.IsKnown {
		t.Errorf("expected network %s to still be known in list, but it was not", ssid)
	}

	// 5. Check that no duplicate connection was created
	mockBackend := b.(*MockBackend)
	count := 0
	for _, c := range mockBackend.KnownNetworks {
		if c.SSID == ssid {
			count++
		}
	}
	if count > 1 {
		t.Errorf("expected only one known connection for SSID '%s', but found %d", ssid, count)
	}
}

func init() {
	DefaultActionSleep = 0
}
