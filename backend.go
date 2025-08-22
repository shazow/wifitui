package main

// Connection represents a single network, visible or known.
type Connection struct {
	SSID      string
	IsActive  bool
	IsKnown   bool
	IsSecure  bool
	IsVisible bool
	Strength  uint8 // 0-100
}

// Backend defines the interface for managing network connections.
type Backend interface {
	// BuildNetworkList scans (if shouldScan is true) and returns all networks.
	BuildNetworkList(shouldScan bool) ([]Connection, error)
	// ActivateConnection activates a known network.
	ActivateConnection(conn Connection) error
	// ForgetNetwork removes a known network configuration.
	ForgetNetwork(conn Connection) error
	// JoinNetwork connects to a new network, potentially creating a new configuration.
	JoinNetwork(conn Connection, password string) error
	// GetSecrets retrieves the password for a known connection.
	GetSecrets(conn Connection) (string, error)
	// UpdateSecret changes the password for a known connection.
	UpdateSecret(conn Connection, newPassword string) error
}
