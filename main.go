package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/shazow/wifitui/internal/tui"
	"github.com/shazow/wifitui/wifi"
)

var (
	// Version is the version of the application. It is set at build time.
	Version string = "dev"
)

// main is the entry point of the application
func main() {
	var (
		rootFlagSet = flag.NewFlagSet("wifitui", flag.ExitOnError)
		theme       = rootFlagSet.String("theme", "", "path to theme toml file (env: WIFITUI_THEME)")
		version     = rootFlagSet.Bool("version", false, "display version")
	)

	var b wifi.Backend
	var err error

	listFlagSet := flag.NewFlagSet("list", flag.ExitOnError)
	listJSON := listFlagSet.Bool("json", false, "output in JSON format")
	listCmd := &ffcli.Command{
		Name:      "list",
		ShortHelp: "List wifi networks",
		FlagSet:   listFlagSet,
		Exec: func(ctx context.Context, args []string) error {
			return runList(os.Stdout, *listJSON, b)
		},
	}

	showFlagSet := flag.NewFlagSet("show", flag.ExitOnError)
	showJSON := showFlagSet.Bool("json", false, "output in JSON format")
	showCmd := &ffcli.Command{
		Name:      "show",
		ShortHelp: "Show a wifi network",
		FlagSet:   showFlagSet,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("show requires an ssid")
			}
			return runShow(os.Stdout, *showJSON, args[0], b)
		},
	}

	connectFlagSet := flag.NewFlagSet("connect", flag.ExitOnError)
	connectPassphrase := connectFlagSet.String("passphrase", "", "passphrase for the network")
	connectSecurity := connectFlagSet.String("security", "wpa", "security type (open, wep, wpa)")
	connectHidden := connectFlagSet.Bool("hidden", false, "network is hidden")
	connectCmd := &ffcli.Command{
		Name:      "connect",
		ShortHelp: "Connect to a wifi network",
		FlagSet:   connectFlagSet,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("connect requires an ssid")
			}
			var security wifi.SecurityType
			switch *connectSecurity {
			case "open":
				security = wifi.SecurityOpen
			case "wep":
				security = wifi.SecurityWEP
			case "wpa":
				security = wifi.SecurityWPA
			default:
				return fmt.Errorf("invalid security type: %s", *connectSecurity)
			}
			return runConnect(os.Stdout, args[0], *connectPassphrase, security, *connectHidden, b)
		},
	}

	// TODO: Add a `wifitui tui` sub-command that is just an alias for the root command.

	root := &ffcli.Command{
		ShortUsage:  "wifitui [flags] <subcommand> [args...]",
		FlagSet:     rootFlagSet,
		Subcommands: []*ffcli.Command{listCmd, showCmd, connectCmd},
		Exec: func(ctx context.Context, args []string) error {
			return runTUI(b)
		},
	}

	// Parse flags using ff to get theme and version.
	// We need to do this before root.Run so we can load the theme.
	// root.Run will parse them again, but that's fine.
	err = ff.Parse(rootFlagSet, os.Args[1:],
		ff.WithEnvVarPrefix("WIFITUI"),
		ff.WithIgnoreUndefined(true), // Ignore subcommand flags for now
	)
	if err != nil {
		if err == flag.ErrHelp {
			// ff.Parse doesn't print usage on ErrHelp, so we do it manually.
			root.FlagSet.Usage()
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if err := tui.LoadTheme(*theme); err != nil {
		fmt.Fprintf(os.Stderr, "error loading theme: %v\n", err)
		os.Exit(1)
	}

	if *version {
		fmt.Println(Version)
		os.Exit(0)
	}

	b, err = GetBackend()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := root.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
