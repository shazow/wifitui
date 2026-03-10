package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shazow/wifitui/internal/helpers"
	"github.com/shazow/wifitui/internal/tui"
	"github.com/shazow/wifitui/wifi"
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

// writeJSON encodes v as indented JSON and writes it to w.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// filterVisibleConnections returns only the connections that are currently visible.
func filterVisibleConnections(connections []wifi.Connection) []wifi.Connection {
	var visible []wifi.Connection
	for _, c := range connections {
		if c.IsVisible {
			visible = append(visible, c)
		}
	}
	return visible
}

// findConnectionBySSID returns the first connection matching the given SSID and true,
// or the zero value and false if no match is found.
func findConnectionBySSID(connections []wifi.Connection, ssid string) (wifi.Connection, bool) {
	for _, c := range connections {
		if c.SSID == ssid {
			return c, true
		}
	}
	return wifi.Connection{}, false
}

// writeConnectionDetails writes human-readable details for a connection to w.
func writeConnectionDetails(w io.Writer, c wifi.Connection, secret string) error {
	var writeErr error
	write := func(format string, args ...any) {
		if writeErr != nil {
			return
		}
		_, writeErr = fmt.Fprintf(w, format, args...)
	}
	write("SSID: %s\n", c.SSID)
	write("Passphrase: %s\n", secret)
	write("Active: %t\n", c.IsActive)
	write("Known: %t\n", c.IsKnown)
	write("Secure: %t\n", c.IsSecure)
	write("Visible: %t\n", c.IsVisible)
	write("Hidden: %t\n", c.IsHidden)
	write("Strength: %d%%\n", c.Strength())
	if c.LastConnected != nil {
		write("Last Connected: %s\n", helpers.FormatDuration(*c.LastConnected))
	}
	return writeErr
}

func runList(w io.Writer, jsonOut bool, all bool, scan bool, b wifi.Backend) error {
	connections, err := b.BuildNetworkList(scan)
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	if !all {
		connections = filterVisibleConnections(connections)
	}

	if jsonOut {
		return writeJSON(w, connections)
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

	c, found := findConnectionBySSID(connections, ssid)
	if !found {
		return fmt.Errorf("network not found: %s: %w", ssid, wifi.ErrNotFound)
	}

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
		return writeJSON(w, connectionWithSecret{Connection: c, Passphrase: secret})
	}

	return writeConnectionDetails(w, c, secret)
}

func attemptConnect(opts wifi.JoinOptions, shouldScan bool, b wifi.Backend) error {
	// Populate the backend's internal state (e.g. NetworkManager's Connections
	// and AccessPoints maps).
	// ActivateConnection and JoinNetwork rely on this state being present.
	if _, err := b.BuildNetworkList(shouldScan); err != nil {
		return fmt.Errorf("failed to load networks: %w", err)
	}

	if opts.Password != "" || opts.Identity != "" || opts.IsHidden {
		return b.JoinNetwork(opts)
	}

	return b.ActivateConnection(opts.SSID)
}

// RetryConfig defines the configuration for connection retries.
type RetryConfig struct {
	// Total is the maximum duration to keep retrying the connection.
	Total time.Duration
	// Interval is the duration to wait between each retry attempt.
	Interval time.Duration
}

func runConnect(w io.Writer, opts wifi.JoinOptions, retry RetryConfig, b wifi.Backend) error {
	start := time.Now()
	shouldScan := false

	for {
		fmt.Fprintf(w, "Connecting to network %q with scan=%v...\n", opts.SSID, shouldScan)

		err := attemptConnect(opts, shouldScan, b)
		if err == nil {
			return nil
		}

		if !shouldScan {
			shouldScan = true
			if retry.Total > 0 && time.Since(start) < retry.Total {
				fmt.Fprintf(w, "Quick connect failed: %q\n", err)
				continue
			}
		}

		if retry.Total == 0 || time.Since(start) >= retry.Total {
			return err
		}

		fmt.Fprintf(w, "Connection failed: %q\nRetrying in %v...\n", err, retry.Interval)
		time.Sleep(retry.Interval)
	}
}

func runRadio(w io.Writer, action string, b wifi.Backend) error {
	var enabled bool
	switch action {
	case "on":
		enabled = true
	case "off":
		enabled = false
	case "", "toggle":
		current, err := b.IsWirelessEnabled()
		if err != nil {
			return fmt.Errorf("failed to get wireless state: %w", err)
		}
		enabled = !current
	default:
		return fmt.Errorf("invalid radio action: %q (expected on, off, or toggle)", action)
	}

	if enabled {
		fmt.Fprintln(w, "Enabling WiFi radio...")
	} else {
		fmt.Fprintln(w, "Disabling WiFi radio...")
	}

	if err := b.SetWireless(enabled); err != nil {
		return fmt.Errorf("failed to set wireless state: %w", err)
	}

	if enabled {
		fmt.Fprintln(w, "WiFi radio is on")
	} else {
		fmt.Fprintln(w, "WiFi radio is off")
	}
	return nil
}
