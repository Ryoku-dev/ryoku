package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	wm "ryoku-wm"
)

// The neutral settings store as wayfire reads it: the desktop.* fields wayfire
// can express, the wm.wayfire.* exclusives that have no neutral key, the
// desktop.json codec that fills them, and the wayfire defaults the Hub overlays
// user values on. Only the leaves wayfire honours are modelled; apply reports
// everything else in the store as unhonored rather than parsing it.
//
// wayfire has no include directive, so the generated wayfire.ini is not a diff
// over a shipped base: apply composes it from the shipped defaults, the store
// and the seed files in one pass. A leaf left out of the structs below would
// never reach that file.

// Appearance: the frame and effect leaves wayfire's own sections cover. Gaps,
// rounding, opacities, shadows, glow, dimming and the blur tuning have no
// wayfire key (blur itself is a plugin presence, carried below as BlurEnabled),
// so they are absent here and reported unhonored when the store carries them.
type Appearance struct {
	BorderSize           int    `json:"borderSize"`
	ActiveBorder         string `json:"activeBorder"`
	InactiveBorder       string `json:"inactiveBorder"`
	BorderFollowsPalette bool   `json:"borderFollowsPalette"`
	Animations           bool   `json:"animations"`
	BlurEnabled          bool   `json:"blurEnabled"`
}

// Input: the keyboard, pointer and touchpad leaves wayfire's input and vswipe
// sections cover. FollowMouse and MiddleClickPaste are absent: wayfire has no
// focus-follows-mouse mode and no middle-click-paste switch, so apply reports
// them unhonored instead of pretending.
type Input struct {
	KbLayout           string  `json:"kbLayout"`
	KbVariant          string  `json:"kbVariant"`
	KbOptions          string  `json:"kbOptions"`
	NumlockByDefault   bool    `json:"numlockByDefault"`
	Sensitivity        float64 `json:"sensitivity"`
	AccelProfile       string  `json:"accelProfile"`
	LeftHanded         bool    `json:"leftHanded"`
	MouseNaturalScroll bool    `json:"mouseNaturalScroll"`
	MouseScrollFactor  float64 `json:"mouseScrollFactor"`
	NaturalScroll      bool    `json:"naturalScroll"`
	TouchScrollFactor  float64 `json:"touchScrollFactor"`
	TapToClick         bool    `json:"tapToClick"`
	TapAndDrag         bool    `json:"tapAndDrag"`
	Clickfinger        bool    `json:"clickfinger"`
	MiddleEmulation    bool    `json:"middleEmulation"`
	DisableWhileTyping bool    `json:"disableWhileTyping"`
	RepeatRate         int     `json:"repeatRate"`
	RepeatDelay        int     `json:"repeatDelay"`
	WorkspaceSwipe     bool    `json:"workspaceSwipe"`
	SwipeFingers       int     `json:"swipeFingers"`
}

// Cursor: wayfire takes the cursor theme and size from the input section and
// has no hide timeout or keypress hide, so those two neutral leaves are
// reported unhonored.
type Cursor struct {
	Theme string `json:"theme"`
	Size  int    `json:"size"`
}

// WindowRule = one user rule: optional app-id/title match plus one action.
type WindowRule struct {
	Class  string `json:"class"`
	Title  string `json:"title"`
	Action string `json:"action"`
	Value  string `json:"value"`
}

// AppOverride: per-app appearance. Numeric fields use -1 for "inherit" and the
// strings use "" for unset; wayfire expresses opacity (as a window-rules alpha)
// and reports the rest unhonored per override.
type AppOverride struct {
	Class      string  `json:"class"`
	Title      string  `json:"title"`
	Opacity    float64 `json:"opacity"`
	Rounding   int     `json:"rounding"`
	BorderSize int     `json:"borderSize"`
	Blur       string  `json:"blur"`
	Shadow     string  `json:"shadow"`
	Dim        string  `json:"dim"`
	Anim       string  `json:"anim"`
	Opaque     string  `json:"opaque"`
}

// Autostart is one startup command. wayfire's autostart section runs each
// value through sh, keyed by any unique id, so the whole row translates.
type Autostart struct {
	Command string `json:"command"`
}

// Keybind = a user shortcut. action "exec" runs Value, which is the one shape
// wayfire's command plugin binds natively; every other action is reported
// unhonored, because wayfire binds shell commands rather than window actions.
type Keybind struct {
	Keys    string `json:"keys"`
	Action  string `json:"action"`
	Value   string `json:"value"`
	Release bool   `json:"release,omitempty"`
}

// AnimKind is one wayfire animation's tuning: a duration plus the easing word
// wayfire's duration options accept ("circle" and friends), rendered as
// "<ms>ms <curve>".
type AnimKind struct {
	DurationMs int    `json:"durationMs"`
	Curve      string `json:"curve"`
}

