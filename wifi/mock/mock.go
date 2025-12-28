package mock

import (
	"fmt"
	//"math/rand"
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
		{SSID: "Police Surveillance 2", Strength: 48, Security: wifi.SecurityWPA, IsVisible: true},
		{SSID: "I Believe Wi Can Fi", Security: wifi.SecurityWEP, IsVisible: true},
		{SSID: "Hot singles in your area", Security: wifi.SecurityWPA, IsVisible: true},
		{SSID: "Password is password", Strength: 87, LastConnected: ago(12456 * time.Hour), IsKnown: true, AutoConnect: true, Security: wifi.SecurityWPA, IsVisible: true},
		{SSID: "TacoBoutAGoodSignal", Strength: 99, Security: wifi.SecurityWPA, IsVisible: true},
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
			SSID:     "HideYoKidsHideYoWiFi",
			Strength: 25,
			IsKnown:  true,
			Security: wifi.SecurityWPA,
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
	// if shouldScan {
	// 	s := rand.NewSource(time.Now().Unix())
	// 	r := rand.New(s)
	// 	for i := range m.VisibleConnections {
	// 		if m.VisibleConnections[i].Strength > 0 {
	// 			m.VisibleConnections[i].Strength = uint8(r.Intn(70) + 30)
	// 		}
	// 	}
	// }

	// Reproduce bug from networkmanager backend:
	// Iterate through visible connections and append them to the result list,
	// potentially creating duplicates if multiple APs with the same SSID exist.
	// We still maintain a map to simulate the "updating best AP" part of the bug,
	// but we mistakenly append to the result list every time we see a new or better AP.

	processed := make(map[string]wifi.Connection)
	var result []wifi.Connection

	// Process visible connections with the buggy logic
	for _, c := range m.VisibleConnections {
		if existing, ok := processed[c.SSID]; ok {
			if c.Strength <= existing.Strength {
				continue
			}
		}
		processed[c.SSID] = c

		// This is the bug: we append to the result list even if we've already added this SSID.
		// In the real bug, this happened because the append was inside the loop over APs.
		// We need to ensure we populate IsKnown correctly here too.

		connToAdd := c
		isKnown := false
		for _, kc := range m.KnownConnections {
			if kc.SSID == c.SSID {
				isKnown = true
				// Merge known connection details
				knownConn := kc.Connection
				connToAdd.IsKnown = true
				connToAdd.AutoConnect = knownConn.AutoConnect
				connToAdd.Security = knownConn.Security
				connToAdd.LastConnected = knownConn.LastConnected
				// Preserve the visible strength and active status
				// connToAdd.Strength = c.Strength (already set)
				// connToAdd.IsActive = c.IsActive (already set)
				break
			}
		}
		if !isKnown {
			connToAdd.IsKnown = false
			connToAdd.AutoConnect = false
		}

		result = append(result, connToAdd)
	}

	// Also ensure known connections that weren't visible are added (if that's desired behavior).
	// The original mock logic added known connections even if not visible.
	for _, kc := range m.KnownConnections {
		if _, ok := processed[kc.SSID]; !ok {
			result = append(result, kc.Connection)
			processed[kc.SSID] = kc.Connection
		}
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
