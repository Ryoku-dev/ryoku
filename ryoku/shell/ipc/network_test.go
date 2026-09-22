package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// apSecurity derives the reference SecurityType from the NetworkManager AP flag
// triple. WPA3 (SAE) and Enterprise (802.1X) win over the plain RSN/WPA/WEP/open
// ladder, and each flag family is checked against the right bit.
func TestApSecurity(t *testing.T) {
	const (
		privacy = 0x1
		psk     = 0x100
		km8021x = 0x200
		sae     = 0x400
	)
	cases := []struct {
		name            string
		flags, wpa, rsn uint32
		want            string
	}{
		{"open", 0, 0, 0, "None"},
		{"wep", privacy, 0, 0, "Wep"},
		{"wpa only", privacy, psk, 0, "Wpa"},
		{"wpa2 rsn", privacy, 0, psk, "Wpa2"},
		{"wpa2 measured", 0x1, 392, 392, "Wpa2"}, // the live "M" network
		{"wpa3 sae in rsn", privacy, 0, sae, "Wpa3"},
		{"wpa3 sae in wpa", privacy, sae, 0, "Wpa3"},
		{"enterprise rsn", privacy, 0, km8021x, "Enterprise"},
		{"enterprise beats wpa2", privacy, 0, km8021x | psk, "Enterprise"},
	}
	for _, c := range cases {
		if got := apSecurity(c.flags, c.wpa, c.rsn); got != c.want {
			t.Errorf("%s: apSecurity(%#x,%#x,%#x) = %q, want %q", c.name, c.flags, c.wpa, c.rsn, got, c.want)
		}
	}
}

// connectivityFromState maps the NMDeviceState to the three-state reveal label:
// only 100 is Connected, the 40..90 activation range is Connecting, everything
// else Disconnected.
func TestConnectivityFromState(t *testing.T) {
	cases := []struct {
		state uint32
		want  string
	}{
		{100, "Connected"},
		{40, "Connecting"}, {60, "Connecting"}, {90, "Connecting"},
		{30, "Disconnected"}, {20, "Disconnected"}, {0, "Disconnected"},
		{120, "Disconnected"}, // failed
	}
	for _, c := range cases {
		if got := connectivityFromState(c.state); got != c.want {
			t.Errorf("connectivityFromState(%d) = %q, want %q", c.state, got, c.want)
		}
	}
}

// wifiConnectSettings omits the security block for an open network, derives
// key-mgmt from the resolved AP's security (sae for WPA3, wpa-psk otherwise),
// pins the band only when the caller supplied a BSSID and the AP is known, and
// marks 802-11-wireless.hidden only for a hidden network.
func TestWifiConnectSettings(t *testing.T) {
	wpa2 := &apInfo{Security: "Wpa2", Frequency: 5240}
	wpa3 := &apInfo{Security: "Wpa3", Frequency: 5240}
	cases := []struct {
		name         string
		ssid         string
		password     string
		bssid        string
		ap           *apInfo
		hidden       bool
		wantSecurity bool
		wantKeyMgmt  string
		wantBandKey  bool
		wantBand     string
	}{
		{name: "open network", ssid: "cafe"},
		{name: "wpa2 with bssid pins band a", ssid: "home", password: "hunter2", bssid: "AA:BB:CC:DD:EE:FF", ap: wpa2, wantSecurity: true, wantKeyMgmt: "wpa-psk", wantBandKey: true, wantBand: "a"},
		{name: "wpa3 uses sae", ssid: "secure", password: "hunter2", bssid: "AA:BB:CC:DD:EE:FF", ap: wpa3, wantSecurity: true, wantKeyMgmt: "sae", wantBandKey: true, wantBand: "a"},
		{name: "nil ap omits band", ssid: "roam", password: "hunter2", bssid: "AA:BB:CC:DD:EE:FF", wantSecurity: true, wantKeyMgmt: "wpa-psk"},
		{name: "empty bssid omits band", ssid: "roam", password: "hunter2", ap: wpa2, wantSecurity: true, wantKeyMgmt: "wpa-psk"},
		{name: "hidden open network", ssid: "secret", hidden: true},
		{name: "hidden wpa2 keeps security", ssid: "secret", password: "hunter2", hidden: true, wantSecurity: true, wantKeyMgmt: "wpa-psk"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := wifiConnectSettings(c.ssid, c.password, c.bssid, c.ap, c.hidden)
			if ssid, _ := s["802-11-wireless"]["ssid"].Value().([]byte); string(ssid) != c.ssid {
				t.Errorf("ssid = %q, want %q", ssid, c.ssid)
			}
			if typ, _ := s["connection"]["type"].Value().(string); typ != "802-11-wireless" {
				t.Errorf("type = %q, want 802-11-wireless", typ)
			}
			band, hasBand := s["802-11-wireless"]["band"]
			if hasBand != c.wantBandKey {
				t.Fatalf("band key present = %v, want %v", hasBand, c.wantBandKey)
			}
			if c.wantBandKey {
				if got, _ := band.Value().(string); got != c.wantBand {
					t.Errorf("band = %q, want %q", got, c.wantBand)
				}
			}
			sec, hasSec := s["802-11-wireless-security"]
			if hasSec != c.wantSecurity {
				t.Fatalf("security block present = %v, want %v", hasSec, c.wantSecurity)
			}
			if c.wantSecurity {
				if km, _ := sec["key-mgmt"].Value().(string); km != c.wantKeyMgmt {
					t.Errorf("key-mgmt = %q, want %q", km, c.wantKeyMgmt)
				}
				if psk, _ := sec["psk"].Value().(string); psk != c.password {
					t.Errorf("psk = %q, want %q", psk, c.password)
				}
			}
			if hid, hasHid := s["802-11-wireless"]["hidden"]; hasHid != c.hidden {
				t.Fatalf("hidden key present = %v, want %v", hasHid, c.hidden)
			} else if c.hidden {
				if got, _ := hid.Value().(bool); !got {
					t.Errorf("hidden = %v, want true", got)
				}
			}
		})
	}
}

