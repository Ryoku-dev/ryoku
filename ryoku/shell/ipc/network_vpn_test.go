package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

// A VPN control only accepts WireGuard and VPN profiles.
func TestVpnProfileRefusesOtherTypes(t *testing.T) {
	saved := []savedConn{
		{uuid: "w", typ: "wireguard"},
		{uuid: "v", typ: "vpn"},
		{uuid: "f", typ: "802-11-wireless"},
		{uuid: "e", typ: "802-3-ethernet"},
	}
	for _, ok := range []string{"w", "v"} {
		if _, err := vpnProfile(saved, ok); err != nil {
			t.Errorf("%s refused: %v", ok, err)
		}
	}
	for uuid, want := range map[string]string{"f": "not a VPN", "e": "not a VPN", "x": "no saved", "": "empty"} {
		if _, err := vpnProfile(saved, uuid); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("vpnProfile(%q) err = %v, want %q", uuid, err, want)
		}
	}
}

// Only the listed vpn.data keys are shown.
func TestVpnShownData(t *testing.T) {
	g := map[string]dbus.Variant{"data": dbus.MakeVariant(map[string]string{
		"remote": "vpn.example.com", "username": "me", "cert": "/home/me/c.pem", "ta-dir": "1",
	})}
	want := map[string]string{"remote": "vpn.example.com", "username": "me"}
	if got := vpnShownData(g); !reflect.DeepEqual(got, want) {
		t.Fatalf("vpnShownData = %v, want %v", got, want)
	}
}

func wgSettings() map[string]map[string]dbus.Variant {
	return map[string]map[string]dbus.Variant{
		"connection": {"id": dbus.MakeVariant("work"), "interface-name": dbus.MakeVariant("wg-work")},
		"wireguard": {
			"listen-port": dbus.MakeVariant(uint32(51820)),
			"peers": dbus.MakeVariant([]map[string]dbus.Variant{{
				"public-key":           dbus.MakeVariant("PEERKEY="),
				"endpoint":             dbus.MakeVariant("vpn.example.com:51820"),
				"allowed-ips":          dbus.MakeVariant([]string{"10.0.0.0/24"}),
				"persistent-keepalive": dbus.MakeVariant(uint32(25)),
			}}),
		},
		"ipv4": {
			"method": dbus.MakeVariant("manual"),
			"address-data": dbus.MakeVariant([]map[string]dbus.Variant{{
				"address": dbus.MakeVariant("10.0.0.2"), "prefix": dbus.MakeVariant(uint32(32)),
			}}),
			"dns-data": dbus.MakeVariant([]string{"10.0.0.1"}),
		},
	}
}

// Peers read back with their endpoint, allowed IPs and keepalive.
func TestWgPeersFrom(t *testing.T) {
	got := wgPeersFrom(wgSettings()["wireguard"])
	want := []wgPeer{{PublicKey: "PEERKEY=", Endpoint: "vpn.example.com:51820", AllowedIPs: []string{"10.0.0.0/24"}, Keepalive: 25}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wgPeersFrom = %+v, want %+v", got, want)
	}
	if got := wgPeersFrom(nil); got == nil || len(got) != 0 {
		t.Fatalf("no peers = %v, want an empty list", got)
	}
}

// An edit renames the tunnel and rewrites the chosen peer.
func TestApplyWgEdit(t *testing.T) {
	s := wgSettings()
	err := applyWgEdit(s, wgEditArgs{Name: " home ", PublicKey: "PEERKEY=", Endpoint: "[2001:db8::1]:443", AllowedIPs: []string{"0.0.0.0/0", " ::/0 "}, Keepalive: 0})
	if err != nil {
		t.Fatal(err)
	}
	if s["connection"]["id"].Value() != "home" {
		t.Fatalf("name = %v", s["connection"]["id"])
	}
	p := wgPeersFrom(s["wireguard"])[0]
	if p.Endpoint != "[2001:db8::1]:443" || strings.Join(p.AllowedIPs, " ") != "0.0.0.0/0 ::/0" || p.Keepalive != 0 {
		t.Fatalf("peer = %+v", p)
	}
	s = wgSettings()
	if err := applyWgEdit(s, wgEditArgs{Name: "w", PublicKey: "PEERKEY=", AllowedIPs: []string{"10.0.0.0/24"}}); err != nil {
		t.Fatal(err)
	}
	if p := wgPeersFrom(s["wireguard"])[0]; p.Endpoint != "" {
		t.Fatalf("an empty endpoint left %q", p.Endpoint)
	}
}

// Bad edits are refused with a message the Hub can show.
func TestApplyWgEditRefuses(t *testing.T) {
	base := wgEditArgs{Name: "w", PublicKey: "PEERKEY=", Endpoint: "vpn.example.com:51820", AllowedIPs: []string{"10.0.0.0/24"}}
	for _, c := range []struct {
		edit func(*wgEditArgs)
		want string
	}{
		{func(a *wgEditArgs) { a.Name = " " }, "needs a name"},
		{func(a *wgEditArgs) { a.Endpoint = "vpn.example.com" }, "host:port"},
		{func(a *wgEditArgs) { a.Endpoint = "bad host:51820" }, "host:port"},
		{func(a *wgEditArgs) { a.Endpoint = "vpn.example.com:70000" }, "host:port"},
		{func(a *wgEditArgs) { a.AllowedIPs = []string{"10.0.0.0"} }, "invalid allowed IP"},
		{func(a *wgEditArgs) { a.AllowedIPs = []string{" "} }, "at least one"},
		{func(a *wgEditArgs) { a.PublicKey = "OTHER=" }, "no peer"},
	} {
		a := base
		c.edit(&a)
		if err := applyWgEdit(wgSettings(), a); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: err = %v, want %q", a, err, c.want)
		}
	}
}

// The exported .conf carries every setting nmcli's importer reads back.
func TestWgConfText(t *testing.T) {
	secrets := map[string]dbus.Variant{
		"private-key": dbus.MakeVariant("PRIVKEY="),
		"peers": dbus.MakeVariant([]map[string]dbus.Variant{{
			"public-key": dbus.MakeVariant("PEERKEY="), "preshared-key": dbus.MakeVariant("PSK="),
		}}),
	}
	got, err := wgConfText(wgSettings(), secrets)
	if err != nil {
		t.Fatal(err)
	}
	want := `[Interface]
PrivateKey = PRIVKEY=
Address = 10.0.0.2/32
DNS = 10.0.0.1
ListenPort = 51820

[Peer]
PublicKey = PEERKEY=
PresharedKey = PSK=
AllowedIPs = 10.0.0.0/24
Endpoint = vpn.example.com:51820
PersistentKeepalive = 25
`
	if got != want {
		t.Fatalf("wgConfText =\n%s\nwant\n%s", got, want)
	}
	if _, err := wgConfText(wgSettings(), map[string]dbus.Variant{}); err == nil {
		t.Fatal("a tunnel without a private key was exported")
	}
}
