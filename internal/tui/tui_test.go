package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shazow/wifitui/wifi"
	"github.com/shazow/wifitui/wifi/mock"
)

func TestTuiModel_ScanFinishedUpdatesList(t *testing.T) {
	backend, err := mock.New()
	if err != nil {
		t.Fatalf("mock.New() failed: %v", err)
	}
	connections := []wifi.Network{
		{SSID: "TestNet1"},
		{SSID: "TestNet2"},
	}

	m, err := NewModel(backend)
	if err != nil {
		t.Fatalf("NewModel failed: %v", err)
	}

	// Set a size for the model, otherwise the list component won't have enough space to render.
	sizeMsg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updatedModel, _ := m.Update(sizeMsg)
	m = updatedModel.(*model)

	// Simulate a scan finishing
	msg := scanFinishedMsg{networks: connections}
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(*model)

	// Check the view
	view := m.View()

	if !strings.Contains(view, "TestNet1") {
		t.Errorf("View does not contain 'TestNet1' in\n%s", view)
	}
	if !strings.Contains(view, "TestNet2") {
		t.Errorf("View does not contain 'TestNet2' in\n%s", view)
	}
}

func TestTuiModel_ScanWarningKeepsListVisible(t *testing.T) {
	backend, err := mock.New()
	if err != nil {
		t.Fatalf("mock.New() failed: %v", err)
	}
	mockBackend := backend.(*mock.MockBackend)
	mockBackend.ActionSleep = 0

	m, err := NewModel(cachedBackend{
		Backend: backend,
	})
	if err != nil {
		t.Fatalf("NewModel failed: %v", err)
	}

	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updatedModel.(*model)

	updatedModel, cmd := m.Update(scanMsg{mode: wifi.ScanAuto})
	m = updatedModel.(*model)
	if cmd == nil {
		t.Fatal("scanMsg did not return a command")
	}
	m = runTUITestCommand(t, m, cmd)

	view := m.View()
	if !strings.Contains(view, "Unencrypted_Honeypot") {
		t.Errorf("View does not contain cached network after scan warning in\n%s", view)
	}
	if !strings.Contains(view, "Scan failed: scan not allowed") {
		t.Errorf("View does not contain scan warning in\n%s", view)
	}
	if strings.Contains(view, "Error:") {
		t.Errorf("View should not show the fatal error screen for scan warning in\n%s", view)
	}
}

func TestTuiModel_ManualScanForcesRefresh(t *testing.T) {
	backend, err := mock.New()
	if err != nil {
		t.Fatalf("mock.New() failed: %v", err)
	}
	watch := &watchBackend{
		Backend: backend,
		networks: []wifi.Network{
			{SSID: "ManualScanNet", IsVisible: true},
		},
	}

	m, err := NewModel(watch)
	if err != nil {
		t.Fatalf("NewModel failed: %v", err)
	}
	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updatedModel.(*model)
	m = runTUITestCommand(t, m, cmd)

	if len(watch.listScans) != 1 || watch.listScans[0] != wifi.ScanForce {
		t.Fatalf("manual scan used scans %#v, want only ScanForce", watch.listScans)
	}
}

func TestStartNetworkChangeWatcher(t *testing.T) {
	backend, err := mock.New()
	if err != nil {
		t.Fatalf("mock.New() failed: %v", err)
	}
	changes := make(chan struct{})
	watch := &watchBackend{
		Backend: backend,
		changes: changes,
	}

	cmd := startNetworkChangeWatcher(watch)
	if cmd == nil {
		t.Fatal("startNetworkChangeWatcher returned nil command for watcher backend")
	}

	msg := cmd()
	started, ok := msg.(networkWatchStartedMsg)
	if !ok {
		t.Fatalf("watch command returned %T, want networkWatchStartedMsg", msg)
	}
	if !watch.watchCalled {
		t.Fatal("watch command did not call WatchNetworkChanges")
	}
	if started.changes != (<-chan struct{})(changes) {
		t.Fatal("watch command returned a different changes channel")
	}
	if started.cancel == nil {
		t.Fatal("watch command returned a nil cancel function")
	}

	started.cancel()
	select {
	case <-watch.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("watch cancel function did not cancel watcher context")
	}
}

