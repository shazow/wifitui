package mock

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/shazow/wifitui/wifi"
)

var DefaultActionSleep = 500 * time.Millisecond

// mockConnection wraps a backend.Connection with mock-specific metadata.
type mockConnection struct {
	wifi.Connection
	Secret string
}

// MockBackend is a mock implementation of the backend.Backend interface for testing.
type MockBackend struct {
	VisibleConnections     []wifi.Connection
	KnownConnections       []mockConnection
	ActiveConnectionIndex  int
	ActivateError          error
	ForgetError            error
	JoinError              error
	GetSecretsError        error
	UpdateConnectionError  error
	WirelessEnabled        bool
	IsWirelessEnabledError error
	SetWirelessError       error

	// ActionSleep is a delay before every action, to better emulate a real-world backend for the frontend. Set to 0 during testing.
	ActionSleep time.Duration
}

func ago(duration time.Duration) *time.Time {
	t := time.Now().Add(-duration)
	return &t
}

// NewBackend creates a new mock.Backend with a list of fun wifi networks.
func New() (wifi.Backend, error) {
	initialConnections := []wifi.Connection{
		{SSID: "HideYoKidsHideYoWiFi", LastConnected: ago(2 * time.Hour), IsKnown: true, AutoConnect: true, Security: wifi.SecurityWPA},
		{SSID: "GET off my LAN", Security: wifi.SecurityWPA, LastConnected: ago(761 * time.Hour), IsKnown: true, AutoConnect: false},
		{SSID: "NeverGonnaGiveYouIP", Security: wifi.SecurityWEP, IsVisible: true},
		{SSID: "Unencrypted_Honeypot", Security: wifi.SecurityOpen, IsVisible: true},
		{SSID: "YourWiFi.exe", LastConnected: ago(9 * time.Hour), Security: wifi.SecurityWPA},
		{SSID: "I See Dead Packets", Security: wifi.SecurityWEP, LastConnected: ago(8763 * time.Hour)},
		{SSID: "Dunder MiffLAN", Security: wifi.SecurityWPA, IsVisible: true},
		{SSID: "Police Surveillance 2", AccessPoints: []wifi.AccessPoint{{Strength: 48}}, Security: wifi.SecurityWPA, IsVisible: true},
		{SSID: "I Believe Wi Can Fi", Security: wifi.SecurityWEP, IsVisible: true},
		{SSID: "Hot singles in your area", Security: wifi.SecurityWPA, IsVisible: true},
		{SSID: "Password is password", AccessPoints: []wifi.AccessPoint{{Strength: 87}}, LastConnected: ago(12456 * time.Hour), IsKnown: true, AutoConnect: true, Security: wifi.SecurityWPA, IsVisible: true},
		{SSID: "TacoBoutAGoodSignal", AccessPoints: []wifi.AccessPoint{{Strength: 99}}, Security: wifi.SecurityWPA, IsVisible: true},
		{SSID: "Multi-AP Network", AccessPoints: []wifi.AccessPoint{
			{SSID: "Multi-AP Network", BSSID: "00:11:22:33:44:55", Strength: 80, Frequency: 2412},
			{SSID: "Multi-AP Network", BSSID: "AA:BB:CC:DD:EE:FF", Strength: 60, Frequency: 5180},
			{SSID: "Multi-AP Network", BSSID: "11:22:33:44:55:66", Strength: 40, Frequency: 5240},
		}, Security: wifi.SecurityWPA, IsVisible: true},
		{SSID: "Wi-Fight the Feeling?", Security: wifi.SecurityWEP},
		{SSID: "xX_D4rkR0ut3r_Xx", Security: wifi.SecurityWPA},
		{SSID: "Luke I am your WiFi", Security: wifi.SecurityWEP},
		{SSID: "FreeHugsAndWiFi", LastConnected: ago(400 * time.Hour), Security: wifi.SecurityWPA},
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
		Connection: wifi.Connection{
			SSID:         "HideYoKidsHideYoWiFi",
			AccessPoints: []wifi.AccessPoint{{Strength: 25}},
			IsKnown:      true,
			Security:     wifi.SecurityWPA,
		},
		Secret: "different_secret",
	})

	return &MockBackend{
		VisibleConnections:    initialConnections,
		KnownConnections:      knownConnections,
		ActiveConnectionIndex: -1, // No connection active initially
		ActionSleep:           DefaultActionSleep,
		WirelessEnabled:       true,
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

func (m *MockBackend) BuildNetworkList(shouldScan bool) ([]wifi.Connection, error) {
	time.Sleep(m.ActionSleep)

	if !m.WirelessEnabled {
		return nil, wifi.ErrWirelessDisabled
	}
	// For mock, we can re-randomize strengths on each scan
	if shouldScan {
		s := rand.NewSource(time.Now().Unix())
		r := rand.New(s)
		for i := range m.VisibleConnections {
			if len(m.VisibleConnections[i].AccessPoints) > 0 {
				for j := range m.VisibleConnections[i].AccessPoints {
					m.VisibleConnections[i].AccessPoints[j].Strength = uint8(r.Intn(70) + 30)
				}
			} else {
				// Create a default AP if none exists but it's supposed to be visible (though logic below might add it)
				// Actually, initialConnections should probably have APs initialized.
				// But let's be safe.
				m.VisibleConnections[i].AccessPoints = []wifi.AccessPoint{{
					SSID:     m.VisibleConnections[i].SSID,
					Strength: uint8(r.Intn(70) + 30),
				}}
			}
		}
	}

	// Build a unified list of connections, de-duplicating known networks.
	unified := make(map[string]wifi.Connection)

	// Add all visible connections first.
	for _, c := range m.VisibleConnections {
		unified[c.SSID] = c
	}

	// Add/overwrite with known connections to ensure they are in the list.
	for _, kc := range m.KnownConnections {
		conn := kc.Connection
		if visibleConn, ok := unified[conn.SSID]; ok {
			conn.AccessPoints = visibleConn.AccessPoints
		}
		unified[conn.SSID] = conn
	}

	// Convert map back to a slice for the return value.
	var result []wifi.Connection
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
	time.Sleep(m.ActionSleep)

	if m.ActivateError != nil {
		return m.ActivateError
	}
	// "Act on first match" logic for ambiguity.
	for i, c := range m.KnownConnections {
		if c.SSID == ssid {
			m.setActiveConnection(ssid)
			now := time.Now()
			m.KnownConnections[i].LastConnected = &now
			return nil
		}
	}
	return fmt.Errorf("cannot activate unknown network %s: %w", ssid, wifi.ErrNotFound)
}

func (m *MockBackend) ForgetNetwork(ssid string) error {
	time.Sleep(m.ActionSleep)

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
		return fmt.Errorf("network not found: %s: %w", ssid, wifi.ErrNotFound)
	}

	m.KnownConnections = newKnownConnections

	if activeSSID == ssid {
		m.setActiveConnection("") // Deactivate all
	} else {
		m.setActiveConnection(activeSSID) // Re-sync active connection
	}

	return nil
}

