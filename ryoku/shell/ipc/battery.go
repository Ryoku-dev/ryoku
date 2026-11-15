package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// battery.go owns the laptop battery readout: it reads UPower over the system
// bus, streams the raw device state to QML on the "battery" topic, and exposes
// the same frame over the "battery.state" call. The QML service derives every
// label, bucket, and signed rate from these raw fields; nothing here formats
// text. The physical-battery pick mirrors the quirk the QML side carried: the
// synthetic DisplayDevice drops capacity/health and can flip ready/isPresent
// once a limited cell sits full on AC, so the readout follows the real
// battery device when UPower lists one.
const (
	batteryBus      = upowerBus
	batteryRootPath = upowerPath
	batteryIface    = "org.freedesktop.UPower"
	batteryDevIface = "org.freedesktop.UPower.Device"

	// UPower device kinds and states, reproduced as-is from the spec:
	// 1 is line power, 2 is a battery.
	upDeviceLinePower    = 1
	upDeviceBattery      = 2
	upStateUnknown       = 0
	upStateCharging      = 1
	upStateDischarging   = 2
	upStateEmpty         = 3
	upStateFullyCharged  = 4
	upStatePendingCharge = 5
)

// batteryDisplayPath is the synthetic aggregate device UPower always exports.
const batteryDisplayPath = dbus.ObjectPath("/org/freedesktop/UPower/devices/DisplayDevice")

type batteryObject interface {
	GetProperty(string) (dbus.Variant, error)
	Call(string, dbus.Flags, ...any) *dbus.Call
}

// batteryFrame is the raw device state one subscriber receives. Field names
// mirror the UPower properties the QML facade binds; derived values (charging,
// low, labels) stay on the QML side so the wire carries facts, not taste.
type batteryFrame struct {
	Present         bool    `json:"present"`
	Pct             float64 `json:"pct"`
	State           int     `json:"state"`
	OnAc            bool    `json:"on_ac"`
	EnergyRate      float64 `json:"energy_rate"`
	Energy          float64 `json:"energy"`
	EnergyFull      float64 `json:"energy_full"`
	HealthSupported bool    `json:"health_supported"`
	Health          float64 `json:"health"`
	TimeToEmpty     float64 `json:"time_to_empty"`
	TimeToFull      float64 `json:"time_to_full"`
}

// batteryState holds the system-bus connection, the topic, and the debounce
// timer that coalesces the burst of PropertiesChanged signals a charge-state
// flip produces.
type batteryState struct {
	conn      *dbus.Conn
	root      batteryObject
	objectFor func(path dbus.ObjectPath) batteryObject
	topic     *stateTopic

	mu      sync.Mutex
	pending *time.Timer
}

// startBattery connects to the system bus, registers the topic and the state
// call, watches UPower signals, and publishes the first frame. A missing
// system bus or UPower disables the readout without failing the daemon; the
// QML facade simply keeps reporting no battery.
func (d *daemon) startBattery() {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Printf("ryoku-shell: battery disabled: %v", err)
		return
	}
	b := &batteryState{
		conn:      conn,
		root:      conn.Object(batteryBus, batteryRootPath),
		objectFor: func(path dbus.ObjectPath) batteryObject { return conn.Object(batteryBus, path) },
		topic:     d.registerTopic("battery"),
	}
	d.battery = b

	if err := addUPowerSignalMatch(conn); err != nil {
		log.Printf("ryoku-shell: battery signal match failed: %v", err)
	}
	sigs := make(chan *dbus.Signal, 16)
	conn.Signal(sigs)
	go func() {
		for sig := range sigs {
			if powerStateSignal(sig) {
				b.kick()
			}
		}
	}()

	d.registerCall("battery.state", func(json.RawMessage) (any, error) {
		return b.state(), nil
	})

	b.publish()
}

