package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shazow/wifitui/wifi"
	"github.com/shazow/wifitui/internal/tui"
	"github.com/shazow/wifitui/internal/helpers"
)

func runTUI(b wifi.Backend) error {
	m, err := tui.NewModel(b)
	if err != nil {
		return fmt.Errorf("error initializing model: %w", err)
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %w", err)
	}
	return nil
}

func formatConnection(c wifi.Connection) string {
	var parts []string
	if c.IsVisible {
		parts = append(parts, fmt.Sprintf("%d%%", c.Strength()))
		parts = append(parts, "visible")
	}
	if c.IsSecure {
		parts = append(parts, "secure")
	}
	if c.IsActive {
		parts = append(parts, "active")
	}

	return strings.Join(parts, ", ")
}

func runList(w io.Writer, jsonOut bool, all bool, scan bool, b wifi.Backend) error {
	connections, err := b.BuildNetworkList(scan)
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	if !all {
		var visible []wifi.Connection
		for _, c := range connections {
			if c.IsVisible {
				visible = append(visible, c)
			}
		}
		connections = visible
	}

	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(connections)
	}

	for _, c := range connections {
		fmt.Fprintf(w, "%s\t%s\n", c.SSID, formatConnection(c))
	}

	return nil
}

func runShow(w io.Writer, jsonOut bool, ssid string, b wifi.Backend) error {
	connections, err := b.BuildNetworkList(true)
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	for _, c := range connections {
		if c.SSID == ssid {
			secret, err := b.GetSecrets(ssid)
			if err != nil {
				// If we can't get a secret for a known network, that's an error.
				// But for a visible-only network, it's expected.
				if c.IsKnown {
					return fmt.Errorf("failed to get network secret: %w", err)
				}
				secret = "" // No secret available
			}

			if jsonOut {
				// We need a custom struct to include the passphrase
				type connectionWithSecret struct {
					wifi.Connection
					Passphrase string `json:"passphrase,omitempty"`
				}
				data := connectionWithSecret{
					Connection: c,
					Passphrase: secret,
				}
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				return enc.Encode(data)
			}

			fmt.Fprintf(w, "SSID: %s\n", c.SSID)
			fmt.Fprintf(w, "Passphrase: %s\n", secret)
			fmt.Fprintf(w, "Active: %t\n", c.IsActive)
			fmt.Fprintf(w, "Known: %t\n", c.IsKnown)
			fmt.Fprintf(w, "Secure: %t\n", c.IsSecure)
			fmt.Fprintf(w, "Visible: %t\n", c.IsVisible)
			fmt.Fprintf(w, "Hidden: %t\n", c.IsHidden)
			fmt.Fprintf(w, "Strength: %d%%\n", c.Strength())
			if c.LastConnected != nil {
				fmt.Fprintf(w, "Last Connected: %s\n", helpers.FormatDuration(*c.LastConnected))
			}
			return nil
		}
	}

	return fmt.Errorf("network not found: %s: %w", ssid, wifi.ErrNotFound)
}

func runConnect(w io.Writer, ssid string, passphrase string, security wifi.SecurityType, isHidden bool, b wifi.Backend) error {
	if passphrase != "" || isHidden {
		fmt.Fprintf(w, "Joining network %q...\n", ssid)
		return b.JoinNetwork(ssid, passphrase, security, isHidden)
	}

	fmt.Fprintf(w, "Activating existing network %q...\n", ssid)
	return b.ActivateConnection(ssid)
}
