package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	wm "ryoku-wm"
)

// writeNlFakeBackend writes the fake night-light backend: a script that stays
// alive under its own name for the requested seconds. Its comm is the script's
// basename, the name the watcher greps for. It is not a renamed copy of sleep:
// on uutils coreutils (Ubuntu 26.04 runners) sleep is a multicall binary that
// dispatches on argv[0], so a copy under another name exits at once.
func writeNlFakeBackend(t *testing.T, path string) {
	t.Helper()
	script := "#!/bin/sh\n" +
		"sleep \"${1:-600}\" &\n" +
		"child=$!\n" +
		"trap 'kill \"$child\" 2>/dev/null; exit 0' TERM INT HUP\n" +
		"wait \"$child\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestNightlightWatcherEndToEnd drives the real watcher: a fake backend binary,
// a fake provider that names it in caps, a PATH-shimmed ryoku-cmd-nightlight that
// starts/stops it and writes the marker and temp files, and the inotify loop
// publishing to a live topic subscription. It proves a toggle intent reaches QML
// as a pushed frame with no polling anywhere. The injected backend name is one no
// real process shares, so the watcher cannot see a real backend on the dev box.
func TestNightlightWatcherEndToEnd(t *testing.T) {
	const backend = "nlfakelight"

	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)

	bin := t.TempDir()
	fake := filepath.Join(bin, backend)
	writeNlFakeBackend(t, fake)

	// The daemon's caps probe runs ryoku-wm-<name> caps; this fake reports the
	// backend name the watcher must grep for, injected through the same wmc the
	// rest of the daemon reads.
	provider := filepath.Join(bin, "ryoku-wm-testwm")
	providerScript := "#!/usr/bin/env bash\n" +
		"[[ \"$1\" == caps ]] && printf '%s' " +
		"'{\"name\":\"testwm\",\"supports\":[\"nightLight\"],\"workspaceModel\":\"dynamic\",\"nightLightProcess\":\"" + backend + "\"}'\n" +
		"exit 0\n"
	if err := os.WriteFile(provider, []byte(providerScript), 0o755); err != nil {
		t.Fatal(err)
	}

	pidFile := filepath.Join(state, "fake.pid")
	marker := filepath.Join(state, "ryoku-nightlight-enabled")
	tempFile := filepath.Join(state, "ryoku-nightlight")
	shim := filepath.Join(bin, "ryoku-cmd-nightlight")
	script := strings.NewReplacer(
		"@PID@", pidFile, "@MARKER@", marker, "@TEMP@", tempFile, "@FAKE@", fake,
	).Replace(`#!/usr/bin/env bash
set -u
pid="@PID@"; marker="@MARKER@"; temp="@TEMP@"; fake="@FAKE@"
running() { [[ -f $pid ]] && kill -0 "$(cat "$pid")" 2>/dev/null; }
case "${1:-toggle}" in
  on)
    printf '%s\n' "${2:-4000}" >"$temp"; : >"$marker"
    running && exit 0
    setsid "$fake" 600 >/dev/null 2>&1 </dev/null & echo $! >"$pid"
    ;;
  off)
    running && kill "$(cat "$pid")" 2>/dev/null
    rm -f "$pid" "$marker"
    ;;
  toggle)
    if running; then "$0" off; else "$0" on "$(cat "$temp" 2>/dev/null || echo 4000)"; fi
    ;;
esac
exit 0
`)
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// The shim and the fake provider must win over the packaged binaries on PATH;
	// without this the test drives the real ones and the watcher never sees the
	// fake backend.
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	d := &daemon{wmc: wm.OpenNamed("testwm")}
	d.startNightlight()
	topic := d.topic("nightlight")
	sub := topic.subscribe()
	defer topic.unsubscribe(sub)
	t.Cleanup(func() {
		_, _ = d.callHandler("nightlight.set")(json.RawMessage(`{"on":false}`))
	})

	first := frameOn(t, <-sub.frames)
	if first.On {
		t.Fatalf("startup frame should be off: %s", first.Raw)
	}

	handler := d.callHandler("nightlight.toggle")
	if _, err := handler(json.RawMessage(`{}`)); err != nil {
		t.Fatalf("toggle on: %v", err)
	}
	got := waitForFrame(t, sub, func(f nlFrame) bool { return f.On }, "on:true")
	if got.Temperature != 4000 {
		t.Fatalf("after toggle: want temperature 4000, got %s", got.Raw)
	}

	// The script changes temperature by restarting the backend, so the watcher
	// may coalesce that into one frame; the settled state must carry the new
	// temperature with the light still on.
	set := d.callHandler("nightlight.set")
	if _, err := set(json.RawMessage(`{"on":true,"temperature":3500}`)); err != nil {
		t.Fatalf("set temp: %v", err)
	}
	if got := waitForFrame(t, sub, func(f nlFrame) bool { return f.On && f.Temperature == 3500 }, "on:true temperature:3500"); !got.On || got.Temperature != 3500 {
		t.Fatalf("after set: got %s", got.Raw)
	}

	if _, err := handler(json.RawMessage(`{}`)); err != nil {
		t.Fatalf("toggle off: %v", err)
	}
	if got := waitForFrame(t, sub, func(f nlFrame) bool { return !f.On }, "on:false"); got.On {
		t.Fatalf("after toggle off: want on:false, got %s", got.Raw)
	}
}

