package wm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// The shipped Ryoku bind catalogue: one neutral list of every keybind the
// desktop offers, described by what it does rather than by any compositor's
// binding language. A provider reads this catalogue, resolves the user's
// rebinds, marks what its compositor cannot honour, and reports the result as
// the effective legend the cheatsheet and the Hub draw. Keeping the list here,
// in the seam, means the two surfaces read one source instead of each parsing a
// compositor's own config and drifting apart.

// BindKind groups a bind by who acts on it, so a consumer can colour or section
// the legend without reading the id.
type BindKind string

const (
	BindWM     BindKind = "wm"     // the window manager performs it
	BindShell  BindKind = "shell"  // the Ryoku shell performs it
	BindApp    BindKind = "app"    // it launches or focuses an app
	BindCustom BindKind = "custom" // a compositor's own bind, outside the shared set
)

// BindRow is one line of the effective bind legend a provider reports: a shipped
// bind resolved against the user's rebinds, or a compositor's own custom bind.
type BindRow struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Label    string `json:"label"`
	Hint     string `json:"hint,omitempty"`
	// Keys are the display tokens of the effective chord, ready for the sheet.
	Keys []string `json:"keys"`
	// Default is the shipped chord in Hub form ("SUPER + Q"), the key a rebind is
	// recorded over. A family keeps its {n} so the Hub knows it stands for ten.
	Default string `json:"default"`
	// Chord is the effective chord after the user's rebind, what the session emits.
	Chord      string   `json:"chord"`
	Kind       BindKind `json:"kind"`
	Rebindable bool     `json:"rebindable"`
	Locked     bool     `json:"locked"`
	// Unhonored, when set, is why the running compositor cannot perform this bind.
	// The row is still listed so the sheet reads honestly rather than silently
	// dropping a bind the user expects to see.
	Unhonored string `json:"unhonored,omitempty"`
}

// CatalogBind is one shipped Ryoku bind, compositor-neutral. It is the raw
// material a provider turns into a BindRow.
type CatalogBind struct {
	ID       string
	Category string
	Label    string
	Hint     string
	Chord    string
	Kind     BindKind
	Locked   bool
	// Family means Chord carries {n} and stands for ten binds, n = 1..10, on the
	// digit key n%10 so the tenth is the 0 key. Numpad families read the number
	// pad instead, KP_1..KP_9 then KP_0.
	Family bool
	Numpad bool
}

// IrisCloseCheck succeeds when the iRiS frame is the bar style and its close
// confirmation is switched on, so a close bind can ask the shell first and fall
// through to the compositor. QmlEnv is the module path a bind-spawned qs
// surface needs: it never passes through ryoku-shell's daemon, which is what
// injects the path into the configs it supervises. Both are shell strings the
// emitting providers concatenate into a spawn command.
const IrisCloseCheck = `jq -e '(.barStyle // "qsbar") == "iris" and .inir.closeConfirm.enabled == true' "${XDG_CONFIG_HOME:-$HOME/.config}/ryoku/shell.json" >/dev/null 2>&1`

const QmlEnv = `env QML_IMPORT_PATH="$HOME/.local/lib/qt6/qml" QML2_IMPORT_PATH="$HOME/.local/lib/qt6/qml"`

