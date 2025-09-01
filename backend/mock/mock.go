package mock

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/shazow/wifitui/backend"
)

// mockConnection wraps a backend.Connection with mock-specific metadata.
type mockConnection struct {
	backend.Connection
	Secret string
}

// MockBackend is a mock implementation of the backend.Backend interface for testing.
type MockBackend struct {
	VisibleConnections    []backend.Connection
	KnownConnections      []mockConnection
	ActiveConnectionIndex int
	ActivateError         error
	ForgetError           error
	JoinError             error
	GetSecretsError       error
	UpdateSecretError     error
}

func ago(duration time.Duration) *time.Time {
	t := time.Now().Add(-duration)
	return &t
}

// NewBackend creates a new mock.Backend with a list of fun wifi networks.
func New() (backend.Backend, error) {
	initialConnections := []backend.Connection{
		{SSID: "HideYoKidsHideYoWiFi", Strength: 75, LastConnected: ago(2 * time.Hour), IsKnown: true, Security: backend.SecurityWPA, AutoConnect: true},
		{SSID: "GET off my LAN", Security: backend.SecurityWPA},
		{SSID: "NeverGonnaGiveYouIP", Security: backend.SecurityWEP, IsKnown: true, AutoConnect: false},
		{SSID: "Unencrypted_Honeypot", Security: backend.SecurityOpen},
		{SSID: "YourWiFi.exe", LastConnected: ago(9 * time.Hour), Security: backend.SecurityWPA, IsKnown: true, AutoConnect: true},
		{SSID: "I See Dead Packets", Security: backend.SecurityWEP},
		{SSID: "Dunder MiffLAN", Security: backend.SecurityWPA},
		{SSID: "Police Surveillance 2", Strength: 48, Security: backend.SecurityWPA},
		{SSID: "I Believe Wi Can Fi", Security: backend.SecurityWEP},
		{SSID: "Hot singles in your area", Security: backend.SecurityWPA},
		{SSID: "Password is password", IsKnown: true, Security: backend.SecurityWPA, AutoConnect: true},
		{SSID: "TacoBoutAGoodSignal", Strength: 99, Security: backend.SecurityWPA},
		{SSID: "Wi-Fight the Feeling?", Security: backend.SecurityWEP},
		{SSID: "xX_D4rkR0ut3r_Xx", Security: backend.SecurityWPA},
		{SSID: "Luke I am your WiFi", Security: backend.SecurityWEP},
		{SSID: "FreeHugsAndWiFi", LastConnected: ago(400 * time.Hour), Security: backend.SecurityOpen, IsKnown: true, AutoConnect: false},
	}
	secrets := map[string]string{
		"Password is password": "password",
		"HideYoKidsHideYoWiFi": "hidden",
	}

	var knownConnections []mockConnection
	for _, c := range initialConnections {
		if c.IsKnown {
			knownConnections = append(knownConnections, mockConnection{
				Connection: c,
				Secret:     secrets[c.SSID],
			})
		}
	}

	// For testing duplicate SSIDs
	knownConnections = append(knownConnections, mockConnection{
		Connection: backend.Connection{
			SSID:     "HideYoKidsHideYoWiFi",
			Strength: 25,
			IsKnown:  true,
			Security: backend.SecurityWPA,
		},
		Secret: "different_secret",
	})

	return &MockBackend{
		VisibleConnections:    initialConnections,
		KnownConnections:      knownConnections,
		ActiveConnectionIndex: -1, // No connection active initially
	}, nil
}

func (m *MockBackend) BuildNetworkList(shouldScan bool) ([]backend.Connection, error) {
	// For mock, we can re-randomize strengths on each scan
	if shouldScan {
		s := rand.NewSource(time.Now().Unix())
		r := rand.New(s)
		for i := range m.VisibleConnections {
			if m.VisibleConnections[i].Strength > 0 {
				m.VisibleConnections[i].Strength = uint8(r.Intn(70) + 30)
			}
		}
	}

	// Build a unified list of connections, de-duplicating known networks.
	unified := make(map[string]backend.Connection)

	// Add all visible connections first.
	for _, c := range m.VisibleConnections {
		unified[c.SSID] = c
	}

	// Add/overwrite with known connections to ensure they are in the list.
	for _, kc := range m.KnownConnections {
		conn := kc.Connection
		if visibleConn, ok := unified[conn.SSID]; ok {
			conn.Strength = visibleConn.Strength
		}
		unified[conn.SSID] = conn
	}

	// Get the active SSID beforehand.
	var activeSSID string
	if m.ActiveConnectionIndex >= 0 && m.ActiveConnectionIndex < len(m.KnownConnections) {
		activeSSID = m.KnownConnections[m.ActiveConnectionIndex].SSID
	}

	// Convert map back to a slice for the return value.
	var result []backend.Connection
	for _, c := range unified {
		isKnown := false
		for _, kc := range m.KnownConnections {
			if kc.SSID == c.SSID {
				isKnown = true
				break
			}
		}
		c.IsKnown = isKnown
		c.IsActive = (c.SSID == activeSSID)
		result = append(result, c)
	}

	return result, nil
}

