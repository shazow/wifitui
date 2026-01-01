package wifi

import "time"

// SecurityType represents the security protocol of a network.
type SecurityType int

const (
	SecurityUnknown SecurityType = iota
	SecurityOpen
	SecurityWEP
	SecurityWPA
)

// AccessPoint represents a single access point for a network.
type AccessPoint struct {
	SSID      string
	BSSID     string
	Strength  uint8 // 0-100
	Frequency uint
}

// Connection represents a single network, visible or known.
type Connection struct {
	SSID          string
	IsActive      bool
	IsKnown       bool
	IsSecure      bool
	IsVisible     bool
	IsHidden      bool
	AccessPoints  []AccessPoint
	Security      SecurityType
	LastConnected *time.Time
	AutoConnect   bool
}

// Strength returns the strength of the strongest access point, or 0 if none.
func (c Connection) Strength() uint8 {
	if len(c.AccessPoints) == 0 {
		return 0
	}
	// Sort access points by strength descending to ensure the first one is the strongest.
	// Note: We return the strength of the first AP, assuming the list is sorted or we sort it here.
	// Since AccessPoints might be unsorted, let's find the max.
	maxStrength := uint8(0)
	for _, ap := range c.AccessPoints {
		if ap.Strength > maxStrength {
			maxStrength = ap.Strength
		}
	}
	return maxStrength
}

// UpdateOptions specifies the properties to update for a connection.
// A nil value for a field means that the property should not be changed.
type UpdateOptions struct {
	Password    *string
	AutoConnect *bool
}

// Backend defines the interface for managing network connections.
type Backend interface {
	// BuildNetworkList scans (if shouldScan is true) and returns all networks.
	BuildNetworkList(shouldScan bool) ([]Connection, error)
	// ActivateConnection activates a known network.
	ActivateConnection(ssid string) error
	// ForgetNetwork removes a known network configuration.
	ForgetNetwork(ssid string) error
	// JoinNetwork connects to a new network, potentially creating a new configuration.
	JoinNetwork(ssid string, password string, security SecurityType, isHidden bool) error
	// GetSecrets retrieves the password for a known connection.
	GetSecrets(ssid string) (string, error)
	// UpdateConnection updates a known connection.
	UpdateConnection(ssid string, opts UpdateOptions) error

	// IsWirelessEnabled checks if the wireless radio is enabled.
	IsWirelessEnabled() (bool, error)
	// SetWireless enables or disables the wireless radio.
	SetWireless(enabled bool) error
}
