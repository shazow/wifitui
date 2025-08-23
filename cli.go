package main

import (
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
)


func runTUI() error {
	m, err := initialModel()
	if err != nil {
		return fmt.Errorf("error initializing model: %w", err)
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %w", err)
	}
	return nil
}

func runList(w io.Writer, verbose bool) error {
	backend, err := NewBackend()
	if err != nil {
		return fmt.Errorf("failed to initialize backend: %w", err)
	}

	connections, err := backend.BuildNetworkList(true)
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	for _, c := range connections {
		item := connectionItem{Connection: c}
		fmt.Fprintf(w, "%s\t%s\n", item.plainTitle(), item.plainDescription())
	}

	return nil
}

func runShow(w io.Writer, verbose bool, ssid string) error {
	backend, err := NewBackend()
	if err != nil {
		return fmt.Errorf("failed to initialize backend: %w", err)
	}

	connections, err := backend.BuildNetworkList(true)
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	for _, c := range connections {
		if c.SSID == ssid {
			fmt.Fprintf(w, "SSID: %s\n", c.SSID)
			fmt.Fprintf(w, "Active: %t\n", c.IsActive)
			fmt.Fprintf(w, "Known: %t\n", c.IsKnown)
			fmt.Fprintf(w, "Secure: %t\n", c.IsSecure)
			fmt.Fprintf(w, "Visible: %t\n", c.IsVisible)
			fmt.Fprintf(w, "Hidden: %t\n", c.IsHidden)
			fmt.Fprintf(w, "Strength: %d%%\n", c.Strength)
			if c.LastConnected != nil {
				fmt.Fprintf(w, "Last Connected: %s\n", formatDuration(*c.LastConnected))
			}
			return nil
		}
	}

	return fmt.Errorf("network not found: %s", ssid)
}
