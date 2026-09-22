package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

// network.go owns the NetworkManager integration: it reads Wi-Fi radio state,
// the active connection, scanned access points, saved WireGuard tunnels, and
// global DNS configuration over the system bus. It streams them to QML on the
// "network" state topic and drives Wi-Fi, WireGuard, and DNS changes on request.
// The bus name, object paths, interfaces, and signals are from the
// NetworkManager specification.
// The reveal-side derivations live in QML per contract 06 sec 9: the widget
// filters the currently-connected SSID out of the available list and orders by
// service order (the order NetworkManager returns), so the topic hands over the
// raw access-point list plus the active SSID rather than a pre-filtered one.
const (
	nmBusName       = "org.freedesktop.NetworkManager"
	nmPath          = "/org/freedesktop/NetworkManager"
	nmIface         = "org.freedesktop.NetworkManager"
	nmSettingsPath  = "/org/freedesktop/NetworkManager/Settings"
	nmSettingsIface = "org.freedesktop.NetworkManager.Settings"
	nmDeviceIface   = "org.freedesktop.NetworkManager.Device"
	nmWirelessIface = "org.freedesktop.NetworkManager.Device.Wireless"
	nmApIface       = "org.freedesktop.NetworkManager.AccessPoint"
	nmActiveIface   = "org.freedesktop.NetworkManager.Connection.Active"
	nmSettConnIface = "org.freedesktop.NetworkManager.Settings.Connection"
	nmPropsIface    = "org.freedesktop.DBus.Properties"
)

// NetworkManager device-type enum values this slice reads (NMDeviceType).
const (
	nmDeviceEthernet = 1
	nmDeviceWifi     = 2
)

// networkState holds the one system-bus connection and the topic the network
// frames publish to.
type networkState struct {
	conn  *dbus.Conn
	topic *stateTopic
}

// apInfo is one access point flattened to the fields the reveal renders.
type apInfo struct {
	Ssid        string
	Strength    int
	Security    string
	Bssid       string
	Frequency   int
	Saved       bool
	Active      bool
	Autoconnect bool
}

// savedConn is one stored NetworkManager profile, used to mark an AP "saved"
// (so the reveal knows whether to show a password field) and to enumerate the
// WireGuard tunnels (which are saved profiles of type wireguard).
type savedConn struct {
	path        dbus.ObjectPath
	uuid        string
	id          string
	typ         string
	ssid        string
	autoconnect bool
}