func (m *MockBackend) ActivateConnection(ssid string) error {
	if m.ActivateError != nil {
		return m.ActivateError
	}
	// "Act on first match" logic for ambiguity.
	for i, c := range m.KnownConnections {
		if c.SSID == ssid {
			m.ActiveConnectionIndex = i
			now := time.Now()
			m.KnownConnections[i].LastConnected = &now
			return nil
		}
	}
	return fmt.Errorf("cannot activate unknown network %s: %w", ssid, backend.ErrNotFound)
}

func (m *MockBackend) ForgetNetwork(ssid string) error {
	if m.ForgetError != nil {
		return m.ForgetError
	}

	var activeSSID string
	if m.ActiveConnectionIndex >= 0 && m.ActiveConnectionIndex < len(m.KnownConnections) {
		activeSSID = m.KnownConnections[m.ActiveConnectionIndex].SSID
	}

	var newKnownConnections []mockConnection
	found := false
	for _, c := range m.KnownConnections {
		if c.SSID == ssid {
			found = true
		} else {
			newKnownConnections = append(newKnownConnections, c)
		}
	}

	if !found {
		return fmt.Errorf("network not found: %s: %w", ssid, backend.ErrNotFound)
	}

	m.KnownConnections = newKnownConnections

	// Reset active connection if it was the one forgotten.
	if activeSSID == ssid {
		m.ActiveConnectionIndex = -1
		return nil
	}

	// Otherwise, find the new index of the active connection.
	m.ActiveConnectionIndex = -1
	if activeSSID != "" {
		for i, c := range m.KnownConnections {
			if c.SSID == activeSSID {
				m.ActiveConnectionIndex = i
				break
			}
		}
	}

	return nil
}

func (m *MockBackend) JoinNetwork(ssid string, password string, security backend.SecurityType, isHidden bool) error {
	if m.JoinError != nil {
		return m.JoinError
	}

	var c backend.Connection
	found := false
	for _, vc := range m.VisibleConnections {
		if vc.SSID == ssid {
			c = vc
			found = true
			break
		}
	}
	if !found {
		c = backend.Connection{
			SSID:     ssid,
			Security: security,
			IsHidden: isHidden,
		}
	}

	c.IsKnown = true
	c.AutoConnect = true
	newConnection := mockConnection{
		Connection: c,
		Secret:     password,
	}
	m.KnownConnections = append(m.KnownConnections, newConnection)
	m.ActiveConnectionIndex = len(m.KnownConnections) - 1
	now := time.Now()
	m.KnownConnections[m.ActiveConnectionIndex].LastConnected = &now

	return nil
}

func (m *MockBackend) GetSecrets(ssid string) (string, error) {
	if m.GetSecretsError != nil {
		return "", m.GetSecretsError
	}
	// "Act on first match" logic for ambiguity.
	for _, c := range m.KnownConnections {
		if c.SSID == ssid {
			return c.Secret, nil
		}
	}
	return "", fmt.Errorf("no secrets for %s: %w", ssid, backend.ErrNotFound)
}

func (m *MockBackend) UpdateSecret(ssid string, newPassword string) error {
	if m.UpdateSecretError != nil {
		return m.UpdateSecretError
	}
	// "Act on first match" logic for ambiguity.
	for i, c := range m.KnownConnections {
		if c.SSID == ssid {
			m.KnownConnections[i].Secret = newPassword
			return nil
		}
	}
	return fmt.Errorf("cannot update secret for unknown network %s: %w", ssid, backend.ErrNotFound)
}