// Depthdeck mirrors the [depthdeck] option block one for one, so the deck's
// knobs are store leaves the Hub can draw and apply can write back verbatim.
type Depthdeck struct {
	Enabled              bool    `json:"enabled"`
	Scatter              bool    `json:"scatter"`
	CardEdgeScatter      bool    `json:"cardEdgeScatter"`
	CardScatterReshuffle bool    `json:"cardScatterReshuffle"`
	CardPeekMin          float64 `json:"cardPeekMin"`
	CardPeekMax          float64 `json:"cardPeekMax"`
	AnimationMs          int     `json:"animationMs"`
	Layer1Opacity        float64 `json:"layer1Opacity"`
	Layer1Scale          float64 `json:"layer1Scale"`
	Layer2Opacity        float64 `json:"layer2Opacity"`
	Layer2Scale          float64 `json:"layer2Scale"`
	MaxLayers            int     `json:"maxLayers"`
	MaximizedFrontScale  float64 `json:"maximizedFrontScale"`
}

// Wayfire holds the wm.wayfire.* exclusives: wayfire behaviours with no neutral
// key. This is the wayfire twin of wm.niri.*, so the Hub can surface them
// without every other compositor pretending to have them.
type Wayfire struct {
	Animation WayfireAnim `json:"animation"`
	Depthdeck Depthdeck   `json:"depthdeck"`
}

// WayfireAnim is the per-kind animation tree. The kinds are wayfire's own
// duration options, so a Hub row named wm.wayfire.animation.<kind>.<field>
// reaches the right section.
type WayfireAnim struct {
	WorkspaceSwitch AnimKind `json:"workspaceSwitch"`
	Window          AnimKind `json:"window"`
	Snap            AnimKind `json:"snap"`
}

// wayfireStore is the typed store the composer consumes. Wayfire stays out of
// the flat desktop marshalling (json:"-") because splitStore routes it under
// wm.wayfire.
type wayfireStore struct {
	Appearance   Appearance    `json:"appearance"`
	Input        Input         `json:"input"`
	Cursor       Cursor        `json:"cursor"`
	WindowRules  []WindowRule  `json:"windowRules"`
	AppOverrides []AppOverride `json:"appOverrides"`
	Autostart    []Autostart   `json:"autostart"`
	Keybinds     []Keybind     `json:"keybinds"`
	// KeybindRebinds moves a shipped chord and Unbinds drops one, both keyed on
	// the catalogue's own default; resolveBinds folds them into the emission.
	KeybindRebinds map[string]string `json:"keybindRebinds,omitempty"`
	Unbinds        []string          `json:"unbinds,omitempty"`
	Wayfire        Wayfire           `json:"-"`
}

// neutralStore is desktop.json on disk: { "desktop": {...}, "wm": { "<name>":
// {...} } }. WM is keyed by compositor so apply can see, and preserve, another
// compositor's exclusives without parsing them.
type neutralStore struct {
	Desktop map[string]json.RawMessage `json:"desktop"`
	WM      map[string]json.RawMessage `json:"wm"`
}

// defaultStore is the baseline the Hub overlays user values on and the values
// apply writes when the store is silent. The frame, input and depthdeck numbers
// are the desktop's shipped look: the same border and palette behaviour as the
// sibling providers, wayfire's own repeat rate, and the depthdeck card tuning
// from the plugin's own defaults.
func defaultStore() wayfireStore {
	return wayfireStore{
		Appearance: Appearance{
			BorderSize: 4, ActiveBorder: "#e0563b", InactiveBorder: "#313a4d",
			BorderFollowsPalette: true, Animations: true, BlurEnabled: true,
		},
		Input: Input{
			KbLayout: "us", NumlockByDefault: false,
			Sensitivity: 0, AccelProfile: "", LeftHanded: false,
			MouseNaturalScroll: false, MouseScrollFactor: 1,
			NaturalScroll: true, TouchScrollFactor: 1,
			TapToClick: true, TapAndDrag: true, Clickfinger: false,
			MiddleEmulation: false, DisableWhileTyping: true,
			RepeatRate: 25, RepeatDelay: 600,
			WorkspaceSwipe: false, SwipeFingers: 3,
		},
		Cursor:       Cursor{Theme: "Bibata-Modern-Ice", Size: 24},
		WindowRules:  []WindowRule{},
		AppOverrides: []AppOverride{},
		Autostart:    []Autostart{},
		Keybinds:     []Keybind{},
		Wayfire: Wayfire{
			Animation: WayfireAnim{
				WorkspaceSwitch: AnimKind{DurationMs: 300, Curve: "circle"},
				Window:          AnimKind{DurationMs: 400, Curve: "circle"},
				Snap:            AnimKind{DurationMs: 300, Curve: "circle"},
			},
			Depthdeck: Depthdeck{
				Enabled: true, Scatter: true, CardEdgeScatter: true,
				CardScatterReshuffle: true, CardPeekMin: 24, CardPeekMax: 80,
				AnimationMs: 200, Layer1Opacity: 0.85, Layer1Scale: 0.7,
				Layer2Opacity: 0.7, Layer2Scale: 0.5, MaxLayers: 8,
				MaximizedFrontScale: 0.90,
			},
		},
	}
}

