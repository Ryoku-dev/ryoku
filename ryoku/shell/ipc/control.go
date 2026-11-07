package main

import (
	"os"
	"os/exec"
	"time"

	wm "ryoku-wm"
)

// control.go adds the reference control surface to the CLI over the existing
// socket transport: the menu, audio, brightness, lock-status, and hub verbs.
// Each mirrors one reference control method (menu <name> / close-all, audio
// volume-up/down/mute, brightness up/down, lock check, settings open/close).
// The volume and brightness steps are the reference literals: 5% of the [0,1]
// output-volume range and 5 of the [0,100] brightness range.

// audioArgv maps an audio subcommand to the wpctl invocation that carries it
// out: up/down step 5% of the sink volume, up clamps at 1.0 (100%) to match the
// reference's [0,1] clamp, and mute toggles. These match the media-key binds, so
// the keybind and the IPC verb drive one behaviour.
func audioArgv(sub string) ([]string, bool) {
	switch sub {
	case "up":
		return []string{"set-volume", "-l", "1", "@DEFAULT_AUDIO_SINK@", "5%+"}, true
	case "down":
		return []string{"set-volume", "@DEFAULT_AUDIO_SINK@", "5%-"}, true
	case "mute":
		return []string{"set-mute", "@DEFAULT_AUDIO_SINK@", "toggle"}, true
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
	if err := exec.Command("wpctl", argv...).Run(); err != nil {
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

// hubStartWait bounds how long a cold open waits for the fresh Hub to
// register its targets on the bus.
var hubStartWait = 10 * time.Second

// hubLaunch starts the single flock-guarded Hub and returns a wait for its
// exit. A package var so tests start a fake Hub instead of qs.
var hubLaunch = func() (func() error, error) {
	// -o, or the Hub's own children inherit the locked descriptor: one
	// detached grandchild (ryostore, a kitty, xdg-open) then held
	// /tmp/ryoku-hub.lock for the rest of the session and every later open
	// silently did nothing.
	argv := append([]string{"-n", "-o", "/tmp/ryoku-hub.lock", "qs"}, qsSelect("hub")...)
	cmd := exec.Command("flock", argv...)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd.Wait, nil
}

// hubCall runs one Hub IpcHandler call; true when the instance answered it.
// A Hub on the bus takes it as one cmd; one that is not still gets qs ipc.
func hubCall(target, fn string, args ...string) bool {
	_, err := ipcInvoke("hub", target, fn, "", args)
	return err == nil
}

// hubNav points a running Hub at a section; true when an instance answered.
func hubNav(section string) bool { return hubCall("nav", "open", section) }

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

// hubAlive: a running Hub serves nav on the bus, or, when it is not a bus
// client, answers its nav ipc to a single qs probe.
func hubAlive() bool {
	if busServing("hub", "nav") {
		return true
	}
	_, err := qsIpcOnce(ipcArgv("hub", "nav", "section"))
	return err == nil
}

// hubRaise brings the Hub window to the focused workspace. The Hub is the only
// floating org.quickshell client the shell spawns, so app-id targeting is
// unambiguous.
func (d *daemon) hubRaise() {
	_ = d.wmc.Act(wm.ActionAppFocus, "org.quickshell")
}

// hub opens or closes Ryoku Hub, which replaces the reference settings window.
// The Hub is one app: Bar Studio and every other page are sections inside it,
// reached by in-app navigation, never separate processes. Open on a running
// Hub navigates (when a section is named) and raises the window; open on a
// cold Hub spawns the single flock-guarded instance and then navigates.
// Close asks a running Hub to quit. All fire-and-forget, like the reference's
// void settings open/close.
//
// It serves `hub open [section]`, `hub open users <profile-id>` (the Users
// page opened on one profile) and `hub close`.
func (d *daemon) hub(args []string) string {
	const usage = "err hub: expected open [section], open users <profile-id>, or close"
	if len(args) == 0 || len(args) > 3 {
		return usage
	}
	switch args[0] {
	case "open":
		section := ""
		if len(args) >= 2 {
			section = args[1]
		}
		// target is the IpcHandler the open lands on; a cold Hub is ready for
		// it once that target is served.
		target := "nav"
		navigate := func() bool { return section == "" || hubNav(section) }
		if len(args) == 3 {
			if section != "users" {
				return usage
			}
			profile := args[2]
			target = "users"
			navigate = func() bool { return hubCall("users", "fromProfile", profile) }
		}
		if editorSection := hubEditorSection(section); editorSection != "" && len(args) == 2 {
			go ipcCallN("shell", "desktop", "editSection", editorSection, "")
			return "ok"
		}
		go func() {
			if hubAlive() {
				navigate()
				d.hubRaise()
				return
			}
			wait, err := hubLaunch()
			if err != nil {
				return
			}
			// The fresh instance registers its targets once its root loads. A
			// Hub that never does (not a bus client) still gets the call, which
			// then goes through qs ipc.
			if section != "" {
				busAwaitServing("hub", target, hubStartWait)
				navigate()
			}
			_ = wait()
		}()
		return "ok"
	case "close":
		if len(args) != 1 {
			return usage
		}
		go hubCall("hub", "close")
		return "ok"
	}
	return usage
}
