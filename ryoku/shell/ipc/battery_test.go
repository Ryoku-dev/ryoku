package main

import (
	"fmt"
	"testing"

	"github.com/godbus/dbus/v5"
)

type fakeBatteryObject struct {
	props map[string]dbus.Variant
	calls []string
}

func (f *fakeBatteryObject) GetProperty(name string) (dbus.Variant, error) {
	if v, ok := f.props[name]; ok {
		return v, nil
	}
	return dbus.Variant{}, fmt.Errorf("unknown property %s", name)
}

func (f *fakeBatteryObject) Call(name string, _ dbus.Flags, args ...any) *dbus.Call {
	f.calls = append(f.calls, name)
	if paths, ok := f.props["__paths"]; ok {
		return &dbus.Call{Body: []any{paths.Value()}}
	}
	return &dbus.Call{Err: fmt.Errorf("unexpected call %s", name)}
}

func batteryDeviceProps(typ uint32, present bool, extra map[string]dbus.Variant) map[string]dbus.Variant {
	props := map[string]dbus.Variant{
		batteryDevIface + ".Type":      dbus.MakeVariant(typ),
		batteryDevIface + ".IsPresent": dbus.MakeVariant(present),
	}
	for k, v := range extra {
		props[k] = v
	}
	return props
}

const (
	testBat0    = dbus.ObjectPath("/org/freedesktop/UPower/devices/battery_BAT0")
	testBat1    = dbus.ObjectPath("/org/freedesktop/UPower/devices/battery_BAT1")
	testAdapter = dbus.ObjectPath("/org/freedesktop/UPower/devices/line_power_ADP0")
)

func testBatteryState(onBattery bool, devices map[dbus.ObjectPath]*fakeBatteryObject) *batteryState {
	paths := []dbus.ObjectPath{}
	for _, path := range []dbus.ObjectPath{testAdapter, testBat0, testBat1, batteryDisplayPath} {
		if _, ok := devices[path]; ok {
			paths = append(paths, path)
		}
	}
	root := &fakeBatteryObject{props: map[string]dbus.Variant{
		batteryIface + ".OnBattery": dbus.MakeVariant(onBattery),
		"__paths":                   dbus.MakeVariant(paths),
	}}
	return &batteryState{
		root: root,
		objectFor: func(path dbus.ObjectPath) batteryObject {
			if dev, ok := devices[path]; ok {
				return dev
			}
			return &fakeBatteryObject{props: map[string]dbus.Variant{}}
		},
	}
}

func packProps(pct float64, state uint32, extra map[string]dbus.Variant) *fakeBatteryObject {
	props := map[string]dbus.Variant{
		batteryDevIface + ".Percentage": dbus.MakeVariant(pct),
		batteryDevIface + ".State":      dbus.MakeVariant(state),
	}
	for k, v := range extra {
		props[k] = v
	}
	return &fakeBatteryObject{props: batteryDeviceProps(upDeviceBattery, true, props)}
}

func TestBatteryStateLevelFollowsAggregateHealthStaysOnPack(t *testing.T) {
	// Two packs: BAT0 is parked at 99 % pending-charge while BAT1 carries the
	// load. The aggregate says 55 % discharging; presence, health and capacity
	// still come from the first physical pack.
	bat0 := packProps(99, upStatePendingCharge, map[string]dbus.Variant{
		batteryDevIface + ".EnergyFull": dbus.MakeVariant(49.5),
		batteryDevIface + ".Capacity":   dbus.MakeVariant(96.0),
	})
	bat1 := packProps(20, upStateDischarging, nil)
	display := packProps(55, upStateDischarging, map[string]dbus.Variant{
		batteryDevIface + ".EnergyRate":  dbus.MakeVariant(11.5),
		batteryDevIface + ".TimeToEmpty": dbus.MakeVariant(int64(5400)),
		batteryDevIface + ".EnergyFull":  dbus.MakeVariant(99.0),
	})
	b := testBatteryState(true, map[dbus.ObjectPath]*fakeBatteryObject{
		testAdapter:        {props: batteryDeviceProps(upDeviceLinePower, true, nil)},
		testBat0:           bat0,
		testBat1:           bat1,
		batteryDisplayPath: display,
	})

	frame := b.state()
	if frame.Pct != 55 || frame.State != upStateDischarging {
		t.Fatalf("level must follow the aggregate, got %+v", frame)
	}
	if frame.EnergyRate != 11.5 || frame.TimeToEmpty != 5400 {
		t.Fatalf("rate and time must follow the aggregate: %+v", frame)
	}
	if !frame.Present || !frame.HealthSupported || frame.Health != 96 || frame.EnergyFull != 49.5 {
		t.Fatalf("presence, health and capacity must stay on the physical pack: %+v", frame)
	}
	if frame.OnAc {
		t.Fatalf("OnBattery=true must not read as on AC: %+v", frame)
	}
}