func (m *MockBackend) JoinNetwork(ssid string, password string, security wifi.SecurityType, isHidden bool) error {
	time.Sleep(m.ActionSleep)

	if m.JoinError != nil {
		return m.JoinError
	}

	var c wifi.Connection
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
		c = wifi.Connection{
			SSID:     ssid,
			Security: security,
			IsHidden: isHidden,
		}
	}

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
	time.Sleep(m.ActionSleep)

	if m.GetSecretsError != nil {
		return "", m.GetSecretsError
	}
	// "Act on first match" logic for ambiguity.
	for _, c := range m.KnownConnections {
		if c.SSID == ssid {
			return c.Secret, nil
		}
	}
	return "", fmt.Errorf("no secrets for %s: %w", ssid, wifi.ErrNotFound)
}

func (m *MockBackend) UpdateConnection(ssid string, opts wifi.UpdateOptions) error {
	time.Sleep(m.ActionSleep)

	if m.UpdateConnectionError != nil {
		return m.UpdateConnectionError
	}
	// "Act on first match" logic for ambiguity.
	for i, c := range m.KnownConnections {
		if c.SSID == ssid {
			if opts.Password != nil {
				m.KnownConnections[i].Secret = *opts.Password
			}
			if opts.AutoConnect != nil {
				m.KnownConnections[i].AutoConnect = *opts.AutoConnect
			}
			return nil
		}
	}
	return fmt.Errorf("cannot update connection for unknown network %s: %w", ssid, wifi.ErrNotFound)
}

func (m *MockBackend) IsWirelessEnabled() (bool, error) {
	time.Sleep(m.ActionSleep)

	if m.IsWirelessEnabledError != nil {
		return false, m.IsWirelessEnabledError
	}
	return m.WirelessEnabled, nil
}

func (m *MockBackend) SetWireless(enabled bool) error {
	time.Sleep(m.ActionSleep)

	if m.SetWirelessError != nil {
		return m.SetWirelessError
	}
	m.WirelessEnabled = enabled
	return nil
}
