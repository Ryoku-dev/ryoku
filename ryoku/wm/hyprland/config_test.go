package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	wm "ryoku-wm"
)

// A custom keybind with release mode on must emit the Hyprland release flag, and
// a press keybind must not carry it: this is written into settings.lua, so a
// regression would silently change when shortcuts fire.
func TestGenKeybindReleaseFlag(t *testing.T) {
	press := genKeybind(Keybind{Keys: "SUPER + M", Action: "exec", Value: "kitty"})
	if got, want := press, "hl.bind(\"SUPER + M\", hl.dsp.exec_cmd(\"kitty\"))\n"; got != want {
		t.Fatalf("press bind:\n got %q\nwant %q", got, want)
	}
	release := genKeybind(Keybind{Keys: "SUPER + M", Action: "exec", Value: "kitty", Release: true})
	if got, want := release, "hl.bind(\"SUPER + M\", hl.dsp.exec_cmd(\"kitty\"), { release = true })\n"; got != want {
		t.Fatalf("release bind:\n got %q\nwant %q", got, want)
	}
}

// A custom bind on a number-pad digit binds both keypad faces: the digit for
// NumLock on and its NumLock-off twin, so it fires whichever way NumLock sits,
// the same coverage the shipped families get. A non-numpad chord stays one bind.
func TestGenKeybindNumpadTwin(t *testing.T) {
	got := genKeybind(Keybind{Keys: "SUPER + KP_5", Action: "exec", Value: "kitty"})
	want := "hl.bind(\"SUPER + KP_5\", hl.dsp.exec_cmd(\"kitty\"))\n" +
		"hl.bind(\"SUPER + KP_Begin\", hl.dsp.exec_cmd(\"kitty\"))\n"
	if got != want {
		t.Fatalf("numpad custom bind:\n got %q\nwant %q", got, want)
	}
	if plain := genKeybind(Keybind{Keys: "SUPER + M", Action: "close"}); strings.Count(plain, "hl.bind(") != 1 {
		t.Errorf("non-numpad bind emitted %d lines, want 1: %q", strings.Count(plain, "hl.bind("), plain)
	}
}

// Hyprland's rebind path is a single K() lookup, so a bind rebound onto the
// number pad registers one keysym and fires in one NumLock state only. The legend
// row names that limit rather than leaving the user a chord that half works.
func TestRebindNumpadHint(t *testing.T) {
	o := defaultOverrides()
	o.KeybindRebinds = map[string]string{"SUPER + Q": "SUPER + KP_1"}
	var row wm.BindRow
	for _, r := range buildBindRows(repoModules(), o) {
		if r.ID == "window.close" {
			row = r
			break
		}
	}
	if row.ID == "" {
		t.Fatal("window.close row missing")
	}
	if row.Chord != "SUPER + KP_1" {
		t.Errorf("chord = %q, want SUPER + KP_1", row.Chord)
	}
	if !strings.Contains(row.Hint, "Works with NumLock on") {
		t.Errorf("hint = %q, want it to name the NumLock-on limit", row.Hint)
	}
}

// A family rebind is stored as one {n} entry, but binds.lua looks up each member
// and both number-pad faces through its own K() lookup, so renderRebinds expands
// the placeholder: a digit family into its ten [SUPER + d] entries, a keypad
// family into twenty, the ten KP_ digits and their NumLock-off twins. A
// non-family rebind is left as the single entry it always was.
func TestRenderRebindsExpandsFamily(t *testing.T) {
	digit := string(renderRebinds(Overrides{KeybindRebinds: map[string]string{"SUPER + {n}": "SUPER + CTRL + {n}"}}))
	if n := strings.Count(digit, "] = "); n != 10 {
		t.Errorf("digit family expanded to %d entries, want 10:\n%s", n, digit)
	}
	for _, want := range []string{`["SUPER + 1"] = "SUPER + CTRL + 1"`, `["SUPER + 0"] = "SUPER + CTRL + 0"`} {
		if !strings.Contains(digit, want) {
			t.Errorf("digit family missing entry %s", want)
		}
	}

	kp := string(renderRebinds(Overrides{KeybindRebinds: map[string]string{"SUPER + KP_{n}": "SUPER + ALT + KP_{n}"}}))
	if n := strings.Count(kp, "] = "); n != 20 {
		t.Errorf("keypad family expanded to %d entries, want 20:\n%s", n, kp)
	}
	for _, want := range []string{`["SUPER + KP_1"] = "SUPER + ALT + KP_1"`, `["SUPER + KP_End"] = "SUPER + ALT + KP_End"`} {
		if !strings.Contains(kp, want) {
			t.Errorf("keypad family missing entry %s", want)
		}
	}

	plain := string(renderRebinds(Overrides{KeybindRebinds: map[string]string{"SUPER + Q": "SUPER + X"}}))
	if n := strings.Count(plain, "] = "); n != 1 {
		t.Errorf("plain rebind expanded to %d entries, want 1:\n%s", n, plain)
	}
}