type nlFrame struct {
	On          bool
	Temperature int
	Raw         string
}

func frameOn(t *testing.T, b []byte) nlFrame {
	t.Helper()
	var f struct {
		On          bool `json:"on"`
		Temperature int  `json:"temperature"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("frame not json: %v (%s)", err, b)
	}
	return nlFrame{On: f.On, Temperature: f.Temperature, Raw: strings.TrimSpace(string(b))}
}

// waitForFrame reads frames until one satisfies want, failing the test after a
// bounded wait (the watcher settles within its poll timeout).
func waitForFrame(t *testing.T, sub *topicSubscriber, want func(nlFrame) bool, desc string) nlFrame {
	t.Helper()
	deadline := time.After(8 * time.Second)
	for {
		select {
		case f := <-sub.frames:
			got := frameOn(t, f)
			if want(got) {
				return got
			}
		case <-deadline:
			t.Fatalf("no frame with %s within 8s", desc)
		}
	}
}

// nlFake is the fake night-light environment: a backend binary, a provider that
// names it in caps, and a PATH-shimmed ryoku-cmd-nightlight, all laid ahead of
// the packaged binaries on PATH.
type nlFake struct {
	state   string
	marker  string
	temp    string
	backend string
	fake    string
	shim    string
}

// buildNlFake lays that environment into two temp dirs. The shim writes the
// enabled marker before it spawns the backend and then blocks until the backend
// is up, mirroring the real race window and the provider's act. capsFailFirst
// makes the provider's caps probe fail once, to exercise the lazy backend name.
func buildNlFake(t *testing.T, backend string, capsFailFirst bool) nlFake {
	t.Helper()
	state := t.TempDir()
	bin := t.TempDir()

	// The fake backend stays alive under <backend>, the name the watcher greps
	// for and the provider reports.
	fake := filepath.Join(bin, backend)
	writeNlFakeBackend(t, fake)

	fail := "0"
	if capsFailFirst {
		fail = "1"
	}
	provider := filepath.Join(bin, "ryoku-wm-testwm")
	provScript := strings.NewReplacer(
		"@BACKEND@", backend, "@COUNT@", filepath.Join(state, "caps-count"), "@FAIL@", fail,
	).Replace(`#!/usr/bin/env bash
set -u
[[ "${1:-}" == caps ]] || exit 0
if [[ "@FAIL@" == 1 ]]; then
  n=$(cat "@COUNT@" 2>/dev/null || echo 0)
  printf '%s\n' "$((n + 1))" >"@COUNT@"
  if (( n == 0 )); then echo "provider still coming up" >&2; exit 1; fi
fi
printf '%s' '{"name":"testwm","supports":["nightLight"],"nightLightProcess":"@BACKEND@"}'
`)
	if err := os.WriteFile(provider, []byte(provScript), 0o755); err != nil {
		t.Fatal(err)
	}

	pid := filepath.Join(state, "fake.pid")
	marker := filepath.Join(state, "ryoku-nightlight-enabled")
	temp := filepath.Join(state, "ryoku-nightlight")
	shim := filepath.Join(bin, "ryoku-cmd-nightlight")
	shimScript := strings.NewReplacer(
		"@PID@", pid, "@MARKER@", marker, "@TEMP@", temp, "@FAKE@", fake, "@NAME@", backend,
	).Replace(`#!/usr/bin/env bash
set -u
pid="@PID@"; marker="@MARKER@"; temp="@TEMP@"; fake="@FAKE@"; name="@NAME@"
up() { pgrep -x "$name" >/dev/null 2>&1; }
case "${1:-toggle}" in
  on)
    # The real script's on always re-asserts: the provider's nightlight.on
    # kills any running backend and spawns a fresh one. So a hotplug re-claim
    # is observable as a fresh pid, and a re-on keeps the saved temp.
    printf '%s\n' "${2:-$(cat "$temp" 2>/dev/null || echo 4000)}" >"$temp"
    : >"$marker"
    if [[ -f $pid ]]; then kill "$(cat "$pid")" 2>/dev/null; fi
    sleep 0.3
    setsid "$fake" 600 >/dev/null 2>&1 </dev/null & echo $! >"$pid"
    for _ in $(seq 1 200); do up && break; sleep 0.01; done
    ;;
  toggle)
    if up; then "$0" off; else "$0" on; fi
    ;;
  off)
    [[ -f $pid ]] && kill "$(cat "$pid")" 2>/dev/null
    rm -f "$pid" "$marker"
    ;;
