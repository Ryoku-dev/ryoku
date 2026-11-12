package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSoundsPatchValidatesCustomFiles(t *testing.T) {
	s := newTestStore(t)
	if err := s.patch("sounds.events.power-plug.file", rm(`"/home/me/plug.ogg"`)); err != nil {
		t.Fatalf("absolute .ogg rejected: %v", err)
	}
	if got := s.cur.Sounds.Events.PowerPlug.File; got != "/home/me/plug.ogg" {
		t.Fatalf("power-plug file = %q", got)
	}
	if err := s.patch("sounds.events.power-plug.file", rm(`"file:///home/me/plug.wav"`)); err != nil || s.cur.Sounds.Events.PowerPlug.File != "/home/me/plug.wav" {
		t.Fatalf("file:// URL not stored as a path: %v %q", err, s.cur.Sounds.Events.PowerPlug.File)
	}
	for _, bad := range []string{`"plug.ogg"`, `"/home/me/plug.txt"`} {
		if err := s.patch("sounds.events.power-plug.file", rm(bad)); err == nil {
			t.Fatalf("patch %s accepted, want rejected", bad)
		}
	}
	if err := s.patch("sounds.events.doorbell.enabled", rm(`true`)); err == nil {
		t.Fatal("unknown sound event accepted")
	}
	if err := s.patch("sounds.volume", rm(`150`)); err == nil {
		t.Fatal("volume 150 accepted")
	}
}

// A hand-edited file that names only some fields keeps the defaults of the rest,
// and a bad custom path clears instead of failing the whole file.
func TestSoundsLoadKeepsDefaultsAndClearsBadFile(t *testing.T) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(`{"sounds":{"events":{"notification":{"file":"relative.ogg"},"device-added":{"enabled":true}}}}`), &raw); err != nil {
		t.Fatal(err)
	}
	s, err := buildSettings(raw, false)
	if err != nil {
		t.Fatalf("lenient load failed: %v", err)
	}
	ev := s.Sounds.Events
	if !s.Sounds.Enabled || s.Sounds.Volume != 100 {
		t.Fatalf("master defaults lost: %+v", s.Sounds)
	}
	if !ev.Notification.Enabled || ev.Notification.File != "" {
		t.Fatalf("notification = %+v, want enabled with the built-in clip", ev.Notification)
	}
	if !ev.DeviceAdded.Enabled || ev.DeviceRemoved.Enabled || !ev.PowerPlug.Enabled {
		t.Fatalf("events = %+v", ev)
	}
}

func TestResolveSoundFallsBackToBuiltIn(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	custom := filepath.Join(t.TempDir(), "mine.ogg")
	if err := os.WriteFile(custom, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := resolveSound(soundPowerPlug, custom); !ok || got != custom {
		t.Fatalf("existing custom file = %q %v", got, ok)
	}
	got, ok := resolveSound(soundPowerPlug, filepath.Join(t.TempDir(), "gone.ogg"))
	if !ok || filepath.Base(got) != "power-plug.oga" {
		t.Fatalf("missing custom file played %q %v, want the built-in clip", got, ok)
	}
	for _, name := range soundNames {
		if _, err := soundAssets.ReadFile("assets/" + name + ".oga"); err != nil {
			t.Errorf("no built-in clip for %s", name)
		}
	}
}

// The shell's notification server and the Stage service both ask for their
// chime through the `sound` verb, so the master switch and each cue's own
// switch in Settings > Desktop > Sounds have to hold there.
func TestSoundVerbHonoursHubSwitches(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	bin := t.TempDir()
	played := filepath.Join(t.TempDir(), "played")
	shim := "#!/bin/sh\nfor f; do :; done\nprintf '%s\\n' \"${f##*/}\" >> '" + played + "'\n"
	if err := os.WriteFile(filepath.Join(bin, "pw-play"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	// Another test's cached clips sit in its removed temp dir, and its last
	// plays would hold these cues inside their gap.
	forget := func() {
		soundMu.Lock()
		soundReady, soundLast = map[string]string{}, map[string]time.Time{}
		soundMu.Unlock()
	}
	forget()
	t.Cleanup(forget)

	s := newTestStore(t)
	d := &daemon{settings: s}
	set := func(path, value string) {
		t.Helper()
		if err := s.patch(path, rm(value)); err != nil {
			t.Fatalf("patch %s: %v", path, err)
		}
	}
	ask := func(events ...string) {
		t.Helper()
		for _, ev := range events {
			if got := d.dispatch("sound " + ev); got != "ok" {
				t.Fatalf("sound %s = %q, want ok", ev, got)
			}
		}
	}
	// Players run in the background: wait for the plays expected so far, then
	// a little longer so one that should not have happened shows up too.
	plays := func(n int) []string {
		t.Helper()
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			if len(strings.Fields(readTrim(played))) >= n {
				break
			}
		}
		time.Sleep(150 * time.Millisecond)
		return strings.Fields(readTrim(played))
	}

	set("sounds.enabled", `false`)
	ask(soundNotification, soundNotificationCritical)
	set("sounds.enabled", `true`)
	ask(soundShutter)
	if got := plays(1); len(got) != 1 || got[0] != "shutter.oga" {
		t.Fatalf("with the master switch off, then the shutter: played %v, want only the shutter", got)
	}

	set("sounds.events.notification.enabled", `false`)
	ask(soundNotification, soundNotificationCritical)
	if got := plays(2); len(got) != 2 || got[1] != "notification-critical.oga" {
		t.Fatalf("with the notification cue off: played %v, want only the urgent cue added", got)
	}

	set("sounds.events.notification.enabled", `true`)
	ask(soundNotification)
	if got := plays(3); len(got) != 3 || got[2] != "notification.oga" {
		t.Fatalf("with the notification cue back on: played %v", got)
	}
}

func TestPowerCueBatteryFull(t *testing.T) {
	var c powerCue
	now := time.Unix(0, 0)
	charging := powerState{haveAC: true, online: true, present: true, percent: 98}
	full := powerState{haveAC: true, online: true, present: true, full: true, percent: 100}
	if cues := c.step(full, now); len(cues) != 0 {
		t.Fatalf("booting full played %v, want none", cues)
	}
	c = powerCue{}
	c.step(charging, now)
	if cues := c.step(full, now); len(cues) != 1 || cues[0] != soundBatteryFull {
		t.Fatalf("reaching full played %v, want [battery-full]", cues)
	}
	if cues := c.step(full, now); len(cues) != 0 {
		t.Fatalf("staying full played %v, want none", cues)
	}
	// A battery that reports Full on the cable but not after unplugging stays quiet.
	if cues := c.step(powerState{haveAC: true, online: false, present: true, full: true, percent: 100}, now); len(cues) != 1 || cues[0] != soundPowerUnplug {
		t.Fatalf("unplugging while full played %v, want [power-unplug]", cues)
	}
}

func TestUSBDevicesSkipsHubsAndInterfaces(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"usb1", "1-0:1.0", "1-2", "1-2:1.0", "3-1.4"} {
		if err := os.Mkdir(filepath.Join(root, n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := usbDevices(root)
	if len(got) != 2 || !got["1-2"] || !got["3-1.4"] {
		t.Fatalf("usbDevices = %v, want 1-2 and 3-1.4", got)
	}
}
