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

func TestBatteryStatePicksPhysicalBatteryOverDisplayDevice(t *testing.T) {
	// The quirk this pins: a cell held at a charge limit on AC reports
	// pending-charge and full health only on the physical device; the synthetic
	// DisplayDevice drops health and can flip presence. The readout must follow
	// the physical battery.
	bat := &fakeBatteryObject{props: batteryDeviceProps(upDeviceBattery, true, map[string]dbus.Variant{
		batteryDevIface + ".Percentage":       dbus.MakeVariant(81.0),
		batteryDevIface + ".State":            dbus.MakeVariant(uint32(upStatePendingCharge)),
		batteryDevIface + ".EnergyRate":       dbus.MakeVariant(12.5),
		batteryDevIface + ".Energy":           dbus.MakeVariant(40.0),
		batteryDevIface + ".EnergyFull":       dbus.MakeVariant(49.5),
		batteryDevIface + ".HealthSupported":  dbus.MakeVariant(true),
		batteryDevIface + ".HealthPercentage": dbus.MakeVariant(96.0),
		batteryDevIface + ".TimeToEmpty":      dbus.MakeVariant(0.0),
		batteryDevIface + ".TimeToFull":       dbus.MakeVariant(1800.0),
	})}
	line := &fakeBatteryObject{props: batteryDeviceProps(upDeviceLinePower, true, nil)}
	display := &fakeBatteryObject{props: batteryDeviceProps(upDeviceBattery, true, map[string]dbus.Variant{
		batteryDevIface + ".Percentage": dbus.MakeVariant(81.0),
		batteryDevIface + ".State":      dbus.MakeVariant(uint32(upStateCharging)),
	})}
	root := &fakeBatteryObject{props: map[string]dbus.Variant{
		batteryIface + ".OnBattery": dbus.MakeVariant(false),
		"__paths": dbus.MakeVariant([]dbus.ObjectPath{
			"/org/freedesktop/UPower/devices/line_power_ADP0",
			"/org/freedesktop/UPower/devices/battery_BAT0",
			batteryDisplayPath,
		}),
	}}
	b := &batteryState{
		root: root,
		objectFor: func(path dbus.ObjectPath) batteryObject {
			switch path {
			case "/org/freedesktop/UPower/devices/battery_BAT0":
				return bat
			case "/org/freedesktop/UPower/devices/line_power_ADP0":
				return line
			}
			return display
		},
	}

	frame := b.state()
	if !frame.Present || frame.Pct != 81.0 || frame.State != upStatePendingCharge {
		t.Fatalf("frame = %+v, want BAT0 pending-charge at 81", frame)
	}
	if !frame.OnAc {
		t.Fatal("OnAc: OnBattery=false must read as on AC")
	}
	if !frame.HealthSupported || frame.Health != 96.0 || frame.EnergyFull != 49.5 {
		t.Fatalf("health/energy not read from the physical device: %+v", frame)
	}
	if frame.TimeToFull != 1800.0 || frame.EnergyRate != 12.5 {
		t.Fatalf("rates/times not read from the physical device: %+v", frame)
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
