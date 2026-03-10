package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	flags "github.com/jessevdk/go-flags"
	"github.com/shazow/wifitui/internal/tui"
	"github.com/shazow/wifitui/wifi"
)

var (
	// Version is the version of the application. It is set at build time.
	Version string = "dev"

	// Avoid retrying scans too frequently, else the scan requests get lost.
	defaultRetryInterval = 10 * time.Second
)

// parseSecurityType converts a security string (open, wep, wpa, wpa-eap) to a wifi.SecurityType.
func parseSecurityType(s string) (wifi.SecurityType, error) {
	switch s {
	case "open":
		return wifi.SecurityOpen, nil
	case "wep":
		return wifi.SecurityWEP, nil
	case "wpa":
		return wifi.SecurityWPA, nil
	case "wpa-eap":
		return wifi.SecurityWPAEAP, nil
	default:
		return wifi.SecurityUnknown, fmt.Errorf("invalid security type: %s", s)
	}
}

// parseRetryConfig parses a retry duration string of the form "DURATION" or "DURATION:INTERVAL"
// into a RetryConfig. The default retry interval is used when no interval is specified.
func parseRetryConfig(s string) (RetryConfig, error) {
	retry := RetryConfig{Interval: defaultRetryInterval}
	if s == "" {
		return retry, nil
	}

	parts := strings.Split(s, ":")
	var err error
	retry.Total, err = time.ParseDuration(parts[0])
	if err != nil {
		return RetryConfig{}, fmt.Errorf("invalid duration format for --retry-for: %w", err)
	}

	if len(parts) > 1 {
		retry.Interval, err = time.ParseDuration(parts[1])
		if err != nil {
			return RetryConfig{}, fmt.Errorf("invalid sleep duration format for --retry-for: %w", err)
		}
	}

	return retry, nil
}

// Options defines the root-level flags
type Options struct {
	Theme   string `long:"theme" description:"path to theme toml file" env:"WIFITUI_THEME"`
	Version bool   `long:"version" description:"display version"`

	Tui     TuiCommand     `command:"tui" description:"Run the TUI (default)"`
	List    ListCommand    `command:"list" description:"List wifi networks"`
	Show    ShowCommand    `command:"show" description:"Show a wifi network"`
	Connect ConnectCommand `command:"connect" description:"Connect to a wifi network"`
	Radio   RadioCommand   `command:"radio" description:"Control the wifi radio (on|off|toggle)"`
}

// TuiCommand defines the handler for the "tui" subcommand
type TuiCommand struct{}

// ListCommand defines the flags and arguments for the "list" subcommand
type ListCommand struct {
	JSON bool `long:"json" description:"output in JSON format"`
	All  bool `long:"all" description:"list all saved and visible networks"`
	Scan bool `long:"scan" description:"scan for new visible networks"`
}

// ShowCommand defines the flags and arguments for the "show" subcommand
type ShowCommand struct {
	JSON bool `long:"json" description:"output in JSON format"`
	Args struct {
		SSID string `positional-arg-name:"ssid" required:"true"`
	} `positional-args:"yes"`
}

// ConnectCommand defines the flags and arguments for the "connect" subcommand
type ConnectCommand struct {
	Passphrase        string `long:"passphrase" description:"passphrase for the network"`
	Identity          string `long:"identity" description:"identity/username for enterprise networks (WPA-EAP)"`
	AnonymousIdentity string `long:"anonymous-identity" description:"anonymous identity for enterprise networks"`
	EAP               string `long:"eap" description:"EAP method for enterprise networks (e.g. peap, ttls)"`
	Phase2Auth        string `long:"phase2-auth" description:"Phase 2 authentication for enterprise networks (e.g. mschapv2)"`
	Security          string `long:"security" default:"wpa" description:"security type" choice:"open" choice:"wep" choice:"wpa" choice:"wpa-eap"`
	Hidden            bool   `long:"hidden" description:"network is hidden"`
	RetryFor          string `long:"retry-for" description:"duration to retry connection (e.g. 60s or 2m:20s)" value-name:"DURATION[:INTERVAL]"`
	Args              struct {
		SSID string `positional-arg-name:"ssid" required:"true"`
	} `positional-args:"yes"`
}

// RadioCommand defines the argument for the "radio" subcommand
// Action may be one of: on, off, toggle.
type RadioCommand struct {
	Args struct {
		Action string `positional-arg-name:"action"`
	} `positional-args:"yes"`
}

// We need a global backend to be accessible by the command handlers.
var b wifi.Backend
var opts Options

// Execute is the handler for the "tui" subcommand
func (c *TuiCommand) Execute(args []string) error {
	if opts.Theme != "" {
		f, err := os.Open(opts.Theme)
		if err != nil {
			return fmt.Errorf("failed to open theme file: %w", err)
		}
		defer f.Close()
		loadedTheme, err := tui.LoadTheme(f)
		if err != nil {
			return fmt.Errorf("failed to load theme: %w", err)
		}
		tui.CurrentTheme = loadedTheme
	}
	return runTUI(b)
}

// Execute is the handler for the "list" subcommand
func (c *ListCommand) Execute(args []string) error {
	return runList(os.Stdout, c.JSON, c.All, c.Scan, b)
}

// Execute is the handler for the "show" subcommand
func (c *ShowCommand) Execute(args []string) error {
	return runShow(os.Stdout, c.JSON, c.Args.SSID, b)
}

// Execute is the handler for the "connect" subcommand
func (c *ConnectCommand) Execute(args []string) error {
	security, err := parseSecurityType(c.Security)
	if err != nil {
		return err
	}

	retry, err := parseRetryConfig(c.RetryFor)
	if err != nil {
		return err
	}

	opts := wifi.JoinOptions{
		SSID:              c.Args.SSID,
		Password:          c.Passphrase,
		Identity:          c.Identity,
		AnonymousIdentity: c.AnonymousIdentity,
		EAP:               c.EAP,
		Phase2Auth:        c.Phase2Auth,
		Security:          security,
		IsHidden:          c.Hidden,
	}

	return runConnect(os.Stdout, opts, retry, b)
}

// Execute is the handler for the "radio" subcommand
func (c *RadioCommand) Execute(args []string) error {
	return runRadio(os.Stdout, c.Args.Action, b)
}

// run is the main entry point that returns an error instead of calling os.Exit directly.
func run() error {
	// Manually check for --version flag before parsing to avoid unnecessary backend init.
	for _, arg := range os.Args[1:] {
		if arg == "--version" {
			fmt.Println(Version)
			return nil
		}
	}

	// Initialize the backend before parsing, so it's available to Execute methods.
	var err error
	b, err = GetBackend()
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}

	parser := flags.NewParser(&opts, flags.HelpFlag)
	parser.ShortDescription = "A simple TUI for managing wifi connections."
	parser.LongDescription = "wifitui is a TUI and CLI for managing wifi connections."

	// Parse arguments.
	_, err = parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok {
			if flagsErr.Type == flags.ErrHelp {
				// Help was requested, so print the help message.
				parser.WriteHelp(os.Stdout)
				return nil
			}
			if flagsErr.Type == flags.ErrCommandRequired {
				// No command was specified, so run the TUI by default.
				return opts.Tui.Execute(nil)
			}
		}

		// For any other error, return it.
		return err
	}

	return nil
}

// main is the entry point of the application
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
