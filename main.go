package main

import (
	"fmt"
	"log"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/godbus/dbus/v5"
)

// connectionItem holds the information for a single Wi-Fi connection
type connectionItem struct {
	ssid     string
	path     dbus.ObjectPath
	settings map[string]map[string]dbus.Variant
}

func main() {
	// --- Step 1: Fetch all Wi-Fi connections from D-Bus ---
	connections, err := fetchConnections()
	if err != nil {
		log.Fatalf("Error fetching connections: %v", err)
	}

	if len(connections) == 0 {
		fmt.Println("No Wi-Fi connections found.")
		os.Exit(0)
	}

	// Create a map to look up connection details by path, since the struct is not comparable
	connectionMap := make(map[dbus.ObjectPath]connectionItem)
	var networkOptions []huh.Option[dbus.ObjectPath]
	for _, conn := range connections {
		connectionMap[conn.path] = conn
		networkOptions = append(networkOptions, huh.NewOption(conn.ssid, conn.path))
	}

	// --- Step 2: Create a 'huh.Select' form to choose a network ---
	var selectedPath dbus.ObjectPath
	selectForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[dbus.ObjectPath]().
				Title("Which Wi-Fi network would you like to edit?").
				Options(networkOptions...).
				Value(&selectedPath),
		),
	).WithTheme(huh.ThemeCatppuccin())

	err = selectForm.Run()
	if err != nil {
		// This can happen if the user presses Ctrl+C
		fmt.Println("Aborted.")
		os.Exit(1)
	}

	// Look up the full connection details from the selected path
	selectedConnection := connectionMap[selectedPath]

	// --- Step 3: Fetch the current password for the selected network ---
	fmt.Println("Requesting secrets for the selected Wi-Fi connection...")

	// Only show the Polkit dialog warning if the network is actually secured
	if _, ok := selectedConnection.settings["802-11-wireless-security"]; ok {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("--> You will likely see a Polkit authentication dialog now. <--"))
	}

	currentPassword, err := getSecrets(selectedConnection)
	if err != nil {
		log.Fatalf("Error fetching secrets: %v", err)
	}

	// --- Step 4: Create a 'huh.Input' form to edit the password ---
	// Set the initial value of newPassword to the current password.
	newPassword := currentPassword
	editForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("Editing password for '%s'", selectedConnection.ssid)).
				Description("Enter the new password.").
				// Set Password to false to make it visible
				Password(false).
				// Point the value to our variable.
				Value(&newPassword),
		),
	).WithTheme(huh.ThemeCatppuccin())

	err = editForm.Run()
	if err != nil {
		fmt.Println("Aborted.")
		os.Exit(1)
	}

	// --- Step 5: Save the new password back to NetworkManager ---
	if newPassword != currentPassword {
		fmt.Println("Saving new password...")
		err = updateSecret(selectedConnection, newPassword)
		if err != nil {
			log.Fatalf("Error saving secret: %v", err)
		}
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("✓ Password updated successfully!"))
	} else {
		fmt.Println("Password unchanged. Exiting.")
	}
}

// --- D-Bus Logic ---

const (
	nmDest          = "org.freedesktop.NetworkManager"
	nmSettingsPath  = "/org/freedesktop/NetworkManager/Settings"
	nmSettingsIface = "org.freedesktop.NetworkManager.Settings"
	nmConnIface     = "org.freedesktop.NetworkManager.Settings.Connection"
)

func fetchConnections() ([]connectionItem, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	obj := conn.Object(nmDest, nmSettingsPath)

	var connectionPaths []dbus.ObjectPath
	err = obj.Call(nmSettingsIface+".ListConnections", 0).Store(&connectionPaths)
	if err != nil {
		return nil, err
	}

	var items []connectionItem
	for _, path := range connectionPaths {
		connObj := conn.Object(nmDest, path)
		settings, err := getSettings(connObj)
		if err != nil {
			continue
		}

		if connType, ok := settings["connection"]["type"]; ok && connType.Value() == "802-11-wireless" {
			ssidBytes, ok := settings["802-11-wireless"]["ssid"].Value().([]byte)
			if ok {
				items = append(items, connectionItem{
					ssid:     string(ssidBytes),
					path:     path,
					settings: settings,
				})
			}
		}
	}

	return items, nil
}

func getSecrets(item connectionItem) (string, error) {
	// If the connection has no security setting, it's an open network.
	if _, ok := item.settings["802-11-wireless-security"]; !ok {
		return "", nil
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}
	defer conn.Close()

	obj := conn.Object(nmDest, item.path)

	var secrets map[string]map[string]dbus.Variant
	err = obj.Call(nmConnIface+".GetSecrets", 0, "802-11-wireless-security").Store(&secrets)
	if err != nil {
		return "", fmt.Errorf("failed to get secrets (did you authenticate?): %w", err)
	}

	psk, ok := secrets["802-11-wireless-security"]["psk"]
	if !ok {
		return "", nil // No PSK found, but the security setting exists (e.g., 802.1x)
	}

	return psk.Value().(string), nil
}

func updateSecret(item connectionItem, newPassword string) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	obj := conn.Object(nmDest, item.path)
	currentSettings, err := getSettings(obj)
	if err != nil {
		return err
	}

	if _, ok := currentSettings["802-11-wireless-security"]; !ok {
		currentSettings["802-11-wireless-security"] = make(map[string]dbus.Variant)
	}
	currentSettings["802-11-wireless-security"]["psk"] = dbus.MakeVariant(newPassword)

	return obj.Call(nmConnIface+".Update", 0, currentSettings).Store()
}

func getSettings(obj dbus.BusObject) (map[string]map[string]dbus.Variant, error) {
	var settings map[string]map[string]dbus.Variant
	err := obj.Call(nmConnIface+".GetSettings", 0).Store(&settings)
	return settings, err
}

