package mock

import (
	"fmt"
	"math/rand"
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

// NewBackend creates a new mock.Backend with a list of fun wifi networks.
func New() backend.Backend {
	networks := []string{
		"HideYoKidsHideYoWiFi",
		"GET off my LAN",
		"NeverGonnaGiveYouIP",
		"Unencrypted_Honeypot",
		"YourWiFi.exe",
		"I See Dead Packets",
		"Dunder MiffLAN",
		"Police Surveillance 2",
		"I Believe Wi Can Fi",
		"Hot singles in your area",
		"Password is password",
		"TacoBoutAGoodSignal",
		"Wi-Fight the Feeling?",
		"xX_D4rkR0ut3r_Xx",
		"Luke, I am your WiFi",
		"FreeHugsAndWiFi",
	}

	connections := make([]backend.Connection, len(networks))
	secrets := make(map[string]string)

	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)

	for i, ssid := range networks {
		isSecure := ssid != "Unencrypted_Honeypot" && ssid != "FreeHugsAndWiFi"
		connections[i] = backend.Connection{
			SSID:      ssid,
			IsVisible: true,
			Strength:  uint8(r.Intn(70) + 30), // 30-100
			IsSecure:  isSecure,
		}
		if isSecure {
			if ssid == "Password is password" {
				secrets[ssid] = "password"
			}
		}
	}

	return &Backend{
		Connections: connections,
		Secrets:     secrets,
	}
}

func (m *Backend) BuildNetworkList(shouldScan bool) ([]backend.Connection, error) {
	// For mock, we can re-randomize strengths on each scan
	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)
	for i := range m.Connections {
		m.Connections[i].Strength = uint8(r.Intn(70) + 30)
	}
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