// catalogue is the shipped set, in the order the legend presents it. The four
// move-workspace-to-screen chords carry their modifiers in canonical order
// (SUPER CTRL ALT), the same order NormChord produces, so every shipped chord is
// already its own normal form. Modifier order never reaches the compositor as
// anything but a set, so this is a presentation choice, not a behaviour one.
var catalogue = []CatalogBind{
	// Windows
	{ID: "window.close", Category: "Windows", Label: "Close window", Hint: "Close the focused window", Chord: "SUPER + Q", Kind: BindWM},
	{ID: "window.fullscreen", Category: "Windows", Label: "Fullscreen", Hint: "Toggle fullscreen on the focused window", Chord: "SUPER + F", Kind: BindWM},
	{ID: "window.float", Category: "Windows", Label: "Float or tile", Hint: "Float the window, or tile it back", Chord: "SUPER + A", Kind: BindWM},
	{ID: "window.pin", Category: "Windows", Label: "Pin window", Hint: "Keep a floating window on every workspace", Chord: "SUPER + SHIFT + P", Kind: BindWM},
	{ID: "window.resize", Category: "Windows", Label: "Resize mode", Hint: "Resize from the keyboard: arrows or hjkl, Esc exits", Chord: "SUPER + R", Kind: BindWM},
	{ID: "window.presetHeight", Category: "Windows", Label: "Step window height", Hint: "Cycle the window through its preset heights", Chord: "SUPER + SHIFT + R", Kind: BindWM},
	{ID: "column.tabbed", Category: "Windows", Label: "Tabbed column", Hint: "Stack the column into tabs, one window showing at a time", Chord: "SUPER + T", Kind: BindWM},
	{ID: "column.maximize", Category: "Windows", Label: "Maximise", Hint: "Fill the screen width, keeping gaps and bar", Chord: "SUPER + D", Kind: BindWM},
	{ID: "column.center", Category: "Windows", Label: "Centre", Hint: "Centre the focused window on screen", Chord: "SUPER + C", Kind: BindWM},
	{ID: "window.focusPrevious", Category: "Windows", Label: "Last window", Hint: "Switch to the previously focused window", Chord: "ALT + Tab", Kind: BindWM},

	// Focus
	{ID: "focus.left", Category: "Focus", Label: "Focus left", Chord: "SUPER + Left", Kind: BindWM},
	{ID: "focus.right", Category: "Focus", Label: "Focus right", Chord: "SUPER + Right", Kind: BindWM},
	{ID: "focus.up", Category: "Focus", Label: "Focus up", Chord: "SUPER + Up", Kind: BindWM},
	{ID: "focus.down", Category: "Focus", Label: "Focus down", Chord: "SUPER + Down", Kind: BindWM},
	{ID: "column.first", Category: "Focus", Label: "First column", Hint: "Jump to the first column of the workspace", Chord: "SUPER + Home", Kind: BindWM},
	{ID: "column.last", Category: "Focus", Label: "Last column", Hint: "Jump to the last column", Chord: "SUPER + End", Kind: BindWM},

	// Move
	{ID: "move.left", Category: "Move", Label: "Move window left", Chord: "SUPER + SHIFT + Left", Kind: BindWM},
	{ID: "move.right", Category: "Move", Label: "Move window right", Chord: "SUPER + SHIFT + Right", Kind: BindWM},
	{ID: "move.up", Category: "Move", Label: "Move window up", Chord: "SUPER + SHIFT + Up", Kind: BindWM},
	{ID: "move.down", Category: "Move", Label: "Move window down", Chord: "SUPER + SHIFT + Down", Kind: BindWM},
	{ID: "column.mergeLeft", Category: "Move", Label: "Merge left", Hint: "Pull the window into the column or group on the left, or out of its own", Chord: "SUPER + bracketleft", Kind: BindWM},
	{ID: "column.mergeRight", Category: "Move", Label: "Merge right", Hint: "Pull the window into the column or group on the right, or out of its own", Chord: "SUPER + bracketright", Kind: BindWM},

	// Resize
	{ID: "resize.narrower", Category: "Resize", Label: "Narrower", Chord: "SUPER + CTRL + Left", Kind: BindWM},
	{ID: "resize.wider", Category: "Resize", Label: "Wider", Chord: "SUPER + CTRL + Right", Kind: BindWM},
	{ID: "resize.shorter", Category: "Resize", Label: "Shorter", Chord: "SUPER + CTRL + Up", Kind: BindWM},
	{ID: "resize.taller", Category: "Resize", Label: "Taller", Chord: "SUPER + CTRL + Down", Kind: BindWM},
	{ID: "resize.resetHeight", Category: "Resize", Label: "Reset height", Hint: "Give the window its automatic height back", Chord: "SUPER + CTRL + R", Kind: BindWM},

	// Workspaces
	{ID: "workspace.focus", Category: "Workspaces", Label: "Workspace 1 to 10", Hint: "Focus that workspace on this screen", Chord: "SUPER + {n}", Kind: BindWM, Family: true},
	{ID: "workspace.moveWindow", Category: "Workspaces", Label: "Send window to workspace", Hint: "Move the window to that workspace and follow it", Chord: "SUPER + ALT + {n}", Kind: BindWM, Family: true},
	{ID: "workspace.moveWindowSilent", Category: "Workspaces", Label: "Send window quietly", Hint: "Move the window there and stay here", Chord: "SUPER + SHIFT + {n}", Kind: BindWM, Family: true},
	{ID: "workspace.focus.numpad", Category: "Workspaces", Label: "Workspace 1 to 10, number pad", Hint: "Focus that workspace from the number pad", Chord: "SUPER + KP_{n}", Kind: BindWM, Family: true, Numpad: true},
	{ID: "workspace.moveWindow.numpad", Category: "Workspaces", Label: "Send window, number pad", Chord: "SUPER + ALT + KP_{n}", Kind: BindWM, Family: true, Numpad: true},
	{ID: "workspace.moveWindowSilent.numpad", Category: "Workspaces", Label: "Send window quietly, number pad", Chord: "SUPER + SHIFT + KP_{n}", Kind: BindWM, Family: true, Numpad: true},
	{ID: "workspace.prev", Category: "Workspaces", Label: "Previous workspace", Chord: "SUPER + Prior", Kind: BindWM},
	{ID: "workspace.next", Category: "Workspaces", Label: "Next workspace", Chord: "SUPER + Next", Kind: BindWM},
	{ID: "workspace.prevWheel", Category: "Workspaces", Label: "Previous workspace, wheel", Chord: "SUPER + mouse_up", Kind: BindWM},
	{ID: "workspace.nextWheel", Category: "Workspaces", Label: "Next workspace, wheel", Chord: "SUPER + mouse_down", Kind: BindWM},
	{ID: "workspace.moveWindowPrev", Category: "Workspaces", Label: "Send window to previous", Hint: "Move the window one workspace up", Chord: "SUPER + SHIFT + Prior", Kind: BindWM},
	{ID: "workspace.moveWindowNext", Category: "Workspaces", Label: "Send window to next", Hint: "Move the window one workspace down", Chord: "SUPER + SHIFT + Next", Kind: BindWM},
	{ID: "workspace.reorderUp", Category: "Workspaces", Label: "Reorder workspace up", Hint: "Swap this workspace with the one above it", Chord: "SUPER + CTRL + Prior", Kind: BindWM},
	{ID: "workspace.reorderDown", Category: "Workspaces", Label: "Reorder workspace down", Hint: "Swap this workspace with the one below it", Chord: "SUPER + CTRL + Next", Kind: BindWM},
	{ID: "workspace.hideWindow", Category: "Workspaces", Label: "Hide window", Hint: "Tuck the window into the scratchpad; press again on it to bring it back", Chord: "SUPER + H", Kind: BindWM},
	{ID: "workspace.scratchpad", Category: "Workspaces", Label: "Scratchpad", Hint: "Show or hide the scratchpad", Chord: "SUPER + ALT + H", Kind: BindWM},
	{ID: "workspace.overview", Category: "Workspaces", Label: "Overview", Hint: "Live previews of every workspace", Chord: "SUPER + Tab", Kind: BindShell},
	{ID: "workspace.overviewDesktops", Category: "Workspaces", Label: "Overview by desktop", Hint: "Step through desktops, blocks of ten workspaces, inside the overview", Chord: "SUPER + ALT + Tab", Kind: BindShell},

	// Displays
	{ID: "display.focus.left", Category: "Displays", Label: "Focus screen left", Chord: "SUPER + ALT + Left", Kind: BindWM},
	{ID: "display.focus.right", Category: "Displays", Label: "Focus screen right", Chord: "SUPER + ALT + Right", Kind: BindWM},
	{ID: "display.focus.up", Category: "Displays", Label: "Focus screen up", Chord: "SUPER + ALT + Up", Kind: BindWM},
	{ID: "display.focus.down", Category: "Displays", Label: "Focus screen down", Chord: "SUPER + ALT + Down", Kind: BindWM},
	{ID: "display.moveWindow.left", Category: "Displays", Label: "Send window to screen left", Chord: "SUPER + ALT + SHIFT + Left", Kind: BindWM},
	{ID: "display.moveWindow.right", Category: "Displays", Label: "Send window to screen right", Chord: "SUPER + ALT + SHIFT + Right", Kind: BindWM},
	{ID: "display.moveWindow.up", Category: "Displays", Label: "Send window to screen up", Chord: "SUPER + ALT + SHIFT + Up", Kind: BindWM},
	{ID: "display.moveWindow.down", Category: "Displays", Label: "Send window to screen down", Chord: "SUPER + ALT + SHIFT + Down", Kind: BindWM},
	{ID: "display.moveWorkspace.left", Category: "Displays", Label: "Send workspace to screen left", Hint: "Move the whole workspace to the screen on the left", Chord: "SUPER + CTRL + ALT + Left", Kind: BindWM},
	{ID: "display.moveWorkspace.right", Category: "Displays", Label: "Send workspace to screen right", Hint: "Move the whole workspace to the screen on the right", Chord: "SUPER + CTRL + ALT + Right", Kind: BindWM},
	{ID: "display.moveWorkspace.up", Category: "Displays", Label: "Send workspace to screen up", Hint: "Move the whole workspace to the screen above", Chord: "SUPER + CTRL + ALT + Up", Kind: BindWM},
	{ID: "display.moveWorkspace.down", Category: "Displays", Label: "Send workspace to screen down", Hint: "Move the whole workspace to the screen below", Chord: "SUPER + CTRL + ALT + Down", Kind: BindWM},
	{ID: "display.cycle", Category: "Displays", Label: "Mirror or extend displays", Hint: "Cycle internal only, external only, both", Chord: "SUPER + P", Kind: BindWM},

	// Apps
	{ID: "app.terminal", Category: "Apps", Label: "Terminal", Chord: "SUPER + Return", Kind: BindApp},
	{ID: "app.files", Category: "Apps", Label: "Files", Chord: "SUPER + E", Kind: BindApp},
	{ID: "app.browser", Category: "Apps", Label: "Browser", Chord: "SUPER + B", Kind: BindApp},
	{ID: "app.editor", Category: "Apps", Label: "Editor", Chord: "SUPER + N", Kind: BindApp},
	{ID: "app.notes", Category: "Apps", Label: "Notes", Chord: "SUPER + O", Kind: BindApp},
	{ID: "app.yazi", Category: "Apps", Label: "Yazi", Hint: "Terminal file manager", Chord: "SUPER + ALT + E", Kind: BindApp},
	{ID: "app.ryotunes", Category: "Apps", Label: "Ryotunes", Hint: "Music player; a second press focuses it", Chord: "SUPER + J", Kind: BindApp},

	// Shell
	{ID: "shell.launcher", Category: "Shell", Label: "App launcher", Chord: "SUPER + Space", Kind: BindShell},
	{ID: "shell.ask", Category: "Shell", Label: "Ask Rashin", Hint: "AI chat bar", Chord: "ALT + Space", Kind: BindShell},
	{ID: "shell.rashin", Category: "Shell", Label: "Rashin", Hint: "The AI companion app; a second press focuses it", Chord: "SUPER + ALT + Space", Kind: BindShell},
	{ID: "shell.cheatsheet", Category: "Shell", Label: "Keybind cheatsheet", Hint: "Press again to close", Chord: "SUPER + K", Kind: BindShell},
	{ID: "shell.lock", Category: "Shell", Label: "Lock the screen", Chord: "SUPER + L", Kind: BindShell},
	{ID: "shell.quicksettings", Category: "Shell", Label: "Quick settings", Hint: "Power, logout, restart, shutdown, wifi", Chord: "SUPER + Escape", Kind: BindShell},
	{ID: "shell.wallpaper", Category: "Shell", Label: "Wallpaper picker", Chord: "SUPER + W", Kind: BindShell},
	{ID: "shell.wallpaperRandom", Category: "Shell", Label: "Random wallpaper", Hint: "Random wallpaper with a random transition", Chord: "SUPER + SHIFT + W", Kind: BindShell},
	{ID: "shell.ryovm", Category: "Shell", Label: "Ryovm", Hint: "Summon the VM window to this workspace", Chord: "SUPER + SHIFT + V", Kind: BindShell},
	{ID: "shell.clipboard", Category: "Shell", Label: "Clipboard", Chord: "SUPER + V", Kind: BindShell},
	{ID: "shell.visualizer", Category: "Shell", Label: "Audio visualiser", Hint: "Toggle the desktop visualiser", Chord: "SUPER + M", Kind: BindShell},
	{ID: "shell.visualizerOverlay", Category: "Shell", Label: "Visualiser over windows", Hint: "Raise it above windows; again to flip back", Chord: "SUPER + SHIFT + M", Kind: BindShell},
	{ID: "shell.visualizerPlace", Category: "Shell", Label: "Place the visualiser", Hint: "Drag the ring or orb into place", Chord: "SUPER + ALT + M", Kind: BindShell},
	{ID: "shell.voice", Category: "Shell", Label: "Voice typing", Hint: "Speech to text; tap again to stop", Chord: "SUPER + grave", Kind: BindShell},
	{ID: "shell.settings", Category: "Shell", Label: "Ryoku settings", Chord: "SUPER + comma", Kind: BindShell},
	{ID: "shell.screenshot", Category: "Shell", Label: "Capture", Hint: "Screenshot and recording", Chord: "SUPER + SHIFT + S", Kind: BindShell},
	{ID: "shell.screenshotPrint", Category: "Shell", Label: "Screenshot", Chord: "Print", Kind: BindShell},
	{ID: "shell.screenshotMonitor", Category: "Shell", Label: "Screenshot, whole screen", Chord: "SHIFT + Print", Kind: BindShell},
	{ID: "shell.colorPicker", Category: "Shell", Label: "Pick a colour", Chord: "SUPER + SHIFT + C", Kind: BindShell},
	{ID: "shell.restartAudio", Category: "Shell", Label: "Recover audio", Hint: "When sound stops coming back", Chord: "SUPER + SHIFT + A", Kind: BindShell},
	{ID: "shell.inhibitShortcuts", Category: "Shell", Label: "Pass shortcuts to the app", Hint: "The focused app receives every shortcut until pressed again", Chord: "SUPER + SHIFT + Escape", Kind: BindShell},

	// Media. Locked because these ride the dedicated media keys the shell owns,
	// not a rebindable chord.
	{ID: "media.volumeUp", Category: "Media", Label: "Volume up", Chord: "XF86AudioRaiseVolume", Kind: BindShell, Locked: true},
	{ID: "media.volumeDown", Category: "Media", Label: "Volume down", Chord: "XF86AudioLowerVolume", Kind: BindShell, Locked: true},
	{ID: "media.mute", Category: "Media", Label: "Mute", Chord: "XF86AudioMute", Kind: BindShell, Locked: true},
	{ID: "media.play", Category: "Media", Label: "Play or pause", Chord: "XF86AudioPlay", Kind: BindShell, Locked: true},
	{ID: "media.next", Category: "Media", Label: "Next track", Chord: "XF86AudioNext", Kind: BindShell, Locked: true},
	{ID: "media.prev", Category: "Media", Label: "Previous track", Chord: "XF86AudioPrev", Kind: BindShell, Locked: true},

	// Hardware. Locked for the same reason as media: the laptop's function keys.
	{ID: "hardware.brightnessUp", Category: "Hardware", Label: "Brightness up", Chord: "XF86MonBrightnessUp", Kind: BindShell, Locked: true},
	{ID: "hardware.brightnessDown", Category: "Hardware", Label: "Brightness down", Chord: "XF86MonBrightnessDown", Kind: BindShell, Locked: true},
	{ID: "hardware.touchpadToggle", Category: "Hardware", Label: "Toggle touchpad", Chord: "XF86TouchpadToggle", Kind: BindShell, Locked: true},
	{ID: "hardware.touchpadOn", Category: "Hardware", Label: "Touchpad on", Chord: "XF86TouchpadOn", Kind: BindShell, Locked: true},
	{ID: "hardware.touchpadOff", Category: "Hardware", Label: "Touchpad off", Chord: "XF86TouchpadOff", Kind: BindShell, Locked: true},

	// Mouse
	{ID: "mouse.move", Category: "Mouse", Label: "Move window with the mouse", Chord: "SUPER + mouse:272", Kind: BindWM},
	{ID: "mouse.resize", Category: "Mouse", Label: "Resize window with the mouse", Chord: "SUPER + mouse:273", Kind: BindWM},
}

