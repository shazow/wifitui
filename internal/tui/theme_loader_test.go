package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestLoadTheme(t *testing.T) {
	tomlData := `
		Primary = "#FF0000"
		Subtle = ["#00FF00", "#00EE00"]
		Success = "#0000FF"
		Error = "#FFFF00"
		Normal = "#FF00FF"
		Disabled = "#00FFFF"
		Border = "#800080"
		SignalHigh = "#008000"
		SignalLow = "#FFA500"
	`

	reader := strings.NewReader(tomlData)
	err := LoadTheme(reader)
	if err != nil {
		t.Fatalf("LoadTheme failed: %v", err)
	}

	// Verify a single color
	expectedColor := lipgloss.Color("#FF0000")
	if CurrentTheme.Primary != expectedColor {
		t.Errorf("Expected Primary color to be %v, but got %v", expectedColor, CurrentTheme.Primary)
	}

	// Verify an adaptive color
	adaptiveColor, ok := CurrentTheme.Subtle.(lipgloss.AdaptiveColor)
	if !ok {
		t.Fatalf("Expected Subtle color to be an AdaptiveColor, but it's not")
	}
	if adaptiveColor.Light != "#00FF00" {
		t.Errorf("Expected Subtle light color to be #00FF00, but got %s", adaptiveColor.Light)
	}
	if adaptiveColor.Dark != "#00EE00" {
		t.Errorf("Expected Subtle dark color to be #00EE00, but got %s", adaptiveColor.Dark)
	}
}

func TestLoadTheme_NilReader(t *testing.T) {
	// Keep a copy of the original theme
	originalTheme := CurrentTheme

	err := LoadTheme(nil)
	if err != nil {
		t.Fatalf("LoadTheme(nil) should not return an error, but got: %v", err)
	}

	// Verify that the theme has not changed
	if CurrentTheme.Primary != originalTheme.Primary {
		t.Errorf("Theme should not change when reader is nil")
	}
}

func TestLoadTheme_InvalidToml(t *testing.T) {
	invalidTomlData := `Primary = `
	reader := strings.NewReader(invalidTomlData)
	err := LoadTheme(reader)
	if err == nil {
		t.Fatalf("LoadTheme should have failed for invalid TOML, but it didn't")
	}
}
