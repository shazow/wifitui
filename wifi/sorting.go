package wifi

import "sort"

// SortNetworks sorts a slice of Network structs in place.
// The sorting order is:
// 1. Active network first.
// 2. Visible networks, sorted by signal strength (strongest first).
// 3. Non-visible known networks, sorted by LastConnected timestamp (most recent first).
// 4. Fallback to SSID alphabetically.
func SortNetworks(networks []Network) {
	sort.SliceStable(networks, func(i, j int) bool {
		a := networks[i]
		b := networks[j]

		// Active networks first.
		if a.IsActive != b.IsActive {
			return a.IsActive
		}

		// Visible networks before non-visible.
		if a.IsVisible != b.IsVisible {
			return a.IsVisible
		}

		if a.IsVisible {
			// Both are visible. Sort by strength descending.
			if a.Strength() != b.Strength() {
				return a.Strength() > b.Strength()
			}
		} else {
			// Both are not visible. Sort by LastConnected descending.
			if a.LastConnected != nil && b.LastConnected == nil {
				return true
			}
			if a.LastConnected == nil && b.LastConnected != nil {
				return false
			}
			if a.LastConnected != nil && b.LastConnected != nil {
				if !a.LastConnected.Equal(*b.LastConnected) {
					return a.LastConnected.After(*b.LastConnected)
				}
			}
		}

		// Fallback to sorting by SSID.
		return a.SSID < b.SSID
	})
}

// SortAccessPoints sorts by {Signal Strength, Frequency} descending.
func SortAccessPoints(accessPoints []AccessPoint) {
	sort.SliceStable(accessPoints, func(i, j int) bool {
		a := accessPoints[i]
		b := accessPoints[j]

		if a.Strength != b.Strength {
			return a.Strength > b.Strength
		}
		return a.Frequency > b.Frequency
	})
}
