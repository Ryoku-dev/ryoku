package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

// autoprofile switches the power profile to Power Saver while the machine runs on
// battery and restores the prior profile on AC, when the user opts in via
// performance.json's autoPowerSaverOnBattery. It rides the same sysfs power state
// powersounds reads and the same power-profiles-daemon connection the profile
// topic uses, so QML sees the switch on its normal PowerProfiles stream (and the
// shell's Perf policy then lightens the desktop). The decision is a pure step over
// the AC state and the saved profile, unit-tested without a bus, sysfs, or clock.

const ppSaver = "power-saver"

const (
	autoProfileReconcile = 5 * time.Minute
	upowerBus            = "org.freedesktop.UPower"
	upowerPath           = dbus.ObjectPath("/org/freedesktop/UPower")
)

// autoProfile tracks one on-battery episode: the profile to restore on AC, and
// whether we already switched this episode, so a manual change while on battery is
// left alone until the next unplug.
type autoProfile struct {
	saved  string // profile to restore when back on AC; "" = nothing to restore
	active bool   // we switched to Power Saver for the current on-battery episode
}

// step folds one power reading into the tracker and returns the profile to set, or
// "" for no change. It switches once when a battery episode begins, restores on AC
// (or when the feature is turned off) only while the profile is still the saver we
// set, and otherwise keeps out of the way.
func (a *autoProfile) step(enabled, onBattery bool, current string, avail []string) string {
	hasSaver := false
	for _, p := range avail {
		if p == ppSaver {
			hasSaver = true
			break
		}
	}
	if enabled && hasSaver && onBattery {
		if a.active {
			return "" // already switched this episode; do not fight a manual change
		}
		a.active = true
		if current != ppSaver {
			a.saved = current
			return ppSaver
		}
		a.saved = "" // already on saver; nothing to restore to
		return ""
	}
	if a.active {
		a.active = false
		restore := a.saved
		a.saved = ""
		if restore != "" && current == ppSaver {
			return restore
		}
	}
	return ""
}

// powerStateSignal reports the UPower and power-profiles signals that can
// change this policy. The connection only receives installed bus matches, but
// keeping the predicate explicit makes unrelated profile traffic harmless.
func powerStateSignal(sig *dbus.Signal) bool {
	if sig == nil {
		return false
	}
	if sig.Path == dbus.ObjectPath(ppPath) {
		return sig.Name == propertiesInterface+".PropertiesChanged"
	}
	return strings.HasPrefix(string(sig.Path), string(upowerPath)) &&
		(sig.Name == propertiesInterface+".PropertiesChanged" ||
			sig.Name == "org.freedesktop.UPower.DeviceAdded" ||
			sig.Name == "org.freedesktop.UPower.DeviceRemoved")
}

func addUPowerSignalMatch(conn *dbus.Conn) error {
	return conn.AddMatchSignal(
		dbus.WithMatchSender(upowerBus),
		dbus.WithMatchPathNamespace(upowerPath),
	)
}

// watchAutoPowerSaver reacts to UPower and power-profiles D-Bus signals. The
// watchdog is armed only after that event source is unavailable.
func (d *daemon) watchAutoPowerSaver() {
	if d.pp == nil || d.pp.conn == nil {
		d.reconcileSaverExtras(context.Background(), false, "")
		return
	}
	eventsHealthy := true
	if err := addUPowerSignalMatch(d.pp.conn); err != nil {
		log.Printf("ryoku-shell: auto power profile signal match failed, using slow reconciliation: %v", err)
		eventsHealthy = false
	}
	signals := make(chan *dbus.Signal, 16)
	d.pp.conn.Signal(signals)
	defer d.pp.conn.RemoveSignal(signals)
	var a autoProfile
	prevAC, first := true, true
	reconcile := func() {
		st := readPowerState()
		onBattery := st.present && st.discharging
		onAC := !onBattery
		active := d.pp.activeProfile()
		if !first && onAC != prevAC {
			d.pp.noteACFlip()
		}
		to := a.step(perfFlag("autoPowerSaverOnBattery"), onBattery, active, d.pp.profiles())
		if to == "" && !first && onAC && !prevAC {
			if saved := readPersistedProfile(); saved != "" && saved != active {
				to = saved
			}
		}
		if gameModeActive() {
			to = ""
		}
		if to != "" {
			if err := d.pp.setProfile(to); err != nil {
				log.Printf("ryoku-shell: auto power profile: set %q failed: %v", to, err)
			} else {
				active = to
			}
		}
		d.reconcileSaverExtras(context.Background(), onBattery, active)
		prevAC, first = onAC, false
	}

	reconcile()
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()
	var timerC <-chan time.Time
	if !eventsHealthy {
		timer.Reset(autoProfileReconcile)
		timerC = timer.C
	}
	disconnected := d.pp.conn.Context().Done()
	for {
		select {
		case <-d.quit:
			return
		case <-disconnected:
			disconnected = nil
			signals = nil
			eventsHealthy = false
		case sig, ok := <-signals:
			if !ok {
				signals = nil
				eventsHealthy = false
			} else if !powerStateSignal(sig) {
				continue
			}
		case <-d.saverExtrasKick:
		case <-timerC:
		}
		reconcile()
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timerC = nil
		if !eventsHealthy {
			timer.Reset(autoProfileReconcile)
			timerC = timer.C
		}
	}
}
