//go:build linux

package iwd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/unix"

	"github.com/shazow/wifitui/wifi"
)

const scanCompletionTimeout = 30 * time.Second
const propertyChangeTimeout = 5 * time.Second

// configReloadDelay is how long to wait after editing a known network's
// settings file. iwd rereads the file when inotify reports the change and does
// not signal when it is done, so this gives it time before a following
// connection relies on the new settings.
const configReloadDelay = 500 * time.Millisecond

// knownNetworkTimeout is how long to wait for iwd to load a new settings file.
const knownNetworkTimeout = 5 * time.Second

const dbusPropertiesIface = "org.freedesktop.DBus.Properties"

// IWD constants
const (
	iwdDest              = "net.connman.iwd"
	iwdPath              = "/"
	iwdDeviceIface       = "net.connman.iwd.Device"
	iwdNetworkIface      = "net.connman.iwd.Network"
	iwdStationIface      = "net.connman.iwd.Station"
	iwdKnownNetworkIface = "net.connman.iwd.KnownNetwork"
	iwdAgentManagerIface = "net.connman.iwd.AgentManager"
	iwdAgentIface        = "net.connman.iwd.Agent"
	agentPath            = "/wifitui/agent"
)

// agent implements the net.connman.iwd.Agent D-Bus interface.
// iwd calls RequestPassphrase on this object when it needs credentials.
type agent struct {
	passphrase string
}

func (a *agent) Release() *dbus.Error {
	return nil
}

func (a *agent) RequestPassphrase(_ dbus.ObjectPath) (string, *dbus.Error) {
	return a.passphrase, nil
}

func (a *agent) RequestPrivateKeyPassphrase(_ dbus.ObjectPath) (string, *dbus.Error) {
	return a.passphrase, nil
}

func (a *agent) RequestUserNameAndPassword(_ dbus.ObjectPath) (string, string, *dbus.Error) {
	return "", a.passphrase, nil
}

func (a *agent) Cancel(_ dbus.ObjectPath) *dbus.Error {
	return nil
}

// Backend implements the backend.Backend interface using iwd.
type Backend struct {
	// stateDir is iwd's state directory, set only when MAC randomization
	// can be configured (see macRandomizationStateDir).
	stateDir string
}

// randomMACBackend is a Backend that also implements wifi.MACRandomizer.
type randomMACBackend struct {
	*Backend
}

var _ wifi.MACRandomizer = randomMACBackend{}

// New creates a new iwd.Backend.
func New() (wifi.Backend, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	// Check if iwd is available by calling GetManagedObjects on the root path.
	obj := conn.Object(iwdDest, iwdPath)
	if obj == nil {
		return nil, fmt.Errorf("failed to get dbus object for %s: %w", iwdDest, wifi.ErrNotAvailable)
	}
	var managedObjects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	err = obj.Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&managedObjects)
	if err != nil {
		return nil, fmt.Errorf("iwd is not available: %w", wifi.ErrNotAvailable)
	}

	b := &Backend{}
	if dir, ok := macRandomizationStateDir(conn); ok {
		b.stateDir = dir
		return randomMACBackend{b}, nil
	}
	return b, nil
}

// macRandomizationStateDir returns iwd's state directory if per-network MAC
// randomization can be configured: iwd's main.conf must set
// AddressRandomization=network, and wifitui must be able to edit the network
// files in the state directory, which normally requires root.
func macRandomizationStateDir(conn *dbus.Conn) (string, bool) {
	enabled, err := PerNetworkAddressRandomization(DefaultConfigDir)
	if err != nil || !enabled {
		return "", false
	}
	dir := stateDirectory(conn)
	if unix.Access(dir, unix.R_OK|unix.W_OK|unix.X_OK) != nil {
		return "", false
	}
	return dir, true
}

// getManagedObjects returns all iwd managed objects from D-Bus ObjectManager.
func getManagedObjects(conn *dbus.Conn) (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, error) {
	var objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	err := conn.Object(iwdDest, iwdPath).Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objects)
	return objects, err
}

// getStationDevice finds the first Device object path that has a Station interface.
func getStationDevice(conn *dbus.Conn) (dbus.ObjectPath, error) {
	objects, err := getManagedObjects(conn)
	if err != nil {
		return "", err
	}
	for path, ifaces := range objects {
		if _, hasStation := ifaces[iwdStationIface]; hasStation {
			return path, nil
		}
	}
	return "", fmt.Errorf("no station device found: %w", wifi.ErrNotFound)
}

