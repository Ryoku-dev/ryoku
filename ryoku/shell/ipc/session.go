package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/godbus/dbus/v5"

	wm "ryoku-wm"
)

// session.go runs the session power actions the confirmation dialog triggers.
// Reboot and shutdown go through login1, which is provided by both logind and
// elogind. Logout ends the session through the wm seam.
//
// There is deliberately no suspend action: the reference tree has none.
var sessionActions = map[string]string{
	"reboot":   "Reboot",
	"shutdown": "PowerOff",
}

func sessionActionMethod(action string) (string, bool) {
	method, ok := sessionActions[action]
	return method, ok
}

var login1PowerCall = callLogin1Power

func callLogin1Power(method string) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return fmt.Errorf("connect to the system bus: %w", err)
	}
	defer conn.Close()
	return conn.Object(login1Bus, login1Path).
		Call(login1Interface+"."+method, 0, true).Err
}

// startSession registers the session power-action calls the confirmation dialog
// invokes. Registration only: no process is spawned here.
func (d *daemon) startSession() {
	d.registerCall("session.logout", func(json.RawMessage) (any, error) {
		if err := d.wmc.Act(wm.ActionSessionExit); err != nil {
			log.Printf("ryoku-shell: session logout: %v", err)
		}
		return map[string]any{"ok": true}, nil
	})
	for action := range sessionActions {
		action := action
		d.registerCall("session."+action, func(json.RawMessage) (any, error) {
			runSessionAction(action)
			return map[string]any{"ok": true}, nil
		})
	}
}

// runSessionAction sends the login1 request in the background so a slow system
// bus never blocks the caller.
func runSessionAction(action string) {
	method, ok := sessionActionMethod(action)
	if !ok {
		log.Printf("ryoku-shell: unknown session action %q", action)
		return
	}
	go func() {
		if err := login1PowerCall(method); err != nil {
			log.Printf("ryoku-shell: session %s: %v", action, err)
		}
	}()
}