esac
exit 0
`)
	if err := os.WriteFile(shim, []byte(shimScript), 0o755); err != nil {
		t.Fatal(err)
	}
	// The fakes must win over the packaged binaries on PATH.
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Cleanup(func() { _ = exec.Command(shim, "off").Run() })

	return nlFake{state: state, marker: marker, temp: temp, backend: backend, fake: fake, shim: shim}
}

// TestNightlightIntentPublishesTruthAfterSpawn pins the intent path: the script
// writes the enabled marker before the backend spawns, so a scan in that window
// would read off. Because run waits for the whole script (and the script for the
// backend), the intent publishes on:true the moment run returns, with no watcher
// involved: the only publisher here is the intent itself.
func TestNightlightIntentPublishesTruthAfterSpawn(t *testing.T) {
	f := buildNlFake(t, "nlfakeintent", false)

	topic := newStateTopic()
	sub := topic.subscribe()
	defer topic.unsubscribe(sub)
	n := &nightlightState{topic: topic, wmc: wm.OpenNamed("testwm"), stateDir: f.state, tempFile: f.temp}

	if err := n.intent("on", "4200"); err != nil {
		t.Fatalf("intent on: %v", err)
	}
	got := waitForFrame(t, sub, func(fr nlFrame) bool { return fr.On }, "on:true")
	if got.Temperature != 4200 {
		t.Fatalf("intent frame: got %s want on:true temperature:4200", got.Raw)
	}
}

// TestNightlightWatcherSettlesMarkerBeforeBackend pins the watcher path: a marker
// write lands before the backend exists, so the first /proc scan sees nothing.
// The watcher must not publish off on that first scan; it settles, rescanning
// until the late backend appears, and reports on:true well within its 2 s
// window. The old first-scan-wins watcher would leave the frame off until the
// 5 s poll re-check.
func TestNightlightWatcherSettlesMarkerBeforeBackend(t *testing.T) {
	f := buildNlFake(t, "nlfakewatch", false)

	topic := newStateTopic()
	sub := topic.subscribe()
	defer topic.unsubscribe(sub)
	n := &nightlightState{topic: topic, wmc: wm.OpenNamed("testwm"), stateDir: f.state, tempFile: f.temp}

	go n.watch()

	// The startup frame is off: no marker, no backend. Consume it so the marker
	// write below is a fresh inotify wake.
	if first := waitForFrame(t, sub, func(nlFrame) bool { return true }, "startup"); first.On {
		t.Fatalf("startup frame should be off: %s", first.Raw)
	}

	// Write the temp and the marker (the script's declared intent) with no
	// backend yet, then let it arrive late.
	if err := os.WriteFile(f.temp, []byte("4600\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	markerAt := time.Now()
	if err := os.WriteFile(f.marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	back := exec.Command(f.fake, "600")
	if err := back.Start(); err != nil {
		t.Fatalf("start late backend: %v", err)
	}
	t.Cleanup(func() { _ = back.Process.Kill(); _, _ = back.Process.Wait() })

	got := waitForFrame(t, sub, func(fr nlFrame) bool { return fr.On }, "on:true")
	if got.Temperature != 4600 {
		t.Fatalf("settled frame: got %s want on:true temperature:4600", got.Raw)
	}
	if elapsed := time.Since(markerAt); elapsed > 2500*time.Millisecond {
		t.Fatalf("watcher took %v to settle to on, want the 2s settle not the 5s poll", elapsed)
	}
}

// TestNightlightLazyBackendName pins the lazy backend resolution: a daemon whose
// caps probe failed first (the provider was not up yet) must still get a working
// intent once caps answers. The first intent refuses by name and leaves the
// frame off; the second, after the reprobe picks up the name, drives the script
// and publishes on:true.
func TestNightlightLazyBackendName(t *testing.T) {
	f := buildNlFake(t, "nlfakelazy", true) // caps fails on its first probe

	topic := newStateTopic()
	sub := topic.subscribe()
	defer topic.unsubscribe(sub)
	n := &nightlightState{topic: topic, wmc: wm.OpenNamed("testwm"), stateDir: f.state, tempFile: f.temp}

	if err := n.intent("on"); err != errNightlightUnavailable {
		t.Fatalf("first intent before caps answers: got %v want errNightlightUnavailable", err)
	}
	if got := waitForFrame(t, sub, func(nlFrame) bool { return true }, "off"); got.On {
		t.Fatalf("frame after a refused intent should be off: %s", got.Raw)
	}

	if err := n.intent("on", "4000"); err != nil {
		t.Fatalf("second intent after caps answers: %v", err)
	}
	if got := waitForFrame(t, sub, func(fr nlFrame) bool { return fr.On }, "on:true"); !got.On {
		t.Fatalf("frame after a working intent should be on: %s", got.Raw)
	}
}

// TestNightlightHotplugRearms pins the hotplug self-heal: with the light on,
// a connector set that moves re-runs the on intent, and because the provider
// replaces (kills and respawns) the backend on every on, the heal is
// observable as a fresh backend pid. The first set only primes (boot reports
// the steady set, not a change), and a frame whose set did not move never
// churns the backend.
func TestNightlightHotplugRearms(t *testing.T) {
	old := nlRearmGrace
	nlRearmGrace = 200 * time.Millisecond
	t.Cleanup(func() { nlRearmGrace = old })

	f := buildNlFake(t, "nlfakeplug", false)
	topic := newStateTopic()
	sub := topic.subscribe()
	defer topic.unsubscribe(sub)
	n := &nightlightState{topic: topic, wmc: wm.OpenNamed("testwm"), stateDir: f.state, tempFile: f.temp}

	if err := n.intent("on", "4100"); err != nil {
		t.Fatalf("intent on: %v", err)
	}
	waitForFrame(t, sub, func(fr nlFrame) bool { return fr.On }, "on:true")
	first, err := os.ReadFile(filepath.Join(f.state, "fake.pid"))
	if err != nil {
		t.Fatal(err)
	}

	// Prime the set: the first report is the boot state, not a change.
	n.rearmOnOutputs([]wm.Output{{Name: "eDP-1"}})
	// Dock: the set moves; the debounce schedules one re-claim.
	n.rearmOnOutputs([]wm.Output{{Name: "eDP-1"}, {Name: "DP-1"}})
	// An identical later frame must not churn the backend a second time.
	n.rearmOnOutputs([]wm.Output{{Name: "eDP-1"}, {Name: "DP-1"}})

	deadline := time.Now().Add(5 * time.Second)
	for {
		cur, err := os.ReadFile(filepath.Join(f.state, "fake.pid"))
		if err == nil && string(cur) != string(first) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("hotplug did not re-claim the gamma: backend pid unchanged")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !n.running() {
		t.Fatal("after the heal the light must still read on")
	}
	if got := n.savedTemp(); got != 4100 {
		t.Fatalf("the heal must keep the user's temperature: got %d want 4100", got)
	}
}

// TestNightlightHotplugIgnoredWhenOff pins the other half: with no enabled
// marker (the user left the light off), a moving connector set must never
// spawn the backend.
func TestNightlightHotplugIgnoredWhenOff(t *testing.T) {
	old := nlRearmGrace
	nlRearmGrace = 200 * time.Millisecond
	t.Cleanup(func() { nlRearmGrace = old })

	f := buildNlFake(t, "nlfakeplugoff", false)
	topic := newStateTopic()
	n := &nightlightState{topic: topic, wmc: wm.OpenNamed("testwm"), stateDir: f.state, tempFile: f.temp}

	n.rearmOnOutputs([]wm.Output{{Name: "eDP-1"}})
	n.rearmOnOutputs([]wm.Output{{Name: "eDP-1"}, {Name: "DP-1"}})
	time.Sleep(nlRearmGrace + 1500*time.Millisecond)
	if n.running() {
		t.Fatal("a hotplug must not turn the light on for a user who left it off")
	}
	if _, err := os.Stat(f.marker); err == nil {
		t.Fatal("a hotplug must not write the enabled marker")
	}
}

// TestNightlightHotplugAfterOffDoesNotRevive pins the interaction with an
// explicit off: a re-claim scheduled while the light was on must find the
// marker gone when it fires and stay inert.
func TestNightlightHotplugAfterOffDoesNotRevive(t *testing.T) {
	old := nlRearmGrace
	nlRearmGrace = 800 * time.Millisecond
	t.Cleanup(func() { nlRearmGrace = old })

	f := buildNlFake(t, "nlfakeplugstop", false)
	topic := newStateTopic()
	sub := topic.subscribe()
	defer topic.unsubscribe(sub)
	n := &nightlightState{topic: topic, wmc: wm.OpenNamed("testwm"), stateDir: f.state, tempFile: f.temp}

	if err := n.intent("on"); err != nil {
		t.Fatalf("intent on: %v", err)
	}
	waitForFrame(t, sub, func(fr nlFrame) bool { return fr.On }, "on:true")
	n.rearmOnOutputs([]wm.Output{{Name: "eDP-1"}})
	// The user turns the light off while the re-claim is still debouncing.
	if err := n.intent("off"); err != nil {
		t.Fatalf("intent off: %v", err)
	}
	n.rearmOnOutputs([]wm.Output{{Name: "eDP-1"}, {Name: "DP-1"}})

	if got := waitForFrame(t, sub, func(fr nlFrame) bool { return !fr.On }, "on:false"); got.On {
		t.Fatalf("frame after off: %s", got.Raw)
	}
	time.Sleep(nlRearmGrace + 1500*time.Millisecond)
	if n.running() {
		t.Fatal("a hotplug after an explicit off must not revive the backend")
	}
}