// getKnownNetworkPaths returns all object paths that have the KnownNetwork interface.
func getKnownNetworkPaths(conn *dbus.Conn) ([]dbus.ObjectPath, error) {
	objects, err := getManagedObjects(conn)
	if err != nil {
		return nil, err
	}
	var paths []dbus.ObjectPath
	for path, ifaces := range objects {
		if _, ok := ifaces[iwdKnownNetworkIface]; ok {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

// findKnownNetworkPath finds the KnownNetwork object path for a given SSID.
func findKnownNetworkPath(conn *dbus.Conn, ssid string) (dbus.ObjectPath, error) {
	paths, err := getKnownNetworkPaths(conn)
	if err != nil {
		return "", err
	}
	for _, path := range paths {
		nameVar, err := conn.Object(iwdDest, path).GetProperty(iwdKnownNetworkIface + ".Name")
		if err != nil {
			continue
		}
		if name, ok := nameVar.Value().(string); ok && name == ssid {
			return path, nil
		}
	}
	return "", nil
}

// findNetworkPath finds the Network object path for a given SSID.
func findNetworkPath(conn *dbus.Conn, ssid string) (dbus.ObjectPath, error) {
	objects, err := getManagedObjects(conn)
	if err != nil {
		return "", err
	}
	for path, ifaces := range objects {
		if _, ok := ifaces[iwdNetworkIface]; !ok {
			continue
		}
		nameVar, err := conn.Object(iwdDest, path).GetProperty(iwdNetworkIface + ".Name")
		if err != nil {
			continue
		}
		if name, ok := nameVar.Value().(string); ok && name == ssid {
			return path, nil
		}
	}
	return "", nil
}

// dbmToPercent converts signal strength in hundredths of dBm to 0-100 percentage.
// iwd returns values like -5600 meaning -56.00 dBm.
func dbmToPercent(centidBm int16) uint8 {
	dbm := float64(centidBm) / 100.0
	// Typical range: -90 dBm (weak) to -30 dBm (strong)
	if dbm >= -30 {
		return 100
	}
	if dbm <= -90 {
		return 0
	}
	return uint8((dbm + 90) * 100 / 60)
}

// orderedNetwork represents a result from Station.GetOrderedNetworks: (object_path, signal_strength).
type orderedNetwork struct {
	Path     dbus.ObjectPath
	Strength int16
}

// ListNetworks returns all networks and optionally requests a scan first.
func (b *Backend) ListNetworks(scan wifi.ScanMode) (wifi.NetworksResult, error) {
	enabled, err := b.IsWirelessEnabled()
	if err != nil {
		return wifi.NetworksResult{}, err
	}
	if !enabled {
		return wifi.NetworksResult{}, wifi.ErrWirelessDisabled
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return wifi.NetworksResult{}, err
	}

	station, err := getStationDevice(conn)
	if err != nil {
		return wifi.NetworksResult{}, err
	}

	var scanErr error
	if scan != wifi.ScanNever {
		scanErr = scanAndWait(conn, station)
	}

	// GetOrderedNetworks returns a(on): array of (object_path, signal_strength_in_centidBm)
	var ordered []orderedNetwork
	err = conn.Object(iwdDest, station).Call(iwdStationIface+".GetOrderedNetworks", 0).Store(&ordered)
	if err != nil {
		return wifi.NetworksResult{}, fmt.Errorf("failed to get ordered networks: %w", err)
	}

	var connections []wifi.Network
	visibleNetworks := make(map[string]wifi.Network)

	for _, net := range ordered {
		networkObj := conn.Object(iwdDest, net.Path)

		nameVar, err := networkObj.GetProperty(iwdNetworkIface + ".Name")
		if err != nil {
			continue
		}
		ssid, ok := nameVar.Value().(string)
		if !ok {
			continue
		}

		typeVar, _ := networkObj.GetProperty(iwdNetworkIface + ".Type")
		securityType, _ := typeVar.Value().(string)
		var security wifi.SecurityType
		switch securityType {
		case "psk":
			security = wifi.SecurityWPA
		case "8021x":
			security = wifi.SecurityWPA
		case "wep":
			security = wifi.SecurityWEP
		default:
			security = wifi.SecurityOpen
		}

		connectedVar, _ := networkObj.GetProperty(iwdNetworkIface + ".Connected")
		isActive, _ := connectedVar.Value().(bool)

		// Get per-AP info from ExtendedServiceSet
		essVar, _ := networkObj.GetProperty(iwdNetworkIface + ".ExtendedServiceSet")
		var apPaths []dbus.ObjectPath
		if essVar.Value() != nil {
			apPaths, _ = essVar.Value().([]dbus.ObjectPath)
		}

		strength := dbmToPercent(net.Strength)

		if existing, exists := visibleNetworks[ssid]; exists {
			existing.AccessPoints = append(existing.AccessPoints, wifi.AccessPoint{Strength: strength})
			if isActive {
				existing.IsActive = true
			}
			visibleNetworks[ssid] = existing
		} else {
			aps := []wifi.AccessPoint{{Strength: strength}}
			// If there are multiple BSSes, add extra APs (first already counted)
			for i := 1; i < len(apPaths); i++ {
				bssObj := conn.Object(iwdDest, apPaths[i])
				addrVar, _ := bssObj.GetProperty("net.connman.iwd.BasicServiceSet.Address")
				bssid, _ := addrVar.Value().(string)
				aps = append(aps, wifi.AccessPoint{
					BSSID:    bssid,
					Strength: strength, // iwd only gives one signal per network
				})
			}
			// Set BSSID on first AP if available
			if len(apPaths) > 0 {
				bssObj := conn.Object(iwdDest, apPaths[0])
				addrVar, _ := bssObj.GetProperty("net.connman.iwd.BasicServiceSet.Address")
				if bssid, ok := addrVar.Value().(string); ok {
					aps[0].BSSID = bssid
				}
			}

			visibleNetworks[ssid] = wifi.Network{
				SSID:         ssid,
				IsActive:     isActive,
				IsSecure:     security != wifi.SecurityOpen,
				Security:     security,
				IsVisible:    true,
				AccessPoints: aps,
			}
		}
	}

	// Get known networks
	knownPaths, err := getKnownNetworkPaths(conn)
	if err == nil {
		for _, path := range knownPaths {
			knownObj := conn.Object(iwdDest, path)
			nameVar, _ := knownObj.GetProperty(iwdKnownNetworkIface + ".Name")
			ssid, _ := nameVar.Value().(string)
			if ssid == "" {
				continue
			}

			hiddenVar, err := knownObj.GetProperty(iwdKnownNetworkIface + ".Hidden")
			isHidden := false
			if err == nil {
				if val, ok := hiddenVar.Value().(bool); ok {
					isHidden = val
				}
			}

			autoConnectVar, err := knownObj.GetProperty(iwdKnownNetworkIface + ".AutoConnect")
			autoConnect := false
			if err == nil {
				if val, ok := autoConnectVar.Value().(bool); ok {
					autoConnect = val
				}
			}

			randomizeMAC := false
			if b.stateDir != "" {
				if path, err := b.knownNetworkFile(knownObj, ssid); err == nil {
					if data, err := os.ReadFile(path); err == nil {
						randomizeMAC = keyfileBool(data, settingsGroup, alwaysRandomizeAddressKey)
					}
				}
			}

			if c, exists := visibleNetworks[ssid]; exists {
				c.IsKnown = true
				c.IsHidden = isHidden
				c.AutoConnect = autoConnect
				c.RandomizeMAC = randomizeMAC
				visibleNetworks[ssid] = c
			} else {
				connections = append(connections, wifi.Network{SSID: ssid, IsKnown: true, IsHidden: isHidden, AutoConnect: autoConnect, RandomizeMAC: randomizeMAC})
			}
		}
	}

	for _, c := range visibleNetworks {
		connections = append(connections, c)
	}

	return wifi.NetworksResult{Networks: connections, ScanError: scanErr}, nil
}

func scanAndWait(conn *dbus.Conn, station dbus.ObjectPath) error {
	signals := make(chan *dbus.Signal, 10)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)

	matchOptions := []dbus.MatchOption{
		dbus.WithMatchObjectPath(station),
		dbus.WithMatchInterface(dbusPropertiesIface),
		dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchArg(0, iwdStationIface),
	}
	if err := conn.AddMatchSignal(matchOptions...); err != nil {
		return newScanFailure(wifi.ScanStageSetup, fmt.Errorf("subscribe to scan completion: %w", err))
	}
	defer conn.RemoveMatchSignal(matchOptions...)

	if err := conn.Object(iwdDest, station).Call(iwdStationIface+".Scan", 0).Err; err != nil {
		return newScanFailure(wifi.ScanStageRequest, err)
	}

	return waitForScanCompletion(signals, station, scanCompletionTimeout)
}

func waitForScanCompletion(signals <-chan *dbus.Signal, station dbus.ObjectPath, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	scanningStarted := false
	for {
		select {
		case signal, ok := <-signals:
			if !ok {
				cause := fmt.Errorf("%w: D-Bus signal channel closed", wifi.ErrScanProtocol)
				return newScanFailure(wifi.ScanStageCompletion, cause)
			}

			scanning, matched, err := scanStateChange(signal, station)
			if err != nil {
				return newScanFailure(wifi.ScanStageCompletion, err)
			}
			if !matched {
				continue
			}
			if scanning {
				scanningStarted = true
				continue
			}
			if scanningStarted {
				return nil
			}
		case <-timer.C:
			cause := fmt.Errorf("%w after %s waiting for Scanning to complete", wifi.ErrScanTimeout, timeout)
			return newScanFailure(wifi.ScanStageCompletion, cause)
		}
	}
}

func scanStateChange(signal *dbus.Signal, station dbus.ObjectPath) (scanning, matched bool, err error) {
	if signal == nil {
		return false, false, fmt.Errorf("%w: received an empty D-Bus signal", wifi.ErrScanProtocol)
	}
	if signal.Name != dbusPropertiesIface+".PropertiesChanged" || signal.Path != station {
		return false, false, nil
	}
	if len(signal.Body) < 2 {
		return false, false, fmt.Errorf("%w: PropertiesChanged has %d body fields", wifi.ErrScanProtocol, len(signal.Body))
	}
	iface, ok := signal.Body[0].(string)
	if !ok {
		return false, false, fmt.Errorf("%w: PropertiesChanged interface has type %T", wifi.ErrScanProtocol, signal.Body[0])
	}
	if iface != iwdStationIface {
		return false, false, nil
	}
	changed, ok := signal.Body[1].(map[string]dbus.Variant)
	if !ok {
		return false, false, fmt.Errorf("%w: PropertiesChanged values have type %T", wifi.ErrScanProtocol, signal.Body[1])
	}
	value, ok := changed["Scanning"]
	if !ok {
		return false, false, nil
	}
	scanning, ok = value.Value().(bool)
	if !ok {
		return false, false, fmt.Errorf("%w: Scanning has type %T", wifi.ErrScanProtocol, value.Value())
	}
	return scanning, true, nil
}

func newScanFailure(stage wifi.ScanStage, cause error) *wifi.ScanFailure {
	return &wifi.ScanFailure{
		Backend: "iwd",
		Stage:   stage,
		Code:    dbusErrorName(cause),
		Cause:   cause,
	}
}

func dbusErrorName(err error) string {
	var value dbus.Error
	if errors.As(err, &value) {
		return value.Name
	}
	var pointer *dbus.Error
	if errors.As(err, &pointer) && pointer != nil {
		return pointer.Name
	}
	return ""
}

func (b *Backend) ActivateNetwork(ssid string) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	networkPath, err := findNetworkPath(conn, ssid)
	if err != nil {
		return err
	}
	if networkPath == "" {
		return fmt.Errorf("network %s not found: %w", ssid, wifi.ErrNotFound)
	}
	network := conn.Object(iwdDest, networkPath)

	// iwd treats connecting to the connected network as a no-op. Reconnect
	// instead, like NetworkManager does, so that changed settings such as
	// MAC randomization take effect.
	if connected, err := network.GetProperty(iwdNetworkIface + ".Connected"); err == nil && connected.Value() == true {
		station, err := getStationDevice(conn)
		if err != nil {
			return err
		}
		if err := conn.Object(iwdDest, station).Call(iwdStationIface+".Disconnect", 0).Err; err != nil {
			return fmt.Errorf("failed to disconnect before reconnecting: %w", err)
		}
	}

	// Network.Connect takes no arguments
	return network.Call(iwdNetworkIface+".Connect", 0).Err
}

func (b *Backend) ForgetNetwork(ssid string) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	path, err := findKnownNetworkPath(conn, ssid)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("cannot forget: network %s is not known: %w", ssid, wifi.ErrNotFound)
	}
	// KnownNetwork.Forget takes no arguments
	return conn.Object(iwdDest, path).Call(iwdKnownNetworkIface+".Forget", 0).Err
}

