package mock

import (
	"testing"

	"github.com/shazow/wifitui/wifi"
)

// Helper to find a connection in a slice
func findConnection(connections []wifi.Connection, ssid string) *wifi.Connection {
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
	if len(mock.KnownConnections) == 0 {
		t.Fatal("New() returned no known connections")
	}
	if mock.ActiveConnectionIndex != -1 {
		t.Errorf("expected ActiveConnectionIndex to be -1, got %d", mock.ActiveConnectionIndex)
	}
}

func TestBuildNetworkList(t *testing.T) {
	b, _ := New()
	mock := b.(*MockBackend)
	knownSSID := "Password is password"

	// Activate a connection to test IsActive flag
	err := mock.ActivateConnection("HideYoKidsHideYoWiFi")
	if err != nil {
		t.Fatalf("ActivateConnection failed: %v", err)
	}

	networks, err := b.BuildNetworkList(false)
	if err != nil {
		t.Fatalf("BuildNetworkList() failed: %v", err)
	}

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

func TestActivateConnection(t *testing.T) {
	b, _ := New()
	mockBackend := b.(*MockBackend)
	ssid := "Password is password"

	err := b.ActivateConnection(ssid)
	if err != nil {
		t.Fatalf("ActivateConnection() failed: %v", err)
	}

	if mockBackend.ActiveConnectionIndex == -1 {
		t.Fatal("ActiveConnectionIndex was not set")
	}
	if mockBackend.KnownConnections[mockBackend.ActiveConnectionIndex].SSID != ssid {
		t.Errorf("expected active connection to be %s, but got %s", ssid, mockBackend.KnownConnections[mockBackend.ActiveConnectionIndex].SSID)
	}

	err = b.ActivateConnection("non-existent-network")
	if err == nil {
		t.Fatal("ActivateConnection() with non-existent network should have failed, but did not")
	}
}

func TestForgetNetwork(t *testing.T) {
	b, _ := New()
	mockBackend := b.(*MockBackend)

	// --- Test index shifting ---
	// Activate a network that is not the first one.
	ssidToActivate := "Password is password"
	err := b.ActivateConnection(ssidToActivate)
	if err != nil {
		t.Fatalf("ActivateConnection() failed: %v", err)
	}

	initialActiveIndex := mockBackend.ActiveConnectionIndex
	if initialActiveIndex <= 0 {
		t.Fatalf("Test setup failed: expected active index > 0, got %d", initialActiveIndex)
	}

	// Forget a network that appears *before* the active one.
	ssidToForget := mockBackend.KnownConnections[0].SSID
	if ssidToForget == ssidToActivate {
		t.Fatalf("Test setup failed: network to forget is the same as the active one")
	}

	err = b.ForgetNetwork(ssidToForget)
	if err != nil {
		t.Fatalf("ForgetNetwork() failed: %v", err)
	}

	// Check that the active index was shifted correctly.
	expectedIndex := initialActiveIndex - 1
	if mockBackend.ActiveConnectionIndex != expectedIndex {
		t.Errorf("active index should have shifted to %d, but got %d", expectedIndex, mockBackend.ActiveConnectionIndex)
	}
	if mockBackend.KnownConnections[mockBackend.ActiveConnectionIndex].SSID != ssidToActivate {
		t.Errorf("active connection SSID is incorrect after forgetting another network")
	}

	// --- Test forgetting the active network ---
	err = b.ForgetNetwork(ssidToActivate)
	if err != nil {
		t.Fatalf("ForgetNetwork() of active network failed: %v", err)
	}
	if mockBackend.ActiveConnectionIndex != -1 {
		t.Errorf("ActiveConnectionIndex should be -1 after forgetting active network, got %d", mockBackend.ActiveConnectionIndex)
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

	lastIndex := len(mockBackend.KnownConnections) - 1
	if mockBackend.KnownConnections[lastIndex].SSID != newSSID {
		t.Fatalf("JoinNetwork() did not add the new network to known connections")
	}
	if mockBackend.ActiveConnectionIndex != lastIndex {
		t.Errorf("newly joined network should be active, expected index %d but got %d", lastIndex, mockBackend.ActiveConnectionIndex)
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

func TestUpdateConnectionForDuplicateSSID(t *testing.T) {
	b, _ := New()
	mockBackend := b.(*MockBackend)
	ssid := "HideYoKidsHideYoWiFi"
	newPassword := "new-password"

	opts := wifi.UpdateOptions{Password: &newPassword}
	err := b.UpdateConnection(ssid, opts)
	if err != nil {
		t.Fatalf("UpdateConnection() failed: %v", err)
	}

	// Verify that only the first entry was updated
	if mockBackend.KnownConnections[0].SSID != ssid || mockBackend.KnownConnections[0].Secret != newPassword {
		t.Errorf("first instance of duplicate SSID was not updated correctly")
	}
	if mockBackend.KnownConnections[1].SSID == ssid && mockBackend.KnownConnections[1].Secret == newPassword {
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

func TestUpdateConnection(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("failed to create mock backend: %v", err)
	}

	ssid := "Password is password"
	autoConnect := false
	opts := wifi.UpdateOptions{AutoConnect: &autoConnect}
	err = b.UpdateConnection(ssid, opts)
	if err != nil {
		t.Fatalf("failed to set autoconnect to false: %v", err)
	}

	conns, err := b.BuildNetworkList(false)
	if err != nil {
		t.Fatalf("failed to build network list: %v", err)
	}

	conn := findConnection(conns, ssid)
	if conn == nil {
		t.Fatalf("did not find network %s", ssid)
	}

	if conn.AutoConnect {
		t.Errorf("expected autoconnect to be false, but it is true")
	}

	autoConnect = true
	opts = wifi.UpdateOptions{AutoConnect: &autoConnect}
	err = b.UpdateConnection(ssid, opts)
	if err != nil {
		t.Fatalf("failed to set autoconnect to true: %v", err)
	}

	conns, err = b.BuildNetworkList(false)
	if err != nil {
		t.Fatalf("failed to build network list: %v", err)
	}

	conn = findConnection(conns, ssid)
	if conn == nil {
		t.Fatalf("did not find network %s", ssid)
	}

	if !conn.AutoConnect {
		t.Errorf("expected autoconnect to be true, but it is false")
	}

	newPassword := "new-password"
	opts = wifi.UpdateOptions{Password: &newPassword}
	err = b.UpdateConnection(ssid, opts)
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

func TestBuildNetworkList_WirelessDisabled(t *testing.T) {
	b, _ := New()
	mockBackend := b.(*MockBackend)
	mockBackend.WirelessEnabled = false

	_, err := b.BuildNetworkList(false)
	if err == nil {
		t.Fatal("BuildNetworkList() should have failed, but did not")
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

	// 2a. Check BuildNetworkList output
	networks, err := b.BuildNetworkList(false)
	if err != nil {
		t.Fatalf("BuildNetworkList() failed: %v", err)
	}
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

	// 4a. Check BuildNetworkList output again
	networks, err = b.BuildNetworkList(false)
	if err != nil {
		t.Fatalf("BuildNetworkList() failed after second join: %v", err)
	}
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
	for _, c := range mockBackend.KnownConnections {
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
