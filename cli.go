package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shazow/wifitui/internal/helpers"
	"github.com/shazow/wifitui/internal/tui"
	"github.com/shazow/wifitui/wifi"
)

func runTUI(b wifi.Backend) error {
	m, err := tui.NewModel(b)
	if err != nil {
		return fmt.Errorf("error initializing model: %w", err)
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %w", err)
	}
	return nil
}

func formatConnection(c wifi.Connection) string {
	var parts []string
	if c.IsVisible {
		parts = append(parts, fmt.Sprintf("%d%%", c.Strength()))
		parts = append(parts, "visible")
	}
	if c.IsSecure {
		parts = append(parts, "secure")
	}
	if c.IsActive {
		parts = append(parts, "active")
	}

	return strings.Join(parts, ", ")
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func filterVisibleConnections(connections []wifi.Connection) []wifi.Connection {
	var visible []wifi.Connection
	for _, c := range connections {
		if c.IsVisible {
			visible = append(visible, c)
		}
	}
	return visible
}

func findConnectionBySSID(connections []wifi.Connection, ssid string) (wifi.Connection, bool) {
	for _, c := range connections {
		if c.SSID == ssid {
			return c, true
		}
	}
	return wifi.Connection{}, false
}

func writeConnectionDetails(w io.Writer, c wifi.Connection, secret string) error {
	var writeErr error
	write := func(format string, args ...any) {
		if writeErr != nil {
			return
		}
		_, writeErr = fmt.Fprintf(w, format, args...)
	}
	write("SSID: %s\n", c.SSID)
	write("Passphrase: %s\n", secret)
	write("Active: %t\n", c.IsActive)
	write("Known: %t\n", c.IsKnown)
	write("Secure: %t\n", c.IsSecure)
	write("Visible: %t\n", c.IsVisible)
	write("Hidden: %t\n", c.IsHidden)
	write("Strength: %d%%\n", c.Strength())
	if c.LastConnected != nil {
		write("Last Connected: %s\n", helpers.FormatDuration(*c.LastConnected))
	}
	return writeErr
}

func runList(w io.Writer, jsonOut bool, all bool, scan bool, b wifi.Backend) error {
	connections, err := b.BuildNetworkList(scan)
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	if !all {
		connections = filterVisibleConnections(connections)
	}

	if jsonOut {
		return writeJSON(w, connections)
	}

	for _, c := range connections {
		fmt.Fprintf(w, "%s\t%s\n", c.SSID, formatConnection(c))
	}

	return nil
}

func runDevices(w io.Writer, jsonOut bool, b wifi.Backend) error {
	devices, err := b.ListDevices()
	if err != nil {
		return fmt.Errorf("failed to list devices: %w", err)
	}
	if jsonOut {
		return writeJSON(w, devices)
	}
	for _, d := range devices {
		fmt.Fprintln(w, d.Name)
	}
	return nil
}

func runShow(w io.Writer, jsonOut bool, ssid string, b wifi.Backend) error {
	connections, err := b.BuildNetworkList(true)
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	c, found := findConnectionBySSID(connections, ssid)
	if !found {
		return fmt.Errorf("network not found: %s: %w", ssid, wifi.ErrNotFound)
	}

	secret, err := b.GetSecrets(ssid)
	if err != nil {
		if c.IsKnown {
			return fmt.Errorf("failed to get network secret: %w", err)
		}
		secret = ""
	}

	if jsonOut {
		type connectionWithSecret struct {
			wifi.Connection
			Passphrase string `json:"passphrase,omitempty"`
		}
		return writeJSON(w, connectionWithSecret{Connection: c, Passphrase: secret})
	}

	return writeConnectionDetails(w, c, secret)
}

func attemptConnect(ssid string, passphrase string, security wifi.SecurityType, isHidden bool, shouldScan bool, requireVisible bool, b wifi.Backend) error {
	connections, err := b.BuildNetworkList(shouldScan)
	if err != nil {
		return fmt.Errorf("failed to load networks: %w", err)
	}

	if passphrase != "" || isHidden {
		if requireVisible && !isHidden {
			conn, found := findConnectionBySSID(connections, ssid)
			if !found || !conn.IsVisible {
				return fmt.Errorf("network not visible yet: %s: %w", ssid, wifi.ErrNotFound)
			}
		}
		return b.JoinNetwork(ssid, passphrase, security, isHidden)
	}

	return b.ActivateConnection(ssid)
}

type RetryConfig struct {
	Total          time.Duration
	Interval       time.Duration
	RequireVisible bool
}

func runConnect(w io.Writer, ssid string, passphrase string, security wifi.SecurityType, isHidden bool, retry RetryConfig, b wifi.Backend) error {
	start := time.Now()
	shouldScan := false

	for {
		fmt.Fprintf(w, "Connecting to network %q with scan=%v...\n", ssid, shouldScan)

		err := attemptConnect(ssid, passphrase, security, isHidden, shouldScan, retry.RequireVisible, b)
		if err == nil {
			return nil
		}

		if !shouldScan {
			shouldScan = true
			if retry.Total > 0 && time.Since(start) < retry.Total {
				fmt.Fprintf(w, "Quick connect failed: %q\n", err)
				continue
			}
		}

		if retry.Total == 0 || time.Since(start) >= retry.Total {
			return err
		}

		if errors.Is(err, wifi.ErrNotFound) {
			fmt.Fprintf(w, "SSID %q not visible yet. Rescanning in %v...\n", ssid, retry.Interval)
		} else {
			fmt.Fprintf(w, "Connection failed: %q\nRetrying in %v...\n", err, retry.Interval)
		}
		time.Sleep(retry.Interval)
	}
}

func runRadio(w io.Writer, action string, b wifi.Backend) error {
	var enabled bool
	switch action {
	case "on":
		enabled = true
	case "off":
		enabled = false
	case "", "toggle":
		current, err := b.IsWirelessEnabled()
		if err != nil {
			return fmt.Errorf("failed to get wireless state: %w", err)
		}
		enabled = !current
	default:
		return fmt.Errorf("invalid radio action: %q (expected on, off, or toggle)", action)
	}

	if enabled {
		fmt.Fprintln(w, "Enabling WiFi radio...")
	} else {
		fmt.Fprintln(w, "Disabling WiFi radio...")
	}

	if err := b.SetWireless(enabled); err != nil {
		return fmt.Errorf("failed to set wireless state: %w", err)
	}

	if enabled {
		fmt.Fprintln(w, "WiFi radio is on")
	} else {
		fmt.Fprintln(w, "WiFi radio is off")
	}
	return nil
}

func runCompletion(w io.Writer, shell string) error {
	switch shell {
	case "bash":
		_, err := io.WriteString(w, bashCompletionScript)
		return err
	case "zsh":
		_, err := io.WriteString(w, zshCompletionScript)
		return err
	case "fish":
		_, err := io.WriteString(w, fishCompletionScript)
		return err
	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}
}

const bashCompletionScript = `_wifitui_complete() {
  local cur prev words cword
  _init_completion || return

  local cmds="tui list show connect devices radio completion"
  local opts="--help --version --theme --device"

  if [[ ${prev} == "--device" ]]; then
    COMPREPLY=( $(compgen -W "$(wifitui devices 2>/dev/null)" -- "$cur") )
    return
  fi

  if [[ ${prev} == "connect" ]]; then
    COMPREPLY=( $(compgen -W "$(wifitui list --all --scan 2>/dev/null | cut -f1)" -- "$cur") )
    return
  fi

  case "${words[1]}" in
    connect)
      if [[ ${prev} == "--security" ]]; then
        COMPREPLY=( $(compgen -W "open wep wpa" -- "$cur") )
      else
        COMPREPLY=( $(compgen -W "--passphrase --security --hidden --retry --wait --device" -- "$cur") )
      fi
      ;;
    list)
      COMPREPLY=( $(compgen -W "--json --all --scan --device" -- "$cur") )
      ;;
    show)
      COMPREPLY=( $(compgen -W "--json --device $(wifitui list --all 2>/dev/null | cut -f1)" -- "$cur") )
      ;;
    devices)
      COMPREPLY=( $(compgen -W "--json" -- "$cur") )
      ;;
    radio)
      COMPREPLY=( $(compgen -W "on off toggle --device" -- "$cur") )
      ;;
    completion)
      COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
      ;;
    *)
      COMPREPLY=( $(compgen -W "$cmds $opts" -- "$cur") )
      ;;
  esac
}
complete -F _wifitui_complete wifitui
`

const zshCompletionScript = `#compdef wifitui

_wifitui() {
  local -a commands
  commands=(
    'tui:run tui mode'
    'list:list networks'
    'show:show a network'
    'connect:connect to a network'
    'devices:list wireless devices'
    'radio:control wifi radio'
    'completion:print completion script'
  )

  _arguments \
    '--device[wireless interface]:device:->device' \
    '--theme[theme file]:file:_files' \
    '--version[show version]' \
    '1:command:->command' \
    '*::arg:->args'

  case $state in
    command)
      _describe -t commands 'wifitui command' commands
      ;;
    device)
      _values 'device' ${(f)"$(wifitui devices 2>/dev/null)"}
      ;;
    args)
      case $words[2] in
        connect)
          _arguments '--passphrase[passphrase]' '--security[security type]:security:(open wep wpa)' '--hidden[hidden network]' '--retry[retry duration]' '--wait[wait duration]' '1:ssid:->ssid'
          ;;
        devices)
          _arguments '--json[json output]'
          ;;
      esac
      if [[ $words[2] == connect ]]; then
        _values 'ssid' ${(f)"$(wifitui list --all --scan 2>/dev/null | cut -f1)"}
      fi
      ;;
  esac
}

_wifitui "$@"
`

const fishCompletionScript = `complete -c wifitui -f
complete -c wifitui -l device -a "(wifitui devices 2>/dev/null)" -d "wireless interface"
complete -c wifitui -l theme -r -d "theme file"
complete -c wifitui -l version -d "show version"

complete -c wifitui -n "not __fish_seen_subcommand_from tui list show connect devices radio completion" -a "tui list show connect devices radio completion"

complete -c wifitui -n "__fish_seen_subcommand_from connect" -l passphrase -r
complete -c wifitui -n "__fish_seen_subcommand_from connect" -l security -a "open wep wpa"
complete -c wifitui -n "__fish_seen_subcommand_from connect" -l hidden
complete -c wifitui -n "__fish_seen_subcommand_from connect" -l retry -r
complete -c wifitui -n "__fish_seen_subcommand_from connect" -l wait -r
complete -c wifitui -n "__fish_seen_subcommand_from connect" -a "(wifitui list --all --scan 2>/dev/null | cut -f1)"

complete -c wifitui -n "__fish_seen_subcommand_from devices" -l json
complete -c wifitui -n "__fish_seen_subcommand_from list" -l json
complete -c wifitui -n "__fish_seen_subcommand_from list" -l all
complete -c wifitui -n "__fish_seen_subcommand_from list" -l scan
complete -c wifitui -n "__fish_seen_subcommand_from show" -l json
complete -c wifitui -n "__fish_seen_subcommand_from show" -a "(wifitui list --all 2>/dev/null | cut -f1)"
complete -c wifitui -n "__fish_seen_subcommand_from radio" -a "on off toggle"
complete -c wifitui -n "__fish_seen_subcommand_from completion" -a "bash zsh fish"
`
