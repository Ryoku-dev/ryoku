package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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

var hubStartWait = 10 * time.Second

// hubLockBusy is flock's exit code when another open already holds the Hub's lock
const hubLockBusy = 75

// errHubStarting: this launch found another open already starting the one Hub
var errHubStarting = errors.New("another open is already starting Settings")

// a package var so tests start a fake Hub instead of qs; start is the page a cold
// launch opens first (RYOKU_HUB_START), so Home is never built for a deep link
var hubLaunch = func(screenScales, start string) (func() error, error) {
	// -o, or the Hub's own children inherit the locked descriptor: one
	// detached grandchild (ryostore, a kitty, xdg-open) then held
	// /tmp/ryoku-hub.lock for the rest of the session and every later open
	// silently did nothing.
	argv := append([]string{"-n", "-E", strconv.Itoa(hubLockBusy), "-o", "/tmp/ryoku-hub.lock", "qs"}, qsSelect("hub")...)
	cmd := exec.Command("flock", argv...)
	cmd.Env = os.Environ()
	if screenScales != "" {
		cmd.Env = append(cmd.Env, "QT_SCREEN_SCALE_FACTORS="+screenScales)
	}
	if start != "" {
		cmd.Env = append(cmd.Env, "RYOKU_HUB_START="+start)
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return func() error {
		err := cmd.Wait()
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == hubLockBusy {
			return errHubStarting
		}
		return err
	}, nil
}

// hubScreenScales turns displays.ui_scale into Qt's per-screen factors: the
// Hub draws with Lithic's fixed sizes, and Qt's own scaling keeps native text
// sharp where scaling the window's items would blur it
func hubScreenScales(settings map[string]any) string {
	v, err := getByPath(settings, []string{"displays", "ui_scale"})
	if err != nil {
		return ""
	}
	scales, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	names := make([]string, 0, len(scales))
	for name := range scales {
		names = append(names, name)
	}
	sort.Strings(names)
	var parts []string
	for _, name := range names {
		f, ok := scales[name].(float64)
		if !ok || !(f > 0) || name == "" || strings.ContainsAny(name, "=;") {
			continue
		}
		// the shell clamps a surface's scale to the same range
		f = math.Max(0.5, math.Min(2, f))
		if f == 1 {
			continue
		}
		parts = append(parts, name+"="+strconv.FormatFloat(f, 'f', -1, 64))
	}
	return strings.Join(parts, ";")
}

func hubCall(target, fn string, args ...string) bool {
	_, err := ipcInvoke("hub", target, fn, "", args)
	return err == nil
}

// hubNav points a running Hub at a page id, alias or "page/row"; true when an
// instance answered.
func hubNav(section string) bool { return hubCall("nav", "open", section) }

// Retired Settings links that now name their owning Stage Editor catalogue.
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

// hub opens or closes Ryoku Settings. Settings is one app: every page is
// reached by in-app navigation, never a separate process. The section is a
// page id, an old section name the Hub's alias table resolves, or "page/row".
// Open on a running instance navigates (when a page is named) and raises the
// window; open on a cold one spawns the single flock-guarded instance and
// then navigates. Close asks a running instance to quit. All fire-and-forget,
// like the reference's void settings open/close.
func (d *daemon) hub(args []string) string {
	const usage = "err hub: expected open [page[/row]], open users <profile-id>, or close"
	if len(args) == 0 || len(args) > 3 {
		return usage
	}
	switch args[0] {
	case "open":
		section := ""
		if len(args) >= 2 {
			section = args[1]
		}
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
			// a page opens with the process; a users profile still goes over the bus
			start := ""
			if target == "nav" {
				start = section
			}
			wait, err := hubLaunch(hubScreenScales(d.settingsFrame()), start)
			if err != nil {
				return
			}
			// a Hub that never joins the bus still gets the call through qs ipc
			if target != "nav" && section != "" {
				busAwaitServing("hub", target, hubStartWait)
				navigate()
			}
			// an open racing another cold open loses the lock; its page goes to the
			// instance the other one started, once that serves
			if errors.Is(wait(), errHubStarting) && target == "nav" && busAwaitServing("hub", "nav", hubStartWait) {
				navigate()
				d.hubRaise()
			}
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

// directHub runs `hub open` and `hub close` with the daemon down, so a stopped
// daemon never locks you out of Settings: a running instance is reached over qs
// ipc, else the one flock-guarded instance starts on the page
func directHub(args []string) error {
	if len(args) == 1 && args[0] == "close" {
		hubCall("hub", "close")
		return nil
	}
	if len(args) == 0 || len(args) > 3 || args[0] != "open" {
		return fmt.Errorf("hub: expected open [page[/row]] or close")
	}
	section := ""
	if len(args) >= 2 {
		section = args[1]
	}
	if hubAlive() {
		if section != "" {
			hubNav(section)
		}
		return nil
	}
	raw, _, _ := loadSettingsFile(filepath.Join(ryokuConfigDir(), "shell.json"))
	_, err := hubLaunch(hubScreenScales(raw), section)
	return err
}
