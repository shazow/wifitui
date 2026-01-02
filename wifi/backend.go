// AddAccessPoint adds the access points from 'other' to this connection.
// It returns ErrAccessPointMismatch if the security or SSID do not match.
// It also merges other metadata (Active, Visible, Known, etc.) if applicable.
func (c *Connection) AddAccessPoint(other Connection) error {
	if c.SSID != other.SSID || c.Security != other.Security {
		return ErrAccessPointMismatch
	}

	c.AccessPoints = append(c.AccessPoints, other.AccessPoints...)

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