// kick schedules a rescan shortly after the last signal: one state flip
// arrives as several PropertiesChanged signals, and each readout is the whole
// device, so rescanning mid-burst only repeats work. The publish is still
// event-driven; the debounce window is 100ms, not a poll.
func (b *batteryState) kick() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pending == nil {
		b.pending = time.AfterFunc(100*time.Millisecond, b.publish)
		return
	}
	b.pending.Reset(100 * time.Millisecond)
}

// state enumerates the devices, picks the physical battery when UPower lists
// one and the DisplayDevice otherwise, and reads the frame. Desktops without
// a battery report present=false with the line-power state still live.
func (b *batteryState) state() batteryFrame {
	frame := batteryFrame{OnAc: true}
	if b.root == nil {
		return frame
	}
	if v, err := b.root.GetProperty(batteryIface + ".OnBattery"); err == nil {
		if onBattery, ok := v.Value().(bool); ok {
			frame.OnAc = !onBattery
		}
	}

	dev := b.pickDevice()
	if dev == nil {
		return frame
	}

	var typ, state float64
	if v, err := dev.GetProperty(batteryDevIface + ".Type"); err == nil {
		typ, _ = toFloat(v.Value())
	}
	if v, err := dev.GetProperty(batteryDevIface + ".IsPresent"); err == nil {
		frame.Present, _ = v.Value().(bool)
	}
	frame.Present = frame.Present && int(typ) == upDeviceBattery

	readInto := func(dst *float64, prop string) {
		if v, err := dev.GetProperty(batteryDevIface + "." + prop); err == nil {
			*dst, _ = toFloat(v.Value())
		}
	}
	readInto(&frame.Pct, "Percentage")
	if v, err := dev.GetProperty(batteryDevIface + ".State"); err == nil {
		f, _ := toFloat(v.Value())
		state = f
	}
	frame.State = int(state)
	readInto(&frame.EnergyRate, "EnergyRate")
	readInto(&frame.Energy, "Energy")
	readInto(&frame.EnergyFull, "EnergyFull")
	readInto(&frame.Health, "HealthPercentage")
	readInto(&frame.TimeToEmpty, "TimeToEmpty")
	readInto(&frame.TimeToFull, "TimeToFull")
	if v, err := dev.GetProperty(batteryDevIface + ".HealthSupported"); err == nil {
		frame.HealthSupported, _ = v.Value().(bool)
	}
	return frame
}

// pickDevice returns the physical laptop battery when UPower enumerates one,
// falling back to the synthetic DisplayDevice. The enumeration order is
// UPower's; the first present battery wins, matching the device list the QML
// side walked before.
func (b *batteryState) pickDevice() batteryObject {
	var paths []dbus.ObjectPath
	call := b.root.Call(batteryIface+".EnumerateDevices", dbus.Flags(0))
	if call.Err != nil || call.Store(&paths) != nil {
		return b.objectFor(batteryDisplayPath)
	}
	for _, path := range paths {
		if path == batteryDisplayPath {
			continue
		}
		dev := b.objectFor(path)
		v, err := dev.GetProperty(batteryDevIface + ".Type")
		if err != nil {
			continue
		}
		f, _ := toFloat(v.Value())
		if int(f) != upDeviceBattery {
			continue
		}
		if pv, err := dev.GetProperty(batteryDevIface + ".IsPresent"); err == nil {
			if present, ok := pv.Value().(bool); ok && present {
				return dev
			}
		}
	}
	return b.objectFor(batteryDisplayPath)
}

// publish marshals the whole battery state and hands it to the topic, which
// drops it if byte-identical to the last frame.
func (b *batteryState) publish() {
	if b.topic == nil {
		return
	}
	frame, err := json.Marshal(b.state())
	if err != nil {
		return
	}
	b.topic.publish(frame)
}

// toFloat widens the dbus numeric kinds to float64 so one reader covers the
// uint32 enums and the double measurements UPower mixes on the device.
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case uint32:
		return float64(n), true
	case int32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case int:
		return float64(n), true
	}
	return 0, false
}
