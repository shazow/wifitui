//go:build linux

package iwd

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/shazow/wifitui/wifi"
)

// iwd keeps its global settings in main.conf and per-network settings in
// files in its state directory. MAC randomization is only configurable
// through these files, not over D-Bus. See iwd.config(5) and iwd.network(5).

const (
	// DefaultConfigDir is the directory iwd reads main.conf from.
	DefaultConfigDir = "/etc/iwd"
	defaultStateDir  = "/var/lib/iwd"

	iwdBasePath    = "/net/connman/iwd"
	iwdDaemonIface = "net.connman.iwd.Daemon"

	settingsGroup             = "Settings"
	securityGroup             = "Security"
	alwaysRandomizeAddressKey = "AlwaysRandomizeAddress"
)

// PerNetworkAddressRandomization reports whether iwd's main.conf in configDir
// sets [General] AddressRandomization=network. iwd ignores the per-network
// AlwaysRandomizeAddress setting unless it does.
func PerNetworkAddressRandomization(configDir string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(configDir, "main.conf"))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	value, _ := keyfileValue(data, "General", "AddressRandomization")
	return value == "network", nil
}

// stateDirectory returns the directory where iwd keeps its network files.
func stateDirectory(conn *dbus.Conn) string {
	var info map[string]dbus.Variant
	err := conn.Object(iwdDest, iwdBasePath).Call(iwdDaemonIface+".GetInfo", 0).Store(&info)
	if err == nil {
		if dir, ok := info["StateDirectory"].Value().(string); ok && dir != "" {
			return dir
		}
	}
	return defaultStateDir
}

// networkFileName returns the name of iwd's settings file for a network with
// the given SSID and iwd network type ("psk", "open" or "8021x").
func networkFileName(ssid string, networkType string) string {
	name := ssid
	for i := 0; i < len(ssid); i++ {
		c := ssid[i]
		isAlnum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if !isAlnum && c != ' ' && c != '_' && c != '-' {
			name = "=" + hex.EncodeToString([]byte(ssid))
			break
		}
	}
	return name + "." + networkType
}

// iwdNetworkType returns iwd's network type for a security type that iwd can
// provision from a settings file.
func iwdNetworkType(security wifi.SecurityType) (string, bool) {
	switch security {
	case wifi.SecurityOpen:
		return "open", true
	case wifi.SecurityWPA:
		return "psk", true
	}
	return "", false
}

// parseKeyfileLine splits a keyfile line into a group header or a key and
// value. Comments and blank lines return neither.
func parseKeyfileLine(line string) (group string, key string, value string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", ""
	}
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		return trimmed[1 : len(trimmed)-1], "", ""
	}
	if i := strings.IndexByte(trimmed, '='); i > 0 {
		return "", strings.TrimSpace(trimmed[:i]), strings.TrimSpace(trimmed[i+1:])
	}
	return "", "", ""
}

// keyfileValue returns the raw value of key in group from iwd keyfile data.
func keyfileValue(data []byte, group string, key string) (string, bool) {
	current := ""
	for _, line := range strings.Split(string(data), "\n") {
		g, k, v := parseKeyfileLine(line)
		if g != "" {
			current = g
		} else if current == group && k == key {
			return v, true
		}
	}
	return "", false
}

// keyfileBool returns whether key in group is set to true.
func keyfileBool(data []byte, group string, key string) bool {
	value, _ := keyfileValue(data, group, key)
	return value == "true" || value == "1"
}

// setKeyfileValue returns data with key in group set to value, or removed
// when value is empty. The group is added when needed, and all other lines
// are kept as they are.
func setKeyfileValue(data []byte, group string, key string, value string) []byte {
	lines := strings.Split(string(data), "\n")
	current := ""
	groupFound := false
	for i, line := range lines {
		g, k, _ := parseKeyfileLine(line)
		if g != "" {
			current = g
			groupFound = groupFound || g == group
			continue
		}
		if current != group || k != key {
			continue
		}
		if value == "" {
			lines = append(lines[:i], lines[i+1:]...)
		} else {
			lines[i] = key + "=" + value
		}
		return []byte(strings.Join(lines, "\n"))
	}
	if value == "" {
		return data
	}

	entry := key + "=" + value
	if groupFound {
		for i, line := range lines {
			if g, _, _ := parseKeyfileLine(line); g == group {
				lines = append(lines[:i+1], append([]string{entry}, lines[i+1:]...)...)
				return []byte(strings.Join(lines, "\n"))
			}
		}
	}

	var b strings.Builder
	b.Write(data)
	if len(data) > 0 {
		if !strings.HasSuffix(string(data), "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "[%s]\n%s\n", group, entry)
	return []byte(b.String())
}

// escapeKeyfileValue escapes a string value for an iwd keyfile.
func escapeKeyfileValue(value string) string {
	value = strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\r", `\r`, "\t", `\t`).Replace(value)
	if strings.HasPrefix(value, " ") {
		value = `\s` + value[1:]
	}
	return value
}

// writeFileAtomic replaces path with data. iwd watches its state directory,
// so the file is written under a temporary name that iwd ignores and then
// renamed into place.
func writeFileAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".wifitui-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)

	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
