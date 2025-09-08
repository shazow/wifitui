package tui

import (
	"fmt"
	"io"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/lipgloss"
)

// adaptiveColor is a wrapper around lipgloss.TerminalColor that can be unmarshaled
// from a TOML file as either a single color string or a pair of strings for light/dark themes.
type adaptiveColor struct {
	lipgloss.TerminalColor
}

// UnmarshalTOML implements the toml.Unmarshaler interface.
func (ac *adaptiveColor) UnmarshalTOML(data interface{}) error {
	switch value := data.(type) {
	case string:
		ac.TerminalColor = lipgloss.Color(value)
		return nil
	case []interface{}:
		if len(value) != 2 {
			return fmt.Errorf("adaptive color must be a pair of two strings, but got %d", len(value))
		}
		s := make([]string, 2)
		for i, item := range value {
			var ok bool
			s[i], ok = item.(string)
			if !ok {
				return fmt.Errorf("adaptive color pair must contain strings, but got %T", item)
			}
		}
		ac.TerminalColor = lipgloss.AdaptiveColor{Light: s[0], Dark: s[1]}
		return nil
	}

	return fmt.Errorf("unsupported type for adaptiveColor: %T", data)
}

// themeFile is a private struct used for unmarshaling the TOML theme file.
// It uses the adaptiveColor type to handle flexible color definitions.
type themeFile struct {
	Primary    adaptiveColor `toml:"Primary"`
	Subtle     adaptiveColor `toml:"Subtle"`
	Success    adaptiveColor `toml:"Success"`
	Error      adaptiveColor `toml:"Error"`
	Normal     adaptiveColor `toml:"Normal"`
	Disabled   adaptiveColor `toml:"Disabled"`
	Border     adaptiveColor `toml:"Border"`
	SignalHigh adaptiveColor `toml:"SignalHigh"`
	SignalLow  adaptiveColor `toml:"SignalLow"`
}

// LoadTheme loads a theme from the given reader and overrides the default theme.
// If the reader is nil, it does nothing.
func LoadTheme(r io.Reader) error {
	if r == nil {
		return nil
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	var tf themeFile
	if err := toml.Unmarshal(data, &tf); err != nil {
		return err
	}

	// Create a new Theme and populate it from the themeFile.
	// This keeps the main Theme struct clean and unchanged.
	CurrentTheme = Theme{
		Primary:    tf.Primary.TerminalColor,
		Subtle:     tf.Subtle.TerminalColor,
		Success:    tf.Success.TerminalColor,
		Error:      tf.Error.TerminalColor,
		Normal:     tf.Normal.TerminalColor,
		Disabled:   tf.Disabled.TerminalColor,
		Border:     tf.Border.TerminalColor,
		SignalHigh: tf.SignalHigh.TerminalColor,
		SignalLow:  tf.SignalLow.TerminalColor,
	}

	return nil
}
