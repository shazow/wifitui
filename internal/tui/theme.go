package tui

import (
	"fmt"
	"io"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// Color is a wrapper around lipgloss.TerminalColor that can be unmarshaled
// from a TOML file as either a single color string or a pair of strings for light/dark themes.
type Color struct {
	lipgloss.TerminalColor
}

// UnmarshalTOML implements the toml.Unmarshaler interface.
func (c *Color) UnmarshalTOML(data interface{}) error {
	switch value := data.(type) {
	case string:
		c.TerminalColor = lipgloss.Color(value)
		return nil
	case []interface{}:
		if len(value) != 2 {
			return fmt.Errorf("color must be a pair of two strings, but got %d", len(value))
		}
		s := make([]string, 2)
		for i, item := range value {
			var ok bool
			s[i], ok = item.(string)
			if !ok {
				return fmt.Errorf("color pair must contain strings, but got %T", item)
			}
		}
		c.TerminalColor = lipgloss.AdaptiveColor{Light: s[0], Dark: s[1]}
		return nil
	}

	return fmt.Errorf("unsupported type for Color: %T", data)
}

// Theme contains the colors for the application.
type Theme struct {
	// Generic color scheme
	Primary  Color
	Subtle   Color
	Success  Color
	Error    Color
	Normal   Color
	Disabled Color
	Border   Color

	// Feature-specific colors
	SignalHigh Color
	SignalLow  Color
	Saved      Color

	// Icons
	TitleIcon          string
	NetworkSecureIcon  string
	NetworkOpenIcon    string
	NetworkUnknownIcon string
	NetworkSavedIcon   string
}

// CurrentTheme is the active theme for the application.
var CurrentTheme = NewDefaultTheme()

// NewDefaultTheme creates a new default theme.
func NewDefaultTheme() Theme {
	return Theme{
		Primary:    Color{lipgloss.AdaptiveColor{Light: "#FFA500", Dark: "#FFA500"}}, // Orange
		Subtle:     Color{lipgloss.AdaptiveColor{Light: "#BDBDBD", Dark: "#919191"}}, // Gray
		Success:    Color{lipgloss.AdaptiveColor{Light: "#388E3C", Dark: "#81C784"}}, // Green
		Error:      Color{lipgloss.AdaptiveColor{Light: "#D32F2F", Dark: "#E57373"}}, // Red
		Normal:     Color{lipgloss.AdaptiveColor{Light: "#212121", Dark: "#EEEEEE"}}, // Black/White
		Disabled:   Color{lipgloss.AdaptiveColor{Light: "#E0E0E0", Dark: "#626262"}}, // Lighter/Darker Gray
		Border:     Color{lipgloss.AdaptiveColor{Light: "#BDBDBD", Dark: "#616161"}}, // Gray
		SignalHigh: Color{lipgloss.AdaptiveColor{Light: "#00B300", Dark: "#00FF00"}},
		SignalLow:  Color{lipgloss.AdaptiveColor{Light: "#D05F00", Dark: "#BC3C00"}},
		Saved:      Color{lipgloss.AdaptiveColor{Light: "#00459E", Dark: "#54A5F6"}},

		TitleIcon:          "🛜 ",
		NetworkSecureIcon:  "🔒 ",
		NetworkOpenIcon:    "🔓 ",
		NetworkUnknownIcon: "❓ ",
		NetworkSavedIcon:   "💾 ",
	}
}

// LoadTheme loads a theme from the given reader and returns a Theme object.
func LoadTheme(r io.Reader) (Theme, error) {
	if r == nil {
		return Theme{}, fmt.Errorf("reader cannot be nil")
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return Theme{}, err
	}

	var theme Theme
	if err := toml.Unmarshal(data, &theme); err != nil {
		return Theme{}, err
	}

	return theme, nil
}

// FormatSignalStrength returns a color based on the signal strength.
func (theme *Theme) FormatSignalStrength(strength uint8) string {
	var signalHigh, signalLow string
	if adaptiveHigh, ok := theme.SignalHigh.TerminalColor.(lipgloss.AdaptiveColor); ok {
		if adaptiveLow, ok := theme.SignalLow.TerminalColor.(lipgloss.AdaptiveColor); ok {
			if lipgloss.HasDarkBackground() {
				signalHigh = adaptiveHigh.Dark
				signalLow = adaptiveLow.Dark
			} else {
				signalHigh = adaptiveHigh.Light
				signalLow = adaptiveLow.Light
			}
		}
	}
	start, _ := colorful.Hex(signalLow)
	end, _ := colorful.Hex(signalHigh)
	p := float64(strength) / 100.0
	blend := start.BlendRgb(end, p)
	c := lipgloss.Color(blend.Hex())
	return lipgloss.NewStyle().Foreground(c).Render(fmt.Sprintf("%d%%", strength))
}
