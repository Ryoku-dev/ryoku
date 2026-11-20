package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	wm "ryoku-wm"
)

// control.go adds the reference control surface to the CLI over the existing
// socket transport: the menu, bar, audio, brightness, lock-status, and hub
// verbs. Each mirrors one reference control method (menu <name> / close-all,
// per-edge bar toggle and the toggle/reveal/hide-all trio, audio
// volume-up/down/mute, brightness up/down, lock check, settings open/close).
// Brightness keeps its reference step of 5 on the [0,100] range. Output volume
// uses the shared ryoku-volume helper so every surface reads the same setting.

// barEdges are the four frame edges plus "all". The reference offered per-edge
// toggles and a toggle/reveal/hide-all trio whose -x flag excluded bars marked
// hidden-by-default; that flag's meaning was inverted from its name (with -x set
// it acted ONLY on reveal-by-default bars). Ryoku drops the flag for an explicit
// edge list, so "all" is every edge (the reference's default, non-excluding
// path) and a named edge is just that one.
var barEdges = map[string]bool{"top": true, "bottom": true, "left": true, "right": true, "all": true}

// barActions are the three reveal operations a bar edge accepts.
var barActions = map[string]bool{"toggle": true, "reveal": true, "hide": true}

// parseBarEdge validates a `bar <edge> <action>` pair.
func parseBarEdge(args []string) (edge, action string, ok bool) {
	if len(args) != 2 || !barEdges[args[0]] || !barActions[args[1]] {
		return "", "", false
	}
	return args[0], args[1], true
}

