package main

// network_vpn.go: a saved WireGuard tunnel's or VPN profile's settings for the
// Hub's Connections VPN tab. network.vpnGet reads them (and, for a tunnel that
// is up, its traffic); network.vpnSetAutoconnect flips auto-connect;
// network.wgEdit changes a tunnel's name and peer; network.wgExport writes the
// tunnel out as a .conf the user can open, and network.wgReplace loads an
// edited .conf back in place of the tunnel. Its IP address and DNS go through
// the same network.ip* calls a Wi-Fi network's do.

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/godbus/dbus/v5"
)

// vpnDataShown are the vpn.data keys a VPN profile's details show; the rest
// are plugin internals, and secrets never live in vpn.data.
var vpnDataShown = []string{"remote", "gateway", "port", "proto", "username", "user"}

// wgPeer is one WireGuard peer as the Hub shows and edits it.
type wgPeer struct {
	PublicKey  string   `json:"publicKey"`
	Endpoint   string   `json:"endpoint"`
	AllowedIPs []string `json:"allowedIps"`
	Keepalive  uint32   `json:"keepalive"`
}

// wgEditArgs is a tunnel edit: the name, and the peer picked by its public key.
type wgEditArgs struct {
	Uuid       string   `json:"uuid"`
	Name       string   `json:"name"`
	PublicKey  string   `json:"publicKey"`
	Endpoint   string   `json:"endpoint"`
	AllowedIPs []string `json:"allowedIps"`
	Keepalive  uint32   `json:"keepalive"`
}

