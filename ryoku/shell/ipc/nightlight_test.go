package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	wm "ryoku-wm"
)

func TestNightlightSavedTemp(t *testing.T) {
	dir := t.TempDir()
	n := &nightlightState{tempFile: filepath.Join(dir, "ryoku-nightlight")}

	if got := n.savedTemp(); got != nlDefaultTemp {
		t.Fatalf("missing file: got %d want default %d", got, nlDefaultTemp)
	}

	write := func(s string) {
		t.Helper()
		if err := os.WriteFile(n.tempFile, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("3500\n")
	if got := n.savedTemp(); got != 3500 {
		t.Fatalf("saved temp: got %d want 3500", got)
	}
	write("not-a-temp")
	if got := n.savedTemp(); got != nlDefaultTemp {
		t.Fatalf("unparseable: got %d want default", got)
	}
	write("0")
	if got := n.savedTemp(); got != nlDefaultTemp {
		t.Fatalf("zero: got %d want default", got)
	}
}

func TestNightlightFrameShape(t *testing.T) {
	dir := t.TempDir()
	topic := newStateTopic()
	sub := topic.subscribe()
	defer topic.unsubscribe(sub)

	n := &nightlightState{topic: topic, tempFile: filepath.Join(dir, "ryoku-nightlight")}
	if err := os.WriteFile(n.tempFile, []byte("4200\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	n.publish(true)

	frame := string(<-sub.frames)
	var got struct {
		On          bool `json:"on"`
		Temperature int  `json:"temperature"`
	}
	if err := json.Unmarshal([]byte(frame), &got); err != nil {
		t.Fatalf("frame not json: %v (%s)", err, frame)
	}
	if !got.On || got.Temperature != 4200 {
		t.Fatalf("frame: got %+v want {on:true temperature:4200}", got)
	}
}

func TestNightlightRegistration(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	d := &daemon{wmc: wm.OpenNamed("does-not-exist"), quit: make(chan struct{})}
	d.startNightlight()
	t.Cleanup(func() { close(d.quit); d.nightlight.workers.Wait() })

	if d.topic("nightlight") == nil {
		t.Fatal("nightlight topic not registered")
	}
	for _, m := range []string{"nightlight.toggle", "nightlight.set"} {
		if d.callHandler(m) == nil {
			t.Fatalf("call %s not registered", m)
		}
	}
	// The startup frame is published before the watcher goroutine settles, so
	// a subscriber never waits for the first event.
	sub := d.topic("nightlight").subscribe()
	defer d.topic("nightlight").unsubscribe(sub)
	frame := <-sub.frames
	if !strings.Contains(string(frame), `"on":false`) || !strings.Contains(string(frame), `"temperature":4000`) {
		t.Fatalf("startup frame: got %s want on:false temperature:4000", frame)
	}
}

func TestProcCommIsSelf(t *testing.T) {
	// The test binary's own comm is its file name (truncated to 15 chars),
	// never a backend name.
	self := strconv.Itoa(os.Getpid())
	if procCommIs(self, "wlsunset") {
		t.Fatal("test process matched wlsunset")
	}
	if !procCommIs(self, commOfSelf(t)) {
		t.Fatalf("own comm did not match its own comm file")
	}
	if procCommIs("nope", "x") {
		t.Fatal("non-numeric pid matched")
	}
}

// commOfSelf reads the test process's own comm for the positive case.
func commOfSelf(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("/proc/self/comm")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}
