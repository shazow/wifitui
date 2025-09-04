package mock

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/shazow/wifitui/backend"
)

var DefaultConnectSleep = 500 * time.Millisecond

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
	ConnectSleep          time.Duration
}

func ago(duration time.Duration) *time.Time {
	t := time.Now().Add(-duration)
	return &t
}

// NewBackend creates a new mock.Backend with a list of fun wifi networks.
func New() (backend.Backend, error) {
	initialConnections := []backend.Connection{
		{SSID: "HideYoKidsHideYoWiFi", LastConnected: ago(2 * time.Hour), IsKnown: true, AutoConnect: true, Security: backend.SecurityWPA},
		{SSID: "GET off my LAN", Security: backend.SecurityWPA, LastConnected: ago(761 * time.Hour), IsKnown: true, AutoConnect: false},
		{SSID: "NeverGonnaGiveYouIP", Security: backend.SecurityWEP, IsVisible: true},
		{SSID: "Unencrypted_Honeypot", Security: backend.SecurityOpen, IsVisible: true},
		{SSID: "YourWiFi.exe", LastConnected: ago(9 * time.Hour), Security: backend.SecurityWPA},
		{SSID: "I See Dead Packets", Security: backend.SecurityWEP, LastConnected: ago(8763 * time.Hour)},
		{SSID: "Dunder MiffLAN", Security: backend.SecurityWPA, IsVisible: true},
		{SSID: "Police Surveillance 2", Strength: 48, Security: backend.SecurityWPA, IsVisible: true},
		{SSID: "I Believe Wi Can Fi", Security: backend.SecurityWEP, IsVisible: true},
		{SSID: "Hot singles in your area", Security: backend.SecurityWPA, IsVisible: true},
		{SSID: "Password is password", Strength: 87, LastConnected: ago(12456 * time.Hour), IsKnown: true, AutoConnect: true, Security: backend.SecurityWPA, IsVisible: true},
		{SSID: "TacoBoutAGoodSignal", Strength: 99, Security: backend.SecurityWPA, IsVisible: true},
		{SSID: "Wi-Fight the Feeling?", Security: backend.SecurityWEP},
		{SSID: "xX_D4rkR0ut3r_Xx", Security: backend.SecurityWPA},
		{SSID: "Luke I am your WiFi", Security: backend.SecurityWEP},
		{SSID: "FreeHugsAndWiFi", LastConnected: ago(400 * time.Hour), Security: backend.SecurityWPA},
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
		ConnectSleep:          DefaultConnectSleep,
	}, nil
}

// setActiveConnection sets the active connection and ensures all other connections are inactive.
func (m *MockBackend) setActiveConnection(ssid string) {
	m.ActiveConnectionIndex = -1
	for i := range m.KnownConnections {
		isActive := m.KnownConnections[i].SSID == ssid
		m.KnownConnections[i].IsActive = isActive
		if isActive {
			m.ActiveConnectionIndex = i
		}
	}

	// Also update the visible connections slice for consistency
	for i := range m.VisibleConnections {
		m.VisibleConnections[i].IsActive = (m.VisibleConnections[i].SSID == ssid)
	}
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

	// Convert map back to a slice for the return value.
	var result []backend.Connection
	for _, c := range unified {
		// IsActive is now stored on the connection object itself.
		// We still need to determine IsKnown for networks that might only be in the visible list.
		isKnown := false
		for _, kc := range m.KnownConnections {
			if kc.SSID == c.SSID {
				isKnown = true
				break
			}
		}
		c.IsKnown = isKnown
		if !isKnown {
			c.AutoConnect = false
		}
		result = append(result, c)
	}

	return result, nil
}

func (m *MockBackend) ActivateConnection(ssid string) error {
	if m.ActivateError != nil {
		return m.ActivateError
	}
	time.Sleep(m.ConnectSleep)
	// "Act on first match" logic for ambiguity.
	for i, c := range m.KnownConnections {
		if c.SSID == ssid {
			m.setActiveConnection(ssid)
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

	if activeSSID == ssid {
		m.setActiveConnection("") // Deactivate all
	} else {
		m.setActiveConnection(activeSSID) // Re-sync active connection
	}

	return nil
}

func (m *MockBackend) JoinNetwork(ssid string, password string, security backend.SecurityType, isHidden bool) error {
	if m.JoinError != nil {
		return m.JoinError
	}

	var c backend.Connection
	found := false
	foundIndex := -1
	for i, vc := range m.VisibleConnections {
		if vc.SSID == ssid {
			c = vc
			found = true
			foundIndex = i
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

	time.Sleep(m.ConnectSleep)

	c.IsKnown = true
	c.AutoConnect = true
	if found {
		m.VisibleConnections[foundIndex] = c
	}

	newConnection := mockConnection{
		Connection: c,
		Secret:     password,
	}

	// Check if we are replacing an existing known connection, otherwise append.
	foundInKnown := false
	for i, kc := range m.KnownConnections {
		if kc.SSID == ssid {
			m.KnownConnections[i] = newConnection
			foundInKnown = true
			break
		}
	}
	if !foundInKnown {
		m.KnownConnections = append(m.KnownConnections, newConnection)
	}

	m.setActiveConnection(ssid)
	now := time.Now()
	if m.ActiveConnectionIndex != -1 {
		m.KnownConnections[m.ActiveConnectionIndex].LastConnected = &now
	}

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

func (m *MockBackend) SetAutoConnect(ssid string, autoConnect bool) error {
	// "Act on first match" logic for ambiguity.
	for i, c := range m.KnownConnections {
		if c.SSID == ssid {
			m.KnownConnections[i].AutoConnect = autoConnect
			return nil
		}
	}
	return fmt.Errorf("cannot set autoconnect for unknown network %s: %w", ssid, backend.ErrNotFound)
}