// ShippedBinds is the catalogue, in legend order. Each caller gets its own copy,
// so a provider that sorts or filters the set in place cannot corrupt the source
// the next caller reads. CatalogBind holds no reference fields, so the shallow
// copy is fully independent.
func ShippedBinds() []CatalogBind {
	return append([]CatalogBind(nil), catalogue...)
}

// Categories is the distinct category names in catalogue order, so a consumer
// can lay out the legend's sections without inferring the order from the rows.
func Categories() []string {
	seen := make(map[string]bool, len(catalogue))
	var out []string
	for _, b := range catalogue {
		if !seen[b.Category] {
			seen[b.Category] = true
			out = append(out, b.Category)
		}
	}
	return out
}

// ExpandChord substitutes a family placeholder for the nth member's key: {n}
// becomes the digit n%10, so the tenth member lands on the 0 key, and a KP_{n}
// family carries that same digit onto the number pad (KP_1..KP_0). A chord with
// no placeholder is returned unchanged, so a plain chord passes straight through.
func ExpandChord(chord string, n int) string {
	return strings.Replace(chord, "{n}", strconv.Itoa(n%10), 1)
}

// Expand is the ten chords of a family, or the single chord of a plain bind. n
// runs 1..10 onto the digit key n%10, so the tenth workspace lands on the 0 key;
// a numpad family carries KP_{n}, so the same substitution yields KP_1..KP_0.
func (c CatalogBind) Expand() []string {
	if !c.Family {
		return []string{c.Chord}
	}
	out := make([]string, 0, 10)
	for n := 1; n <= 10; n++ {
		out = append(out, ExpandChord(c.Chord, n))
	}
	return out
}

