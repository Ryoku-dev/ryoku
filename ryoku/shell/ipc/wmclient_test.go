package main

import (
	"encoding/json"
	"os/exec"
	"testing"
	"time"

	wm "ryoku-wm"
)

// A focus frame warms the cache; a focus frame with an empty output clears it.
func TestFocusFrameUpdatesCache(t *testing.T) {
	d := &daemon{}
	d.onWMFrame(wm.Frame{Kind: wm.FrameFocus, FocusedOutput: "DP-1"})
	if got := d.activeMonitor(); got != "DP-1" {
		t.Fatalf("activeMonitor() = %q, want DP-1", got)
	}
	d.onWMFrame(wm.Frame{Kind: wm.FrameFocus})
	if got := d.activeMonitor(); got != "" {
		t.Fatalf("after empty focus frame, activeMonitor() = %q, want empty", got)
	}
}

// A cold cache resolves to "" without a fork; callers tolerate no focused output.
func TestActiveMonitorColdCacheEmpty(t *testing.T) {
	d := &daemon{}
	if got := d.activeMonitor(); got != "" {
		t.Fatalf("cold activeMonitor() = %q, want empty", got)
	}
}

// The watcher writes while keybinds read; run with -race to prove it is safe.
func TestMonitorConcurrent(t *testing.T) {
	d := &daemon{}
	done := make(chan struct{})
	go func() {
		for range 2000 {
			d.onWMFrame(wm.Frame{Kind: wm.FrameFocus, FocusedOutput: "DP-1"})
			d.onWMFrame(wm.Frame{Kind: wm.FrameFocus})
		}
		close(done)
	}()
	for range 2000 {
		_ = d.activeMonitor()
	}
	<-done
}

// An overview frame warms the cached overview state the wm topic publishes, so
// the backdrop's blur gate reads the compositor's live overview, not a stale one.
func TestOverviewFrameUpdatesCache(t *testing.T) {
	d := &daemon{}
	d.onWMFrame(wm.Frame{Kind: wm.FrameOverview, OverviewOpen: true})
	d.wmMu.Lock()
	got := d.wmOverview
	d.wmMu.Unlock()
	if !got {
		t.Fatal("overview frame did not set the cached state")
	}
	d.onWMFrame(wm.Frame{Kind: wm.FrameOverview})
	d.wmMu.Lock()
	got = d.wmOverview
	d.wmMu.Unlock()
	if got {
		t.Fatal("a closed overview frame must clear the cached state")
	}
}

// The bar's layout indicator reads Wm.keyboardLayout off the wm topic, so a
// keyboard frame must reach the published snapshot (it was silently dropped
// once) and every section must carry a version so a QML consumer rebinds only
// what moved. This asserts the wire, not just the cache.
func TestPublishCarriesKeyboardAndVersions(t *testing.T) {
	d := &daemon{wmc: wm.OpenNamed("does-not-exist")}
	d.wmTopic = newStateTopic()
	sub := d.wmTopic.subscribe()
	defer d.wmTopic.unsubscribe(sub)

	d.onWMFrame(wm.Frame{Kind: wm.FrameKeyboard, KeyboardLayout: "English (US)", KeyboardLayouts: []string{"us", "dvorak"}})

	select {
	case frame := <-sub.frames:
		var out wmTopicFrame
		if err := json.Unmarshal(frame, &out); err != nil {
			t.Fatal(err)
		}
		if out.KeyboardLayout != "English (US)" {
			t.Errorf("keyboardLayout = %q, want the folded layout", out.KeyboardLayout)
		}
		if len(out.KeyboardLayouts) != 2 {
			t.Errorf("keyboardLayouts = %v, want two", out.KeyboardLayouts)
		}
		if out.Versions["keyboard"] != 1 {
			t.Errorf("versions[keyboard] = %d, want 1", out.Versions["keyboard"])
		}
		if out.Versions["windows"] != 0 {
			t.Errorf("versions[windows] = %d, want 0 (untouched by a keyboard frame)", out.Versions["windows"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no frame published")
	}
}

// The frame is the only path a capability has to a QML consumer, and consumers
// read Wm.caps.<name> === true. A capability that exists but is missing from
// the map reads as absent, so the affordance gated on it silently disappears.
// With no provider installed every Has is false, which is exactly what makes
// the key-presence check the real assertion here.
func TestPublishCarriesEveryCapability(t *testing.T) {
	d := &daemon{wmc: wm.OpenNamed("does-not-exist")}
	d.wmTopic = newStateTopic()
	sub := d.wmTopic.subscribe()
	defer d.wmTopic.unsubscribe(sub)

	d.onWMFrame(wm.Frame{Kind: wm.FrameReady})

	select {
	case frame := <-sub.frames:
		var out wmTopicFrame
		if err := json.Unmarshal(frame, &out); err != nil {
			t.Fatal(err)
		}
		for _, c := range wm.All() {
			if _, ok := out.Caps[c]; !ok {
				t.Errorf("caps frame omits %q, so a consumer cannot gate on it", c)
			}
		}
		if got := out.Caps[wm.CapPersistentScreenCapture]; got {
			t.Error("a missing provider must not report persistent screen capture")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no frame published")
	}
}

func TestFocusUsesCompactTopic(t *testing.T) {
	d := &daemon{wmc: wm.OpenNamed("does-not-exist")}
	d.wmTopic = newStateTopic()
	d.wmFocusTopic = newStateTopic()
	full := d.wmTopic.subscribe()
	focus := d.wmFocusTopic.subscribe()
	defer d.wmTopic.unsubscribe(full)
	defer d.wmFocusTopic.unsubscribe(focus)

	d.onWMFrame(wm.Frame{Kind: wm.FrameFocus, FocusedOutput: "DP-1"})

	select {
	case frame := <-focus.frames:
		var out wmFocusFrame
		if err := json.Unmarshal(frame, &out); err != nil {
			t.Fatal(err)
		}
		if out.FocusedOutput != "DP-1" || out.Version != 1 {
			t.Fatalf("focus frame = %+v, want DP-1 at version 1", out)
		}
		if len(frame) >= 128 {
			t.Fatalf("focus frame is %d bytes; compact topic unexpectedly carries snapshot state", len(frame))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no compact focus frame published")
	}
	select {
	case frame := <-full.frames:
		t.Fatalf("focus change woke full wm topic with %d bytes", len(frame))
	default:
	}

	d.onWMFrame(wm.Frame{Kind: wm.FrameFocus, FocusedOutput: "DP-1"})
	select {
	case frame := <-focus.frames:
		t.Fatalf("duplicate focus published %q", frame)
	default:
	}
}

func BenchmarkActiveMonitorCached(b *testing.B) {
	d := &daemon{}
	d.onWMFrame(wm.Frame{Kind: wm.FrameFocus, FocusedOutput: "DP-1"})
	b.ReportAllocs()
	for b.Loop() {
		if d.activeMonitor() == "" {
			b.Fatal("empty monitor")
		}
	}
}

// BenchmarkMonitorSpawnProxy measures a bare fork+exec: the per-keybind floor the
// cache removes. A provider query is strictly more expensive than this.
func BenchmarkMonitorSpawnProxy(b *testing.B) {
	if _, err := exec.LookPath("true"); err != nil {
		b.Skip("true not on PATH")
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = exec.Command("true").Run()
	}
}