func TestBatteryStateUnknownAggregateFallsBackToActivePack(t *testing.T) {
	// Some UPower versions leave the aggregate state unknown with a cell full
	// on AC; the charging pack then carries the readout, not the parked first.
	bat0 := packProps(99, upStateFullyCharged, nil)
	bat1 := packProps(40, upStateCharging, map[string]dbus.Variant{
		batteryDevIface + ".EnergyRate": dbus.MakeVariant(20.0),
		batteryDevIface + ".TimeToFull": dbus.MakeVariant(int64(1800)),
	})
	display := packProps(70, upStateUnknown, nil)
	b := testBatteryState(false, map[dbus.ObjectPath]*fakeBatteryObject{
		testBat0:           bat0,
		testBat1:           bat1,
		batteryDisplayPath: display,
	})

	frame := b.state()
	if frame.Pct != 40 || frame.State != upStateCharging || frame.EnergyRate != 20 || frame.TimeToFull != 1800 {
		t.Fatalf("expected the charging pack to win, got %+v", frame)
	}
	if !frame.OnAc {
		t.Fatalf("OnBattery=false must read as on AC: %+v", frame)
	}
}

func TestBatteryStateUnknownAggregateWithIdlePacksUsesFirstPack(t *testing.T) {
	bat0 := packProps(81, upStatePendingCharge, nil)
	bat1 := packProps(30, upStateEmpty, nil)
	display := packProps(0, upStateUnknown, nil)
	b := testBatteryState(false, map[dbus.ObjectPath]*fakeBatteryObject{
		testBat0:           bat0,
		testBat1:           bat1,
		batteryDisplayPath: display,
	})

	frame := b.state()
	if frame.Pct != 81 || frame.State != upStatePendingCharge {
		t.Fatalf("expected the first pack, got %+v", frame)
	}
}

func TestBatteryStateDesktopReportsNoBattery(t *testing.T) {
	// No battery in the enumeration: present=false, but the line-power state
	// still rides through so "may I run hot" answers correctly. The display
	// device is battery-kind but reports no pack on a desktop.
	display := &fakeBatteryObject{props: batteryDeviceProps(upDeviceBattery, false, nil)}
	root := &fakeBatteryObject{props: map[string]dbus.Variant{
		batteryIface + ".OnBattery": dbus.MakeVariant(false),
		"__paths":                   dbus.MakeVariant([]dbus.ObjectPath{batteryDisplayPath}),
	}}
	b := &batteryState{
		root:      root,
		objectFor: func(dbus.ObjectPath) batteryObject { return display },
	}

	frame := b.state()
	if frame.Present {
		t.Fatalf("desktop must report no battery: %+v", frame)
	}
	if !frame.OnAc {
		t.Fatalf("desktop must report AC: %+v", frame)
	}
}

func TestBatteryStateSkipsAbsentBattery(t *testing.T) {
	// A removed battery (IsPresent=false) must not win the pick; the display
	// device answers instead, reporting presence=false.
	absent := &fakeBatteryObject{props: batteryDeviceProps(upDeviceBattery, false, nil)}
	display := &fakeBatteryObject{props: batteryDeviceProps(upDeviceBattery, false, nil)}
	root := &fakeBatteryObject{props: map[string]dbus.Variant{
		batteryIface + ".OnBattery": dbus.MakeVariant(true),
		"__paths": dbus.MakeVariant([]dbus.ObjectPath{
			"/org/freedesktop/UPower/devices/battery_BAT0",
			batteryDisplayPath,
		}),
	}}
	b := &batteryState{
		root: root,
		objectFor: func(path dbus.ObjectPath) batteryObject {
			if path == "/org/freedesktop/UPower/devices/battery_BAT0" {
				return absent
			}
			return display
		},
	}

	frame := b.state()
	if frame.Present || frame.OnAc {
		t.Fatalf("absent battery with OnBattery=true: %+v", frame)
	}
}

func TestBatteryStateWithoutCapacityHasNoHealth(t *testing.T) {
	// a pack that cannot report its health leaves Capacity at 0, and one on a
	// bus without the property never sets it: neither may read as 0 % health
	for _, extra := range []map[string]dbus.Variant{
		nil,
		{batteryDevIface + ".Capacity": dbus.MakeVariant(0.0)},
	} {
		bat0 := packProps(64, upStateDischarging, extra)
		b := testBatteryState(true, map[dbus.ObjectPath]*fakeBatteryObject{
			testBat0:           bat0,
			batteryDisplayPath: packProps(64, upStateDischarging, nil),
		})
		if frame := b.state(); frame.HealthSupported || frame.Health != 0 {
			t.Fatalf("no capacity must leave health unsupported, got %+v", frame)
		}
	}
}

func TestToFloatWidensUPowerKinds(t *testing.T) {
	// UPower's TimeToEmpty and TimeToFull are int64 ("x") on the bus
	for _, v := range []any{float64(16831), uint32(16831), int32(16831), uint64(16831), int64(16831), 16831} {
		if f, ok := toFloat(v); !ok || f != 16831 {
			t.Fatalf("toFloat(%T) = %v, %v", v, f, ok)
		}
	}
}