// FamilyRebind resolves a family's stored rebind. A family binds ten chords that
// share one key position (the {n} placeholder) and differ only by modifier, so a
// user rebinds the whole family with a single entry that keeps the placeholder
// and changes only the modifiers: ["SUPER + {n}"] = "SUPER + CTRL + {n}". It
// returns the stored chord and true when the value is that shape, the {n} key
// kept in place and every other token a modifier, otherwise def and false.
// Guarding the shape keeps a stray value from collapsing ten workspace keys onto
// one chord.
func FamilyRebind(def string, rebinds map[string]string) (string, bool) {
	to := strings.TrimSpace(rebinds[def])
	if to == "" || to == def {
		return def, false
	}
	parts := strings.Split(to, " + ")
	defParts := strings.Split(def, " + ")
	// The default's own key position is the placeholder the value must keep, so
	// only the modifier set moves and every member stays on its shipped digit.
	if parts[len(parts)-1] != defParts[len(defParts)-1] {
		return def, false
	}
	for _, tok := range parts[:len(parts)-1] {
		if _, isMod := modifierOrder[strings.ToUpper(tok)]; !isMod {
			return def, false
		}
	}
	return to, true
}

// numpadNumLockOff maps each number-pad digit keysym to the keysym the same key
// sends with NumLock off. A provider binds both so a workspace shortcut on the
// number pad works whichever way NumLock happens to sit.
var numpadNumLockOff = map[string]string{
	"KP_1": "KP_End",
	"KP_2": "KP_Down",
	"KP_3": "KP_Next",
	"KP_4": "KP_Left",
	"KP_5": "KP_Begin",
	"KP_6": "KP_Right",
	"KP_7": "KP_Home",
	"KP_8": "KP_Up",
	"KP_9": "KP_Prior",
	"KP_0": "KP_Insert",
}

