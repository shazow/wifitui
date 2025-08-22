package main

import "sort"

// sortConnections sorts a slice of Connection structs in place.
// The sorting order is:
// 1. Active connections first.
// 2. Visible connections first.
// 3. For visible connections, by strength (strongest first).
// 4. By SSID alphabetically.
func sortConnections(connections []Connection) {
	sort.SliceStable(connections, func(i, j int) bool {
		if connections[i].IsActive != connections[j].IsActive {
			return connections[i].IsActive
		}
		if connections[i].IsVisible != connections[j].IsVisible {
			return connections[i].IsVisible
		}
		// If both are visible, sort by strength. Otherwise, sort by name.
		if connections[i].IsVisible {
			if connections[i].Strength != connections[j].Strength {
				return connections[i].Strength > connections[j].Strength
			}
		}
		return connections[i].SSID < connections[j].SSID
	})
}
