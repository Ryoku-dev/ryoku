package main

import (
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

// Each adapter state reads as what the Hub has to say about it; an adapter
// NetworkManager does not manage is "unmanaged" whatever its state.
func TestWiredState(t *testing.T) {
	for _, c := range []struct {
		state   uint32
		managed bool
		want    string
	}{
		{100, true, "connected"},
		{70, true, "connecting"},
		{30, true, "disconnected"},
		{120, true, "disconnected"},
		{20, true, "unplugged"},
		{10, false, "unmanaged"},
		{30, false, "unmanaged"},
	} {
		if got := wiredState(c.state, c.managed); got != c.want {
			t.Errorf("wiredState(%d, %v) = %q, want %q", c.state, c.managed, got, c.want)
		}
	}
}

// A connected adapter shows its speed, address and profile; one that is down
// shows no speed, since NetworkManager keeps the last negotiated one.
func TestEthernetFrames(t *testing.T) {
	got := ethernetFrames([]wiredDevice{
		{iface: "enp1s0", mac: "AA:BB", state: 100, managed: true, speed: 1000, activeUuid: "u1", activeName: "Wired", ip4: "192.168.1.20"},
		{iface: "enp2s0", state: 20, managed: true, speed: 100},
	})
	if len(got) != 2 {
		t.Fatalf("frames = %v", got)
	}
	up := got[0]
	if up["device"] != "enp1s0" || up["state"] != "connected" || up["speed"] != uint32(1000) ||
		up["ip4"] != "192.168.1.20" || up["activeUuid"] != "u1" || up["activeName"] != "Wired" || up["mac"] != "AA:BB" {
		t.Fatalf("connected frame = %v", up)
	}
	if got[1]["state"] != "unplugged" || got[1]["speed"] != uint32(0) {
		t.Fatalf("unplugged frame = %v", got[1])
	}
	if frames := ethernetFrames(nil); frames == nil || len(frames) != 0 {
		t.Fatalf("no adapters = %v, want an empty list", frames)
	}
}

// Only Ethernet profiles are listed, with their bound adapter and whether
// they are up.
func TestEthernetProfileFrames(t *testing.T) {
	saved := []savedConn{
		{uuid: "e1", id: "Wired", typ: "802-3-ethernet", autoconnect: true},
		{uuid: "e2", id: "Lab", typ: "802-3-ethernet", iface: "enp2s0"},
		{uuid: "w1", id: "Home", typ: "802-11-wireless", ssid: "Home"},
	}
	got := ethernetProfileFrames(saved, map[string]dbus.ObjectPath{"e1": "/a/1"})
	if len(got) != 2 {
		t.Fatalf("profiles = %v", got)
	}
	if got[0]["uuid"] != "e1" || got[0]["active"] != true || got[0]["autoconnect"] != true || got[0]["device"] != "" {
		t.Fatalf("first profile = %v", got[0])
	}
	if got[1]["uuid"] != "e2" || got[1]["active"] != false || got[1]["device"] != "enp2s0" {
		t.Fatalf("second profile = %v", got[1])
	}
}

// A wired control only accepts an Ethernet profile.
func TestEthernetProfileRefusesOtherTypes(t *testing.T) {
	saved := []savedConn{
		{uuid: "e1", typ: "802-3-ethernet"},
		{uuid: "w1", typ: "802-11-wireless"},
		{uuid: "v1", typ: "wireguard"},
	}
	if _, err := ethernetProfile(saved, "e1"); err != nil {
		t.Fatalf("ethernet profile refused: %v", err)
	}
	for uuid, want := range map[string]string{"w1": "not an Ethernet", "v1": "not an Ethernet", "x": "no saved", "": "empty"} {
		if _, err := ethernetProfile(saved, uuid); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ethernetProfile(%q) err = %v, want %q", uuid, err, want)
		}
	}
}
