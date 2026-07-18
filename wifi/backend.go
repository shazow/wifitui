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
	Frequency uint  // MHz
}

// Network represents a single Wi-Fi network, visible or known.
type Network struct {
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
func (c Network) Strength() uint8 {
	if len(c.AccessPoints) == 0 {
		return 0
	}
	// Find the maximum strength across all access points.
	maxStrength := uint8(0)
	for _, ap := range c.AccessPoints {
		if ap.Strength > maxStrength {
			maxStrength = ap.Strength
		}
	}
	return maxStrength
}

// AddAccessPoint adds the access points from 'other' to this network.
// It returns ErrAccessPointMismatch if the security or SSID do not match.
// It also merges other metadata (Active, Visible, Known, etc.) if applicable.
func (c *Network) AddAccessPoint(other Network) error {
	if c.SSID != other.SSID || c.Security != other.Security {
		return ErrAccessPointMismatch
	}

	c.AccessPoints = append(c.AccessPoints, other.AccessPoints...)

	// Not expecting many access points so we do a full sort here rather than merging in sorted order
	SortAccessPoints(c.AccessPoints)

	if other.IsActive {
		c.IsActive = true
	}
	if other.IsVisible {
		c.IsVisible = true
	}
	if other.IsKnown {
		c.IsKnown = true
		c.AutoConnect = other.AutoConnect
		if other.LastConnected != nil {
			c.LastConnected = other.LastConnected
		}
	}
	return nil
}

// UpdateOptions specifies the properties to update for a known network.
// A nil value for a field means that the property should not be changed.
type UpdateOptions struct {
	Password    *string
	AutoConnect *bool
}

// ScanMode controls whether listing networks should request a scan first.
type ScanMode int

const (
	// ScanNever returns the backend's current network list without requesting a scan.
	ScanNever ScanMode = iota
	// ScanAuto requests a scan when the backend decides its current list is stale.
	ScanAuto
	// ScanForce requests a scan even if the backend's current list is fresh.
	ScanForce
)

// NetworksResult contains the networks returned by a list operation.
type NetworksResult struct {
	Networks []Network
	// ScanError is non-nil when a requested scan failed and Networks contains
	// fallback data instead.
	ScanError error
}

// Backend defines the interface for managing Wi-Fi networks.
type Backend interface {
	// ListNetworks returns all networks and optionally requests a scan first.
	ListNetworks(scan ScanMode) (NetworksResult, error)
	// ActivateNetwork activates a known network.
	ActivateNetwork(ssid string) error
	// ForgetNetwork removes a known network configuration.
	ForgetNetwork(ssid string) error
	// JoinNetwork connects to a new network, potentially creating a new configuration.
	JoinNetwork(ssid string, password string, security SecurityType, isHidden bool) error
	// GetSecrets retrieves the password for a known network.
	GetSecrets(ssid string) (string, error)
	// UpdateNetwork updates a known network.
	UpdateNetwork(ssid string, opts UpdateOptions) error

	// IsWirelessEnabled checks if the wireless radio is enabled.
	IsWirelessEnabled() (bool, error)
	// SetWireless enables or disables the wireless radio.
	SetWireless(enabled bool) error
}
