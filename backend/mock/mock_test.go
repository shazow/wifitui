package mock

import (
	"testing"

	"github.com/shazow/wifitui/backend"
)

func TestNew(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if b == nil {
		t.Fatal("New() returned nil backend")
	}
}

func TestBuildNetworkList(t *testing.T) {
	b, _ := New()
	networks, err := b.BuildNetworkList(true)
	if err != nil {
		t.Fatalf("BuildNetworkList() failed: %v", err)
	}
	if len(networks) == 0 {
		t.Fatal("BuildNetworkList() returned no networks")
	}
}

func TestActivateConnection(t *testing.T) {
	b, _ := New()
	mockBackend := b.(*MockBackend)
	ssid := mockBackend.Connections[0].SSID

	err := b.ActivateConnection(ssid)
	if err != nil {
		t.Fatalf("ActivateConnection() failed: %v", err)
	}

	for _, c := range mockBackend.Connections {
		if c.SSID == ssid {
			if !c.IsActive {
				t.Errorf("connection %s should be active but is not", ssid)
			}
		} else {
			if c.IsActive {
				t.Errorf("connection %s should be inactive but is not", c.SSID)
			}
		}
	}

	err = b.ActivateConnection("non-existent-network")
	if err == nil {
		t.Fatal("ActivateConnection() with non-existent network should have failed, but did not")
	}
}

func TestForgetNetwork(t *testing.T) {
	b, _ := New()
	mockBackend := b.(*MockBackend)
	connectionToForget := mockBackend.Connections[0]
	ssid := connectionToForget.SSID

	err := b.ForgetNetwork(ssid)
	if err != nil {
		t.Fatalf("ForgetNetwork() failed: %v", err)
	}

	for _, c := range mockBackend.Connections {
		if c.SSID == ssid {
			t.Errorf("connection %s should have been forgotten but was not", ssid)
		}
	}

	_, err = b.GetSecrets(ssid)
	if err == nil {
		t.Errorf("secrets for %s should have been forgotten but were not", ssid)
	}

	err = b.ForgetNetwork("non-existent-network")
	if err == nil {
		t.Fatal("ForgetNetwork() with non-existent network should have failed, but did not")
	}
}

func TestJoinNetwork(t *testing.T) {
	b, _ := New()
	mockBackend := b.(*MockBackend)

	// Join a new network
	newSSID := "new-network"
	password := "password"
	err := b.JoinNetwork(newSSID, password, backend.SecurityWPA, false)
	if err != nil {
		t.Fatalf("JoinNetwork() failed: %v", err)
	}

	var newConnection *backend.Connection
	for i := range mockBackend.Connections {
		if mockBackend.Connections[i].SSID == newSSID {
			newConnection = &mockBackend.Connections[i]
			break
		}
	}

	if newConnection == nil {
		t.Fatalf("JoinNetwork() did not add the new network")
	}
	if !newConnection.IsActive {
		t.Error("newly joined network should be active")
	}
	if !newConnection.IsKnown {
		t.Error("newly joined network should be known")
	}
	if newConnection.Security != backend.SecurityWPA {
		t.Errorf("expected security %v, got %v", backend.SecurityWPA, newConnection.Security)
	}

	secret, err := b.GetSecrets(newSSID)
	if err != nil {
		t.Fatalf("GetSecrets() failed: %v", err)
	}
	if secret != password {
		t.Errorf("expected password %s, got %s", password, secret)
	}

	// Join an existing network
	existingSSID := mockBackend.Connections[0].SSID
	err = b.JoinNetwork(existingSSID, "", backend.SecurityOpen, false)
	if err != nil {
		t.Fatalf("JoinNetwork() for existing network failed: %v", err)
	}
}

func TestGetSecrets(t *testing.T) {
	b, _ := New()
	ssid := "Password is password"
	expectedSecret := "password"

	secret, err := b.GetSecrets(ssid)
	if err != nil {
		t.Fatalf("GetSecrets() failed: %v", err)
	}
	if secret != expectedSecret {
		t.Errorf("expected secret %s, got %s", expectedSecret, secret)
	}

	_, err = b.GetSecrets("non-existent-network")
	if err == nil {
		t.Fatal("GetSecrets() with non-existent network should have failed, but did not")
	}
}

func TestUpdateSecret(t *testing.T) {
	b, _ := New()
	ssid := "Password is password"
	newPassword := "new-password"

	err := b.UpdateSecret(ssid, newPassword)
	if err != nil {
		t.Fatalf("UpdateSecret() failed: %v", err)
	}

	secret, err := b.GetSecrets(ssid)
	if err != nil {
		t.Fatalf("GetSecrets() failed: %v", err)
	}
	if secret != newPassword {
		t.Errorf("expected secret %s, got %s", newPassword, secret)
	}

	err = b.UpdateSecret("non-existent-network", "new-password")
	if err == nil {
		t.Fatal("UpdateSecret() with non-existent network should have failed, but did not")
	}
}

func TestGetSecretsForKnownNetworkWithoutSecret(t *testing.T) {
	testCases := []struct {
		name         string
		ssid         string
		security     backend.SecurityType
		joinPassword string
	}{
		{
			name:         "Open network",
			ssid:         "Unencrypted_Honeypot",
			security:     backend.SecurityOpen,
			joinPassword: "",
		},
		{
			name:         "Secure network joined without password",
			ssid:         "TacoBoutAGoodSignal",
			security:     backend.SecurityWPA,
			joinPassword: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := New()

			// Simulate joining the network to make it "known"
			err := b.JoinNetwork(tc.ssid, tc.joinPassword, tc.security, false)
			if err != nil {
				t.Fatalf("JoinNetwork() failed: %v", err)
			}

			// Now try to get secrets for it, which should succeed even without a password
			secret, err := b.GetSecrets(tc.ssid)
			if err != nil {
				t.Fatalf("GetSecrets() failed: %v", err)
			}
			if secret != "" {
				t.Errorf("expected empty secret, got '%s'", secret)
			}
		})
	}
}
