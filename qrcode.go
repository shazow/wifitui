package main

import (
	"strings"

	qrcode "github.com/skip2/go-qrcode"
	"github.com/shazow/wifitui/backend"
)

// EscapeWifiString handles the special character escaping for SSID and Password.
func EscapeWifiString(s string) string {
	// A replacer is more efficient than calling strings.Replace multiple times.
	r := strings.NewReplacer(
		`\`, `\\`,
		`;`, `\;`,
		`,`, `\,`,
		`:`, `\:`,
		`"`, `\"`,
	)
	return r.Replace(s)
}

// GenerateWifiQRCode builds the correctly formatted Wi-Fi connection string and returns the TUI-friendly QR code string.
func GenerateWifiQRCode(ssid, password string, security backend.SecurityType, isHidden bool) (string, error) {
	var b strings.Builder

	// Start with the required prefix and SSID
	b.WriteString("WIFI:S:")
	b.WriteString(EscapeWifiString(ssid))
	b.WriteString(";")

	// Set Authentication Type and Password
	switch security {
	case backend.SecurityWPA:
		b.WriteString("T:WPA;P:")
		b.WriteString(EscapeWifiString(password))
		b.WriteString(";")
	case backend.SecurityWEP:
		b.WriteString("T:WEP;P:")
		b.WriteString(EscapeWifiString(password))
		b.WriteString(";")
	case backend.SecurityOpen:
		b.WriteString("T:nopass;")
	default:
		// Don't set T if security is unknown, most readers will assume WPA.
	}

	// Add hidden flag if necessary
	if isHidden {
		b.WriteString("H:true;")
	}

	// Add the final terminator
	b.WriteString(";;")

	// Generate the QR code string for the terminal
	q, err := qrcode.New(b.String(), qrcode.Medium)
	if err != nil {
		return "", err
	}

	return q.ToSmallString(false), nil
}
