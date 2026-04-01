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
	defaultWaitDuration  = 60 * time.Second
)

// parseSecurityType converts a security string (open, wep, wpa) to a wifi.SecurityType.
func parseSecurityType(s string) (wifi.SecurityType, error) {
	switch s {
	case "open":
		return wifi.SecurityOpen, nil
	case "wep":
		return wifi.SecurityWEP, nil
	case "wpa":
		return wifi.SecurityWPA, nil
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
		return RetryConfig{}, fmt.Errorf("invalid duration format for --retry: %w", err)
	}

	if len(parts) > 1 {
		retry.Interval, err = time.ParseDuration(parts[1])
		if err != nil {
			return RetryConfig{}, fmt.Errorf("invalid sleep duration format for --retry: %w", err)
		}
	}

	return retry, nil
}

// Options defines the root-level flags
type Options struct {
	Theme   string `long:"theme" description:"path to theme toml file" env:"WIFITUI_THEME"`
	Device  string `long:"device" description:"wireless interface to use"`
	Version bool   `long:"version" description:"display version"`

	Tui        TuiCommand        `command:"tui" description:"Run the TUI (default)"`
	List       ListCommand       `command:"list" description:"List wifi networks"`
	Show       ShowCommand       `command:"show" description:"Show a wifi network"`
	Connect    ConnectCommand    `command:"connect" description:"Connect to a wifi network"`
	Devices    DevicesCommand    `command:"devices" description:"List wireless devices"`
	Radio      RadioCommand      `command:"radio" description:"Control the wifi radio (on|off|toggle)"`
	Completion CompletionCommand `command:"completion" description:"Generate shell completion script"`
}

type TuiCommand struct{}

type ListCommand struct {
	JSON bool `long:"json" description:"output in JSON format"`
	All  bool `long:"all" description:"list all saved and visible networks"`
	Scan bool `long:"scan" description:"scan for new visible networks"`
}

type ShowCommand struct {
	JSON bool `long:"json" description:"output in JSON format"`
	Args struct {
		SSID string `positional-arg-name:"ssid" required:"true"`
	} `positional-args:"yes"`
}

type ConnectCommand struct {
	Passphrase string `long:"passphrase" description:"passphrase for the network"`
	Security   string `long:"security" default:"wpa" description:"security type" choice:"open" choice:"wep" choice:"wpa"`
	Hidden     bool   `long:"hidden" description:"network is hidden"`
	Retry      string `long:"retry" description:"duration to retry connection (e.g. 60s or 2m:20s)" value-name:"DURATION[:INTERVAL]"`
	RetryFor   string `long:"retry-for" hidden:"true" description:"deprecated alias for --retry"`
	Wait       string `long:"wait" optional:"true" optional-value:"60s" value-name:"DURATION" description:"wait for SSID to appear (optional timeout, default 60s)"`
	Args       struct {
		SSID string `positional-arg-name:"ssid" required:"true"`
	} `positional-args:"yes"`
}

type DevicesCommand struct {
	JSON bool `long:"json" description:"output in JSON format with metadata"`
}

type CompletionCommand struct {
	Args struct {
		Shell string `positional-arg-name:"shell" required:"true" choice:"bash" choice:"zsh" choice:"fish"`
	} `positional-args:"yes"`
}

type RadioCommand struct {
	Args struct {
		Action string `positional-arg-name:"action"`
	} `positional-args:"yes"`
}

var b wifi.Backend
var opts Options

func configureBackendDevice() error {
	if opts.Device == "" {
		return nil
	}
	if err := b.SetDevice(opts.Device); err != nil {
		return fmt.Errorf("failed to select device %q: %w", opts.Device, err)
	}
	return nil
}

func (c *TuiCommand) Execute(args []string) error {
	if err := configureBackendDevice(); err != nil {
		return err
	}
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

func (c *ListCommand) Execute(args []string) error {
	if err := configureBackendDevice(); err != nil {
		return err
	}
	return runList(os.Stdout, c.JSON, c.All, c.Scan, b)
}

func (c *ShowCommand) Execute(args []string) error {
	if err := configureBackendDevice(); err != nil {
		return err
	}
	return runShow(os.Stdout, c.JSON, c.Args.SSID, b)
}

func (c *ConnectCommand) Execute(args []string) error {
	if err := configureBackendDevice(); err != nil {
		return err
	}
	security, err := parseSecurityType(c.Security)
	if err != nil {
		return err
	}

	retryArg := c.Retry
	if retryArg == "" {
		retryArg = c.RetryFor
	}
	retry, err := parseRetryConfig(retryArg)
	if err != nil {
		return err
	}

	if c.Wait != "" {
		retry.RequireVisible = true
		waitDuration := defaultWaitDuration
		if c.Wait != "60s" {
			waitDuration, err = time.ParseDuration(c.Wait)
			if err != nil {
				return fmt.Errorf("invalid duration format for --wait: %w", err)
			}
		}
		if retry.Total < waitDuration {
			retry.Total = waitDuration
		}
	}

	return runConnect(os.Stdout, c.Args.SSID, c.Passphrase, security, c.Hidden, retry, b)
}

func (c *DevicesCommand) Execute(args []string) error {
	if err := configureBackendDevice(); err != nil {
		return err
	}
	return runDevices(os.Stdout, c.JSON, b)
}

func (c *CompletionCommand) Execute(args []string) error {
	return runCompletion(os.Stdout, c.Args.Shell)
}

func (c *RadioCommand) Execute(args []string) error {
	if err := configureBackendDevice(); err != nil {
		return err
	}
	return runRadio(os.Stdout, c.Args.Action, b)
}

func run() error {
	for _, arg := range os.Args[1:] {
		if arg == "--version" {
			fmt.Println(Version)
			return nil
		}
	}

	var err error
	b, err = GetBackend()
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}

	parser := flags.NewParser(&opts, flags.HelpFlag)
	parser.ShortDescription = "A simple TUI for managing wifi connections."
	parser.LongDescription = "wifitui is a TUI and CLI for managing wifi connections."

	_, err = parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok {
			if flagsErr.Type == flags.ErrHelp {
				parser.WriteHelp(os.Stdout)
				return nil
			}
			if flagsErr.Type == flags.ErrCommandRequired {
				return opts.Tui.Execute(nil)
			}
		}
		return err
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
