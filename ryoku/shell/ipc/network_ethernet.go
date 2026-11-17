package main

// network_ethernet.go: the wired adapters and saved Ethernet profiles for the
// Hub's Connections page. The frame's `ethernet` list has one entry per wired
// adapter (its state, link speed and address) and `ethernetProfiles` the saved
// profiles a cable can use; network.ethernetConnect, ethernetDisconnect and
// ethernetSetAutoconnect act on them. A profile's IP settings go through the
// same network.ip* calls a Wi-Fi network's do.

import (
	"encoding/json"
	"fmt"

	"github.com/godbus/dbus/v5"
)

const nmWiredIface = "org.freedesktop.NetworkManager.Device.Wired"

// wiredDevice is what the frame shows of one wired adapter.
type wiredDevice struct {
	iface      string
	mac        string
	state      uint32 // NMDeviceState
	managed    bool
	speed      uint32 // Mb/s, 0 when unknown or down
	activeUuid string
	activeName string
	ip4        string
}

func (n *networkState) registerEthernetCalls(d *daemon) {
	d.registerCall("network.ethernetConnect", func(raw json.RawMessage) (any, error) {
		var a struct {
			Device string `json:"device"`
			Uuid   string `json:"uuid"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, n.ethernetConnect(a.Device, a.Uuid)
	})
	d.registerCall("network.ethernetDisconnect", func(raw json.RawMessage) (any, error) {
		var a struct {
			Device string `json:"device"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, n.ethernetDisconnect(a.Device)
	})
	d.registerCall("network.ethernetSetAutoconnect", func(raw json.RawMessage) (any, error) {
		var a struct {
			Uuid        string `json:"uuid"`
			Autoconnect bool   `json:"autoconnect"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		c, err := ethernetProfile(n.savedConnections(), a.Uuid)
		if err != nil {
			return nil, err
		}
		return nil, n.updateAutoconnect(c.path, a.Autoconnect)
	})
}

// wiredDevices reads every Ethernet device NetworkManager knows, in its order.
func (n *networkState) wiredDevices() []wiredDevice {
	var paths []dbus.ObjectPath
	if n.obj(nmPath).Call(nmIface+".GetAllDevices", 0).Store(&paths) != nil {
		return nil
	}
	var out []wiredDevice
	for _, p := range paths {
		o := n.obj(p)
		if n.uintProp(o, nmDeviceIface+".DeviceType") != nmDeviceEthernet {
			continue
		}
		dev := wiredDevice{
			iface:   n.stringProp(o, nmDeviceIface+".Interface"),
			mac:     n.stringProp(o, nmDeviceIface+".HwAddress"),
			state:   n.uintProp(o, nmDeviceIface+".State"),
			managed: n.boolProp(o, nmDeviceIface+".Managed"),
			speed:   n.uintProp(o, nmWiredIface+".Speed"),
		}
		if ac := n.pathProp(o, nmDeviceIface+".ActiveConnection"); ac != "" && ac != "/" {
			a := n.obj(ac)
			dev.activeUuid = n.stringProp(a, nmActiveIface+".Uuid")
			dev.activeName = n.stringProp(a, nmActiveIface+".Id")
		}
		if cfg := n.pathProp(o, nmDeviceIface+".Ip4Config"); cfg != "" && cfg != "/" && dev.state == 100 {
			if v, err := n.obj(cfg).GetProperty(nmIp4Iface + ".AddressData"); err == nil {
				if data, ok := v.Value().([]map[string]dbus.Variant); ok {
					dev.ip4 = firstAddress(data)
				}
			}
		}
		out = append(out, dev)
	}
	return out
}

// wiredState names an adapter's NMDeviceState for the Hub: no cable (20,
// unavailable) and not managed by NetworkManager (10) are told apart from a
// plain disconnect, since each needs a different fix.
func wiredState(state uint32, managed bool) string {
	switch {
	case !managed || state == 10:
		return "unmanaged"
	case state == 20:
		return "unplugged"
	case state == 100:
		return "connected"
	case state >= 40 && state < 100:
		return "connecting"
	default:
		return "disconnected"
	}
}

// ethernetFrames is the frame's `ethernet` list. Pure, so it is unit-tested
// without a bus.
func ethernetFrames(devs []wiredDevice) []map[string]any {
	out := []map[string]any{}
	for _, d := range devs {
		speed := d.speed
		if d.state != 100 {
			speed = 0
		}
		out = append(out, map[string]any{
			"device":     d.iface,
			"mac":        d.mac,
			"state":      wiredState(d.state, d.managed),
			"speed":      speed,
			"ip4":        d.ip4,
			"activeUuid": d.activeUuid,
			"activeName": d.activeName,
		})
	}
	return out
}

// ethernetProfileFrames is the frame's `ethernetProfiles` list: each saved
// Ethernet profile, the adapter it is bound to ("" for any) and whether it is
// up. Pure, so it is unit-tested without a bus.
func ethernetProfileFrames(saved []savedConn, active map[string]dbus.ObjectPath) []map[string]any {
	out := []map[string]any{}
	for _, c := range saved {
		if c.typ != "802-3-ethernet" {
			continue
		}
		_, up := active[c.uuid]
		out = append(out, map[string]any{
			"uuid":        c.uuid,
			"name":        c.id,
			"device":      c.iface,
			"autoconnect": c.autoconnect,
			"active":      up,
		})
	}
	return out
}

// ethernetProfile finds a saved Ethernet profile by UUID, refusing any other
// type so a wired control can never switch a Wi-Fi network or a VPN.
func ethernetProfile(saved []savedConn, uuid string) (savedConn, error) {
	if uuid == "" {
		return savedConn{}, fmt.Errorf("empty uuid")
	}
	for _, c := range saved {
		if c.uuid == uuid {
			if c.typ != "802-3-ethernet" {
				return savedConn{}, fmt.Errorf("not an Ethernet profile: %s", uuid)
			}
			return c, nil
		}
	}
	return savedConn{}, fmt.Errorf("no saved connection %s", uuid)
}

// wiredDeviceByIface resolves an adapter name to its device, refusing anything
// that is not a wired adapter.
func (n *networkState) wiredDeviceByIface(iface string) (dbus.ObjectPath, error) {
	if iface == "" {
		return "", fmt.Errorf("empty device")
	}
	var dev dbus.ObjectPath
	if err := n.obj(nmPath).Call(nmIface+".GetDeviceByIpIface", 0, iface).Store(&dev); err != nil {
		return "", fmt.Errorf("no device %s", iface)
	}
	if n.uintProp(n.obj(dev), nmDeviceIface+".DeviceType") != nmDeviceEthernet {
		return "", fmt.Errorf("not a wired adapter: %s", iface)
	}
	return dev, nil
}

// ethernetConnect brings an adapter up, with the given saved profile or, with
// no uuid, the one NetworkManager picks (it makes a DHCP profile when the
// adapter has none).
func (n *networkState) ethernetConnect(iface, uuid string) error {
	dev, err := n.wiredDeviceByIface(iface)
	if err != nil {
		return err
	}
	conn := dbus.ObjectPath("/")
	if uuid != "" {
		c, err := ethernetProfile(n.savedConnections(), uuid)
		if err != nil {
			return err
		}
		if c.iface != "" && c.iface != iface {
			return fmt.Errorf("%s is bound to %s", c.id, c.iface)
		}
		conn = c.path
	}
	var active dbus.ObjectPath
	return n.obj(nmPath).Call(nmIface+".ActivateConnection", 0, conn, dev, dbus.ObjectPath("/")).Store(&active)
}

// ethernetDisconnect takes an adapter down until it is connected again; it
// does not rejoin on its own while the cable stays in.
func (n *networkState) ethernetDisconnect(iface string) error {
	dev, err := n.wiredDeviceByIface(iface)
	if err != nil {
		return err
	}
	return n.obj(dev).Call(nmDeviceIface+".Disconnect", 0).Err
}