// The shipped input.lua puts keyboard focus under the cursor, and the
// diff-based config must not re-emit that default, or settings.lua would override
// the shipped module with the same value for no reason.
func TestDefaultFollowMouseMatchesShippedInput(t *testing.T) {
	const cursorFocus = 1
	if got := defaultOverrides().Input.FollowMouse; got != cursorFocus {
		t.Fatalf("default input.follow_mouse = %d, want %d", got, cursorFocus)
	}
	inputConfig, err := os.ReadFile(filepath.Join("..", "..", "hyprland", "modules", "input.lua"))
	if err != nil {
		t.Fatalf("read shipped input config: %v", err)
	}
	if !regexp.MustCompile(`(?m)^[[:space:]]*follow_mouse[[:space:]]*=[[:space:]]*1,[[:space:]]*$`).Match(inputConfig) {
		t.Fatal("shipped input.lua must put keyboard focus under the cursor")
	}
	if config := genConfig(defaultOverrides(), false); strings.Contains(config, "follow_mouse =") {
		t.Fatalf("default settings.lua overrides input.lua:\n%s", config)
	}
}

// Hyprland tames a maximise-on-open by refusing the client's request outright, a
// catch-all suppress_event rule. This is the whole of the Hyprland side, so it
// is pinned by the exact line and by its presence in the full config when the
// setting is on and its absence when off.
func TestGenTameMaximizeOnOpen(t *testing.T) {
	on := defaultOverrides() // TameMaximizeOnOpen defaults on
	want := `hl.window_rule({ name = "ryoku-tame-maximize-on-open", match = { class = ".*" }, suppress_event = "maximize" })` + "\n"
	if got := genTameMaximizeOnOpen(on); got != want {
		t.Fatalf("rule on:\n got %q\nwant %q", got, want)
	}
	if !strings.Contains(genLua(on, false), "ryoku-tame-maximize-on-open") {
		t.Fatal("the full config must carry the tame rule when the setting is on")
	}

	off := defaultOverrides()
	off.Windows.TameMaximizeOnOpen = false
	if got := genTameMaximizeOnOpen(off); got != "" {
		t.Fatalf("rule off must emit nothing, got %q", got)
	}
	if strings.Contains(genLua(off, false), "suppress_event") {
		t.Fatalf("the full config must carry no suppress rule when the setting is off:\n%s", genLua(off, false))
	}
}

// "DYNAMIC" is a store role, not a theme on disk: the loaded overrides must
// carry the concrete wallpaper-following theme, or settings.lua exports an
// XCURSOR_THEME no loader can open and the pointer falls back to a bitmap.
func TestLoadStoreResolvesDynamicCursor(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "desktop.json")
	body := `{"desktop":{"cursor":{"theme":"DYNAMIC","size":18}}}`
	if err := os.WriteFile(store, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	o := loadStore(store)
	if o.Cursor.Theme != wm.CursorThemeMaterial {
		t.Fatalf("loaded theme = %q, want %q", o.Cursor.Theme, wm.CursorThemeMaterial)
	}
	if cfg := genLua(o, false); !strings.Contains(cfg, `hl.env("XCURSOR_THEME", "`+wm.CursorThemeMaterial+`")`) {
		t.Fatalf("settings.lua does not export the resolved theme:\n%s", cfg)
	}
}

// The border follows the wallpaper unless the user pins a fixed colour. Following
// omits col.active_border from settings.lua so decoration.lua's palette border
// wins, and the live act pushes the palette colours on a wallpaper change. Fixed
// pins the colour in settings.lua and the act is a no-op, so a wallpaper change
// never overrides the chosen colour.
func TestBorderFollowsPaletteGatesConfigAndAct(t *testing.T) {
	writeStore := func(t *testing.T, followBorder bool) {
		home := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", home)
		ryoku := filepath.Join(home, "ryoku")
		if err := os.MkdirAll(ryoku, 0o755); err != nil {
			t.Fatal(err)
		}
		store := fmt.Sprintf(`{"desktop":{"appearance":{"borderFollowsPalette":%t}}}`, followBorder)
		if err := os.WriteFile(filepath.Join(ryoku, "desktop.json"), []byte(store), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("following omits the pin and the act applies", func(t *testing.T) {
		writeStore(t, true)
		o := loadStore(desktopStorePath())
		if !borderFollowsPalette(o) {
			t.Fatal("a following border under a wallpaper-driven theme must follow the palette")
		}
		if cfg := genConfig(o, borderFollowsPalette(o)); strings.Contains(cfg, "col.active_border") {
			t.Fatalf("a following border must omit col.active_border\n%s", cfg)
		}
		var got []string
		restore := stubCtl(t, func(args ...string) ([]byte, error) { got = args; return nil, nil })
		defer restore()
		if err := runAct([]string{"decoration.borderColors", "#112233", "#445566"}); err != nil {
			t.Fatal(err)
		}
		if len(got) == 0 || got[0] != "eval" {
			t.Fatalf("a following border must push the palette colours live, got %q", got)
		}
	})

	t.Run("fixed keeps the pin and the act is a no-op", func(t *testing.T) {
		writeStore(t, false)
		o := loadStore(desktopStorePath())
		if borderFollowsPalette(o) {
			t.Fatal("a pinned border must not follow the palette")
		}
		if cfg := genConfig(o, borderFollowsPalette(o)); !strings.Contains(cfg, "col.active_border") {
			t.Fatalf("a fixed border must pin col.active_border\n%s", cfg)
		}
		called := false
		restore := stubCtl(t, func(...string) ([]byte, error) { called = true; return nil, nil })
		defer restore()
		if err := runAct([]string{"decoration.borderColors", "#112233", "#445566"}); err != nil {
			t.Fatal(err)
		}
		if called {
			t.Fatal("a fixed border must not push colours live")
		}
	})
}
