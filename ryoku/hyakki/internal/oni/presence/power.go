package presence

import (
	"context"

	"github.com/godbus/dbus/v5"
)

// power-profiles-daemon under its current name, then its older one
var powerProfiles = []struct {
	name, path, iface string
}{
	{"org.freedesktop.UPower.PowerProfiles", "/org/freedesktop/UPower/PowerProfiles", "org.freedesktop.UPower.PowerProfiles"},
	{"net.hadess.PowerProfiles", "/net/hadess/PowerProfiles", "net.hadess.PowerProfiles"},
}

const powerSaverProfile = "power-saver"

// Power sets whether the desktop runs its power-saver profile; Oni's
// breathing then drops to the rest tier.
func (s *Service) Power(saver bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.saver == saver {
		return
	}
	s.saver = saver
	s.publish()
}

// WatchPowerSaver reports the power-saver profile now and on every change
// until ctx ends. It waits on the system bus and polls nothing; without
// power-profiles-daemon it reports nothing and returns.
func WatchPowerSaver(ctx context.Context, report func(saver bool)) {
	bus, err := dbus.ConnectSystemBus(dbus.WithContext(ctx))
	if err != nil {
		return
	}
	defer bus.Close()
	for _, pp := range powerProfiles {
		obj := bus.Object(pp.name, dbus.ObjectPath(pp.path))
		v, err := obj.GetProperty(pp.iface + ".ActiveProfile")
		if err != nil {
			continue
		}
		profile, _ := v.Value().(string)
		err = bus.AddMatchSignal(
			dbus.WithMatchObjectPath(dbus.ObjectPath(pp.path)),
			dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
			dbus.WithMatchMember("PropertiesChanged"),
		)
		if err != nil {
			return
		}
		signals := make(chan *dbus.Signal, 4)
		bus.Signal(signals)
		report(profile == powerSaverProfile)
		for {
			select {
			case <-ctx.Done():
				return
			case sig, ok := <-signals:
				if !ok {
					return
				}
				if profile, ok := changedProfile(sig, pp.iface); ok {
					report(profile == powerSaverProfile)
				}
			}
		}
	}
}

// changedProfile reads ActiveProfile out of a PropertiesChanged signal.
func changedProfile(sig *dbus.Signal, iface string) (string, bool) {
	if sig.Name != "org.freedesktop.DBus.Properties.PropertiesChanged" || len(sig.Body) < 2 {
		return "", false
	}
	if changed, ok := sig.Body[0].(string); !ok || changed != iface {
		return "", false
	}
	props, ok := sig.Body[1].(map[string]dbus.Variant)
	if !ok {
		return "", false
	}
	v, ok := props["ActiveProfile"]
	if !ok {
		return "", false
	}
	profile, ok := v.Value().(string)
	return profile, ok
}
