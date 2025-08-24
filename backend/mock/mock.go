package mock

import (
	"fmt"
	"time"

	"github.com/shazow/wifitui/backend"
)

// Backend is a mock implementation of the backend.Backend interface for testing.
type Backend struct {
	Connections     []backend.Connection
	Secrets         map[string]string
	ActivateError   error
	ForgetError     error
	JoinError       error
	GetSecretsError error
	UpdateSecretError error
}

// New creates a new mock.Backend with some default data.
func New() *Backend {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	return &Backend{
		Connections: []backend.Connection{
			{SSID: "TestNet 1", IsVisible: true, Strength: 80},
			{SSID: "TestNet 2", IsVisible: true, Strength: 50, IsSecure: true},
			{SSID: "TestNet 3", IsVisible: false, IsKnown: true, LastConnected: &now},
			{SSID: "TestNet 4", IsVisible: false, IsKnown: true, LastConnected: &yesterday},
			{SSID: "VisibleOnly", IsVisible: true, IsSecure: true},
		},
		Secrets: map[string]string{
			"TestNet 2": "password123",
			"TestNet 3": "password3",
			"TestNet 4": "password4",
		},
	}
}

func (m *Backend) BuildNetworkList(shouldScan bool) ([]backend.Connection, error) {
	return m.Connections, nil
}

func (m *Backend) ActivateConnection(ssid string) error {
	return m.ActivateError
}

func (m *Backend) ForgetNetwork(ssid string) error {
	return m.ForgetError
}

func (m *Backend) JoinNetwork(ssid string, password string) error {
	return m.JoinError
}

func (m *Backend) GetSecrets(ssid string) (string, error) {
	if m.GetSecretsError != nil {
		return "", m.GetSecretsError
	}
	secret, ok := m.Secrets[ssid]
	if !ok {
		return "", fmt.Errorf("no secrets for %s", ssid)
	}
	return secret, nil
}

func (m *Backend) UpdateSecret(ssid string, newPassword string) error {
	return m.UpdateSecretError
}