// audioArgv validates audio subcommands and returns the shared command for each.
// Volume steps go through ryoku-volume so every surface reads the same setting.
func audioArgv(sub string) ([]string, bool) {
	switch sub {
	case "up":
		return []string{"ryoku-volume", "up"}, true
	case "down":
		return []string{"ryoku-volume", "down"}, true
	case "mute":
		return []string{"wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "toggle"}, true
	}
	return nil, false
}

// brightnessArgv maps a brightness subcommand to a ryoku-cmd-brightness step.
// The reference stepped brightness by 5 on a 0..100 scale (5%); the helper
// drives both the laptop backlight and any DDC/CI external monitor at once.
func brightnessArgv(sub string) ([]string, bool) {
	switch sub {
	case "up":
		return []string{"+5"}, true
	case "down":
		return []string{"-5"}, true
	}
	return nil, false
}

// audio runs a volume change. The OSD flashes on its own when the sink volume
// moves, so the verb only performs the change; that OSD flash is the reference's
// documented side effect.
func (d *daemon) audio(sub string) string {
	argv, ok := audioArgv(sub)
	if !ok {
		return "err audio: expected up, down, or mute"
	}
	// The output cue fires for every up/down/mute, even when the sink change
	// itself fails; input volume/mute have no verb here and stay silent.
	d.playSound(soundVolumeChange)
	if err := exec.Command(argv[0], argv[1:]...).Run(); err != nil {
		return "err audio: " + err.Error()
	}
	return "ok"
}

// setAudioStep persists the shared volume increment through the settings
// daemon, the sole writer of shell.json. It changes only future volume steps.
func (d *daemon) setAudioStep(raw string) string {
	step, err := strconv.Atoi(raw)
	if err != nil || step < 1 || step > 20 {
		return "err audio: volume step must be an integer from 1 to 20"
	}
	if d.settings == nil {
		return "err audio: settings are not ready"
	}
	value, _ := json.Marshal(step)
	if err := d.settings.patch("audio.volumeStep", value); err != nil {
		return "err audio: " + err.Error()
	}
	return "ok"
}

// brightness runs a backlight change through the shared brightness helper.
func (d *daemon) brightness(sub string) string {
	argv, ok := brightnessArgv(sub)
	if !ok {
		return "err brightness: expected up or down"
	}
	if err := exec.Command("ryoku-cmd-brightness", argv...).Run(); err != nil {
		return "err brightness: " + err.Error()
	}
	return "ok"
}

// isLocked accepts only generation- and session-bound compositor proof. A
// marker from a killed or different-session locker is stale and removed.
func isLocked() bool {
	marker := lockMarker()
	if _, err := os.Stat(marker); err != nil {
		return false
	}
	if !lockProofValid() {
		_ = os.Remove(marker)
		return false
	}
	return true
}

// menuClose closes every open menu on the active monitor (reference close-all).
func (d *daemon) menuClose() string {
	d.ensure("shell")
	return shellIpc("closeAllMenus", d.activeMonitor())
}

// barToggle drives the per-edge bar reveal state (reference bar toggle/reveal/
// hide, per edge or across all).
//
// ryoku: the shell exposes no setBar IPC function. Its bar reveal is a single
// whole-bar toggle driven in-process by the global:ryoku:barToggle keybind
// (CustomShortcut -> ShellState.barRevealed), with no per-edge reveal/hide
// grammar. This CLI verb still emits setBar on the shell target, so it is a
// no-op pending the retire phase (add a shell IPC fn there, or drop the verb).
// Do NOT invent a shell IPC function here.
func (d *daemon) barToggle(edge, action string) string {
	d.ensure("shell")
	return shellIpc("setBar", d.activeMonitor(), edge, action)
}

// hub opens or closes Ryoku Hub, which replaces the reference settings window.
// The Hub is one app: Bar Studio and every other page are sections inside it,
// reached by in-app navigation, never separate processes. Open on a running
// Hub navigates (when a section is named) and raises the window; open on a
// cold Hub spawns the single flock-guarded instance and then navigates.
// Close asks a running Hub to quit. All fire-and-forget, like the reference's
// void settings open/close.
func hubSelect() []string {
	if shellDir != "" {
		return []string{"-p", filepath.Join(shellDir, "..", "hub", "quickshell")}
	}
	return []string{"-c", "hub"}
}

// hubNav points a running Hub at a section; true when an instance answered.
func hubNav(section string) bool {
	argv := append(hubSelect(), "ipc", "call", "nav", "open", section)
	return exec.Command("qs", argv...).Run() == nil
}

// Retired Hub links now name their owning Stage Editor catalogue.
func hubEditorSection(section string) string {
	switch section {
	case "desktop-scene", "stage":
		return "depth"
	case "desktop-scene-visualizer", "visualizer":
		return "visualizer"
	case "desktop-scene-widgets", "widgets":
		return "widgets"
	}
	return ""
}

// hubAlive: a running Hub answers its nav ipc.
func hubAlive() bool {
	argv := append(hubSelect(), "ipc", "call", "nav", "section")
	return exec.Command("qs", argv...).Run() == nil
}

// hubRaise brings the Hub window to the focused workspace. The Hub is the only
// floating org.quickshell client the shell spawns, so app-id targeting is
// unambiguous.
func (d *daemon) hubRaise() {
	_ = d.wmc.Act(wm.ActionAppFocus, "org.quickshell")
}

func (d *daemon) hub(sub, section string) string {
	switch sub {
	case "open":
		if editorSection := hubEditorSection(section); editorSection != "" {
			go ipcCallN("shell", "desktop", "editSection", editorSection, "")
			return "ok"
		}
		go func() {
			if hubAlive() {
				if section != "" {
					hubNav(section)
				}
				d.hubRaise()
				return
			}
			// -o, or the Hub's own children inherit the locked descriptor:
			// one detached grandchild (ryostore, a kitty, xdg-open) then held
			// /tmp/ryoku-hub.lock for the rest of the session and every later
			// open silently did nothing.
			argv := append([]string{"-n", "-o", "/tmp/ryoku-hub.lock", "qs"}, hubSelect()...)
			cmd := exec.Command("flock", argv...)
			if err := cmd.Start(); err != nil {
				return
			}
			if section != "" {
				// The fresh instance answers IPC once its root loads; nudge the
				// section as soon as it does.
				for range 40 {
					time.Sleep(250 * time.Millisecond)
					if hubNav(section) {
						break
					}
				}
			}
			_ = cmd.Wait()
		}()
		return "ok"
	case "close":
		argv := append(hubSelect(), "ipc", "call", "hub", "close")
		go func() { _ = exec.Command("qs", argv...).Run() }()
		return "ok"
	}
	return "err hub: expected open [section] or close"
}
