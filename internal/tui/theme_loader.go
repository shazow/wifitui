package tui

import (
	"os"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/lipgloss"
)

// themeFile represents the structure of the theme TOML file.
// We use pointers to strings so we can distinguish between a missing value
// and an empty string. This allows users to override only the colors they want.
type themeFile struct {
	Primary    *string `toml:"Primary,omitempty"`
	Subtle     *string `toml:"Subtle,omitempty"`
	Success    *string `toml:"Success,omitempty"`
	Error      *string `toml:"Error,omitempty"`
	Normal     *string `toml:"Normal,omitempty"`
	Disabled   *string `toml:"Disabled,omitempty"`
	Border     *string `toml:"Border,omitempty"`
	SignalHigh *string `toml:"SignalHigh,omitempty"`
	SignalLow  *string `toml:"SignalLow,omitempty"`
}

// LoadTheme loads a theme from the given path and overrides the default theme.
// If the path is empty, it does nothing.
func LoadTheme(path string) error {
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var tf themeFile
	if err := toml.Unmarshal(data, &tf); err != nil {
		return err
	}

	// Start with the default theme and override it with the loaded values.
	theme := NewDefaultTheme()

	if tf.Primary != nil {
		theme.Primary = lipgloss.Color(*tf.Primary)
	}
	if tf.Subtle != nil {
		theme.Subtle = lipgloss.Color(*tf.Subtle)
	}
	if tf.Success != nil {
		theme.Success = lipgloss.Color(*tf.Success)
	}
	if tf.Error != nil {
		theme.Error = lipgloss.Color(*tf.Error)
	}
	if tf.Normal != nil {
		theme.Normal = lipgloss.Color(*tf.Normal)
	}
	if tf.Disabled != nil {
		theme.Disabled = lipgloss.Color(*tf.Disabled)
	}
	if tf.Border != nil {
		theme.Border = lipgloss.Color(*tf.Border)
	}
	if tf.SignalHigh != nil {
		theme.SignalHigh = lipgloss.Color(*tf.SignalHigh)
	}
	if tf.SignalLow != nil {
		theme.SignalLow = lipgloss.Color(*tf.SignalLow)
	}

	CurrentTheme = theme
	return nil
}
