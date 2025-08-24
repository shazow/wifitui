package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"
)

var (
	// Version is the version of the application. It is set at build time.
	Version string = "dev"
)

// main is the entry point of the application
func main() {
	var (
		rootFlagSet = flag.NewFlagSet("wifitui", flag.ExitOnError)
		verbose     = rootFlagSet.Bool("v", false, "verbose output")
		version     = rootFlagSet.Bool("version", false, "display version")
	)

	listCmd := &ffcli.Command{
		Name:      "list",
		ShortHelp: "List wifi networks",
		Exec: func(ctx context.Context, args []string) error {
			return runList(os.Stdout, *verbose)
		},
	}

	showCmd := &ffcli.Command{
		Name:      "show",
		ShortHelp: "Show a wifi network",
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("show requires an ssid")
			}
			return runShow(os.Stdout, *verbose, args[0])
		},
	}

	root := &ffcli.Command{
		ShortUsage:  "wifitui [flags] <subcommand> [args...]",
		FlagSet:     rootFlagSet,
		Subcommands: []*ffcli.Command{listCmd, showCmd},
		Exec: func(ctx context.Context, args []string) error {
			return runTUI()
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

	if err := root.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
