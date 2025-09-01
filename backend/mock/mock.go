package mock

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/shazow/wifitui/backend"
)

// MockBackend is a mock implementation of the backend.Backend interface for testing.
type MockBackend struct {
	Connections       []backend.Connection
	Secrets           map[string]string
	ActivateError     error
	ForgetError       error
	JoinError         error
	GetSecretsError   error
	UpdateSecretError error
}

func ago(duration time.Duration) *time.Time {
	t := time.Now().Add(-duration)
	return &t
}

// NewBackend creates a new mock.Backend with a list of fun wifi networks.
func New() (backend.Backend, error) {
	connections := []backend.Connection{
		{SSID: "HideYoKidsHideYoWiFi", Strength: 75, LastConnected: ago(2 * time.Hour), IsKnown: true, Security: backend.SecurityWPA},
		{SSID: "GET off my LAN", Security: backend.SecurityWPA},
		{SSID: "NeverGonnaGiveYouIP", Security: backend.SecurityWEP},
		{SSID: "Unencrypted_Honeypot", Security: backend.SecurityOpen},
		{SSID: "YourWiFi.exe", LastConnected: ago(9 * time.Hour), Security: backend.SecurityWPA},
		{SSID: "I See Dead Packets", Security: backend.SecurityWEP},
		{SSID: "Dunder MiffLAN", Security: backend.SecurityWPA},
		{SSID: "Police Surveillance 2", Strength: 48, Security: backend.SecurityWPA},
		{SSID: "I Believe Wi Can Fi", Security: backend.SecurityWEP},
		{SSID: "Hot singles in your area", Security: backend.SecurityWPA},
		{SSID: "Password is password", IsKnown: true, Security: backend.SecurityWPA},
		{SSID: "TacoBoutAGoodSignal", Strength: 99, Security: backend.SecurityWPA},
		{SSID: "Wi-Fight the Feeling?", Security: backend.SecurityWEP},
		{SSID: "xX_D4rkR0ut3r_Xx", Security: backend.SecurityWPA},
		{SSID: "Luke I am your WiFi", Security: backend.SecurityWEP},
		{SSID: "FreeHugsAndWiFi", LastConnected: ago(400 * time.Hour), Security: backend.SecurityWPA},
	}
	secrets := map[string]string{
		"Password is password": "password",
		"HideYoKidsHideYoWiFi": "hidden",
	}

	return &MockBackend{
		Connections: connections,
		Secrets:     secrets,
	}, nil
}

func (m *MockBackend) BuildNetworkList(shouldScan bool) ([]backend.Connection, error) {
	// For mock, we can re-randomize strengths on each scan
	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)
	for i := range m.Connections {
		if m.Connections[i].Strength > 0 {
			// Only randomize if we have a strength already
			m.Connections[i].Strength = uint8(r.Intn(70) + 30)
		}
	}
	return m.Connections, nil
}

func (m *MockBackend) Connect(ssid string) error {
	found := false
	for i := range m.Connections {
		if m.Connections[i].SSID == ssid {
			m.Connections[i].IsActive = true
			now := time.Now()
			m.Connections[i].LastConnected = &now
			found = true
		} else {
			m.Connections[i].IsActive = false
		}
	}
	if !found {
		return fmt.Errorf("network not found: %s", ssid)
	}
	return nil
}

func (m *MockBackend) ActivateConnection(ssid string) error {
	if m.ActivateError != nil {
		return m.ActivateError
	}
	return m.Connect(ssid)
}

func (m *MockBackend) ForgetNetwork(ssid string) error {
	if m.ForgetError != nil {
		return m.ForgetError
	}

	// Find existing network and remove it
	var newConnections []backend.Connection
	found := false
	for _, c := range m.Connections {
		if c.SSID == ssid {
			found = true
		} else {
			newConnections = append(newConnections, c)
		}
	}

	if found {
		m.Connections = newConnections
		delete(m.Secrets, ssid)
		return nil
	}

	return fmt.Errorf("network not found: %s", ssid)
}

func (m *MockBackend) JoinNetwork(ssid string, password string, security backend.SecurityType, isHidden bool) error {
	if m.JoinError != nil {
		return m.JoinError
	}

	// Find existing network or create a new one
	var c *backend.Connection
	for i := range m.Connections {
		if m.Connections[i].SSID == ssid {
			c = &m.Connections[i]
			break
		}
	}
	if c == nil {
		// Not found, create a new one
		m.Connections = append(m.Connections, backend.Connection{
			SSID: ssid,
		})
		c = &m.Connections[len(m.Connections)-1]
	}

	c.IsKnown = true
	c.Security = security

	if password != "" {
		if m.Secrets == nil {
			m.Secrets = make(map[string]string)
		}
		m.Secrets[ssid] = password
	}

	return m.Connect(ssid)
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
	if m.UpdateSecretError != nil {
		return m.UpdateSecretError
	}
	if _, ok := m.Secrets[ssid]; ok {
		m.Secrets[ssid] = newPassword
		return nil
	}
	return fmt.Errorf("no secrets for %s", ssid)
}