// registerAgent exports a temporary Agent on the D-Bus connection and registers
// it with iwd's AgentManager. The returned cleanup function unregisters the agent.
func registerAgent(conn *dbus.Conn, password string) (cleanup func(), err error) {
	a := &agent{passphrase: password}
	err = conn.Export(a, agentPath, iwdAgentIface)
	if err != nil {
		return nil, fmt.Errorf("failed to export agent: %w", err)
	}

	agentManagerPath := dbus.ObjectPath("/net/connman/iwd")
	err = conn.Object(iwdDest, agentManagerPath).Call(iwdAgentManagerIface+".RegisterAgent", 0, dbus.ObjectPath(agentPath)).Err
	if err != nil {
		conn.Export(nil, agentPath, iwdAgentIface)
		return nil, fmt.Errorf("failed to register agent: %w", err)
	}

	return func() {
		_ = conn.Object(iwdDest, agentManagerPath).Call(iwdAgentManagerIface+".UnregisterAgent", 0, dbus.ObjectPath(agentPath)).Err
		conn.Export(nil, agentPath, iwdAgentIface)
	}, nil
}

func (b *Backend) JoinNetwork(ssid string, password string, security wifi.SecurityType, isHidden bool) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	// Register a temporary agent so iwd can request the passphrase.
	if password != "" {
		cleanup, err := registerAgent(conn, password)
		if err != nil {
			return fmt.Errorf("failed to set up credentials agent: %w", err)
		}
		defer cleanup()
	}

	if isHidden {
		station, err := getStationDevice(conn)
		if err != nil {
			return err
		}
		return conn.Object(iwdDest, station).Call(iwdStationIface+".ConnectHiddenNetwork", 0, ssid).Err
	}

	networkPath, err := findNetworkPath(conn, ssid)
	if err != nil {
		return err
	}
	if networkPath == "" {
		return fmt.Errorf("network %s not found: %w", ssid, wifi.ErrNotFound)
	}
	return conn.Object(iwdDest, networkPath).Call(iwdNetworkIface+".Connect", 0).Err
}

