package main

import (
	"context"
	"encoding/json"
	"log"
	"slices"
	"sort"
	"time"

	"github.com/godbus/dbus/v5"
)

// bluez.go streams BlueZ presence and adapter power on the "bluetooth" state
// topic. Quickshell's own BlueZ client attaches only when bluetoothd is already
// up as qs starts, and never notices it leaving, so BluetoothPower.qml reads
// this frame for exactly those two cases. Event-driven: one system-bus
// connection, NameOwnerChanged for org.bluez plus the adapter ObjectManager and
// PropertiesChanged signals, no polling.
const (
	bluezBusName       = "org.bluez"
	bluezAdapterIface  = "org.bluez.Adapter1"
	objectManagerIface = "org.freedesktop.DBus.ObjectManager"
	bluezSettle        = 150 * time.Millisecond
	bluezCallTimeout   = 3 * time.Second
)

type bluezAdapter struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Powered bool   `json:"powered"`
}

type bluezFrame struct {
	Running  bool           `json:"running"`
	Adapters []bluezAdapter `json:"adapters"`
}

type bluezObjects = map[dbus.ObjectPath]map[string]map[string]dbus.Variant

type bluezEvent int

const (
	bluezIgnore bluezEvent = iota
	bluezRefresh
	bluezGone
)

// bluezFrameFields are the adapter properties the frame carries; any other
// change (Discovering during a scan, for one) publishes nothing.
var bluezFrameFields = []string{"Powered", "Alias", "Address"}

type bluezState struct {
	topic  *stateTopic
	fetch  func() (bluezObjects, error)
	settle time.Duration
}

func (d *daemon) startBluez() {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Printf("ryoku-shell: bluetooth topic disabled: %v", err)
		return
	}
	b := &bluezState{
		topic:  d.registerTopic("bluetooth"),
		fetch:  func() (bluezObjects, error) { return fetchBluezObjects(conn) },
		settle: bluezSettle,
	}
	for _, opts := range [][]dbus.MatchOption{
		{
			dbus.WithMatchSender("org.freedesktop.DBus"),
			dbus.WithMatchInterface("org.freedesktop.DBus"),
			dbus.WithMatchMember("NameOwnerChanged"),
			dbus.WithMatchArg(0, bluezBusName),
		},
		{
			dbus.WithMatchSender(bluezBusName),
			dbus.WithMatchInterface(objectManagerIface),
		},
		{
			dbus.WithMatchSender(bluezBusName),
			dbus.WithMatchInterface(propertiesInterface),
			dbus.WithMatchMember("PropertiesChanged"),
			dbus.WithMatchArg(0, bluezAdapterIface),
		},
	} {
		if err := conn.AddMatchSignal(opts...); err != nil {
			log.Printf("ryoku-shell: bluetooth signal match failed: %v", err)
		}
	}
	sigs := make(chan *dbus.Signal, 64)
	conn.Signal(sigs)
	go b.run(sigs)
}

// fetchBluezObjects never auto-starts bluetoothd: an absent service reads as
// not running instead of being launched by the daemon.
func fetchBluezObjects(conn *dbus.Conn) (bluezObjects, error) {
	ctx, cancel := context.WithTimeout(context.Background(), bluezCallTimeout)
	defer cancel()
	var objs bluezObjects
	err := conn.Object(bluezBusName, "/").CallWithContext(ctx,
		objectManagerIface+".GetManagedObjects", dbus.FlagNoAutoStart).Store(&objs)
	return objs, err
}

// run publishes the first frame, then coalesces each burst of relevant signals
// into one frame after the settle window. The latest owner event in a burst
// wins, so a restart (lost then gained) ends on a fresh read.
func (b *bluezState) run(sigs <-chan *dbus.Signal) {
	b.apply(bluezRefresh)
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	pending := bluezIgnore
	for {
		select {
		case sig, ok := <-sigs:
			if !ok {
				timer.Stop()
				return
			}
			ev := classifyBluezSignal(sig)
			if ev == bluezIgnore {
				continue
			}
			if pending == bluezIgnore {
				timer.Reset(b.settle)
			}
			pending = ev
		case <-timer.C:
			b.apply(pending)
			pending = bluezIgnore
		}
	}
}

func (b *bluezState) apply(ev bluezEvent) {
	frame := bluezFrame{Adapters: []bluezAdapter{}}
	if ev == bluezRefresh {
		if objs, err := b.fetch(); err == nil {
			frame = bluezFrameFromObjects(objs)
		}
	}
	raw, err := json.Marshal(frame)
	if err != nil {
		return
	}
	b.topic.publish(raw)
}

func bluezFrameFromObjects(objs bluezObjects) bluezFrame {
	frame := bluezFrame{Running: true, Adapters: []bluezAdapter{}}
	for path, ifaces := range objs {
		props, ok := ifaces[bluezAdapterIface]
		if !ok {
			continue
		}
		a := bluezAdapter{Path: string(path)}
		a.Name, _ = props["Alias"].Value().(string)
		a.Address, _ = props["Address"].Value().(string)
		a.Powered, _ = props["Powered"].Value().(bool)
		frame.Adapters = append(frame.Adapters, a)
	}
	sort.Slice(frame.Adapters, func(i, j int) bool { return frame.Adapters[i].Path < frame.Adapters[j].Path })
	return frame
}

func classifyBluezSignal(sig *dbus.Signal) bluezEvent {
	if sig == nil {
		return bluezIgnore
	}
	switch sig.Name {
	case "org.freedesktop.DBus.NameOwnerChanged":
		if len(sig.Body) < 3 {
			return bluezIgnore
		}
		name, _ := sig.Body[0].(string)
		owner, _ := sig.Body[2].(string)
		if name != bluezBusName {
			return bluezIgnore
		}
		if owner == "" {
			return bluezGone
		}
		return bluezRefresh
	case objectManagerIface + ".InterfacesAdded":
		if len(sig.Body) >= 2 {
			if ifaces, ok := sig.Body[1].(map[string]map[string]dbus.Variant); ok {
				if _, ok := ifaces[bluezAdapterIface]; ok {
					return bluezRefresh
				}
			}
		}
	case objectManagerIface + ".InterfacesRemoved":
		if len(sig.Body) >= 2 {
			if ifaces, ok := sig.Body[1].([]string); ok && slices.Contains(ifaces, bluezAdapterIface) {
				return bluezRefresh
			}
		}
	case propertiesInterface + ".PropertiesChanged":
		if len(sig.Body) < 2 {
			return bluezIgnore
		}
		if iface, _ := sig.Body[0].(string); iface != bluezAdapterIface {
			return bluezIgnore
		}
		changed, _ := sig.Body[1].(map[string]dbus.Variant)
		var invalidated []string
		if len(sig.Body) >= 3 {
			invalidated, _ = sig.Body[2].([]string)
		}
		for _, field := range bluezFrameFields {
			if _, ok := changed[field]; ok || slices.Contains(invalidated, field) {
				return bluezRefresh
			}
		}
	}
	return bluezIgnore
}
