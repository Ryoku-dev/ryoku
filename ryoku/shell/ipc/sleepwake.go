package main

import (
	"log"
	"time"

	"github.com/godbus/dbus/v5"

	wm "ryoku-wm"
)

// sleepwake.go keeps the panel lit across suspend. On a hybrid laptop the
// display is wired to one GPU by the hardware MUX; when that GPU is the
// discrete one, waking from s2idle re-trains the panel asynchronously, and a
// single "power the outputs back on" can land before the driver has finished
// and be dropped. The only thing that used to restore DPMS on resume was the
// idle daemon's after_sleep_cmd, which never fires when idle timeouts are
// switched off -- and then a lid-close comes back to a black screen with a
// live session behind it.
//
// So the shell daemon watches logind's PrepareForSleep over the system bus and
// holds the outputs awake itself for a short window after every wake, through
// the compositor seam (output.power), the same path every surface uses. It is
// compositor-neutral and independent of the idle policy: a box with idle
// disabled is exactly the box that had no wake handler at all.
//
// The window is a re-assert loop, not a one-shot: each pass powers the outputs
// on, so a pass that raced the driver's re-train is covered by the next. It
// runs even while the session is locked: before_sleep_cmd locks before
// suspend, so a wake that skipped a locked session would leave the user
// facing a black screen they cannot even authenticate against. qylock draws
// its own surface and never touches DPMS, so powering the panel on is safe
// and makes the lock visible.
var (
	// wakeHold is how long the daemon keeps re-asserting the panel after a
	// wake. The measured hardware can drop a DPMS-on that lands while the
	// driver is still re-training the panel (a shallow seconds-long suspend
	// re-trains LATE and swallowed a 6 s window), so the hold runs well past
	// the slowest observed re-train. The act is idempotent and cheap.
	wakeHold = 15 * time.Second
	// wakeStep is the re-assert cadence.
	wakeStep = time.Second
)

// startSleepWake subscribes to logind's PrepareForSleep. A missing system bus
// or logind disables the guard without failing the daemon; the idle daemon's
// after_sleep_cmd remains as a second writer for the configured case.
func (d *daemon) startSleepWake() {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Printf("ryoku-shell: sleep wake guard disabled: %v", err)
		return
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.login1"),
		dbus.WithMatchObjectPath("/org/freedesktop/login1"),
		dbus.WithMatchInterface("org.freedesktop.login1.Manager"),
		dbus.WithMatchMember("PrepareForSleep"),
	); err != nil {
		log.Printf("ryoku-shell: sleep wake match failed: %v", err)
		return
	}
	sigs := make(chan *dbus.Signal, 4)
	conn.Signal(sigs)
	go func() {
		defer conn.Close()
		for {
			select {
			case <-d.quit:
				return
			case sig := <-sigs:
				// PrepareForSleep carries one boolean: true = going to sleep,
				// false = waking. Only the wake edge needs the guard.
				if len(sig.Body) < 1 {
					continue
				}
				if sleeping, _ := sig.Body[0].(bool); !sleeping {
					go d.holdAwake()
				}
			}
		}
	}()
}

// holdAwake re-asserts the outputs on for wakeHold, so a panel the compositor
// dropped across suspend (or a DPMS-on that raced the driver's re-train) comes
// back without the idle daemon's after_sleep_cmd.
func (d *daemon) holdAwake() {
	if !d.wmc.Can(wm.CapOutputPower) {
		return
	}
	deadline := time.Now().Add(wakeHold)
	for time.Now().Before(deadline) {
		select {
		case <-d.quit:
			return
		case <-time.After(wakeStep):
		}
		// A provider that lost the seam (compositor restarting) ends the hold
		// early; the idle daemon's hook still covers the configured case.
		if err := d.wmc.Act(wm.ActionOutputPower, "on"); err != nil {
			return
		}
	}
}
