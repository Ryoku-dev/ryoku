package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/keyboard"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
	wm "ryoku-wm"
)

// ---- reconcilers: the keyboard layout, on every screen that asks for one ------
//
// The four layers, and why the boot one traps people, are described in
// internal/keyboard. These two reconcilers cover the two directions: adopting
// the layout the installer was told about into a desktop still on the shipped
// default, and reporting when the layers have drifted apart afterwards.

// configuredKbLayout is the primary layout from the neutral store: the first of a
// possibly comma-separated desktop.input.kbLayout ("fr,us" -> "fr"), the one a
// login screen and a boot prompt need since neither can switch.
func configuredKbLayout() string {
	raw := readFileSafe(filepath.Join(sys.ConfigHome(), "ryoku", "desktop.json"))
	layout, ok := hyprGetKbLayout(raw)
	if !ok {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(layout, ",", 2)[0])
}

func reconcileKeymap(checkOnly bool) recResult {
	layout := configuredKbLayout()
	if layout == "" {
		return okRes(i18n.T("no session keyboard layout recorded yet"))
	}
	km := keyboard.ConsoleKeymap()
	// Compared in xkb terms, so a console keymap that spells the same layout
	// differently (uk for gb) is not reported as drift.
	consoleDrifted := km != "" && keyboard.ConsoleAsXkb(km) != layout
	stale, imgPath := keyboard.BootStale()
	if !consoleDrifted && !stale {
		return okRes(i18n.T("keyboard layout %q matches on the session, login screen, console, and boot prompt"), layout)
	}

	if checkOnly {
		switch {
		case consoleDrifted && stale:
			return wouldRes(i18n.T("console keymap is %q but the session uses %q, and %s predates %s so the disk passphrase prompt is older still"),
				km, layout, filepath.Base(imgPath), keyboard.VconsolePath).
				withFix("ryoku keyboard apply")
		case consoleDrifted:
			return wouldRes(i18n.T("console keymap is %q but the session uses %q, so the login screen and TTYs disagree with the desktop"), km, layout).
				withFix("ryoku keyboard apply")
		default:
			return wouldRes(i18n.T("%s predates %s, so the disk passphrase prompt still uses the keymap baked in when it was built"),
				filepath.Base(imgPath), keyboard.VconsolePath).
				withFix("ryoku keyboard apply")
		}
	}

	// Apply mode: make the layout global. Push the desktop's layout onto the
	// console and greeter, then rebuild the boot image so the disk passphrase
	// prompt follows too. ApplySystem rewrites vconsole.conf, so a rebuild is
	// needed whenever we touched it or the image was already stale; one
	// unconditional rebuild covers both.
	if consoleDrifted {
		l := keyboard.DesktopLayout()
		if l.Primary() == "" {
			l = keyboard.Layout{Layout: layout}
		}
		if err := keyboard.ApplySystem(l); err != nil {
			return warnRes(i18n.T("console keymap is %q but the session uses %q"), km, layout).
				withFix("ryoku keyboard apply")
		}
	}
	if err := keyboard.RebuildBootImage(); err != nil {
		return warnRes(i18n.T("set the console and greeter to %q, but could not rebuild the boot image so the passphrase prompt still lags: %v"), layout, err).
			withFix("ryoku keyboard apply")
	}
	return fixedRes(i18n.T("set the boot prompt, greeter, console, and desktop to %q"), layout)
}

// ---- reconciler: adopt the keyboard the installer was told about --------------

// keyboardSeedMarker records that the one-time adoption has run, so a later
// deliberate pick in Ryoku Settings is never quietly undone on the next doctor.
func keyboardSeedMarker() string {
	return filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku", "migrations", "keyboard-layout-seed")
}

// hyprGetKbLayout pulls desktop.input.kbLayout out of the neutral store.
func hyprGetKbLayout(raw string) (string, bool) {
	var o struct {
		Desktop struct {
			Input struct {
				KbLayout *string `json:"kbLayout"`
			} `json:"input"`
		} `json:"desktop"`
	}
	if json.Unmarshal([]byte(raw), &o) != nil || o.Desktop.Input.KbLayout == nil {
		return "", false
	}
	return *o.Desktop.Input.KbLayout, true
}

// hyprSetKbLayout rewrites desktop.input.kbLayout, leaving every other key intact.
func hyprSetKbLayout(raw, layout string) (string, error) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return "", err
	}
	desktop, _ := doc["desktop"].(map[string]any)
	if desktop == nil {
		desktop = map[string]any{}
		doc["desktop"] = desktop
	}
	input, _ := desktop["input"].(map[string]any)
	if input == nil {
		input = map[string]any{}
		desktop["input"] = input
	}
	input["kbLayout"] = layout
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// reconcileKeyboardSeed adopts the layout the machine already records when the
// desktop is still on the shipped default. A keyboard cannot report its own
// legends, so installing on AZERTY and finding the desktop on QWERTY is the
// normal first boot; this closes that gap once.
func reconcileKeyboardSeed(checkOnly bool) recResult {
	marker := keyboardSeedMarker()
	if sys.Exists(marker) {
		return okRes(i18n.T("keyboard layout already adopted once"))
	}
	mark := func() {
		if checkOnly {
			return
		}
		_ = os.MkdirAll(filepath.Dir(marker), 0o755)
		_ = os.WriteFile(marker, []byte("done\n"), 0o644)
	}
	store := filepath.Join(sys.ConfigHome(), "ryoku", "desktop.json")
	if !sys.Exists(store) {
		mark()
		return okRes(i18n.T("no saved desktop input to seed a layout into"))
	}
	raw := readFileSafe(store)
	cur, ok := hyprGetKbLayout(raw)
	// Only the untouched shipped default is adopted over. Anything else is a
	// choice, including a deliberate "us".
	if !ok || cur != "us" {
		mark()
		return okRes(i18n.T("keyboard layout is a deliberate choice; leaving it"))
	}
	got := keyboard.Detect(keyboard.X11Layout(), keyboard.ConsoleKeymap(), keyboard.SystemLocale())
	if got.Layout == "" || got.Layout == "us" {
		mark()
		return okRes(i18n.T("nothing on this system points at a non-US keyboard"))
	}
	if checkOnly {
		return wouldRes(i18n.T("%s says this is a %q keyboard but the desktop is still on us"), got.Source, got.Layout).
			withFix("ryoku doctor")
	}
	fixed, err := hyprSetKbLayout(raw, got.Layout)
	if err != nil {
		return failRes(i18n.T("could not update desktop settings: %v"), err)
	}
	if err := writeStore(store, []byte(fixed)); err != nil {
		return failRes(i18n.T("could not save the detected layout: %v"), err).withFix("ryoku doctor")
	}
	_, _ = wm.Open().Apply(store)
	mark()
	return fixedRes(i18n.T("adopted the %q keyboard layout from %s; run `ryoku keyboard apply` to put it on the login screen and boot prompt too"), got.Layout, got.Source)
}
