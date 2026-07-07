package mock

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/shazow/wifitui/wifi"
)

var DefaultActionSleep = 500 * time.Millisecond

// mockNetwork wraps a backend.Network with mock-specific metadata.
type mockNetwork struct {
	wifi.Network
	Secret string
}

// MockBackend is a mock implementation of the backend.Backend interface for testing.
type MockBackend struct {
	VisibleNetworks        []wifi.Network
	KnownNetworks          []mockNetwork
	ActiveNetworkIndex     int
	ActivateError          error
	ForgetError            error
	JoinError              error
	GetSecretsError        error
	UpdateNetworkError     error
	WirelessEnabled        bool
	IsWirelessEnabledError error
	SetWirelessError       error

	// DisableRandomization prevents signal strength changes on scan, useful for deterministic testing.
	DisableRandomization bool

	// ActionSleep is a delay before every action, to better emulate a real-world backend for the frontend. Set to 0 during testing.
	ActionSleep time.Duration
}

func ago(duration time.Duration) *time.Time {
	t := time.Now().Add(-duration)
	return &t
}

// New creates a new mock.Backend with a list of fun wifi networks.
func New() (wifi.Backend, error) {
	initialNetworks := []wifi.Network{
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
		{SSID: "TacoBoutAGoodSignal", AccessPoints: []wifi.AccessPoint{{Strength: 99}}, Security: wifi.SecurityWPA, IsVisible: true},
		{SSID: "Wi-Fight the Feeling?", Security: wifi.SecurityWEP},
		{SSID: "xX_D4rkR0ut3r_Xx", Security: wifi.SecurityWPA},
		{SSID: "Luke I am your WiFi", Security: wifi.SecurityWEP},
		{SSID: "FreeHugsAndWiFi", LastConnected: ago(400 * time.Hour), Security: wifi.SecurityWPA},
		// Multi-AP test
		{SSID: "Mesh Network", IsVisible: true, IsKnown: true, Security: wifi.SecurityWPA, AccessPoints: []wifi.AccessPoint{
			{BSSID: "AA:BB:CC:DD:EE:03", Strength: 95, Frequency: 5240},
			{BSSID: "AA:BB:CC:DD:EE:01", Strength: 80, Frequency: 2412},
			{BSSID: "AA:BB:CC:DD:EE:02", Strength: 40, Frequency: 5180},
			{BSSID: "AA:BB:CC:DD:EE:04", Strength: 20, Frequency: 2462},
		}},

		// Aggregated APs example (instead of duplicates)
		{SSID: "Password is password", AccessPoints: []wifi.AccessPoint{
			{Strength: 87},
			{Strength: 67},
			{Strength: 91},
		}, LastConnected: ago(1), IsKnown: true, AutoConnect: true, Security: wifi.SecurityWPA, IsVisible: true, IsActive: true},
	}
	secrets := map[string]string{
		"Password is password": "password",
		"HideYoKidsHideYoWiFi": "hidden",
	}

	var knownNetworks []mockNetwork
	for _, c := range initialNetworks {
		if c.IsKnown {
			knownNetworks = append(knownNetworks, mockNetwork{
				Network: c,
				Secret:  secrets[c.SSID],
			})
		}
	}

	// For testing duplicate SSIDs (different security/known status, although in reality SSID+Security is the key, but backend usually keys by SSID)
	knownNetworks = append(knownNetworks, mockNetwork{
		Network: wifi.Network{
			SSID:         "HideYoKidsHideYoWiFi",
			AccessPoints: []wifi.AccessPoint{{Strength: 25}},
			IsKnown:      true,
			Security:     wifi.SecurityWPA,
		},
		Secret: "different_secret",
	})

	return &MockBackend{
		VisibleNetworks:    initialNetworks,
		KnownNetworks:      knownNetworks,
		ActiveNetworkIndex: -1, // No network active initially
		ActionSleep:        DefaultActionSleep,
		WirelessEnabled:    true,
	}, nil
}

// setActiveNetwork sets the active network and ensures all other networks are inactive.
func (m *MockBackend) setActiveNetwork(ssid string) {
	m.ActiveNetworkIndex = -1
	for i := range m.KnownNetworks {
		isActive := m.KnownNetworks[i].SSID == ssid
		m.KnownNetworks[i].IsActive = isActive
		if isActive {
			m.ActiveNetworkIndex = i
		}
	}

	// Also update the visible networks slice for consistency
	for i := range m.VisibleNetworks {
		m.VisibleNetworks[i].IsActive = (m.VisibleNetworks[i].SSID == ssid)
	}
}