// bandForFrequency labels the reveal band from an AP centre frequency: 2.4 GHz
// below 2500, 5 GHz through 5924, 6 GHz from 5925, and "" for an unknown 0.
func TestBandForFrequency(t *testing.T) {
	cases := []struct {
		mhz  int
		want string
	}{
		{0, ""},
		{2412, "2.4"}, {2484, "2.4"}, {2499, "2.4"},
		{2500, "5"}, {5240, "5"}, {5924, "5"},
		{5925, "6"}, {5955, "6"}, {7115, "6"},
	}
	for _, c := range cases {
		if got := bandForFrequency(c.mhz); got != c.want {
			t.Errorf("bandForFrequency(%d) = %q, want %q", c.mhz, got, c.want)
		}
	}
}

// nmBandForFrequency maps a frequency to NetworkManager's band value: bg for
// 2.4 GHz, a for 5 GHz, and "" for 6 GHz and unknown, because NM has no safe
// 6 GHz value and an invalid one fails AddAndActivateConnection.
func TestNmBandForFrequency(t *testing.T) {
	cases := []struct {
		mhz  int
		want string
	}{
		{0, ""},
		{2412, "bg"},
		{5240, "a"}, {5924, "a"},
		{5955, ""}, {7115, ""},
	}
	for _, c := range cases {
		if got := nmBandForFrequency(c.mhz); got != c.want {
			t.Errorf("nmBandForFrequency(%d) = %q, want %q", c.mhz, got, c.want)
		}
	}
}

// keyMgmtForSecurity selects sae only for WPA3 so a WPA3-only 5 GHz network can
// be joined, and wpa-psk for WPA2/WPA and anything unknown.
func TestKeyMgmtForSecurity(t *testing.T) {
	cases := []struct {
		security string
		want     string
	}{
		{"Wpa3", "sae"},
		{"Wpa2", "wpa-psk"},
		{"Wpa", "wpa-psk"},
		{"None", "wpa-psk"},
		{"", "wpa-psk"},
	}
	for _, c := range cases {
		if got := keyMgmtForSecurity(c.security); got != c.want {
			t.Errorf("keyMgmtForSecurity(%q) = %q, want %q", c.security, got, c.want)
		}
	}
}