func (n *networkState) registerVpnCalls(d *daemon) {
	d.registerCall("network.vpnGet", func(raw json.RawMessage) (any, error) {
		var a struct {
			Uuid string `json:"uuid"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return n.vpnGet(a.Uuid)
	})
	d.registerCall("network.vpnSetAutoconnect", func(raw json.RawMessage) (any, error) {
		var a struct {
			Uuid        string `json:"uuid"`
			Autoconnect bool   `json:"autoconnect"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		c, err := vpnProfile(n.savedConnections(), a.Uuid)
		if err != nil {
			return nil, err
		}
		return nil, n.updateAutoconnect(c.path, a.Autoconnect)
	})
	d.registerCall("network.wgEdit", func(raw json.RawMessage) (any, error) {
		var a wgEditArgs
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return n.wgEdit(a)
	})
	d.registerCall("network.wgExport", func(raw json.RawMessage) (any, error) {
		var a struct {
			Uuid string `json:"uuid"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		path, err := n.wgExport(a.Uuid)
		if err != nil {
			return nil, err
		}
		return map[string]any{"path": path}, nil
	})
	d.registerCall("network.wgReplace", func(raw json.RawMessage) (any, error) {
		var a struct {
			Uuid string `json:"uuid"`
			Path string `json:"path"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		uuid, err := n.wgReplace(a.Uuid, a.Path)
		if err != nil {
			return nil, err
		}
		return map[string]any{"uuid": uuid}, nil
	})
}

// vpnProfile finds a saved WireGuard or VPN profile by UUID, refusing every
// other type so a VPN control can never change Wi-Fi or Ethernet.
func vpnProfile(saved []savedConn, uuid string) (savedConn, error) {
	if uuid == "" {
		return savedConn{}, fmt.Errorf("empty uuid")
	}
	for _, c := range saved {
		if c.uuid == uuid {
			if c.typ != "wireguard" && c.typ != "vpn" {
				return savedConn{}, fmt.Errorf("not a VPN profile: %s", uuid)
			}
			return c, nil
		}
	}
	return savedConn{}, fmt.Errorf("no saved connection %s", uuid)
}

func (n *networkState) vpnGet(uuid string) (map[string]any, error) {
	c, err := vpnProfile(n.savedConnections(), uuid)
	if err != nil {
		return nil, err
	}
	s, err := n.editableSettings(c.path)
	if err != nil {
		return nil, err
	}
	_, active := n.activeConnections()[uuid]
	out := map[string]any{
		"uuid":        uuid,
		"name":        c.id,
		"type":        c.typ,
		"autoconnect": c.autoconnect,
		"active":      active,
		"ip":          ipv4FromSettings(s["ipv4"]),
	}
	if c.typ == "vpn" {
		out["service"] = vpnServiceName(c.vpnService)
		out["data"] = vpnShownData(s["vpn"])
		return out, nil
	}
	iface, _ := s["connection"]["interface-name"].Value().(string)
	out["interface"] = iface
	out["listenPort"] = uintOf(s["wireguard"]["listen-port"])
	out["peers"] = wgPeersFrom(s["wireguard"])
	if active && iface != "" {
		out["rxBytes"] = readCounter(iface, "rx_bytes")
		out["txBytes"] = readCounter(iface, "tx_bytes")
	}
	return out, nil
}

// readCounter reads one of an interface's sysfs traffic counters; 0 when it
// is missing.
func readCounter(iface, name string) uint64 {
	raw, err := os.ReadFile(filepath.Join("/sys/class/net", iface, "statistics", name))
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	return v
}

func uintOf(v dbus.Variant) uint32 {
	u, _ := v.Value().(uint32)
	return u
}

// vpnShownData keeps the vpn.data entries worth showing. Pure.
func vpnShownData(g map[string]dbus.Variant) map[string]string {
	out := map[string]string{}
	data, _ := g["data"].Value().(map[string]string)
	for _, k := range vpnDataShown {
		if v := data[k]; v != "" {
			out[k] = v
		}
	}
	return out
}

// wgPeersFrom reads a wireguard group's peers. Pure.
func wgPeersFrom(g map[string]dbus.Variant) []wgPeer {
	out := []wgPeer{}
	peers, _ := g["peers"].Value().([]map[string]dbus.Variant)
	for _, p := range peers {
		peer := wgPeer{AllowedIPs: []string{}}
		peer.PublicKey, _ = p["public-key"].Value().(string)
		peer.Endpoint, _ = p["endpoint"].Value().(string)
		if ips, ok := p["allowed-ips"].Value().([]string); ok {
			peer.AllowedIPs = append(peer.AllowedIPs, ips...)
		}
		peer.Keepalive = uintOf(p["persistent-keepalive"])
		out = append(out, peer)
	}
	return out
}

// applyWgEdit returns the settings with a.Name as the connection name and the
// peer whose key is a.PublicKey given a.Endpoint, a.AllowedIPs and
// a.Keepalive, after validating them. Peer secrets are not in GetSettings;
// NetworkManager keeps the stored ones across Update. Pure.
func applyWgEdit(s map[string]map[string]dbus.Variant, a wgEditArgs) error {
	name := strings.TrimSpace(a.Name)
	if name == "" {
		return fmt.Errorf("the tunnel needs a name")
	}
	endpoint := strings.TrimSpace(a.Endpoint)
	if endpoint != "" {
		if _, err := netip.ParseAddrPort(endpoint); err != nil && !validHostPort(endpoint) {
			return fmt.Errorf("enter the endpoint as host:port, like vpn.example.com:51820")
		}
	}
	allowed := []string{}
	for _, raw := range a.AllowedIPs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		p, err := netip.ParsePrefix(raw)
		if err != nil {
			return fmt.Errorf("invalid allowed IP %q (use 10.0.0.0/24 or 0.0.0.0/0)", raw)
		}
		allowed = append(allowed, p.String())
	}
	if len(allowed) == 0 {
		return fmt.Errorf("the peer needs at least one allowed IP")
	}
	if s["wireguard"] == nil {
		return fmt.Errorf("not a WireGuard tunnel")
	}
	peers, _ := s["wireguard"]["peers"].Value().([]map[string]dbus.Variant)
	found := false
	for _, p := range peers {
		if key, _ := p["public-key"].Value().(string); key == a.PublicKey {
			if endpoint == "" {
				delete(p, "endpoint")
			} else {
				p["endpoint"] = dbus.MakeVariant(endpoint)
			}
			p["allowed-ips"] = dbus.MakeVariant(allowed)
			p["persistent-keepalive"] = dbus.MakeVariant(a.Keepalive)
			found = true
		}
	}
	if !found {
		return fmt.Errorf("no peer with that public key")
	}
	s["wireguard"]["peers"] = dbus.MakeVariant(peers)
	s["connection"]["id"] = dbus.MakeVariant(name)
	return nil
}

var hostLabel = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

// validHostPort accepts a DNS name with a port, as WireGuard endpoints may be.
func validHostPort(s string) bool {
	i := strings.LastIndex(s, ":")
	if i <= 0 {
		return false
	}
	port, err := strconv.Atoi(s[i+1:])
	if err != nil || port < 1 || port > 65535 {
		return false
	}
	for _, label := range strings.Split(s[:i], ".") {
		if !hostLabel.MatchString(label) {
			return false
		}
	}
	return true
}

// wgEdit saves a tunnel edit; a tunnel that is up takes it at once when
// NetworkManager can reapply it, else on its next connect (applied false).
func (n *networkState) wgEdit(a wgEditArgs) (map[string]any, error) {
	c, err := vpnProfile(n.savedConnections(), a.Uuid)
	if err != nil {
		return nil, err
	}
	if c.typ != "wireguard" {
		return nil, fmt.Errorf("not a WireGuard tunnel")
	}
	s, err := n.editableSettings(c.path)
	if err != nil {
		return nil, err
	}
	if err := applyWgEdit(s, a); err != nil {
		return nil, err
	}
	if err := n.obj(c.path).Call(nmSettConnIface+".Update", 0, s).Err; err != nil {
		return nil, err
	}
	dev := n.deviceOfConnection(a.Uuid)
	if dev == "" {
		return map[string]any{"applied": true}, nil
	}
	return map[string]any{"applied": n.reapply(dev) == nil}, nil
}

// wgSecrets reads a tunnel's private key and its peers' preshared keys.
func (n *networkState) wgSecrets(path dbus.ObjectPath) (map[string]dbus.Variant, error) {
	var s map[string]map[string]dbus.Variant
	if err := n.obj(path).Call(nmSettConnIface+".GetSecrets", 0, "wireguard").Store(&s); err != nil {
		return nil, fmt.Errorf("could not read the tunnel's keys: %v", err)
	}
	return s["wireguard"], nil
}

// wgExportDir holds exported tunnels: under the runtime dir, which only the
// user can read and which is cleared at logout, since a .conf carries the
// private key.
func wgExportDir() string {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "ryoku-wireguard")
}

