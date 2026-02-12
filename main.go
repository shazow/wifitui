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

	Tui        TuiCommand        `command:"tui" description:"Run the TUI (default)"`
	List       ListCommand       `command:"list" description:"List wifi networks"`
	Show       ShowCommand       `command:"show" description:"Show a wifi network"`
	Connect    ConnectCommand    `command:"connect" description:"Connect to a wifi network"`
	Disconnect DisconnectCommand `command:"disconnect" description:"Disconnect from the active wifi network"`
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
	Passphrase string `long:"passphrase" description:"passphrase for the network"`
	Security   string `long:"security" default:"wpa" description:"security type (open, wep, wpa)"`
	Hidden     bool   `long:"hidden" description:"network is hidden"`
	Args       struct {
		SSID string `positional-arg-name:"ssid" required:"true"`
	} `positional-args:"yes"`
}

// DisconnectCommand defines the handler for the "disconnect" subcommand
type DisconnectCommand struct{}

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

// Execute is the handler for the "disconnect" subcommand
func (c *DisconnectCommand) Execute(args []string) error {
	return runDisconnect(os.Stdout, b)
}

// main is the entry point of the application
func main() {
	parser := flags.NewParser(&opts, flags.HelpFlag)
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

	// Parse arguments.
	_, err = parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok {
			if flagsErr.Type == flags.ErrHelp {
				// Help was requested, so print the help message.
				parser.WriteHelp(os.Stdout)
				os.Exit(0)
			}
			if flagsErr.Type == flags.ErrCommandRequired {
				// No command was specified, so run the TUI by default.
				if err := opts.Tui.Execute(nil); err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
					os.Exit(1)
				}
				os.Exit(0)
			}
		}

		// For any other error, print it and exit.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