// ssidSaved matches only stored wifi profiles with an equal SSID, ignoring
// non-wifi profiles that happen to share the string.
func TestSsidSaved(t *testing.T) {
	saved := []savedConn{
		{typ: "802-11-wireless", ssid: "home"},
		{typ: "wireguard", id: "vpn", ssid: ""},
		{typ: "802-3-ethernet", ssid: "cafe"}, // wrong type, same string
	}
	if !ssidSaved("home", saved) {
		t.Error("home should be saved")
	}
	if ssidSaved("cafe", saved) {
		t.Error("cafe is ethernet, not a saved wifi profile")
	}
	if ssidSaved("unknown", saved) {
		t.Error("unknown should not be saved")
	}
}

// ssidAutoconnect returns a saved wifi profile's stored autoconnect and false
// for anything that is not a saved wifi profile.
func TestSsidAutoconnect(t *testing.T) {
	saved := []savedConn{
		{typ: "802-11-wireless", ssid: "home", autoconnect: true},
		{typ: "802-11-wireless", ssid: "cafe", autoconnect: false},
		{typ: "802-3-ethernet", ssid: "wired", autoconnect: true}, // wrong type
	}
	if !ssidAutoconnect("home", saved) {
		t.Error("home autoconnect should be on")
	}
	if ssidAutoconnect("cafe", saved) {
		t.Error("cafe autoconnect was turned off")
	}
	if ssidAutoconnect("wired", saved) {
		t.Error("wired is ethernet, not a saved wifi profile")
	}
	if ssidAutoconnect("unknown", saved) {
		t.Error("unknown is not saved")
	}
}

// apFrame carries every field the reveal binds to.
func TestApFrame(t *testing.T) {
	f := apFrame(&apInfo{Ssid: "M", Strength: 50, Security: "Wpa2", Bssid: "D6:31:27:89:88:78", Frequency: 5280, Saved: true, Active: true, Autoconnect: true})
	for _, k := range []string{"ssid", "strength", "security", "bssid", "frequency", "saved", "active", "autoconnect"} {
		if _, ok := f[k]; !ok {
			t.Errorf("apFrame missing key %q", k)
		}
	}
	if f["ssid"] != "M" || f["strength"] != 50 || f["active"] != true {
		t.Errorf("apFrame values wrong: %+v", f)
	}
}

// A profile edit must drop the deprecated ipv4/ipv6 address and route mirrors:
// NetworkManager rejects the round-trip otherwise (godbus re-encodes the ipv6
// a(ayuay) pair as aav), while address-data/route-data carry the real config.
func TestDropLegacyAddresses(t *testing.T) {
	s := map[string]map[string]dbus.Variant{
		"connection": {"id": dbus.MakeVariant("M")},
		"ipv4": {
			"method":       dbus.MakeVariant("manual"),
			"addresses":    dbus.MakeVariant([][]uint32{{1, 24, 0}}),
			"routes":       dbus.MakeVariant([][]uint32{}),
			"address-data": dbus.MakeVariant([]map[string]dbus.Variant{{"address": dbus.MakeVariant("192.168.77.5")}}),
		},
		"ipv6": {
			"method":    dbus.MakeVariant("auto"),
			"addresses": dbus.MakeVariant([]dbus.Variant{}),
			"routes":    dbus.MakeVariant([]dbus.Variant{}),
		},
	}
	dropLegacyAddresses(s)
	for _, group := range []string{"ipv4", "ipv6"} {
		for _, key := range []string{"addresses", "routes"} {
			if _, ok := s[group][key]; ok {
				t.Errorf("%s.%s survived the prune", group, key)
			}
		}
	}
	if _, ok := s["ipv4"]["address-data"]; !ok {
		t.Error("ipv4.address-data was dropped; the static address would be lost")
	}
	if s["ipv4"]["method"].Value() != "manual" || s["connection"]["id"].Value() != "M" {
		t.Errorf("unrelated settings changed: %+v", s)
	}
}