// wgExport writes the tunnel as a .conf (mode 0600) and returns its path.
func (n *networkState) wgExport(uuid string) (string, error) {
	c, err := vpnProfile(n.savedConnections(), uuid)
	if err != nil {
		return "", err
	}
	if c.typ != "wireguard" {
		return "", fmt.Errorf("not a WireGuard tunnel")
	}
	s, err := n.editableSettings(c.path)
	if err != nil {
		return "", err
	}
	secrets, err := n.wgSecrets(c.path)
	if err != nil {
		return "", err
	}
	text, err := wgConfText(s, secrets)
	if err != nil {
		return "", err
	}
	iface, _ := s["connection"]["interface-name"].Value().(string)
	if iface == "" {
		iface = "wg0"
	}
	dir := wgExportDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, iface+".conf")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return "", err
	}
	return path, os.Chmod(path, 0o600)
}

// wgConfText renders a tunnel's settings and secrets as a wg-quick .conf.
// Pure, so it is unit-tested without a bus.
func wgConfText(s map[string]map[string]dbus.Variant, secrets map[string]dbus.Variant) (string, error) {
	key, _ := secrets["private-key"].Value().(string)
	if key == "" {
		return "", fmt.Errorf("the tunnel has no private key stored")
	}
	var b strings.Builder
	b.WriteString("[Interface]\nPrivateKey = " + key + "\n")
	var addrs, dns []string
	for _, family := range []string{"ipv4", "ipv6"} {
		if data, ok := s[family]["address-data"].Value().([]map[string]dbus.Variant); ok {
			for _, a := range data {
				addr, _ := a["address"].Value().(string)
				prefix := uintOf(a["prefix"])
				if addr != "" {
					addrs = append(addrs, fmt.Sprintf("%s/%d", addr, prefix))
				}
			}
		}
		if family == "ipv4" {
			dns = append(dns, ipv4FromSettings(s["ipv4"]).DNS...)
		} else if d, ok := s["ipv6"]["dns"].Value().([][]byte); ok {
			for _, raw := range d {
				if a, ok := netip.AddrFromSlice(raw); ok {
					dns = append(dns, a.String())
				}
			}
		}
	}
	if len(addrs) > 0 {
		b.WriteString("Address = " + strings.Join(addrs, ", ") + "\n")
	}
	if len(dns) > 0 {
		b.WriteString("DNS = " + strings.Join(dns, ", ") + "\n")
	}
	if port := uintOf(s["wireguard"]["listen-port"]); port != 0 {
		fmt.Fprintf(&b, "ListenPort = %d\n", port)
	}
	if mtu := uintOf(s["wireguard"]["mtu"]); mtu != 0 {
		fmt.Fprintf(&b, "MTU = %d\n", mtu)
	}
	psks := map[string]string{}
	if peers, ok := secrets["peers"].Value().([]map[string]dbus.Variant); ok {
		for _, p := range peers {
			pub, _ := p["public-key"].Value().(string)
			psk, _ := p["preshared-key"].Value().(string)
			if pub != "" && psk != "" {
				psks[pub] = psk
			}
		}
	}
	for _, p := range wgPeersFrom(s["wireguard"]) {
		b.WriteString("\n[Peer]\nPublicKey = " + p.PublicKey + "\n")
		if psk := psks[p.PublicKey]; psk != "" {
			b.WriteString("PresharedKey = " + psk + "\n")
		}
		if len(p.AllowedIPs) > 0 {
			b.WriteString("AllowedIPs = " + strings.Join(p.AllowedIPs, ", ") + "\n")
		}
		if p.Endpoint != "" {
			b.WriteString("Endpoint = " + p.Endpoint + "\n")
		}
		if p.Keepalive != 0 {
			fmt.Fprintf(&b, "PersistentKeepalive = %d\n", p.Keepalive)
		}
	}
	return b.String(), nil
}

