package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/shazow/wifitui/backend"
)

var (
	// Version is the version of the application. It is set at build time.
	Version string = "dev"
)

// main is the entry point of the application
func main() {
	var (
		rootFlagSet = flag.NewFlagSet("wifitui", flag.ExitOnError)
		version     = rootFlagSet.Bool("version", false, "display version")
	)

	var b backend.Backend
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
			var security backend.SecurityType
			switch *connectSecurity {
			case "open":
				security = backend.SecurityOpen
			case "wep":
				security = backend.SecurityWEP
			case "wpa":
				security = backend.SecurityWPA
			default:
				return fmt.Errorf("invalid security type: %s", *connectSecurity)
			}
			return runConnect(os.Stdout, args[0], *connectPassphrase, security, *connectHidden, b)
		},
	}

	root := &ffcli.Command{
		ShortUsage:  "wifitui [flags] <subcommand> [args...]",
		FlagSet:     rootFlagSet,
		Subcommands: []*ffcli.Command{listCmd, showCmd, connectCmd},
		Exec: func(ctx context.Context, args []string) error {
			return runTUI(b)
		},
	}

	if err := root.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
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