// NumpadAliases returns the chord again with its number-pad digit swapped for the
// NumLock-off keysym, or nothing when the chord holds no number-pad digit. The
// token is matched whole, so KP_1 never trips on a longer neighbour.
func NumpadAliases(chord string) []string {
	parts := strings.Split(chord, " + ")
	for i, p := range parts {
		if off, ok := numpadNumLockOff[p]; ok {
			aliased := append([]string(nil), parts...)
			aliased[i] = off
			return []string{strings.Join(aliased, " + ")}
		}
	}
	return []string{}
}

// displayNames turns a raw keysym into the token the sheet shows a user. The
// modifiers and the awkward keysyms get a readable face; anything not listed
// falls through as itself.
var displayNames = map[string]string{
	"SUPER":                 "Super",
	"SHIFT":                 "Shift",
	"CTRL":                  "Ctrl",
	"ALT":                   "Alt",
	"Return":                "Enter",
	"Escape":                "Esc",
	"space":                 "Space",
	"Prior":                 "Page Up",
	"Next":                  "Page Down",
	"grave":                 "`",
	"comma":                 ",",
	"bracketleft":           "[",
	"bracketright":          "]",
	"mouse:272":             "LMB",
	"mouse:273":             "RMB",
	"mouse_up":              "Scroll Up",
	"mouse_down":            "Scroll Down",
	"XF86AudioRaiseVolume":  "Vol +",
	"XF86AudioLowerVolume":  "Vol -",
	"XF86AudioMute":         "Mute",
	"XF86AudioPlay":         "Play",
	"XF86AudioNext":         "Next",
	"XF86AudioPrev":         "Prev",
	"XF86MonBrightnessUp":   "Brightness +",
	"XF86MonBrightnessDown": "Brightness -",
	"XF86TouchpadToggle":    "Touchpad",
	"XF86TouchpadOn":        "Touchpad On",
	"XF86TouchpadOff":       "Touchpad Off",
}