var importedUuid = regexp.MustCompile(`\(([0-9a-f-]{36})\)`)

// wgReplace loads an edited .conf in place of a tunnel and returns the new
// UUID. A tunnel that is up goes down first, since nmcli brings an imported
// one up at once on the same interface; a file nmcli refuses then brings the
// old tunnel back as it was. The new profile takes the old one's name and
// auto-connect, and stays up only when the old one was. A file loaded from
// the export folder is removed then, since it holds the private key.
func (n *networkState) wgReplace(uuid, path string) (string, error) {
	old, err := vpnProfile(n.savedConnections(), uuid)
	if err != nil {
		return "", err
	}
	if old.typ != "wireguard" {
		return "", fmt.Errorf("not a WireGuard tunnel")
	}
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	ac, wasActive := n.activeConnections()[uuid]
	if wasActive {
		if err := n.obj(nmPath).Call(nmIface+".DeactivateConnection", 0, ac).Err; err != nil {
			return "", err
		}
	}
	restore := func() {
		if wasActive {
			var active dbus.ObjectPath
			_ = n.obj(nmPath).Call(nmIface+".ActivateConnection", 0, old.path, dbus.ObjectPath("/"), dbus.ObjectPath("/")).Store(&active)
		}
	}
	out, err := exec.Command("nmcli", "connection", "import", "type", "wireguard", "file", path).CombinedOutput()
	if err != nil {
		restore()
		return "", fmt.Errorf("the file was not loaded: %s", strings.TrimSpace(string(out)))
	}
	m := importedUuid.FindStringSubmatch(string(out))
	if m == nil {
		restore()
		return "", fmt.Errorf("the file was loaded but its new profile was not found: %s", strings.TrimSpace(string(out)))
	}
	fresh, err := n.connByUuid(m[1])
	if err != nil {
		return "", err
	}
	if ac, up := n.activeConnections()[m[1]]; up && !wasActive {
		_ = n.obj(nmPath).Call(nmIface+".DeactivateConnection", 0, ac).Err
	}
	s, err := n.editableSettings(fresh)
	if err != nil {
		return "", err
	}
	s["connection"]["id"] = dbus.MakeVariant(old.id)
	s["connection"]["autoconnect"] = dbus.MakeVariant(old.autoconnect)
	if err := n.obj(fresh).Call(nmSettConnIface+".Update", 0, s).Err; err != nil {
		return "", err
	}
	if err := n.obj(old.path).Call(nmSettConnIface+".Delete", 0).Err; err != nil {
		return "", err
	}
	if filepath.Dir(filepath.Clean(path)) == wgExportDir() {
		_ = os.Remove(path)
	}
	return m[1], nil
}
