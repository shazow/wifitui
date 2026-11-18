package tui

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
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

		AccessPointIcon = "🛰"
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

	if loadedTheme.AccessPointIcon != "🛰" {
		t.Errorf("Expected AccessPointIcon to be 🛰, but got %q", loadedTheme.AccessPointIcon)
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

func TestFormatSignalStrength(t *testing.T) {
	previousRenderer := lipgloss.DefaultRenderer()
	t.Cleanup(func() { lipgloss.SetDefaultRenderer(previousRenderer) })
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.TrueColor)
	lipgloss.SetDefaultRenderer(renderer)

	const (
		singleHigh = `"#ff0000"`
		singleLow  = `"#0000ff"`
		pairedHigh = `["#00ff00", "#ffff00"]`
		pairedLow  = `["#0000ff", "#00ffff"]`
	)
	tests := []struct {
		name string
		dark bool
		high string
		low  string
		want [3]lipgloss.Color
	}{
		{"single on light", false, singleHigh, singleLow, [3]lipgloss.Color{"#0000ff", "#800080", "#ff0000"}},
		{"single on dark", true, singleHigh, singleLow, [3]lipgloss.Color{"#0000ff", "#800080", "#ff0000"}},
		{"single high", false, singleHigh, pairedLow, [3]lipgloss.Color{"#0000ff", "#800080", "#ff0000"}},
		{"single low", true, pairedHigh, singleLow, [3]lipgloss.Color{"#0000ff", "#808080", "#ffff00"}},
		{"pairs on light", false, pairedHigh, pairedLow, [3]lipgloss.Color{"#0000ff", "#008080", "#00ff00"}},
		{"pairs on dark", true, pairedHigh, pairedLow, [3]lipgloss.Color{"#00ffff", "#80ff80", "#ffff00"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			renderer.SetHasDarkBackground(test.dark)
			data := fmt.Sprintf("SignalHigh = %s\nSignalLow = %s\n", test.high, test.low)
			theme, err := LoadTheme(strings.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			for i, strength := range [...]uint8{0, 50, 100} {
				want := renderer.NewStyle().Foreground(test.want[i]).Render(fmt.Sprintf("%d%%", strength))
				if got := theme.FormatSignalStrength(strength); got != want {
					t.Errorf("FormatSignalStrength(%d) = %q, want %q", strength, got, want)
				}
			}
		})
	}
}