// DisplayKeys splits a chord into the tokens the sheet renders. A family
// placeholder becomes its range ("1 … 0", "Num 1 … 0"), a number-pad digit reads
// "Num 3", and everything else takes its readable face or passes through.
func DisplayKeys(chord string) []string {
	parts := strings.Split(chord, " + ")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, displayToken(p))
	}
	return out
}

func displayToken(tok string) string {
	if name, ok := displayNames[tok]; ok {
		return name
	}
	switch tok {
	case "{n}":
		return "1 \u2026 0"
	case "KP_{n}":
		return "Num 1 \u2026 0"
	}
	if d, ok := strings.CutPrefix(tok, "KP_"); ok && len(d) == 1 && d[0] >= '0' && d[0] <= '9' {
		return "Num " + d
	}
	return tok
}

// modifierOrder is the canonical modifier sequence a normalised chord carries, so
// two spellings of the same combo collapse to one string for clash detection.
var modifierOrder = map[string]int{
	"SUPER": 0,
	"CTRL":  1,
	"ALT":   2,
	"SHIFT": 3,
}

// NormChord is a chord's clash-detection form: modifiers upper-cased and put in
// canonical order, single spaces, the key last. Two chords that bind the same
// keys in a different spelling normalise to the same string.
func NormChord(chord string) string {
	var mods, keys []string
	for _, raw := range strings.Split(chord, "+") {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			continue
		}
		up := strings.ToUpper(tok)
		if _, isMod := modifierOrder[up]; isMod {
			mods = append(mods, up)
			continue
		}
		keys = append(keys, tok)
	}
	sort.SliceStable(mods, func(i, j int) bool {
		return modifierOrder[mods[i]] < modifierOrder[mods[j]]
	})
	return strings.Join(append(mods, keys...), " + ")
}

// decodeBinds turns a provider's binds answer into rows. An empty answer yields
// an empty slice rather than nil, so a caller ranges over it without a nil guard.
func decodeBinds(out []byte) ([]BindRow, error) {
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return []BindRow{}, nil
	}
	var rows []BindRow
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("binds: %w", err)
	}
	return rows, nil
}