func (m *MockBackend) ListNetworks(scan wifi.ScanMode) (wifi.NetworksResult, error) {
	time.Sleep(m.ActionSleep)

	if !m.WirelessEnabled {
		return wifi.NetworksResult{}, wifi.ErrWirelessDisabled
	}
	// For mock, we can re-randomize strengths on each scan
	if scan != wifi.ScanNever && !m.DisableRandomization {
		s := rand.NewSource(time.Now().Unix())
		r := rand.New(s)
		for i := range m.VisibleNetworks {
			for j := range m.VisibleNetworks[i].AccessPoints {
				if m.VisibleNetworks[i].AccessPoints[j].Strength > 0 {
					m.VisibleNetworks[i].AccessPoints[j].Strength = uint8(r.Intn(70) + 30)
				}
			}
		}
	}

	// Aggregate and prepare result
	// Note: In this new version of MockBackend, we assume VisibleNetworks are already aggregated in New().
	// But we still need to merge known networks if they are not visible?
	// The original logic was complex because it simulated a bug. Now we simulate "correct" behavior.

	processed := make(map[string]wifi.Network)

	// Process visible networks
	for _, c := range m.VisibleNetworks {
		// In previous "buggy" version we appended blindly. Now we use a map if we want to ensure uniqueness,
		// but since we manually constructed the list in New(), let's assume they are unique enough for mock purposes.
		// However, to be safe and consistent with other backends, let's map by SSID.

		networkToAdd := c

		// Check against known networks to update status
		isKnown := false
		for _, kc := range m.KnownNetworks {
			if kc.SSID == c.SSID {
				isKnown = true
				// Merge known network details
				knownNetwork := kc.Network
				networkToAdd.IsKnown = true
				networkToAdd.AutoConnect = knownNetwork.AutoConnect
				networkToAdd.Security = knownNetwork.Security
				networkToAdd.LastConnected = knownNetwork.LastConnected
				break
			}
		}
		if !isKnown {
			networkToAdd.IsKnown = false
			networkToAdd.AutoConnect = false
		}

		processed[c.SSID] = networkToAdd
	}

	// Add known networks that weren't visible
	for _, kc := range m.KnownNetworks {
		if _, ok := processed[kc.SSID]; !ok {
			processed[kc.SSID] = kc.Network
		}
	}

	var result []wifi.Network
	for _, c := range processed {
		wifi.SortAccessPoints(c.AccessPoints)
		result = append(result, c)
	}

	return wifi.NetworksResult{Networks: result}, nil
}

func (m *MockBackend) ActivateNetwork(ssid string) error {
	time.Sleep(m.ActionSleep)

	if m.ActivateError != nil {
		return m.ActivateError
	}
	// "Act on first match" logic for ambiguity.
	for i, c := range m.KnownNetworks {
		if c.SSID == ssid {
			m.setActiveNetwork(ssid)
			now := time.Now()
			m.KnownNetworks[i].LastConnected = &now
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
	if m.ActiveNetworkIndex >= 0 && m.ActiveNetworkIndex < len(m.KnownNetworks) {
		activeSSID = m.KnownNetworks[m.ActiveNetworkIndex].SSID
	}

	var newKnownNetworks []mockNetwork
	found := false
	for _, c := range m.KnownNetworks {
		if c.SSID == ssid {
			found = true
		} else {
			newKnownNetworks = append(newKnownNetworks, c)
		}
	}

	if !found {
		return fmt.Errorf("network not found: %s: %w", ssid, wifi.ErrNotFound)
	}

	m.KnownNetworks = newKnownNetworks

	if activeSSID == ssid {
		m.setActiveNetwork("") // Deactivate all
	} else {
		m.setActiveNetwork(activeSSID) // Re-sync active network
	}

	return nil
}

func (m *MockBackend) JoinNetwork(ssid string, password string, security wifi.SecurityType, isHidden bool) error {
	time.Sleep(m.ActionSleep)

	if m.JoinError != nil {
		return m.JoinError
	}

	var c wifi.Network
	found := false
	foundIndex := -1
	for i, vc := range m.VisibleNetworks {
		if vc.SSID == ssid {
			c = vc
			found = true
			foundIndex = i
			break
		}
	}
	if !found {
		c = wifi.Network{
			SSID:     ssid,
			Security: security,
			IsHidden: isHidden,
		}
	}

	c.IsKnown = true
	c.AutoConnect = true
	if found {
		m.VisibleNetworks[foundIndex] = c
	}

	newNetwork := mockNetwork{
		Network: c,
		Secret:  password,
	}

	// Check if we are replacing an existing known connection, otherwise append.
	foundInKnown := false
	for i, kc := range m.KnownNetworks {
		if kc.SSID == ssid {
			m.KnownNetworks[i] = newNetwork
			foundInKnown = true
			break
		}
	}
	if !foundInKnown {
		m.KnownNetworks = append(m.KnownNetworks, newNetwork)
	}

	m.setActiveNetwork(ssid)
	now := time.Now()
	if m.ActiveNetworkIndex != -1 {
		m.KnownNetworks[m.ActiveNetworkIndex].LastConnected = &now
	}

	return nil
}

func (m *MockBackend) GetSecrets(ssid string) (string, error) {
	time.Sleep(m.ActionSleep)

	if m.GetSecretsError != nil {
		return "", m.GetSecretsError
	}
	// "Act on first match" logic for ambiguity.
	for _, c := range m.KnownNetworks {
		if c.SSID == ssid {
			return c.Secret, nil
		}
	}
	return "", fmt.Errorf("no secrets for %s: %w", ssid, wifi.ErrNotFound)
}

func (m *MockBackend) UpdateNetwork(ssid string, opts wifi.UpdateOptions) error {
	time.Sleep(m.ActionSleep)

	if m.UpdateNetworkError != nil {
		return m.UpdateNetworkError
	}
	// "Act on first match" logic for ambiguity.
	for i, c := range m.KnownNetworks {
		if c.SSID == ssid {
			if opts.Password != nil {
				m.KnownNetworks[i].Secret = *opts.Password
			}
			if opts.AutoConnect != nil {
				m.KnownNetworks[i].AutoConnect = *opts.AutoConnect
			}
			return nil
		}
	}
	return fmt.Errorf("cannot update network for unknown network %s: %w", ssid, wifi.ErrNotFound)
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