func TestTuiModel_NetworkChangeDebouncesRefresh(t *testing.T) {
	backend, err := mock.New()
	if err != nil {
		t.Fatalf("mock.New() failed: %v", err)
	}
	changes := make(chan struct{})
	watch := &watchBackend{
		Backend: backend,
		changes: changes,
		networks: []wifi.Network{
			{SSID: "UpdatedNet", IsVisible: true},
		},
	}

	m, err := NewModel(watch)
	if err != nil {
		t.Fatalf("NewModel failed: %v", err)
	}
	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updatedModel.(*model)

	updatedModel, firstCmd := m.Update(networkChangedMsg{changes: changes})
	m = updatedModel.(*model)
	if firstCmd == nil {
		t.Fatal("first networkChangedMsg did not schedule a debounce command")
	}
	firstMsg := firstCmd()
	firstBatch, ok := firstMsg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("first networkChangedMsg returned %T, want tea.BatchMsg", firstMsg)
	}
	if len(firstBatch) != 2 {
		t.Fatalf("first networkChangedMsg returned %d commands, want watcher wait and debounce", len(firstBatch))
	}
	if !m.networkRefreshPending {
		t.Fatal("first networkChangedMsg did not mark a refresh as pending")
	}

	updatedModel, secondCmd := m.Update(networkChangedMsg{changes: changes})
	m = updatedModel.(*model)
	if secondCmd == nil {
		t.Fatal("second networkChangedMsg did not keep waiting for watcher changes")
	}
	if !m.networkRefreshPending {
		t.Fatal("second networkChangedMsg cleared the pending debounce")
	}

	updatedModel, refreshCmd := m.Update(networkDebouncedMsg{})
	m = updatedModel.(*model)
	if refreshCmd == nil {
		t.Fatal("networkDebouncedMsg did not return a refresh command")
	}
	if m.networkRefreshPending {
		t.Fatal("networkDebouncedMsg did not clear the pending debounce")
	}

	msg := refreshCmd()
	loaded, ok := msg.(networksLoadedMsg)
	if !ok {
		t.Fatalf("refresh command returned %T, want networksLoadedMsg", msg)
	}
	if len(watch.listScans) != 1 || watch.listScans[0] != wifi.ScanNever {
		t.Fatalf("refresh command used scans %#v, want only ScanNever", watch.listScans)
	}

	updatedModel, _ = m.Update(loaded)
	m = updatedModel.(*model)
	view := m.View()
	if !strings.Contains(view, "UpdatedNet") {
		t.Errorf("View does not contain debounced network refresh result in\n%s", view)
	}
}

func TestTuiModel_EnableRadioSwitchesView(t *testing.T) {
	backend, err := mock.New()
	if err != nil {
		t.Fatalf("mock.New() failed: %v", err)
	}

	m, err := NewModel(backend)
	if err != nil {
		t.Fatalf("NewModel failed: %v", err)
	}

	// Manually trigger the disabled state
	disabledMsg := errorMsg{err: wifi.ErrWirelessDisabled}
	updatedModel, _ := m.Update(disabledMsg)
	m = updatedModel.(*model)

	// Verify we are in the disabled view
	view := m.View()
	if !strings.Contains(view, "WiFi is disabled.") {
		t.Fatalf("View does not contain 'WiFi is disabled.' in\n%s", view)
	}

	// Now, pop the view. This is what happens when the radio is enabled.
	// The OnLeave hook should take care of the rest.
	updatedModel, cmd := m.Update(popViewMsg{})
	m = updatedModel.(*model)

	// The batch command contains a command to start the scanner and a command to do an initial scan.
	// Let's execute the commands and process the resulting messages.
	batchCmd := cmd().(tea.BatchMsg)
	var scanCmd tea.Cmd
	for _, c := range batchCmd {
		msg := c()
		// We only care about the scanMsg for this test's purpose.
		if _, ok := msg.(scanMsg); ok {
			updatedModel, scanCmd = m.Update(msg)
			m = updatedModel.(*model)
			break
		}
	}

	if scanCmd == nil {
		t.Fatal("did not find a scan command in the batch")
	}

	// Now execute the command that builds the network list
	scanFinishedMsg := scanCmd()
	updatedModel, _ = m.Update(scanFinishedMsg)
	m = updatedModel.(*model)

	// And we need to give the list a size
	sizeMsg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updatedModel, _ = m.Update(sizeMsg)
	m = updatedModel.(*model)

	view = m.View()
	if strings.Contains(view, "WiFi is disabled.") {
		t.Errorf("View still contains 'WiFi is disabled.' after enabling radio in\n%s", view)
	}
	// The mock backend will return its default list of networks.
	if !strings.Contains(view, "WiFi Network") {
		t.Errorf("View does not contain network list title after enabling radio in\n%s", view)
	}
}

type cachedBackend struct {
	wifi.Backend
}

func (b cachedBackend) ListNetworks(scan wifi.ScanMode) (wifi.NetworksResult, error) {
	result, err := b.Backend.ListNetworks(scan)
	result.ScanError = errors.New("scan not allowed")
	return result, err
}

type watchBackend struct {
	wifi.Backend
	changes     chan struct{}
	watchCalled bool
	ctx         context.Context
	listScans   []wifi.ScanMode
	networks    []wifi.Network
}

func (b *watchBackend) WatchNetworkChanges(ctx context.Context) (<-chan struct{}, error) {
	b.watchCalled = true
	b.ctx = ctx
	return b.changes, nil
}

func (b *watchBackend) ListNetworks(scan wifi.ScanMode) (wifi.NetworksResult, error) {
	b.listScans = append(b.listScans, scan)
	if b.networks != nil {
		return wifi.NetworksResult{Networks: b.networks}, nil
	}
	return b.Backend.ListNetworks(scan)
}

func runTUITestCommand(t *testing.T, m *model, cmd tea.Cmd) *model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, batchCmd := range batch {
			m = runTUITestCommand(t, m, batchCmd)
		}
		return m
	}

	updatedModel, nextCmd := m.Update(msg)
	updated, ok := updatedModel.(*model)
	if !ok {
		t.Fatalf("Update(%T) returned %T, want *model", msg, updatedModel)
	}
	return runTUITestCommand(t, updated, nextCmd)
}
