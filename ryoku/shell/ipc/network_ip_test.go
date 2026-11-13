package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

// A manual profile reads back as the address with its prefix, the gateway and
// the DNS servers; dns-data wins over the packed array when both are present.
func TestIPv4FromSettingsManual(t *testing.T) {
	g := map[string]dbus.Variant{
		"method": dbus.MakeVariant("manual"),
		"address-data": dbus.MakeVariant([]map[string]dbus.Variant{{
			"address": dbus.MakeVariant("10.250.0.5"),
			"prefix":  dbus.MakeVariant(uint32(24)),
		}}),
		"gateway":  dbus.MakeVariant("10.250.0.1"),
		"dns-data": dbus.MakeVariant([]string{"9.9.9.9"}),
		"dns":      dbus.MakeVariant([]uint32{151587081}),
	}
	want := ipConfig{Method: "manual", Address: "10.250.0.5/24", Gateway: "10.250.0.1", DNS: []string{"9.9.9.9"}}
	if got := ipv4FromSettings(g); !reflect.DeepEqual(got, want) {
		t.Fatalf("ipv4FromSettings = %+v, want %+v", got, want)
	}
}

// A DHCP profile, or no ipv4 group at all, reads as "auto" with nothing set;
// an older profile with only the packed array still yields its servers.
func TestIPv4FromSettingsAutoAndLegacyDNS(t *testing.T) {
	if got := ipv4FromSettings(nil); got.Method != "auto" || got.Address != "" || len(got.DNS) != 0 {
		t.Fatalf("no ipv4 group = %+v", got)
	}
	g := map[string]dbus.Variant{
		"method": dbus.MakeVariant("auto"),
		"dns":    dbus.MakeVariant([]uint32{151587081, 16843009}),
	}
	got := ipv4FromSettings(g)
	if got.Method != "auto" || strings.Join(got.DNS, " ") != "9.9.9.9 1.1.1.1" {
		t.Fatalf("legacy dns = %+v", got)
	}
}

// Manual settings land as address-data, gateway, dns-data, the packed array
// and ignore-auto-dns, and the deprecated mirrors are dropped.
func TestApplyIPv4Manual(t *testing.T) {
	g := map[string]dbus.Variant{
		"method":    dbus.MakeVariant("auto"),
		"addresses": dbus.MakeVariant([][]uint32{{1, 24, 2}}),
		"routes":    dbus.MakeVariant([][]uint32{}),
		"may-fail":  dbus.MakeVariant(true),
	}
	out, err := applyIPv4(g, ipConfig{Method: "manual", Address: "192.168.1.50/24", Gateway: "192.168.1.1", DNS: []string{"9.9.9.9", " 1.1.1.1 "}})
	if err != nil {
		t.Fatal(err)
	}
	if out["method"].Value() != "manual" || out["gateway"].Value() != "192.168.1.1" {
		t.Fatalf("method/gateway = %v %v", out["method"], out["gateway"])
	}
	data := out["address-data"].Value().([]map[string]dbus.Variant)
	if data[0]["address"].Value() != "192.168.1.50" || data[0]["prefix"].Value() != uint32(24) {
		t.Fatalf("address-data = %v", data)
	}
	if !reflect.DeepEqual(out["dns-data"].Value(), []string{"9.9.9.9", "1.1.1.1"}) ||
		!reflect.DeepEqual(out["dns"].Value(), []uint32{151587081, 16843009}) ||
		out["ignore-auto-dns"].Value() != true {
		t.Fatalf("dns = %v %v %v", out["dns-data"], out["dns"], out["ignore-auto-dns"])
	}
	if _, ok := out["addresses"]; ok {
		t.Error("the deprecated addresses mirror was kept")
	}
	if out["may-fail"].Value() != true {
		t.Error("an unrelated ipv4 key was dropped")
	}
	if _, ok := g["address-data"]; ok {
		t.Error("the input group was modified")
	}
}

// Back to DHCP clears the address, gateway and DNS a manual profile had.
func TestApplyIPv4BackToAuto(t *testing.T) {
	manual, err := applyIPv4(nil, ipConfig{Method: "manual", Address: "10.0.0.2/8", Gateway: "10.0.0.1", DNS: []string{"9.9.9.9"}})
	if err != nil {
		t.Fatal(err)
	}
	out, err := applyIPv4(manual, ipConfig{Method: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"address-data", "gateway", "dns", "dns-data", "ignore-auto-dns"} {
		if _, ok := out[k]; ok {
			t.Errorf("%s survived the switch back to DHCP", k)
		}
	}
	if out["method"].Value() != "auto" {
		t.Fatalf("method = %v", out["method"])
	}
}

// Bad input is refused with a message the Hub can show, before anything is written.
func TestApplyIPv4Refuses(t *testing.T) {
	for _, c := range []struct {
		cfg  ipConfig
		want string
	}{
		{ipConfig{Method: "manual", Address: "192.168.1.50"}, "192.168.1.50/24"},
		{ipConfig{Method: "manual", Address: "fe80::1/64"}, "192.168.1.50/24"},
		{ipConfig{Method: "manual", Address: "192.168.1.50/24", Gateway: "10.0.0.1"}, "outside"},
		{ipConfig{Method: "manual", Address: "192.168.1.50/24", Gateway: "nope"}, "invalid gateway"},
		{ipConfig{Method: "auto", DNS: []string{"2620:fe::fe"}}, "IPv4 DNS"},
		{ipConfig{Method: "link-local"}, "unknown IP method"},
	} {
		if _, err := applyIPv4(nil, c.cfg); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: err = %v, want it to mention %q", c.cfg, err, c.want)
		}
	}
}
