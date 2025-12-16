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

// AccessPoint represents a physical access point for a network.
type AccessPoint struct {
	BSSID     string
	Strength  uint8
	Frequency uint32 // In MHz
}

// Connection represents a single network, visible or known.
type Connection struct {
	SSID          string
	IsActive      bool
	IsKnown       bool
	IsSecure      bool
	IsVisible     bool
	IsHidden      bool
	Strength      uint8 // 0-100 (Max of all APs)
	Security      SecurityType
	LastConnected *time.Time
	AutoConnect   bool
	AccessPoints  []AccessPoint
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
	// If bssid is empty, the best available access point is chosen.
	ActivateConnection(ssid, bssid string) error
	// ForgetNetwork removes a known network configuration.
	ForgetNetwork(ssid string) error
	// JoinNetwork connects to a new network, potentially creating a new configuration.
	// If bssid is empty, the best available access point is chosen.
	JoinNetwork(ssid, password string, security SecurityType, isHidden bool, bssid string) error
	// GetSecrets retrieves the password for a known connection.
	GetSecrets(ssid string) (string, error)
	// UpdateConnection updates a known connection.
	UpdateConnection(ssid string, opts UpdateOptions) error

	// IsWirelessEnabled checks if the wireless radio is enabled.
	IsWirelessEnabled() (bool, error)
	// SetWireless enables or disables the wireless radio.
	SetWireless(enabled bool) error
}
