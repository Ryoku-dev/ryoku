package teams

import (
	"errors"
	"os"
	"os/exec"
	"sync"
)

// WakeReason is one reason to keep the shared wake unit on: oni-routines,
// which runs `hyakki routines wake` after login, a reboot or an idle exit.
type WakeReason string

// The reasons the wake unit is held on.
const (
	// WakeTeams is a team routine or heartbeat that is on.
	WakeTeams WakeReason = "teams"
	// WakeReminders is an Oni reminder or timer still to ring.
	WakeReminders WakeReason = "reminders"
)

// ErrNoWakeUnit means the wake unit cannot be armed here: a runit session
// without the Ryoku bridge has no way to enable it.
var ErrNoWakeUnit = errors.New("the wake unit cannot be armed here")

// WakeOwner is the one owner of the shared wake unit. Each reason holds it
// on or lets it go; the unit is on while any reason holds it and goes off
// only once every reason has let it go. The unit is set only when that
// changes, or when an earlier attempt to turn it on failed.
type WakeOwner struct {
	set func(on bool) error

	mu    sync.Mutex
	held  map[WakeReason]bool
	known bool
	on    bool
}

// NewWakeOwner returns an owner that turns the unit on or off with set.
func NewWakeOwner(set func(on bool) error) *WakeOwner {
	return &WakeOwner{set: set, held: map[WakeReason]bool{}}
}

// Hold holds the unit on for reason, or lets it go. While some reason holds
// it, the error is why the unit could not be turned on; the next hold tries
// again. Letting the last hold go never fails.
func (w *WakeOwner) Hold(reason WakeReason, on bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if on {
		w.held[reason] = true
	} else {
		delete(w.held, reason)
	}
	want := len(w.held) > 0
	if w.known && w.on == want {
		return nil
	}
	if err := w.set(want); err != nil && want {
		w.known = false
		return err
	}
	w.known, w.on = true, want
	return nil
}

// Held reports whether reason holds the unit on.
func (w *WakeOwner) Held(reason WakeReason) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.held[reason]
}

var wakeUnit = NewWakeOwner(setWakeUnits)

// holdWakeForTeams holds the wake unit on for the teams' routines, or lets
// it go; a team routine never fails for want of the unit.
func holdWakeForTeams(on bool) { _ = wakeUnit.Hold(WakeTeams, on) }

// HoldWakeForReminders holds this process's wake unit on for Oni's
// reminders, or lets it go, beside the teams' own hold.
func HoldWakeForReminders(on bool) error { return wakeUnit.Hold(WakeReminders, on) }

// setWakeUnits enables or disables the wake unit. The Ryoku bridge is
// optional: systemd sessions use systemctl without it; a runit session
// needs it, and reports ErrNoWakeUnit without it.
func setWakeUnits(on bool) error {
	verb := "disable"
	if on {
		verb = "enable"
	}
	host, hostErr := exec.LookPath("ryoku-host")
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		if hostErr == nil {
			return exec.Command(host, "svc", "--user", verb, "--now", wakeTimer).Run()
		}
		return exec.Command("systemctl", "--user", verb, "--now", wakeTimer).Run()
	}
	if hostErr != nil {
		return ErrNoWakeUnit
	}
	return exec.Command(host, "svc", "--user", verb, "--now", wakeRunit).Run()
}
