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
	loadedTheme, err := LoadTheme(reader)
	if err != nil {
		t.Fatalf("LoadTheme failed: %v", err)
	}

	// Verify a single color
	expectedColor := Color{lipgloss.Color("#FF0000")}
	if loadedTheme.Primary != expectedColor {
		t.Errorf("Expected Primary color to be %v, but got %v", expectedColor, loadedTheme.Primary)
	}

	// Verify an adaptive color
	adaptiveColor, ok := loadedTheme.Subtle.TerminalColor.(lipgloss.AdaptiveColor)
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
	_, err := LoadTheme(nil)
	if err == nil {
		t.Fatalf("LoadTheme(nil) should have returned an error, but it didn't")
	}
}

func TestLoadTheme_InvalidToml(t *testing.T) {
	invalidTomlData := `Primary = `
	reader := strings.NewReader(invalidTomlData)
	_, err := LoadTheme(reader)
	if err == nil {
		t.Fatalf("LoadTheme should have failed for invalid TOML, but it didn't")
	}
}