func TestDnsServersForSelection(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		custom   []string
		want     []string
		wantErr  bool
	}{
		{"dhcp", "dhcp", nil, nil, false},
		{"cloudflare", "cloudflare", nil, []string{
			"1.1.1.1", "1.0.0.1",
			"2606:4700:4700::1111", "2606:4700:4700::1001",
		}, false},
		{"google", "google", nil, []string{
			"8.8.8.8", "8.8.4.4",
			"2001:4860:4860::8888", "2001:4860:4860::8844",
		}, false},
		{"custom normalizes and deduplicates", "custom", []string{
			" 192.0.2.53 ", "2001:db8::53", "192.0.2.53",
		}, []string{"192.0.2.53", "2001:db8::53"}, false},
		{"custom requires a server", "custom", nil, nil, true},
		{"custom rejects hostnames", "custom", []string{"dns.example.com"}, nil, true},
		{"unknown provider", "quad9", nil, nil, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := dnsServersForSelection(c.provider, c.custom)
			if (err != nil) != c.wantErr {
				t.Fatalf("dnsServersForSelection() error = %v, wantErr %v", err, c.wantErr)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("dnsServersForSelection() = %#v, want %#v", got, c.want)
			}
		})
	}
}

func TestDnsProviderForServers(t *testing.T) {
	cases := []struct {
		name    string
		servers []string
		want    string
	}{
		{"empty is dhcp", nil, "dhcp"},
		{"cloudflare ignores order", []string{
			"2606:4700:4700::1001", "1.0.0.1",
			"2606:4700:4700::1111", "1.1.1.1",
		}, "cloudflare"},
		{"google", []string{
			"8.8.8.8", "8.8.4.4",
			"2001:4860:4860::8888", "2001:4860:4860::8844",
		}, "google"},
		{"provider subset stays custom", []string{"1.1.1.1"}, "custom"},
		{"unrecognized is custom", []string{"192.0.2.53"}, "custom"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dnsProviderForServers(c.servers); got != c.want {
				t.Errorf("dnsProviderForServers(%#v) = %q, want %q", c.servers, got, c.want)
			}
		})
	}
}

func TestDnsHelperArgs(t *testing.T) {
	got, err := dnsHelperArgs("Custom", []string{"192.0.2.53", "2001:db8::53"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"custom", "192.0.2.53", "2001:db8::53"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dnsHelperArgs() = %#v, want %#v", got, want)
	}
}

func TestDnsHelperPathUsesExecutableDirectory(t *testing.T) {
	previousShellDir := shellDir
	shellDir = ""
	t.Cleanup(func() { shellDir = previousShellDir })

	// Point the packaged-helper probe at an absent path so the executable-dir
	// fallback is reachable regardless of whether this host has ryoku-dns.
	previousPackaged := dnsPackagedHelper
	dnsPackagedHelper = filepath.Join(t.TempDir(), "absent-ryoku-dns")
	t.Cleanup(func() { dnsPackagedHelper = previousPackaged })

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(executable), "ryoku-dns")
	if got := dnsHelperPath(); got != want {
		t.Errorf("dnsHelperPath() = %q, want %q", got, want)
	}
}

func TestDnsHelperPathPrefersPackaged(t *testing.T) {
	pkg := filepath.Join(t.TempDir(), "ryoku-dns")
	if err := os.WriteFile(pkg, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	previousPackaged := dnsPackagedHelper
	dnsPackagedHelper = pkg
	t.Cleanup(func() { dnsPackagedHelper = previousPackaged })

	if got := dnsHelperPath(); got != pkg {
		t.Errorf("dnsHelperPath() = %q, want the packaged helper %q", got, pkg)
	}
}

// TestLiveNetworkFrame exercises the real snapshot path against the running
// NetworkManager and prints the frame, as evidence the topic renders live data.
// Gated so the default `go test` stays deterministic and bus-free.
func TestLiveNetworkFrame(t *testing.T) {
	if os.Getenv("RYOKU_LIVE_DBUS") == "" {
		t.Skip("set RYOKU_LIVE_DBUS=1 to run the live NetworkManager integration")
	}
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		t.Skipf("no system bus: %v", err)
	}
	defer conn.Close()
	n := &networkState{conn: conn, topic: newStateTopic()}
	sub := n.topic.subscribe()
	defer n.topic.unsubscribe(sub)
	n.publish()
	select {
	case frame := <-sub.frames:
		t.Logf("network frame: %s", frame)
		var m map[string]any
		if err := json.Unmarshal(frame, &m); err != nil {
			t.Fatalf("frame is not valid JSON: %v", err)
		}
		for _, k := range []string{"wifi", "wired", "accessPoints", "wireguard", "dns"} {
			if _, ok := m[k]; !ok {
				t.Errorf("frame missing key %q", k)
			}
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no network frame published")
	}
}
