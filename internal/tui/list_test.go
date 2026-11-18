package tui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/shazow/wifitui/wifi"
)

func TestListModel_NewKey(t *testing.T) {
	m := NewListModel()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	nKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	newComp, _ := m.Update(nKeyMsg)

	editModel, ok := newComp.(*EditModel)
	if !ok {
		t.Errorf("expected a new EditModel but got %T", newComp)
		return
	}

	if got, want := editModel.availableContentWidth(), 96; got != want {
		t.Fatalf("expected edit model content width %d, got %d", want, got)
	}

	if got, want := editModel.ssidAdapter.Model.Width, 92; got != want {
		t.Fatalf("expected SSID input width %d, got %d", want, got)
	}
}

func TestListModel_ConnectPreservesAutoConnect(t *testing.T) {
	m := NewListModel()
	network := wifi.Network{SSID: "Saved network", IsKnown: true, AutoConnect: true}
	m.Update(networksLoadedMsg{network})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if cmd == nil {
		t.Fatal("expected a connect command")
	}
	msg := cmd()
	connect, ok := msg.(connectMsg)
	if !ok {
		t.Fatalf("expected a connectMsg but got %T", msg)
	}
	if connect.item.SSID != network.SSID || connect.autoConnect != network.AutoConnect {
		t.Errorf("connect request for %q with auto-connect %t, want %q with auto-connect %t",
			connect.item.SSID, connect.autoConnect, network.SSID, network.AutoConnect)
	}
}

func TestListModel_RefreshKeepsFilterAndPage(t *testing.T) {
	networks := []wifi.Network{{SSID: "Library"}}
	for i := range 12 {
		networks = append(networks, wifi.Network{SSID: fmt.Sprintf("Cafe %02d", i)})
	}
	refreshed := append([]wifi.Network(nil), networks...)
	for i := 1; i < len(refreshed); i++ {
		refreshed[i].SSID += " updated"
	}
	tests := []struct {
		name string
		msg  tea.Msg
	}{
		{"reload", networksLoadedMsg(refreshed)},
		{"scan", scanFinishedMsg{networks: refreshed}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewListModel()
			m.Update(tea.WindowSizeMsg{Width: 100, Height: 16})
			m.Update(networksLoadedMsg(networks))
			m.list.SetFilterText("Cafe")
			if m.list.Paginator.TotalPages < 2 {
				t.Fatal("expected the filtered results to span multiple pages")
			}
			index := m.list.Paginator.PerPage
			m.list.Select(index)
			_, cmd := m.Update(tt.msg)
			if cmd != nil {
				m.Update(cmd())
			}
			if got := len(m.list.VisibleItems()); got != 12 {
				t.Errorf("visible items = %d, want 12", got)
			}
			wantSSID := refreshed[index+1].SSID
			if item, ok := m.list.SelectedItem().(networkItem); !ok || item.SSID != wantSSID {
				t.Errorf("selected item = %v, want %q", m.list.SelectedItem(), wantSSID)
			}
			if m.list.Paginator.Page != 1 || !m.list.KeyMap.NextPage.Enabled() {
				t.Errorf("refresh left page at %d with next page enabled %t", m.list.Paginator.Page, m.list.KeyMap.NextPage.Enabled())
			}
		})
	}
}

func TestListModel_ForgetFlow(t *testing.T) {
	m := NewListModel()
	item1 := networkItem{Network: wifi.Network{SSID: "TestNetwork1", IsKnown: true}}
	item2 := networkItem{Network: wifi.Network{SSID: "TestNetwork2", IsKnown: true}}
	m.list.SetItems([]list.Item{item1, item2})

	// Press 'f' to start forgetting
	fKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")}
	m.Update(fKeyMsg)

	if !m.isForgetting {
		t.Fatal("isForgetting was not set to true")
	}

	// Press 'n' to cancel
	nKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	m.Update(nKeyMsg)

	if m.isForgetting {
		t.Fatal("isForgetting was not cleared after pressing 'n'")
	}

	// Press 'f' again
	m.Update(fKeyMsg)

	// Press 'j' to navigate, should be ignored
	downKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	m.Update(downKeyMsg)

	if !m.isForgetting {
		t.Fatal("isForgetting should not be cleared after pressing a navigation key")
	}

	// Press 'y' to confirm
	yKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}
	_, cmd := m.Update(yKeyMsg)

	if m.isForgetting {
		t.Fatal("isForgetting was not cleared after pressing 'y'")
	}

	msg := cmd()
	if forgetMsg, ok := msg.(forgetNetworkMsg); !ok {
		t.Errorf("expected a forgetNetworkMsg but got %T", msg)
	} else {
		if forgetMsg.item.SSID != "TestNetwork1" {
			t.Errorf("expected forgetNetworkMsg for 'TestNetwork1' but got for '%s'", forgetMsg.item.SSID)
		}
	}
}

func TestItemDelegate_RenderAPCountAnnotation(t *testing.T) {
	originalTheme := CurrentTheme
	CurrentTheme = NewDefaultTheme()
	CurrentTheme.AccessPointIcon = "📶"
	t.Cleanup(func() {
		CurrentTheme = originalTheme
	})

	d := itemDelegate{listModel: &ListModel{ssidColumnWidth: 30}}
	m := list.New([]list.Item{}, d, 80, 5)

	item := networkItem{Network: wifi.Network{
		SSID:      "Cafe Wifi",
		IsVisible: true,
		AccessPoints: []wifi.AccessPoint{
			{BSSID: "00:00:00:00:00:01"},
			{BSSID: "00:00:00:00:00:02"},
			{BSSID: "00:00:00:00:00:03"},
		},
	}}

	var buf bytes.Buffer
	d.Render(&buf, m, 0, item)
	out := buf.String()

	if !strings.Contains(out, "📶×3") {
		t.Fatalf("expected rendered output to contain AP count annotation, got: %q", out)
	}
}

func TestItemDelegate_RenderNoAnnotationForSingleAP(t *testing.T) {
	originalTheme := CurrentTheme
	CurrentTheme = NewDefaultTheme()
	t.Cleanup(func() {
		CurrentTheme = originalTheme
	})

	d := itemDelegate{listModel: &ListModel{ssidColumnWidth: 30}}
	m := list.New([]list.Item{}, d, 80, 5)

	item := networkItem{Network: wifi.Network{
		SSID:         "Home Wifi",
		IsVisible:    true,
		AccessPoints: []wifi.AccessPoint{{BSSID: "00:00:00:00:00:01"}},
	}}

	var buf bytes.Buffer
	d.Render(&buf, m, 0, item)
	out := buf.String()

	if strings.Contains(out, "×1") {
		t.Fatalf("expected no AP count annotation for a single AP, got: %q", out)
	}
}
