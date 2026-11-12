package main

// network_ip.go: a saved connection's IPv4 settings for the Hub's Connections
// page. network.ipGet reads them; network.ipSet applies new ones and, when the
// connection is up, puts the old ones back after ipRevertAfter unless
// network.ipConfirm keeps them first, so a wrong address or gateway that cuts
// the machine off undoes itself; network.ipRevert undoes at once.

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// ipRevertAfter is how long new settings on a live connection wait for
// network.ipConfirm before the old ones come back.
const ipRevertAfter = 20 * time.Second

// ipConfig is the IPv4 part of a profile the Hub edits. Method "auto" takes
// address and gateway from DHCP; "manual" needs Address. DNS, when set,
// replaces the DNS servers DHCP hands out (ignore-auto-dns).
type ipConfig struct {
	Method  string   `json:"method"`
	Address string   `json:"address"` // "192.168.1.50/24"
	Gateway string   `json:"gateway"`
	DNS     []string `json:"dns"`
}

// ipPending is a change on a live connection waiting for confirmation.
type ipPending struct {
	prev  map[string]dbus.Variant // the profile's ipv4 group before the change
	timer *time.Timer
}

type ipEdits struct {
	mu      sync.Mutex
	pending map[string]*ipPending // by connection UUID
}

func (n *networkState) registerIPCalls(d *daemon) {
	d.registerCall("network.ipGet", func(raw json.RawMessage) (any, error) {
		var a struct {
			Uuid string `json:"uuid"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return n.ipGet(a.Uuid)
	})
	d.registerCall("network.ipSet", func(raw json.RawMessage) (any, error) {
		var a struct {
			Uuid string `json:"uuid"`
			ipConfig
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return n.ipSet(a.Uuid, a.ipConfig)
	})
	d.registerCall("network.ipConfirm", func(raw json.RawMessage) (any, error) {
		var a struct {
			Uuid string `json:"uuid"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, n.ipConfirm(a.Uuid)
	})
	d.registerCall("network.ipRevert", func(raw json.RawMessage) (any, error) {
		var a struct {
			Uuid string `json:"uuid"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, n.ipRevert(a.Uuid)
	})
}

func (n *networkState) connByUuid(uuid string) (dbus.ObjectPath, error) {
	if uuid == "" {
		return "", fmt.Errorf("empty uuid")
	}
	var conn dbus.ObjectPath
	if err := n.obj(nmSettingsPath).Call(nmSettingsIface+".GetConnectionByUuid", 0, uuid).Store(&conn); err != nil {
		return "", fmt.Errorf("no saved connection %s", uuid)
	}
	return conn, nil
}

func (n *networkState) ipGet(uuid string) (ipConfig, error) {
	conn, err := n.connByUuid(uuid)
	if err != nil {
		return ipConfig{}, err
	}
	s, err := n.editableSettings(conn)
	if err != nil {
		return ipConfig{}, err
	}
	return ipv4FromSettings(s["ipv4"]), nil
}

// ipSet validates and saves cfg. On a live connection it reapplies the device
// and arms the revert; on one that is down the change simply waits for the
// next connect. The reply says how many seconds the revert waits (0: none).
func (n *networkState) ipSet(uuid string, cfg ipConfig) (map[string]any, error) {
	conn, err := n.connByUuid(uuid)
	if err != nil {
		return nil, err
	}
	s, err := n.editableSettings(conn)
	if err != nil {
		return nil, err
	}
	prev := copyGroup(s["ipv4"])
	next, err := applyIPv4(s["ipv4"], cfg)
	if err != nil {
		return nil, err
	}
	s["ipv4"] = next
	if err := n.obj(conn).Call(nmSettConnIface+".Update", 0, s).Err; err != nil {
		return nil, err
	}
	dev := n.deviceOfConnection(uuid)
	if dev == "" {
		n.clearPending(uuid)
		return map[string]any{"revertSeconds": 0}, nil
	}
	if err := n.reapply(dev); err != nil {
		n.restoreIPv4(conn, uuid, prev)
		return nil, fmt.Errorf("could not apply the new settings: %v", err)
	}
	n.armRevert(uuid, conn, prev)
	return map[string]any{"revertSeconds": int(ipRevertAfter / time.Second)}, nil
}

func (n *networkState) ipConfirm(uuid string) error {
	if !n.clearPending(uuid) {
		return fmt.Errorf("nothing is waiting to be kept")
	}
	return nil
}

func (n *networkState) ipRevert(uuid string) error {
	n.ip.mu.Lock()
	p := n.ip.pending[uuid]
	n.ip.mu.Unlock()
	if p == nil {
		return fmt.Errorf("nothing is waiting to be undone")
	}
	conn, err := n.connByUuid(uuid)
	if err != nil {
		return err
	}
	return n.restoreIPv4(conn, uuid, p.prev)
}

func (n *networkState) armRevert(uuid string, conn dbus.ObjectPath, prev map[string]dbus.Variant) {
	n.ip.mu.Lock()
	defer n.ip.mu.Unlock()
	if n.ip.pending == nil {
		n.ip.pending = map[string]*ipPending{}
	}
	if old := n.ip.pending[uuid]; old != nil {
		// a second change before confirming keeps the first known-good state
		old.timer.Stop()
		prev = old.prev
	}
	p := &ipPending{prev: prev}
	p.timer = time.AfterFunc(ipRevertAfter, func() { _ = n.restoreIPv4(conn, uuid, prev) })
	n.ip.pending[uuid] = p
}

// clearPending drops a waiting revert; false when there was none.
func (n *networkState) clearPending(uuid string) bool {
	n.ip.mu.Lock()
	defer n.ip.mu.Unlock()
	p := n.ip.pending[uuid]
	if p == nil {
		return false
	}
	p.timer.Stop()
	delete(n.ip.pending, uuid)
	return true
}

// restoreIPv4 writes prev back as the profile's ipv4 group and reapplies it.
func (n *networkState) restoreIPv4(conn dbus.ObjectPath, uuid string, prev map[string]dbus.Variant) error {
	n.clearPending(uuid)
	s, err := n.editableSettings(conn)
	if err != nil {
		return err
	}
	s["ipv4"] = copyGroup(prev)
	if err := n.obj(conn).Call(nmSettConnIface+".Update", 0, s).Err; err != nil {
		return err
	}
	if dev := n.deviceOfConnection(uuid); dev != "" {
		return n.reapply(dev)
	}
	return nil
}

// deviceOfConnection returns the device a connection is active on, or "".
func (n *networkState) deviceOfConnection(uuid string) dbus.ObjectPath {
	active, ok := n.activeConnections()[uuid]
	if !ok {
		return ""
	}
	v, err := n.obj(active).GetProperty(nmActiveIface + ".Devices")
	if err != nil {
		return ""
	}
	devs, _ := v.Value().([]dbus.ObjectPath)
	if len(devs) == 0 {
		return ""
	}
	return devs[0]
}

// reapply hands the device its profile's current settings without a
// disconnect (an empty connection argument means "the saved profile").
func (n *networkState) reapply(dev dbus.ObjectPath) error {
	return n.obj(dev).Call(nmDeviceIface+".Reapply", 0, map[string]map[string]dbus.Variant{}, uint64(0), uint32(0)).Err
}

func copyGroup(g map[string]dbus.Variant) map[string]dbus.Variant {
	out := make(map[string]dbus.Variant, len(g))
	for k, v := range g {
		out[k] = v
	}
	return out
}

// ipv4FromSettings reads the editable IPv4 fields of a profile's ipv4 group.
// Pure, so it is unit-tested without a bus.
func ipv4FromSettings(g map[string]dbus.Variant) ipConfig {
	cfg := ipConfig{Method: "auto", DNS: []string{}}
	if g == nil {
		return cfg
	}
	if m, ok := g["method"].Value().(string); ok && m != "" {
		cfg.Method = m
	}
	if data, ok := g["address-data"].Value().([]map[string]dbus.Variant); ok && len(data) > 0 {
		addr, _ := data[0]["address"].Value().(string)
		prefix, _ := data[0]["prefix"].Value().(uint32)
		if addr != "" {
			cfg.Address = fmt.Sprintf("%s/%d", addr, prefix)
		}
	}
	cfg.Gateway, _ = g["gateway"].Value().(string)
	if dns, ok := g["dns-data"].Value().([]string); ok {
		cfg.DNS = append(cfg.DNS, dns...)
	} else if dns, ok := g["dns"].Value().([]uint32); ok {
		for _, raw := range dns {
			var b [4]byte
			binary.LittleEndian.PutUint32(b[:], raw)
			cfg.DNS = append(cfg.DNS, netip.AddrFrom4(b).String())
		}
	}
	return cfg
}

// applyIPv4 returns g with cfg applied, after validating cfg. DNS is written
// both as dns-data and as the older packed dns array, so every NetworkManager
// release reads the same servers. Pure, so it is unit-tested without a bus.
func applyIPv4(g map[string]dbus.Variant, cfg ipConfig) (map[string]dbus.Variant, error) {
	out := copyGroup(g)
	for _, k := range []string{"addresses", "routes", "address-data", "gateway", "dns", "dns-data", "ignore-auto-dns"} {
		delete(out, k)
	}
	switch cfg.Method {
	case "auto":
		out["method"] = dbus.MakeVariant("auto")
	case "manual":
		prefix, err := netip.ParsePrefix(strings.TrimSpace(cfg.Address))
		if err != nil || !prefix.Addr().Is4() || prefix.Bits() < 1 || prefix.Bits() > 32 {
			return nil, fmt.Errorf("enter the address as 192.168.1.50/24")
		}
		out["method"] = dbus.MakeVariant("manual")
		out["address-data"] = dbus.MakeVariant([]map[string]dbus.Variant{{
			"address": dbus.MakeVariant(prefix.Addr().String()),
			"prefix":  dbus.MakeVariant(uint32(prefix.Bits())),
		}})
		if gw := strings.TrimSpace(cfg.Gateway); gw != "" {
			addr, err := netip.ParseAddr(gw)
			if err != nil || !addr.Is4() {
				return nil, fmt.Errorf("invalid gateway %q", gw)
			}
			if !prefix.Masked().Contains(addr) {
				return nil, fmt.Errorf("the gateway %s is outside %s", addr, prefix.Masked())
			}
			out["gateway"] = dbus.MakeVariant(addr.String())
		}
	default:
		return nil, fmt.Errorf("unknown IP method %q", cfg.Method)
	}
	var data []string
	var packed []uint32
	for _, raw := range cfg.DNS {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		addr, err := netip.ParseAddr(raw)
		if err != nil || !addr.Is4() {
			return nil, fmt.Errorf("invalid IPv4 DNS server %q", raw)
		}
		b := addr.As4()
		data = append(data, addr.String())
		packed = append(packed, binary.LittleEndian.Uint32(b[:]))
	}
	if len(data) > 0 {
		out["dns-data"] = dbus.MakeVariant(data)
		out["dns"] = dbus.MakeVariant(packed)
		out["ignore-auto-dns"] = dbus.MakeVariant(true)
	}
	return out, nil
}