// knownNetworkFile returns the path of the settings file for a known network.
func (b *Backend) knownNetworkFile(known dbus.BusObject, ssid string) (string, error) {
	typeVar, err := known.GetProperty(iwdKnownNetworkIface + ".Type")
	if err != nil {
		return "", err
	}
	networkType, _ := typeVar.Value().(string)
	if networkType == "" {
		return "", fmt.Errorf("unknown iwd network type for %s: %w", ssid, wifi.ErrOperationFailed)
	}
	return filepath.Join(b.stateDir, networkFileName(ssid, networkType)), nil
}

// waitForKnownNetwork waits for iwd to load the settings file for ssid.
func waitForKnownNetwork(conn *dbus.Conn, ssid string) error {
	deadline := time.Now().Add(knownNetworkTimeout)
	for {
		path, err := findKnownNetworkPath(conn, ssid)
		if err != nil {
			return err
		}
		if path != "" {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("iwd did not load the settings for %s: %w", ssid, wifi.ErrOperationFailed)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// JoinNetworkRandomMAC implements wifi.MACRandomizer. It writes the network's
// settings file before connecting, so the first connection already uses a
// random MAC address.
func (b randomMACBackend) JoinNetworkRandomMAC(ssid string, password string, security wifi.SecurityType, isHidden bool) error {
	networkType, ok := iwdNetworkType(security)
	if !ok {
		return fmt.Errorf("MAC randomization is only supported for open and WPA networks with iwd: %w", wifi.ErrNotSupported)
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	path := filepath.Join(b.stateDir, networkFileName(ssid, networkType))
	data, err := os.ReadFile(path)
	created := errors.Is(err, fs.ErrNotExist)
	if err != nil && !created {
		return err
	}
	if created {
		// A new network is provisioned with its credentials. An existing
		// one keeps its own, as iwd ignores new credentials for known
		// networks when connecting.
		if networkType == "psk" && password != "" {
			data = setKeyfileValue(data, securityGroup, "Passphrase", escapeKeyfileValue(password))
		}
		if isHidden {
			data = setKeyfileValue(data, settingsGroup, "Hidden", "true")
		}
	}
	data = setKeyfileValue(data, settingsGroup, alwaysRandomizeAddressKey, "true")
	if err := writeFileAtomic(path, data); err != nil {
		return fmt.Errorf("failed to write iwd settings for %s: %w", ssid, err)
	}

	err = b.connectProvisioned(conn, ssid, isHidden, created)
	if err != nil && created {
		// Don't leave a network that never connected behind as known.
		_ = os.Remove(path)
	}
	return err
}

// connectProvisioned connects to a network after its settings file was
// written. Hidden networks with a settings file can't use
// ConnectHiddenNetwork, so they are found with a scan instead.
func (b randomMACBackend) connectProvisioned(conn *dbus.Conn, ssid string, isHidden bool, created bool) error {
	if created {
		if err := waitForKnownNetwork(conn, ssid); err != nil {
			return err
		}
	} else {
		time.Sleep(configReloadDelay)
	}

	if isHidden {
		station, err := getStationDevice(conn)
		if err != nil {
			return err
		}
		if err := scanAndWait(conn, station); err != nil {
			return err
		}
	}
	return b.ActivateNetwork(ssid)
}

// SetRandomizeMAC implements wifi.MACRandomizer.
func (b randomMACBackend) SetRandomizeMAC(ssid string, randomize bool) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	knownPath, err := findKnownNetworkPath(conn, ssid)
	if err != nil {
		return err
	}
	if knownPath == "" {
		return fmt.Errorf("cannot set MAC randomization: network %s is not known: %w", ssid, wifi.ErrNotFound)
	}
	path, err := b.knownNetworkFile(conn.Object(iwdDest, knownPath), ssid)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read iwd settings for %s: %w", ssid, err)
	}

	value := ""
	if randomize {
		value = "true"
	}
	if err := writeFileAtomic(path, setKeyfileValue(data, settingsGroup, alwaysRandomizeAddressKey, value)); err != nil {
		return fmt.Errorf("failed to write iwd settings for %s: %w", ssid, err)
	}
	time.Sleep(configReloadDelay)
	return nil
}

func (b *Backend) GetSecrets(ssid string) (string, error) {
	return "", fmt.Errorf("getting secrets is not supported by the iwd backend: %w", wifi.ErrNotSupported)
}

func (b *Backend) UpdateNetwork(ssid string, opts wifi.UpdateOptions) error {
	if opts.Password != nil {
		return fmt.Errorf("updating secrets is not supported by the iwd backend: %w", wifi.ErrNotSupported)
	}

	if opts.AutoConnect != nil {
		conn, err := dbus.SystemBus()
		if err != nil {
			return err
		}
		path, err := findKnownNetworkPath(conn, ssid)
		if err != nil {
			return err
		}
		if path == "" {
			return fmt.Errorf("cannot set autoconnect: network %s is not known: %w", ssid, wifi.ErrNotFound)
		}

		variant := dbus.MakeVariant(*opts.AutoConnect)
		return conn.Object(iwdDest, path).Call("org.freedesktop.DBus.Properties.Set", 0, iwdKnownNetworkIface, "AutoConnect", variant).Err
	}

	return nil
}

func (b *Backend) IsWirelessEnabled() (bool, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return false, err
	}
	station, err := getStationDevice(conn)
	if err != nil {
		return false, err
	}
	poweredVar, err := conn.Object(iwdDest, station).GetProperty(iwdDeviceIface + ".Powered")
	if err != nil {
		return false, err
	}
	return poweredVar.Value().(bool), nil
}

func (b *Backend) SetWireless(enabled bool) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	station, err := getStationDevice(conn)
	if err != nil {
		return err
	}
	obj := conn.Object(iwdDest, station)
	variant := dbus.MakeVariant(enabled)
	err = obj.Call("org.freedesktop.DBus.Properties.Set", 0, iwdDeviceIface, "Powered", variant).Err
	if err != nil {
		return err
	}

	// Block until the property is updated.
	signals := make(chan *dbus.Signal, 10)
	matchPath := dbus.WithMatchObjectPath(station)
	matchInterface := dbus.WithMatchInterface("org.freedesktop.DBus.Properties")
	conn.Signal(signals)
	conn.AddMatchSignal(matchInterface, matchPath)
	defer conn.RemoveMatchSignal(matchInterface, matchPath)

	for {
		select {
		case signal := <-signals:
			if signal.Name == "org.freedesktop.DBus.Properties.PropertiesChanged" {
				if len(signal.Body) < 2 {
					continue
				}
				iface, ok := signal.Body[0].(string)
				if !ok || iface != iwdDeviceIface {
					continue
				}
				props, ok := signal.Body[1].(map[string]dbus.Variant)
				if !ok {
					continue
				}
				if val, ok := props["Powered"]; ok {
					if val.Value().(bool) == enabled {
						return nil
					}
				}
			}
		case <-time.After(propertyChangeTimeout):
			return fmt.Errorf("timed out waiting for wireless state change")
		}
	}
}
