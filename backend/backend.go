package backend

import "time"

// Connection represents a single network, visible or known.
type Connection struct {
	SSID          string
	IsActive      bool
	IsKnown       bool
	IsSecure      bool
	IsVisible     bool
	IsHidden      bool
	Strength      uint8 // 0-100
	LastConnected *time.Time
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
	JoinNetwork(ssid string, password string) error
	// GetSecrets retrieves the password for a known connection.
	GetSecrets(ssid string) (string, error)
	// UpdateSecret changes the password for a known connection.
	UpdateSecret(ssid string, newPassword string) error
}
