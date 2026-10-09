package main

import (
	"errors"
	"testing"
	"time"
)

func TestSessionActionMethod(t *testing.T) {
	want := map[string]string{
		"reboot":   "Reboot",
		"shutdown": "PowerOff",
	}
	for action, method := range want {
		got, ok := sessionActionMethod(action)
		if !ok {
			t.Fatalf("sessionActionMethod(%q) missing", action)
		}
		if got != method {
			t.Errorf("sessionActionMethod(%q) = %q, want %q", action, got, method)
		}
	}
	for _, absent := range []string{"logout", "suspend", "hibernate", "", "poweroff"} {
		if _, ok := sessionActionMethod(absent); ok {
			t.Errorf("sessionActionMethod(%q) exists; only reboot and shutdown are login1 actions", absent)
		}
	}
}

func TestRunSessionActionCallsLogin1(t *testing.T) {
	old := login1PowerCall
	t.Cleanup(func() { login1PowerCall = old })

	called := make(chan string, 1)
	login1PowerCall = func(method string) error {
		called <- method
		return errors.New("test rejection")
	}
	runSessionAction("reboot")
	select {
	case method := <-called:
		if method != "Reboot" {
			t.Fatalf("login1 method = %q, want Reboot", method)
		}
	case <-time.After(time.Second):
		t.Fatal("session action did not call login1")
	}
}

// startSession must register exactly the three calls, so QML's confirmation
// dialog can reach each action and nothing else.
func TestStartSessionRegistersCalls(t *testing.T) {
	d := &daemon{}
	d.startSession()
	for _, action := range []string{"logout", "reboot", "shutdown"} {
		if d.callHandler("session."+action) == nil {
			t.Errorf("session.%s call not registered", action)
		}
	}
	if d.callHandler("session.suspend") != nil {
		t.Error("session.suspend registered; no suspend action exists")
	}
}
