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
// text. Level and charge state follow the aggregate DisplayDevice; presence,
// health and capacity follow the physical pack, because the synthetic device
// drops them and can flip isPresent once a limited cell sits full on AC.
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

// state reads the frame from two devices: level, charge state, rate and times
// follow the aggregate DisplayDevice (falling back to the active pack, then the
// first pack), while presence, health and capacity stay on the first physical
// pack, which keeps the metadata the synthetic device drops. Desktops without
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

	packs := b.presentBatteries()
	physical := b.objectFor(batteryDisplayPath)
	if len(packs) > 0 {
		physical = packs[0]
	}
	level := b.levelDevice(packs, physical)

	var typ float64
	if v, err := physical.GetProperty(batteryDevIface + ".Type"); err == nil {
		typ, _ = toFloat(v.Value())
	}
	if v, err := physical.GetProperty(batteryDevIface + ".IsPresent"); err == nil {
		frame.Present, _ = v.Value().(bool)
	}
	frame.Present = frame.Present && int(typ) == upDeviceBattery

	readFrom := func(dev batteryObject, dst *float64, prop string) {
		if v, err := dev.GetProperty(batteryDevIface + "." + prop); err == nil {
			*dst, _ = toFloat(v.Value())
		}
	}
	var state float64
	readFrom(level, &frame.Pct, "Percentage")
	readFrom(level, &state, "State")
	frame.State = int(state)
	readFrom(level, &frame.EnergyRate, "EnergyRate")
	readFrom(level, &frame.Energy, "Energy")
	readFrom(level, &frame.TimeToEmpty, "TimeToEmpty")
	readFrom(level, &frame.TimeToFull, "TimeToFull")
	readFrom(physical, &frame.EnergyFull, "EnergyFull")
	readFrom(physical, &frame.Health, "HealthPercentage")
	if v, err := physical.GetProperty(batteryDevIface + ".HealthSupported"); err == nil {
		frame.HealthSupported, _ = v.Value().(bool)
	}
	// HealthPercentage/HealthSupported only exist on UPower builds that ship
	// them; on the rest the pack still reports Capacity, the wear percentage
	// (energy-full / energy-full-design) every UPower carries. Read it in the
	// same scan so QML never probes the bus itself.
	if !frame.HealthSupported {
		var capacity float64
		readFrom(physical, &capacity, "Capacity")
		if capacity > 0 {
			frame.HealthSupported = true
			frame.Health = capacity
		}
	}
	return frame
}

// presentBatteries lists the physical batteries UPower enumerates, in its
// order, skipping absent packs and the synthetic DisplayDevice.
func (b *batteryState) presentBatteries() []batteryObject {
	var paths []dbus.ObjectPath
	call := b.root.Call(batteryIface+".EnumerateDevices", dbus.Flags(0))
	if call.Err != nil || call.Store(&paths) != nil {
		return nil
	}
	var packs []batteryObject
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
				packs = append(packs, dev)
			}
		}
	}
	return packs
}

// levelDevice is the aggregate unless it is unusable: some UPower versions
// leave its state unknown once a cell sits full on AC. Then the pack that is
// charging or discharging carries the machine, else the first pack. On a
// two-battery laptop the first pack can sit parked at 99 % for hours.
func (b *batteryState) levelDevice(packs []batteryObject, physical batteryObject) batteryObject {
	agg := b.objectFor(batteryDisplayPath)
	if v, err := agg.GetProperty(batteryDevIface + ".State"); err == nil {
		if f, _ := toFloat(v.Value()); int(f) != upStateUnknown {
			return agg
		}
	}
	for _, pack := range packs {
		v, err := pack.GetProperty(batteryDevIface + ".State")
		if err != nil {
			continue
		}
		f, _ := toFloat(v.Value())
		if int(f) == upStateCharging || int(f) == upStateDischarging {
			return pack
		}
	}
	return physical
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
// uint32 enums, the int64 times and the double measurements UPower mixes on
// the device.
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
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	}
	return 0, false
}
