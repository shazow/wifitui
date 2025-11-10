package main

import (
	"fmt"
	"os"

	flags "github.com/jessevdk/go-flags"
	"github.com/shazow/wifitui/internal/tui"
	"github.com/shazow/wifitui/wifi"
)

var (
	// Version is the version of the application. It is set at build time.
	Version string = "dev"
)

// Options defines the root-level flags
type Options struct {
	Theme   string `long:"theme" description:"path to theme toml file" env:"WIFITUI_THEME"`
	Version bool   `long:"version" description:"display version"`

	List    ListCommand    `command:"list" description:"List wifi networks"`
	Show    ShowCommand    `command:"show" description:"Show a wifi network"`
	Connect ConnectCommand `command:"connect" description:"Connect to a wifi network"`
}

// ListCommand defines the flags and arguments for the "list" subcommand
type ListCommand struct {
	JSON bool `long:"json" description:"output in JSON format"`
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
	Passphrase string `long:"passphrase" description:"passphrase for the network"`
	Security   string `long:"security" default:"wpa" description:"security type (open, wep, wpa)"`
	Hidden     bool   `long:"hidden" description:"network is hidden"`
	Args       struct {
		SSID string `positional-arg-name:"ssid" required:"true"`
	} `positional-args:"yes"`
}

// We need a global backend to be accessible by the command handlers.
var b wifi.Backend

// Execute is the handler for the "list" subcommand
func (c *ListCommand) Execute(args []string) error {
	return runList(os.Stdout, c.JSON, b)
}

// Execute is the handler for the "show" subcommand
func (c *ShowCommand) Execute(args []string) error {
	return runShow(os.Stdout, c.JSON, c.Args.SSID, b)
}

// Execute is the handler for the "connect" subcommand
func (c *ConnectCommand) Execute(args []string) error {
	var security wifi.SecurityType
	switch c.Security {
	case "open":
		security = wifi.SecurityOpen
	case "wep":
		security = wifi.SecurityWEP
	case "wpa":
		security = wifi.SecurityWPA
	default:
		return fmt.Errorf("invalid security type: %s", c.Security)
	}
	return runConnect(os.Stdout, c.Args.SSID, c.Passphrase, security, c.Hidden, b)
}

// main is the entry point of the application
func main() {
	var opts Options
	parser := flags.NewParser(&opts, flags.Default)
	parser.ShortDescription = "A simple TUI for managing wifi connections."
	parser.LongDescription = "wifitui is a TUI and CLI for managing wifi connections."

	// Manually check for --version flag before parsing to avoid unnecessary backend init.
	for _, arg := range os.Args[1:] {
		if arg == "--version" {
			fmt.Println(Version)
			os.Exit(0)
		}
	}

	// Initialize the backend before parsing, so it's available to Execute methods.
	var err error
	b, err = GetBackend()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Parse arguments. If a subcommand is found, its Execute method will be called.
	_, err = parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			// Help was requested, output is already printed.
			os.Exit(0)
		}
		// Any other error is printed by go-flags.
		os.Exit(1)
	}

	// If no subcommand was specified (parser.Active is nil), run the TUI.
	if parser.Active == nil {
		if opts.Theme != "" {
			f, err := os.Open(opts.Theme)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to open theme file: %v\n", err)
				os.Exit(1)
			}
			defer f.Close()
			loadedTheme, err := tui.LoadTheme(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to load theme: %v\n", err)
				os.Exit(1)
			}
			tui.CurrentTheme = loadedTheme
		}
		if err := runTUI(b); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
}
