//go:build linux

package iwd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shazow/wifitui/wifi"
)

func TestBuild8021xProfilePEAP(t *testing.T) {
	opts := wifi.JoinOptions{
		SSID:              "eduroam",
		Password:          " secret\\pw ",
		Identity:          "user@example.edu",
		AnonymousIdentity: "anonymous@example.edu",
		EAP:               "peap",
		Phase2Auth:        "mschapv2",
		Security:          wifi.SecurityWPAEAP,
		IsHidden:          true,
	}

	profile := build8021xProfile(opts)

	for _, want := range []string{
		"[Settings]",
		"AutoConnect=true",
		"Hidden=true",
		"[Security]",
		"EAP-Method=PEAP",
		"EAP-Identity=anonymous@example.edu",
		"EAP-PEAP-Phase2-Method=MSCHAPV2",
		"EAP-PEAP-Phase2-Identity=user@example.edu",
		`EAP-PEAP-Phase2-Password=\ssecret\\pw\s`,
	} {
		if !strings.Contains(profile, want) {
			t.Fatalf("profile missing %q:\n%s", want, profile)
		}
	}
}

func TestBuild8021xProfileTTLS(t *testing.T) {
	opts := wifi.JoinOptions{
		SSID:       "corpnet",
		Password:   "pass",
		Identity:   "user",
		EAP:        "ttls",
		Phase2Auth: "pap",
		Security:   wifi.SecurityWPAEAP,
	}

	profile := build8021xProfile(opts)

	for _, want := range []string{
		"EAP-Method=TTLS",
		"EAP-Identity=user",
		"EAP-TTLS-Phase2-Method=Tunneled-PAP",
		"EAP-TTLS-Phase2-Identity=user",
		"EAP-TTLS-Phase2-Password=pass",
	} {
		if !strings.Contains(profile, want) {
			t.Fatalf("profile missing %q:\n%s", want, profile)
		}
	}
}

func TestWrite8021xProfileEncodesSSIDInFilename(t *testing.T) {
	oldDir := iwdProfileDir
	iwdProfileDir = t.TempDir()
	t.Cleanup(func() {
		iwdProfileDir = oldDir
	})

	opts := wifi.JoinOptions{
		SSID:     "Cafe/Guest",
		Password: "pass",
		Identity: "user",
		EAP:      "peap",
		Security: wifi.SecurityWPAEAP,
	}

	if err := write8021xProfile(opts); err != nil {
		t.Fatalf("write8021xProfile() failed: %v", err)
	}

	path := filepath.Join(iwdProfileDir, encodeSSIDForProfileName(opts.SSID)+".8021x")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written profile: %v", err)
	}

	if !strings.Contains(string(content), "EAP-Method=PEAP") {
		t.Fatalf("written profile missing PEAP config:\n%s", string(content))
	}
}
