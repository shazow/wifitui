package main

import (
	"fmt"
	"time"
)

// MockBackend is a mock implementation of the Backend interface for testing.
type MockBackend struct {
	Connections     []Connection
	Secrets         map[string]string
	ActivateError   error
	ForgetError     error
	JoinError       error
	GetSecretsError error
	UpdateSecretError error
}

// NewMockBackend creates a new MockBackend with some default data.
func NewMockBackend() *MockBackend {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	return &MockBackend{
		Connections: []Connection{
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

func (m *MockBackend) BuildNetworkList(shouldScan bool) ([]Connection, error) {
	return m.Connections, nil
}

func (m *MockBackend) ActivateConnection(ssid string) error {
	return m.ActivateError
}

func (m *MockBackend) ForgetNetwork(ssid string) error {
	return m.ForgetError
}

func (m *MockBackend) JoinNetwork(ssid string, password string) error {
	return m.JoinError
}

func (m *MockBackend) GetSecrets(ssid string) (string, error) {
	if m.GetSecretsError != nil {
		return "", m.GetSecretsError
	}
	secret, ok := m.Secrets[ssid]
	if !ok {
		return "", fmt.Errorf("no secrets for %s", ssid)
	}
	return secret, nil
}

func (m *MockBackend) UpdateSecret(ssid string, newPassword string) error {
	return m.UpdateSecretError
}