// startNetwork brings the NetworkManager integration up, registers the topic
// and the control calls, watches every signal under the NetworkManager object
// tree, and publishes the first frame. A missing system bus disables the
// feature without failing the daemon (the QML view stays empty and retries).
func (d *daemon) startNetwork() {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Printf("ryoku-shell: network disabled: %v", err)
		return
	}
	n := &networkState{conn: conn, topic: d.registerTopic("network")}

	// One namespace match covers the whole NetworkManager object tree: device
	// state, access-point add/remove, active-connection and settings changes
	// all live under nmPath, and nothing else does, so a single rule feeds the
	// coalescing republisher without a firehose of unrelated system signals.
	if err := conn.AddMatchSignal(dbus.WithMatchPathNamespace(dbus.ObjectPath(nmPath))); err != nil {
		log.Printf("ryoku-shell: network signal match failed: %v", err)
	}
	sigs := make(chan *dbus.Signal, 64)
	conn.Signal(sigs)
	go n.republishOnSignal(sigs)

	d.registerCall("network.wifiSetEnabled", func(raw json.RawMessage) (any, error) {
		var a struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, n.setWirelessEnabled(a.Enabled)
	})
	d.registerCall("network.wifiScan", func(json.RawMessage) (any, error) {
		return nil, n.wifiScan()
	})
	d.registerCall("network.wifiConnect", func(raw json.RawMessage) (any, error) {
		var a struct {
			Ssid     string `json:"ssid"`
			Password string `json:"password"`
			Bssid    string `json:"bssid"`
			Hidden   bool   `json:"hidden"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, n.wifiConnect(a.Ssid, a.Password, a.Bssid, a.Hidden)
	})
	d.registerCall("network.wifiDisconnect", func(json.RawMessage) (any, error) {
		return nil, n.wifiDisconnect()
	})
	d.registerCall("network.wifiForget", func(raw json.RawMessage) (any, error) {
		var a struct {
			Ssid string `json:"ssid"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, n.wifiForget(a.Ssid)
	})
	d.registerCall("network.wifiSetAutoconnect", func(raw json.RawMessage) (any, error) {
		var a struct {
			Ssid        string `json:"ssid"`
			Autoconnect bool   `json:"autoconnect"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, n.wifiSetAutoconnect(a.Ssid, a.Autoconnect)
	})
	d.registerCall("network.dnsSet", func(raw json.RawMessage) (any, error) {
		var a struct {
			Provider string   `json:"provider"`
			Servers  []string `json:"servers"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		if err := n.setDns(a.Provider, a.Servers); err != nil {
			return nil, err
		}
		n.publish()
		return nil, nil
	})
	d.registerCall("network.wgActivate", func(raw json.RawMessage) (any, error) {
		var a struct {
			Uuid string `json:"uuid"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, n.wgSetActive(a.Uuid, true)
	})
	d.registerCall("network.wgDeactivate", func(raw json.RawMessage) (any, error) {
		var a struct {
			Uuid string `json:"uuid"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, n.wgSetActive(a.Uuid, false)
	})
	// wgImport mirrors the reference exactly: it is the one place the reference
	// shells out (contract 06 sec 4 runs `nmcli connection import type
	// wireguard file <path>`), because parsing a .conf into NetworkManager's
	// nested settings is what nmcli's importer already does. This is an action,
	// not a status read, so it is not the polling defect the topic replaces.
	d.registerCall("network.wgImport", func(raw json.RawMessage) (any, error) {
		var a struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, wgImport(a.Path)
	})
	d.registerCall("network.wgDelete", func(raw json.RawMessage) (any, error) {
		var a struct {
			Uuid string `json:"uuid"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, n.wgDelete(a.Uuid)
	})

	n.publish()
}

// republishOnSignal coalesces bursts of NetworkManager signals into one publish
// after a short settle window, so a scan that adds twenty access points at once
// costs a single frame rather than twenty.
func (n *networkState) republishOnSignal(sigs chan *dbus.Signal) {
	const settle = 150 * time.Millisecond
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	pending := false
	for {
		select {
		case <-sigs:
			if !pending {
				pending = true
				timer.Reset(settle)
			}
		case <-timer.C:
			pending = false
			n.publish()
		}
	}
}

func (n *networkState) obj(path dbus.ObjectPath) dbus.BusObject {
	return n.conn.Object(nmBusName, path)
}

// publish marshals the whole network state and hands it to the topic, which
// drops it if byte-identical to the last frame.
func (n *networkState) publish() {
	if n.topic == nil {
		return
	}
	frame, err := json.Marshal(n.snapshot())
	if err != nil {
		return
	}
	n.topic.publish(frame)
}

// snapshot reads the current NetworkManager state into the QML frame shape.
func (n *networkState) snapshot() map[string]any {
	saved := n.savedConnections()
	active := n.activeConnections()

	wifiDev := n.deviceOfType(nmDeviceWifi)
	wifi := map[string]any{
		"present":      wifiDev != "",
		"enabled":      n.boolProp(n.obj(nmPath), nmIface+".WirelessEnabled"),
		"connectivity": "Disconnected",
		"ssid":         "",
		"strength":     0,
	}
	aps := []map[string]any{}
	if wifiDev != "" {
		wifi["connectivity"] = connectivityFromState(n.uintProp(n.obj(wifiDev), nmDeviceIface+".State"))
		activeAp := n.pathProp(n.obj(wifiDev), nmWirelessIface+".ActiveAccessPoint")
		for _, ap := range n.apPaths(wifiDev) {
			info := n.apInfo(ap)
			if info == nil {
				continue
			}
			info.Active = ap == activeAp && activeAp != "/"
			info.Saved = ssidSaved(info.Ssid, saved)
			info.Autoconnect = ssidAutoconnect(info.Ssid, saved)
			if info.Active {
				wifi["ssid"] = info.Ssid
				wifi["strength"] = info.Strength
			}
			aps = append(aps, apFrame(info))
		}
	}

	wiredDev := n.deviceOfType(nmDeviceEthernet)
	wired := map[string]any{
		"present":      wiredDev != "",
		"connectivity": "Disconnected",
	}
	if wiredDev != "" {
		wired["connectivity"] = connectivityFromState(n.uintProp(n.obj(wiredDev), nmDeviceIface+".State"))
	}

	return map[string]any{
		"wifi":         wifi,
		"wired":        wired,
		"accessPoints": aps,
		"wireguard":    n.wireguardTunnels(saved, active),
		"dns":          n.dnsFrame(),
	}
}

// deviceOfType returns the path of the first device of the given NMDeviceType,
// or "" if none. Service order (GetAllDevices) is preserved; no sort.
func (n *networkState) deviceOfType(devType uint32) dbus.ObjectPath {
	var paths []dbus.ObjectPath
	if n.obj(nmPath).Call(nmIface+".GetAllDevices", 0).Store(&paths) != nil {
		return ""
	}
	for _, p := range paths {
		if n.uintProp(n.obj(p), nmDeviceIface+".DeviceType") == devType {
			return p
		}
	}
	return ""
}

// apPaths lists the access points the wifi device currently sees, in service
// order.
func (n *networkState) apPaths(wifiDev dbus.ObjectPath) []dbus.ObjectPath {
	var paths []dbus.ObjectPath
	if n.obj(wifiDev).Call(nmWirelessIface+".GetAllAccessPoints", 0).Store(&paths) != nil {
		return nil
	}
	return paths
}

// apInfo reads one access point's fields. A missing SSID (a hidden network)
// drops the AP: the reference list keys on SSID.
func (n *networkState) apInfo(ap dbus.ObjectPath) *apInfo {
	o := n.obj(ap)
	ssidV, err := o.GetProperty(nmApIface + ".Ssid")
	if err != nil {
		return nil
	}
	ssidBytes, _ := ssidV.Value().([]byte)
	ssid := decodeSsid(ssidBytes)
	if ssid == "" {
		return nil
	}
	return &apInfo{
		Ssid:      ssid,
		Strength:  int(n.byteProp(o, nmApIface+".Strength")),
		Security:  apSecurity(n.uintProp(o, nmApIface+".Flags"), n.uintProp(o, nmApIface+".WpaFlags"), n.uintProp(o, nmApIface+".RsnFlags")),
		Bssid:     n.stringProp(o, nmApIface+".HwAddress"),
		Frequency: int(n.uintProp(o, nmApIface+".Frequency")),
	}
}

// savedConnections reads every stored profile with the fields needed to mark an
// AP saved and to enumerate WireGuard tunnels.
func (n *networkState) savedConnections() []savedConn {
	var paths []dbus.ObjectPath
	if n.obj(nmSettingsPath).Call(nmSettingsIface+".ListConnections", 0).Store(&paths) != nil {
		return nil
	}
	out := make([]savedConn, 0, len(paths))
	for _, p := range paths {
		var s map[string]map[string]dbus.Variant
		if n.obj(p).Call(nmSettConnIface+".GetSettings", 0).Store(&s) != nil {
			continue
		}
		c := savedConn{path: p, autoconnect: true}
		if conn := s["connection"]; conn != nil {
			c.typ, _ = conn["type"].Value().(string)
			c.id, _ = conn["id"].Value().(string)
			c.uuid, _ = conn["uuid"].Value().(string)
			// NetworkManager omits connection.autoconnect when it is on (its
			// default), so an absent key means enabled; only an explicit false
			// turns it off.
			if v, ok := conn["autoconnect"]; ok {
				if b, ok := v.Value().(bool); ok {
					c.autoconnect = b
				}
			}
		}
		if w := s["802-11-wireless"]; w != nil {
			if b, ok := w["ssid"].Value().([]byte); ok {
				c.ssid = decodeSsid(b)
			}
		}
		out = append(out, c)
	}
	return out
}

// activeConnections maps each active connection's UUID to its object path (used
// to deactivate a WireGuard tunnel by UUID).
func (n *networkState) activeConnections() map[string]dbus.ObjectPath {
	v, err := n.obj(nmPath).GetProperty(nmIface + ".ActiveConnections")
	if err != nil {
		return nil
	}
	paths, _ := v.Value().([]dbus.ObjectPath)
	m := make(map[string]dbus.ObjectPath, len(paths))
	for _, p := range paths {
		uv, err := n.obj(p).GetProperty(nmActiveIface + ".Uuid")
		if err != nil {
			continue
		}
		if u, ok := uv.Value().(string); ok {
			m[u] = p
		}
	}
	return m
}

// wireguardTunnels returns the saved WireGuard profiles in service order, each
// tagged active when its UUID has a live active connection.
func (n *networkState) wireguardTunnels(saved []savedConn, active map[string]dbus.ObjectPath) []map[string]any {
	out := []map[string]any{}
	for _, c := range saved {
		if c.typ != "wireguard" {
			continue
		}
		_, isActive := active[c.uuid]
		out = append(out, map[string]any{
			"uuid":   c.uuid,
			"name":   c.id,
			"active": isActive,
		})
	}
	return out
}

// --- control ---

func (n *networkState) setWirelessEnabled(enabled bool) error {
	return n.obj(nmPath).Call(nmPropsIface+".Set", 0, nmIface, "WirelessEnabled", dbus.MakeVariant(enabled)).Err
}

func (n *networkState) wifiScan() error {
	wifiDev := n.deviceOfType(nmDeviceWifi)
	if wifiDev == "" {
		return fmt.Errorf("no wifi device")
	}
	return n.obj(wifiDev).Call(nmWirelessIface+".RequestScan", 0, map[string]dbus.Variant{}).Err
}

// wifiConnect connects to an SSID. An existing saved profile is (re)activated,
// refreshing its passphrase first when one is supplied; an unknown network gets
// a fresh profile created and activated in one AddAndActivateConnection call,
// keyed to the access point bestApForSsid resolves so NetworkManager binds the
// right BSSID. An empty bssid picks the strongest matching AP (the historical
// behaviour); a non-empty bssid pins that exact AP and locks the profile to its
// band. hidden marks the network as not broadcasting its SSID: no AP resolves,
// so the profile carries 802-11-wireless.hidden and NetworkManager probes for
// it by name. Mirrors the reference lookup-then-add path.
func (n *networkState) wifiConnect(ssid, password, bssid string, hidden bool) error {
	if ssid == "" {
		return fmt.Errorf("empty ssid")
	}
	wifiDev := n.deviceOfType(nmDeviceWifi)
	if wifiDev == "" {
		return fmt.Errorf("no wifi device")
	}
	specific, ap := n.bestApForSsid(wifiDev, ssid, bssid)

	// A pinned AP drives both key management (so WPA3-SAE joins as sae) and the
	// band lock (so autoconnect stops drifting back to 2.4 GHz); both stay unset
	// when no AP resolved.
	apSec := ""
	nmBand := ""
	if ap != nil {
		apSec = ap.Security
		nmBand = nmBandForFrequency(ap.Frequency)
	}

	for _, c := range n.savedConnections() {
		if c.typ == "802-11-wireless" && c.ssid == ssid {
			if password != "" {
				if err := n.updatePassword(c.path, password, keyMgmtForSecurity(apSec)); err != nil {
					return err
				}
			}
			if bssid != "" && nmBand != "" {
				if err := n.updateBand(c.path, nmBand); err != nil {
					return err
				}
			}
			var active dbus.ObjectPath
			if err := n.obj(nmPath).Call(nmIface+".ActivateConnection", 0, c.path, dbus.ObjectPath(wifiDev), specific).Store(&active); err != nil {
				return err
			}
			return n.waitForActivation(active)
		}
	}
	settings := wifiConnectSettings(ssid, password, bssid, ap, hidden)
	var newConn, newActive dbus.ObjectPath
	if err := n.obj(nmPath).Call(nmIface+".AddAndActivateConnection", 0, settings, dbus.ObjectPath(wifiDev), specific).Store(&newConn, &newActive); err != nil {
		return err
	}
	return n.waitForActivation(newActive)
}

// waitForActivation blocks until the connection activates, so a wrong passphrase
// or a timeout returns an error instead of the old silent "success".
func (n *networkState) waitForActivation(active dbus.ObjectPath) error {
	if active == "" || active == "/" {
		return nil
	}
	const stateActivated, stateDeactivated = 2, 4
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		switch n.uintProp(n.obj(active), nmActiveIface+".State") {
		case stateActivated:
			return nil
		case stateDeactivated:
			return fmt.Errorf("could not connect (check the password)")
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("connection timed out")
}

// editableSettings reads a saved profile's settings in a shape that can be
// handed straight back to Update. NetworkManager still reports the deprecated
// ipv4/ipv6 "addresses" and "routes" mirrors of "address-data"/"route-data",
// and the ipv6 pair is an array of structs (a(ayuay)) that godbus decodes to a
// shape it re-encodes as aav, which Update then rejects with a type error. The
// mirrors carry nothing address-data does not, and NetworkManager regenerates
// them, so dropping them is what makes a read-modify-write possible at all.
func (n *networkState) editableSettings(conn dbus.ObjectPath) (map[string]map[string]dbus.Variant, error) {
	var s map[string]map[string]dbus.Variant
	if err := n.obj(conn).Call(nmSettConnIface+".GetSettings", 0).Store(&s); err != nil {
		return nil, err
	}
	dropLegacyAddresses(s)
	return s, nil
}

// dropLegacyAddresses removes the deprecated ipv4/ipv6 address and route
// mirrors from a settings dict. Pure, so the rule is unit-tested without a bus.
func dropLegacyAddresses(s map[string]map[string]dbus.Variant) {
	for _, group := range []string{"ipv4", "ipv6"} {
		keys := s[group]
		if keys == nil {
			continue
		}
		delete(keys, "addresses")
		delete(keys, "routes")
	}
}

// updatePassword rewrites a saved wifi profile's PSK, keeping the rest of its
// settings intact. It fills in key-mgmt only when the profile has none yet,
// from the security the caller resolved, so refreshing the passphrase on a
// WPA3-SAE profile does not silently downgrade it to wpa-psk.
func (n *networkState) updatePassword(conn dbus.ObjectPath, password, keyMgmt string) error {
	s, err := n.editableSettings(conn)
	if err != nil {
		return err
	}
	sec := s["802-11-wireless-security"]
	if sec == nil {
		sec = map[string]dbus.Variant{}
		s["802-11-wireless-security"] = sec
	}
	if _, ok := sec["key-mgmt"]; !ok {
		sec["key-mgmt"] = dbus.MakeVariant(keyMgmt)
	}
	sec["psk"] = dbus.MakeVariant(password)
	return n.obj(conn).Call(nmSettConnIface+".Update", 0, s).Err
}

// updateBand pins a saved wifi profile to one radio band so autoconnect stops
// re-picking the strongest BSS (2.4 GHz at range) after the user chose a
// specific access point.
func (n *networkState) updateBand(conn dbus.ObjectPath, band string) error {
	s, err := n.editableSettings(conn)
	if err != nil {
		return err
	}
	w := s["802-11-wireless"]
	if w == nil {
		w = map[string]dbus.Variant{}
		s["802-11-wireless"] = w
	}
	w["band"] = dbus.MakeVariant(band)
	return n.obj(conn).Call(nmSettConnIface+".Update", 0, s).Err
}

// updateAutoconnect flips a saved wifi profile's autoconnect so the machine
// stops (or resumes) rejoining that network on its own. Turning it off is what
// stops a device from silently associating with an SSID a rogue AP could spoof.
func (n *networkState) updateAutoconnect(conn dbus.ObjectPath, enabled bool) error {
	s, err := n.editableSettings(conn)
	if err != nil {
		return err
	}
	c := s["connection"]
	if c == nil {
		c = map[string]dbus.Variant{}
		s["connection"] = c
	}
	c["autoconnect"] = dbus.MakeVariant(enabled)
	return n.obj(conn).Call(nmSettConnIface+".Update", 0, s).Err
}

// bestApForSsid resolves the access point a connection should bind to. With a
// bssid it returns the AP whose HwAddress matches case-insensitively, so the
// caller can pin an exact band; otherwise it returns the strongest matching AP,
// so a dual-band SSID does not settle on its weaker 2.4 GHz radio. It returns
// "/" and nil when nothing matches, letting NetworkManager choose.
func (n *networkState) bestApForSsid(wifiDev dbus.ObjectPath, ssid, bssid string) (dbus.ObjectPath, *apInfo) {
	bestPath := dbus.ObjectPath("/")
	var best *apInfo
	for _, ap := range n.apPaths(wifiDev) {
		info := n.apInfo(ap)
		if info == nil || info.Ssid != ssid {
			continue
		}
		if bssid != "" {
			if strings.EqualFold(info.Bssid, bssid) {
				return ap, info
			}
			continue
		}
		if best == nil || info.Strength > best.Strength {
			bestPath, best = ap, info
		}
	}
	return bestPath, best
}

func (n *networkState) wifiDisconnect() error {
	wifiDev := n.deviceOfType(nmDeviceWifi)
	if wifiDev == "" {
		return fmt.Errorf("no wifi device")
	}
	return n.obj(wifiDev).Call(nmDeviceIface+".Disconnect", 0).Err
}

func (n *networkState) wifiForget(ssid string) error {
	if ssid == "" {
		return fmt.Errorf("empty ssid")
	}
	for _, c := range n.savedConnections() {
		if c.typ == "802-11-wireless" && c.ssid == ssid {
			return n.obj(c.path).Call(nmSettConnIface+".Delete", 0).Err
		}
	}
	return fmt.Errorf("no saved network: %s", ssid)
}

// wifiSetAutoconnect resolves the saved profile behind ssid and flips its
// autoconnect, mirroring wifiForget's lookup by SSID.
func (n *networkState) wifiSetAutoconnect(ssid string, enabled bool) error {
	if ssid == "" {
		return fmt.Errorf("empty ssid")
	}
	for _, c := range n.savedConnections() {
		if c.typ == "802-11-wireless" && c.ssid == ssid {
			return n.updateAutoconnect(c.path, enabled)
		}
	}
	return fmt.Errorf("no saved network: %s", ssid)
}

func (n *networkState) dnsFrame() map[string]any {
	servers := n.globalDnsServers()
	if servers == nil {
		servers = []string{}
	}
	return map[string]any{
		"provider": dnsProviderForServers(servers),
		"servers":  servers,
	}
}

func (n *networkState) globalDnsServers() []string {
	v, err := n.obj(nmPath).GetProperty(nmIface + ".GlobalDnsConfiguration")
	if err != nil {
		return nil
	}
	config, ok := v.Value().(map[string]dbus.Variant)
	if !ok {
		return nil
	}
	domainsValue, ok := config["domains"]
	if !ok {
		return nil
	}
	domains, ok := domainsValue.Value().(map[string]dbus.Variant)
	if !ok {
		return nil
	}
	defaultDomainValue, ok := domains["*"]
	if !ok {
		return nil
	}
	defaultDomain, ok := defaultDomainValue.Value().(map[string]dbus.Variant)
	if !ok {
		return nil
	}
	serversValue, ok := defaultDomain["servers"]
	if !ok {
		return nil
	}
	servers, _ := serversValue.Value().([]string)
	return servers
}

func (n *networkState) setDns(provider string, custom []string) error {
	args, err := dnsHelperArgs(provider, custom)
	if err != nil {
		return err
	}
	out, err := exec.Command(dnsHelperPath(), args...).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("DNS change failed: %s", message)
	}
	return nil
}

// wgSetActive activates or deactivates a WireGuard tunnel by UUID.
func (n *networkState) wgSetActive(uuid string, active bool) error {
	if uuid == "" {
		return fmt.Errorf("empty uuid")
	}
	if active {
		var conn dbus.ObjectPath
		if err := n.obj(nmSettingsPath).Call(nmSettingsIface+".GetConnectionByUuid", 0, uuid).Store(&conn); err != nil {
			return err
		}
		var out dbus.ObjectPath
		return n.obj(nmPath).Call(nmIface+".ActivateConnection", 0, conn, dbus.ObjectPath("/"), dbus.ObjectPath("/")).Store(&out)
	}
	if ac, ok := n.activeConnections()[uuid]; ok {
		return n.obj(nmPath).Call(nmIface+".DeactivateConnection", 0, ac).Err
	}
	return fmt.Errorf("tunnel not active: %s", uuid)
}

// wgDelete removes a saved WireGuard tunnel by UUID (the reveal's Delete action).
func (n *networkState) wgDelete(uuid string) error {
	if uuid == "" {
		return fmt.Errorf("empty uuid")
	}
	var conn dbus.ObjectPath
	if err := n.obj(nmSettingsPath).Call(nmSettingsIface+".GetConnectionByUuid", 0, uuid).Store(&conn); err != nil {
		return err
	}
	return n.obj(conn).Call(nmSettConnIface+".Delete", 0).Err
}

// wgImport runs nmcli's WireGuard config importer, exactly as the reference
// does (contract 06 sec 4).
func wgImport(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	out, err := exec.Command("nmcli", "connection", "import", "type", "wireguard", "file", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("wireguard import failed: %v: %s", err, out)
	}
	return nil
}

// --- typed property reads ---

func (n *networkState) boolProp(o dbus.BusObject, name string) bool {
	v, err := o.GetProperty(name)
	if err != nil {
		return false
	}
	b, _ := v.Value().(bool)
	return b
}

func (n *networkState) uintProp(o dbus.BusObject, name string) uint32 {
	v, err := o.GetProperty(name)
	if err != nil {
		return 0
	}
	u, _ := v.Value().(uint32)
	return u
}

func (n *networkState) byteProp(o dbus.BusObject, name string) byte {
	v, err := o.GetProperty(name)
	if err != nil {
		return 0
	}
	b, _ := v.Value().(byte)
	return b
}

func (n *networkState) stringProp(o dbus.BusObject, name string) string {
	v, err := o.GetProperty(name)
	if err != nil {
		return ""
	}
	s, _ := v.Value().(string)
	return s
}

func (n *networkState) pathProp(o dbus.BusObject, name string) dbus.ObjectPath {
	v, err := o.GetProperty(name)
	if err != nil {
		return ""
	}
	p, _ := v.Value().(dbus.ObjectPath)
	return p
}

// --- pure helpers (unit-tested) ---
var (
	cloudflareDnsServers = []string{
		"1.1.1.1", "1.0.0.1",
		"2606:4700:4700::1111", "2606:4700:4700::1001",
	}
	googleDnsServers = []string{
		"8.8.8.8", "8.8.4.4",
		"2001:4860:4860::8888", "2001:4860:4860::8844",
	}
)

func dnsServersForSelection(provider string, custom []string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "dhcp":
		return nil, nil
	case "cloudflare":
		return append([]string(nil), cloudflareDnsServers...), nil
	case "google":
		return append([]string(nil), googleDnsServers...), nil
	case "custom":
		servers := make([]string, 0, len(custom))
		seen := make(map[netip.Addr]struct{}, len(custom))
		for _, raw := range custom {
			addr, err := netip.ParseAddr(strings.TrimSpace(raw))
			if err != nil {
				return nil, fmt.Errorf("invalid DNS server %q", raw)
			}
			addr = addr.Unmap()
			if _, exists := seen[addr]; exists {
				continue
			}
			seen[addr] = struct{}{}
			servers = append(servers, addr.String())
		}
		if len(servers) == 0 {
			return nil, fmt.Errorf("enter at least one DNS server")
		}
		return servers, nil
	default:
		return nil, fmt.Errorf("unknown DNS provider %q", provider)
	}
}

func dnsHelperArgs(provider string, custom []string) ([]string, error) {
	normalized := strings.ToLower(strings.TrimSpace(provider))
	servers, err := dnsServersForSelection(normalized, custom)
	if err != nil {
		return nil, err
	}
	args := make([]string, 1, len(servers)+1)
	args[0] = normalized
	return append(args, servers...), nil
}

// dnsPackagedHelper is the installed helper path. A package var (not a literal)
// so a test can point it at an absent path and reach the dev-checkout fallbacks.
var dnsPackagedHelper = "/usr/bin/ryoku-dns"

func dnsHelperPath() string {
	// Prefer the packaged helper: the polkit rule is installed for it and it is
	// the real path on an installed box.
	if _, err := os.Stat(dnsPackagedHelper); err == nil {
		return dnsPackagedHelper
	}
	if shellDir != "" {
		return filepath.Join(shellDir, "..", "..", "system", "hardware", "network", "ryoku-dns")
	}
	if executable, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(executable), "ryoku-dns")
	}
	return "ryoku-dns"
}

func dnsProviderForServers(servers []string) string {
	switch {
	case len(servers) == 0:
		return "dhcp"
	case sameDnsServers(servers, cloudflareDnsServers):
		return "cloudflare"
	case sameDnsServers(servers, googleDnsServers):
		return "google"
	default:
		return "custom"
	}
}

func sameDnsServers(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for _, expected := range want {
		found := false
		for _, actual := range got {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// decodeSsid turns the raw SSID bytes NetworkManager reports into a string.
func decodeSsid(b []byte) string {
	return string(b)
}

// bandForFrequency labels an access point's radio band from its centre
// frequency in MHz for the reveal: 2.4 GHz below 2500, 5 GHz through 5924,
// 6 GHz at or above 5925, and "" when the frequency is unknown.
func bandForFrequency(mhz int) string {
	switch {
	case mhz <= 0:
		return ""
	case mhz < 2500:
		return "2.4"
	case mhz < 5925:
		return "5"
	default:
		return "6"
	}
}

// nmBandForFrequency maps a frequency to NetworkManager's 802-11-wireless.band
// value: "bg" for 2.4 GHz, "a" for 5 GHz. It returns "" for 6 GHz and unknown
// frequencies because NM's band property has no portable 6 GHz value across the
// supported versions and writing an invalid one makes AddAndActivateConnection
// fail, so the caller must omit the key rather than guess.
func nmBandForFrequency(mhz int) string {
	switch bandForFrequency(mhz) {
	case "2.4":
		return "bg"
	case "5":
		return "a"
	default:
		return ""
	}
}

// apFrame flattens an apInfo into the JSON object the QML reveal binds to.
func apFrame(a *apInfo) map[string]any {
	return map[string]any{
		"ssid":        a.Ssid,
		"strength":    a.Strength,
		"security":    a.Security,
		"bssid":       a.Bssid,
		"frequency":   a.Frequency,
		"band":        bandForFrequency(a.Frequency),
		"saved":       a.Saved,
		"active":      a.Active,
		"autoconnect": a.Autoconnect,
	}
}

// ssidSaved reports whether a stored wifi profile matches ssid.
func ssidSaved(ssid string, saved []savedConn) bool {
	for _, c := range saved {
		if c.typ == "802-11-wireless" && c.ssid == ssid {
			return true
		}
	}
	return false
}

// ssidAutoconnect reports the autoconnect state of the stored wifi profile that
// matches ssid, defaulting to true (NetworkManager's own default) when the
// profile leaves the key unset. It is meaningful only for a saved network.
func ssidAutoconnect(ssid string, saved []savedConn) bool {
	for _, c := range saved {
		if c.typ == "802-11-wireless" && c.ssid == ssid {
			return c.autoconnect
		}
	}
	return false
}

// connectivityFromState maps an NMDeviceState to the three-state connectivity
// the reveal shows: 100 activated is Connected, the 40..90 prepare/config/
// need-auth/ip range is Connecting, everything else Disconnected.
func connectivityFromState(state uint32) string {
	switch {
	case state == 100:
		return "Connected"
	case state >= 40 && state < 100:
		return "Connecting"
	default:
		return "Disconnected"
	}
}

// apSecurity derives the reference SecurityType from an access point's flags.
// SAE means WPA3, an 802.1X key-mgmt bit means Enterprise, any RSN bit means
// WPA2, any WPA bit means WPA, the privacy bit alone means WEP, else open.
func apSecurity(flags, wpa, rsn uint32) string {
	const (
		privacy = 0x1
		km8021x = 0x200
		kmSae   = 0x400
	)
	switch {
	case rsn&kmSae != 0 || wpa&kmSae != 0:
		return "Wpa3"
	case rsn&km8021x != 0 || wpa&km8021x != 0:
		return "Enterprise"
	case rsn != 0:
		return "Wpa2"
	case wpa != 0:
		return "Wpa"
	case flags&privacy != 0:
		return "Wep"
	default:
		return "None"
	}
}

// keyMgmtForSecurity picks the key-mgmt for a fresh profile from the security
// apSecurity reports: "sae" for WPA3 so a WPA3-only network can be joined,
// "wpa-psk" for WPA2/WPA and anything unknown (the safe default).
func keyMgmtForSecurity(security string) string {
	if security == "Wpa3" {
		return "sae"
	}
	return "wpa-psk"
}

// wifiConnectSettings builds a minimal NetworkManager profile for a fresh
// connection: an open network gets no security block; a passphrase adds a
// security block whose key-mgmt follows the AP's advertised security so
// WPA3-SAE joins as sae instead of being forced to wpa-psk. A caller-supplied
// bssid pins the profile to the resolved AP's band so autoconnect keeps the
// chosen radio. hidden marks a network that does not broadcast its SSID, which
// is the only way NetworkManager will ever probe for it. Pure so the shape is
// unit-tested without a bus.
func wifiConnectSettings(ssid, password, bssid string, ap *apInfo, hidden bool) map[string]map[string]dbus.Variant {
	wireless := map[string]dbus.Variant{
		"ssid": dbus.MakeVariant([]byte(ssid)),
		"mode": dbus.MakeVariant("infrastructure"),
	}
	if hidden {
		wireless["hidden"] = dbus.MakeVariant(true)
	}
	if bssid != "" && ap != nil {
		if band := nmBandForFrequency(ap.Frequency); band != "" {
			wireless["band"] = dbus.MakeVariant(band)
		}
	}
	settings := map[string]map[string]dbus.Variant{
		"connection": {
			"id":   dbus.MakeVariant(ssid),
			"type": dbus.MakeVariant("802-11-wireless"),
		},
		"802-11-wireless": wireless,
	}
	if password != "" {
		keyMgmt := "wpa-psk"
		if ap != nil {
			keyMgmt = keyMgmtForSecurity(ap.Security)
		}
		settings["802-11-wireless-security"] = map[string]dbus.Variant{
			"key-mgmt": dbus.MakeVariant(keyMgmt),
			"psk":      dbus.MakeVariant(password),
		}
	}
	return settings
}