// readNeutralStore reads desktop.json, false when it is absent or unparseable.
func readNeutralStore(path string) (neutralStore, bool) {
	var ns neutralStore
	b, err := os.ReadFile(path)
	if err != nil {
		return ns, false
	}
	if json.Unmarshal(b, &ns) != nil {
		return ns, false
	}
	return ns, true
}

// loadStore fills the wayfire defaults, then overlays the store's desktop.* and
// wm.wayfire.* leaves. An absent leaf keeps its default, so the composed config
// is always the full effective look, not a partial one.
func loadStore(path string) wayfireStore {
	s := defaultStore()
	ns, ok := readNeutralStore(path)
	if !ok {
		return s
	}
	if b, err := json.Marshal(ns.Desktop); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	s.Cursor.Theme = wm.ResolveCursorTheme(s.Cursor.Theme)
	if raw, ok := ns.WM["wayfire"]; ok {
		_ = json.Unmarshal(raw, &s.Wayfire)
	}
	return s
}

// splitStore renders the store as the namespaced desktop.json tree: desktop.* is
// the flat neutral fields, wm.wayfire.* is the exclusives, so the Hub learns
// both the namespace name and its sections from the defaults it reads.
func splitStore(s wayfireStore) (map[string]any, error) {
	db, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var desktop map[string]json.RawMessage
	if err := json.Unmarshal(db, &desktop); err != nil {
		return nil, err
	}
	wb, err := json.Marshal(s.Wayfire)
	if err != nil {
		return nil, err
	}
	var own map[string]json.RawMessage
	if err := json.Unmarshal(wb, &own); err != nil {
		return nil, err
	}
	return map[string]any{
		"desktop": desktop,
		"wm":      map[string]any{"wayfire": own},
	}, nil
}

// --- paths ----------------------------------------------------------------

func configHome() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	return filepath.Join(os.Getenv("HOME"), ".config")
}

// storePath is the neutral settings store this provider reads.
func storePath() string {
	return filepath.Join(configHome(), "ryoku", "desktop.json")
}

// borderPalettePath is where the border act records the live palette's active
// and inactive colours. The composer reads it so the border tracks the
// wallpaper: wayfire reloads its config on write, so the colours have to be in
// the file as well as pushed live or the next apply would snap back to the
// store colours.
func borderPalettePath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "ryoku", "wayfire-border-palette.json")
}

// borderPaletteColors reads the active and inactive colours the border act last
// wrote from the live palette. ok is false when the file is absent or holds no
// usable colour, so the composer falls back to the store colours; a side that
// did not normalise comes back empty and the composer keeps the store's value
// for that side alone.
func borderPaletteColors() (active, inactive string, ok bool) {
	b, err := os.ReadFile(borderPalettePath())
	if err != nil {
		return "", "", false
	}
	var p struct {
		Active   string `json:"active"`
		Inactive string `json:"inactive"`
	}
	if json.Unmarshal(b, &p) != nil {
		return "", "", false
	}
	_, aok := normBorderHex(p.Active)
	_, iok := normBorderHex(p.Inactive)
	return p.Active, p.Inactive, aok || iok
}

// userEditsWayfireDir is the wayfire slice of the user overlay tree. The
// generated ini lives here so it survives an update as a user edit;
// writeOverlayIni reflects the same bytes into the live wayfire dir so a reload
// picks them up at once.
func userEditsWayfireDir() string {
	return filepath.Join(configHome(), "ryoku", "user_edits", "wayfire")
}

// shareDefaultsPath is the wayfire.ini the package ships: the baseline layer
// the composition starts from. It is a var so a test can point it at a fixture
// instead of the installed package.
var shareDefaultsPath = "/usr/share/ryoku/config/wayfire/wayfire.ini"

// --- disk ------------------------------------------------------------------

func atomicWrite(path string, b []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// writeOverlayIni authors a generated file in the user_edits tree first, then
// reflects the same bytes into the live wayfire dir. The order matters: if the
// live write fails the overlay still holds the new content, so a materialize
// re-lays it rather than resurrecting the old one.
func writeOverlayIni(name string, body []byte) error {
	if err := atomicWrite(filepath.Join(userEditsWayfireDir(), name), body, 0o644); err != nil {
		return err
	}
	return atomicWrite(filepath.Join(wayfireConfigDir(), name), body, 0o644)
}
