package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	wm "ryoku-wm"
)

// The neutral settings store as Hyprland reads it: the typed override model the
// Lua generators consume, the desktop.json codec that splits it across the
// desktop.* (neutral intent) and wm.hyprland.* (exclusives) namespaces, and the
// paths and small helpers the rest of the provider shares.

// Appearance: general / decoration / animations keywords.
type Appearance struct {
	GapsIn               int     `json:"gapsIn"`
	GapsOut              int     `json:"gapsOut"`
	BorderSize           int     `json:"borderSize"`
	Rounding             int     `json:"rounding"`
	RoundingPower        float64 `json:"roundingPower"`
	ActiveOpacity        float64 `json:"activeOpacity"`
	InactiveOpacity      float64 `json:"inactiveOpacity"`
	DimInactive          bool    `json:"dimInactive"`
	DimStrength          float64 `json:"dimStrength"`
	BlurEnabled          bool    `json:"blurEnabled"`
	BlurSize             int     `json:"blurSize"`
	BlurPasses           int     `json:"blurPasses"`
	BlurXray             bool    `json:"blurXray"`
	BlurVibrancy         float64 `json:"blurVibrancy"`
	BlurNoise            float64 `json:"blurNoise"`
	ShadowEnabled        bool    `json:"shadowEnabled"`
	ShadowRange          int     `json:"shadowRange"`
	ShadowPower          int     `json:"shadowPower"`
	GlowEnabled          bool    `json:"glowEnabled"`
	GlowRange            int     `json:"glowRange"`
	GlowColor            string  `json:"glowColor"`
	Animations           bool    `json:"animations"`
	Layout               string  `json:"layout"`
	ActiveBorder         string  `json:"activeBorder"`
	InactiveBorder       string  `json:"inactiveBorder"`
	BorderFollowsPalette bool    `json:"borderFollowsPalette"`
	ResizeOnBorder       bool    `json:"resizeOnBorder"`
	SnapEnabled          bool    `json:"snapEnabled"`
	WobblyWindows        bool    `json:"wobblyWindows"`
	WindowStyle          string  `json:"windowStyle"`
	AnimatedBorder       bool    `json:"animatedBorder"`
	BorderAngleSpeed     float64 `json:"borderAngleSpeed"`
	FullscreenOpacity    float64 `json:"fullscreenOpacity"`
	DimSpecial           float64 `json:"dimSpecial"`
	DimAround            float64 `json:"dimAround"`
	DimModal             bool    `json:"dimModal"`
	BorderPartOfWindow   bool    `json:"borderPartOfWindow"`
	BlurContrast         float64 `json:"blurContrast"`
	BlurBrightness       float64 `json:"blurBrightness"`
	BlurSpecial          bool    `json:"blurSpecial"`
	BlurPopups           bool    `json:"blurPopups"`
	BlurIgnoreOpacity    bool    `json:"blurIgnoreOpacity"`
	BlurNewOptimizations bool    `json:"blurNewOptimizations"`
	BlurVibrancyDarkness float64 `json:"blurVibrancyDarkness"`
	ShadowSharp          bool    `json:"shadowSharp"`
	ShadowScale          float64 `json:"shadowScale"`
	ShadowColor          string  `json:"shadowColor"`
	ExtendBorderGrab     int     `json:"extendBorderGrab"`
	HoverIconOnBorder    bool    `json:"hoverIconOnBorder"`
	NoFocusFallback      bool    `json:"noFocusFallback"`
	ResizeCorner         int     `json:"resizeCorner"`
	GapsWorkspaces       int     `json:"gapsWorkspaces"`
}

// Dwindle: dwindle layout keywords, emitted as a dwindle block diffed against
// the defaults.
type Dwindle struct {
	PreserveSplit      bool    `json:"preserveSplit"`
	SmartSplit         bool    `json:"smartSplit"`
	SmartResizing      bool    `json:"smartResizing"`
	DefaultSplitRatio  float64 `json:"defaultSplitRatio"`
	ForceSplit         string  `json:"forceSplit"` // follow | left/top | right/bottom
	UseActiveForSplits bool    `json:"useActiveForSplits"`
}

// Master: master layout keywords.
type Master struct {
	Mfact         float64 `json:"mfact"`
	NewStatus     string  `json:"newStatus"` // master | slave | inherit
	NewOnTop      bool    `json:"newOnTop"`
	Orientation   string  `json:"orientation"` // left | right | top | bottom | center
	SmartResizing bool    `json:"smartResizing"`
}

// Input: the input keyword plus the pointer-adjacent misc/gestures keys.
type Input struct {
	KbLayout           string  `json:"kbLayout"`
	KbVariant          string  `json:"kbVariant"`
	KbOptions          string  `json:"kbOptions"`
	NumlockByDefault   bool    `json:"numlockByDefault"`
	FollowMouse        int     `json:"followMouse"`
	Sensitivity        float64 `json:"sensitivity"`
	AccelProfile       string  `json:"accelProfile"`
	LeftHanded         bool    `json:"leftHanded"`
	MouseNaturalScroll bool    `json:"mouseNaturalScroll"`
	MouseScrollFactor  float64 `json:"mouseScrollFactor"`
	MiddleClickPaste   bool    `json:"middleClickPaste"`
	NaturalScroll      bool    `json:"naturalScroll"`
	TapToClick         bool    `json:"tapToClick"`
	TapAndDrag         bool    `json:"tapAndDrag"`
	Clickfinger        bool    `json:"clickfinger"`
	MiddleEmulation    bool    `json:"middleEmulation"`
	TouchScrollFactor  float64 `json:"touchScrollFactor"`
	DisableWhileTyping bool    `json:"disableWhileTyping"`
	RepeatRate         int     `json:"repeatRate"`
	RepeatDelay        int     `json:"repeatDelay"`
	WorkspaceSwipe     bool    `json:"workspaceSwipe"`
	SwipeFingers       int     `json:"swipeFingers"`
	SwipeInvert        bool    `json:"swipeInvert"`
	SwipeCreateNew     bool    `json:"swipeCreateNew"`
	SwipeDistance      int     `json:"swipeDistance"`
}

// Cursor: theme + size + the cursor-section niceties.
type Cursor struct {
	Theme           string `json:"theme"`
	Size            int    `json:"size"`
	InactiveTimeout int    `json:"inactiveTimeout"`
	HideOnKeyPress  bool   `json:"hideOnKeyPress"`
}

type EnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Windows is the neutral window-behaviour block. TameMaximizeOnOpen keeps a
// window an app opens maximised inside the gaps instead of edge to edge;
// Hyprland does it by refusing the client's maximise request outright.
type Windows struct {
	TameMaximizeOnOpen bool `json:"tameMaximizeOnOpen"`
}

// WindowRule = one user rule: optional class/title match + one action.
type WindowRule struct {
	Class  string `json:"class"`
	Title  string `json:"title"`
	Action string `json:"action"`
	Value  string `json:"value"`
}

// LayerRule: target a layer-shell surface by namespace.
type LayerRule struct {
	Namespace string `json:"namespace"`
	Action    string `json:"action"`
	Value     string `json:"value"`
}

// AppOverride: per-app appearance overrides. Numeric fields use -1 for
// "inherit"; the toggles use "inherit" plus "off" (or "on" for opaque).
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

type Autostart struct {
	Command string `json:"command"`
}

// Keybind = a user shortcut. action "exec" runs Value; the dispatcher actions
// take no value.
type Keybind struct {
	Keys    string `json:"keys"`
	Action  string `json:"action"`
	Value   string `json:"value"`
	Release bool   `json:"release,omitempty"`
}

// AnimCurve = a user bezier; only the two control points are stored.
type AnimCurve struct {
	Name string  `json:"name"`
	X0   float64 `json:"x0"`
	Y0   float64 `json:"y0"`
	X1   float64 `json:"x1"`
	Y1   float64 `json:"y1"`
}

// AnimItem: override one animation leaf.
type AnimItem struct {
	Leaf    string  `json:"leaf"`
	Enabled bool    `json:"enabled"`
	Speed   float64 `json:"speed"`
	Bezier  string  `json:"bezier"`
	Style   string  `json:"style"`
}

// Anim: curves first (items may reference them), items second.
type Anim struct {
	Items  []AnimItem  `json:"items"`
	Curves []AnimCurve `json:"curves"`
}

type DynamicCursors struct {
	Enabled bool    `json:"enabled"`
	Mode    string  `json:"mode"` // rotate | tilt | stretch
	Shake   bool    `json:"shake"`
	Magnify float64 `json:"magnify"`
}

type Hyprbars struct {
	Enabled  bool `json:"enabled"`
	Height   int  `json:"height"`
	TextSize int  `json:"textSize"`
	Blur     bool `json:"blur"`
	Buttons  bool `json:"buttons"`
}

type Imgborders struct {
	Enabled bool    `json:"enabled"`
	Image   string  `json:"image"`
	Sizes   string  `json:"sizes"`
	Insets  string  `json:"insets"`
	Scale   float64 `json:"scale"`
	Smooth  bool    `json:"smooth"`
	Blur    bool    `json:"blur"`
}

type Hyprglass struct {
	Enabled      bool    `json:"enabled"`
	Preset       string  `json:"preset"`
	BlurStrength float64 `json:"blurStrength"`
	Opacity      float64 `json:"opacity"`
	Tint         string  `json:"tint"`
	Brightness   float64 `json:"brightness"`
	Theme        string  `json:"theme"`
}

type Hyprfocus struct {
	Enabled bool    `json:"enabled"`
	Mode    string  `json:"mode"`
	Opacity float64 `json:"opacity"`
	Bounce  float64 `json:"bounce"`
	Slide   float64 `json:"slide"`
}

// Hyprscrolling is Hyprland core (0.54+) config, not a plugin: it applies
// whenever the tiling layout is "scrolling".
type Hyprscrolling struct {
	ColumnWidth float64 `json:"columnWidth"`
	FollowFocus bool    `json:"followFocus"`
}

// Keysounds is Ryoku's own plugin.
type Keysounds struct {
	Enabled bool    `json:"enabled"`
	Profile string  `json:"profile"`
	Volume  float64 `json:"volume"`
	Release bool    `json:"release"`
}

// ExtraPlugin is a plugin the user added from a git repository: its config keys
// are stored by their full `plugin:` path with the type the control produced.
type ExtraPlugin struct {
	Enabled bool           `json:"enabled"`
	Config  map[string]any `json:"config,omitempty"`
}

type Plugins struct {
	DynamicCursors DynamicCursors         `json:"dynamicCursors"`
	Hyprbars       Hyprbars               `json:"hyprbars"`
	Imgborders     Imgborders             `json:"imgborders"`
	Hyprglass      Hyprglass              `json:"hyprglass"`
	Hyprfocus      Hyprfocus              `json:"hyprfocus"`
	Keysounds      Keysounds              `json:"keysounds"`
	Hyprscrolling  Hyprscrolling          `json:"hyprscrolling"`
	Extra          map[string]ExtraPlugin `json:"extra,omitempty"`
}

type Overrides struct {
	Appearance     Appearance        `json:"appearance"`
	Dwindle        Dwindle           `json:"dwindle"`
	Master         Master            `json:"master"`
	Input          Input             `json:"input"`
	Cursor         Cursor            `json:"cursor"`
	Windows        Windows           `json:"windows"`
	Env            []EnvVar          `json:"env"`
	WindowRules    []WindowRule      `json:"windowRules"`
	Autostart      []Autostart       `json:"autostart"`
	Keybinds       []Keybind         `json:"keybinds"`
	Anim           Anim              `json:"anim"`
	LayerRules     []LayerRule       `json:"layerRules"`
	AppOverrides   []AppOverride     `json:"appOverrides"`
	Plugins        Plugins           `json:"plugins"`
	Apps           map[string]string `json:"apps,omitempty"`
	KeybindRebinds map[string]string `json:"keybindRebinds,omitempty"`

	// Unbinds: default chords a config import must drop so an imported bind
	// shadowing a shipped one wins. Rendered before the keybinds.
	Unbinds []string `json:"unbinds,omitempty"`

	// inputSaved: the store carries an explicit input section, so genConfig pins
	// the kb_* keys unconditionally (keyboard.lua loads earlier and would win).
	inputSaved bool
}

// defaultOverrides mirrors the shipped Hyprland modules so a diff against it
// writes only real divergences.
func defaultOverrides() Overrides {
	return Overrides{
		Appearance: Appearance{
			GapsIn: 12, GapsOut: 18, BorderSize: 2, Rounding: 0, RoundingPower: 4,
			ActiveOpacity: 1, InactiveOpacity: 0.94,
			DimInactive: false, DimStrength: 0.5,
			BlurEnabled: true, BlurSize: 4, BlurPasses: 1,
			BlurXray: false, BlurVibrancy: 0.17, BlurNoise: 0.01,
			ShadowEnabled: true, ShadowRange: 45, ShadowPower: 4,
			GlowEnabled: false, GlowRange: 10, GlowColor: "#ee33cc",
			Animations: true, Layout: "dwindle",
			ActiveBorder: "#e0563b", InactiveBorder: "#313a4d", BorderFollowsPalette: true,
			ResizeOnBorder: true, SnapEnabled: false,
			WobblyWindows: false, WindowStyle: "pop",
			AnimatedBorder: false, BorderAngleSpeed: 3,
			FullscreenOpacity: 1, DimSpecial: 0.2, DimAround: 0.4,
			DimModal: true, BorderPartOfWindow: true,
			BlurContrast: 0.8916, BlurBrightness: 1, BlurSpecial: false, BlurPopups: false,
			BlurIgnoreOpacity: true, BlurNewOptimizations: true, BlurVibrancyDarkness: 0,
			ShadowSharp: false, ShadowScale: 1, ShadowColor: "#000000",
			ExtendBorderGrab: 15, HoverIconOnBorder: true, NoFocusFallback: false,
			ResizeCorner: 0, GapsWorkspaces: 0,
		},
		Dwindle: Dwindle{PreserveSplit: false, SmartSplit: false, SmartResizing: true, DefaultSplitRatio: 1, ForceSplit: "follow", UseActiveForSplits: true},
		Master:  Master{Mfact: 0.55, NewStatus: "slave", NewOnTop: false, Orientation: "left", SmartResizing: true},
		Input: Input{
			KbLayout: "us", KbVariant: "", KbOptions: "", NumlockByDefault: false,
			FollowMouse: 1, Sensitivity: 0, AccelProfile: "",
			LeftHanded: false, MouseNaturalScroll: false, MouseScrollFactor: 1,
			MiddleClickPaste: true,
			NaturalScroll:    false, TapToClick: true, TapAndDrag: true,
			Clickfinger: false, MiddleEmulation: false, TouchScrollFactor: 1,
			DisableWhileTyping: true,
			RepeatRate:         25, RepeatDelay: 600,
			WorkspaceSwipe: false, SwipeFingers: 3,
			SwipeInvert: true, SwipeCreateNew: true, SwipeDistance: 300,
		},
		Cursor:       Cursor{Theme: "Bibata-Modern-Ice", Size: 24, InactiveTimeout: 0, HideOnKeyPress: false},
		Windows:      Windows{TameMaximizeOnOpen: true},
		Env:          []EnvVar{},
		WindowRules:  []WindowRule{},
		Autostart:    []Autostart{},
		Keybinds:     []Keybind{},
		Anim:         Anim{Items: []AnimItem{}, Curves: []AnimCurve{}},
		LayerRules:   []LayerRule{},
		AppOverrides: []AppOverride{},
		Plugins: Plugins{
			DynamicCursors: DynamicCursors{Enabled: false, Mode: "tilt", Shake: true, Magnify: 4.0},
			Hyprbars:       Hyprbars{Enabled: false, Height: 26, TextSize: 11, Blur: true, Buttons: true},
			Imgborders:     Imgborders{Enabled: false, Image: "", Sizes: "8,8,8,8", Insets: "0,0,0,0", Scale: 1.0, Smooth: true, Blur: false},
			Hyprglass:      Hyprglass{Enabled: false, Preset: "clear", BlurStrength: 2.0, Opacity: 1.0, Tint: "8899aa22", Brightness: 1.0, Theme: "dark"},
			Hyprfocus:      Hyprfocus{Enabled: false, Mode: "flash", Opacity: 0.8, Bounce: 0.95, Slide: 20},
			Keysounds:      Keysounds{Enabled: false, Profile: "cherry-mx-brown", Volume: 0.6, Release: true},
			Hyprscrolling:  Hyprscrolling{ColumnWidth: 0.5, FollowFocus: true},
		},
	}
}

// wmHyprlandFields are the top-level Overrides fields with no neutral meaning:
// they persist under wm.hyprland.*, untouched while another compositor is
// active. Everything else is desktop.* intent every compositor honours. This is
// the one split table, driving both the reader and splitStore.
var wmHyprlandFields = map[string]bool{
	"dwindle":    true,
	"master":     true,
	"anim":       true,
	"layerRules": true,
	"plugins":    true,
}

// neutralStore is desktop.json on disk: { "desktop": {...}, "wm": { "hyprland":
// {...} } }. Keys under wm.<other> are preserved by whoever wrote them; this
// provider only ever reads desktop and wm.hyprland.
type neutralStore struct {
	Desktop map[string]json.RawMessage `json:"desktop"`
	WM      struct {
		Hyprland map[string]json.RawMessage `json:"hyprland"`
	} `json:"wm"`
}

// loadStore reads desktop.json into the override model with defaults filled. The
// merge is namespace-agnostic, so a key under the wrong namespace is still
// honoured; the split only matters for a compositor that ignores wm.hyprland.
func loadStore(path string) Overrides {
	o := defaultOverrides()
	ns, ok := readNeutralStore(path)
	if !ok {
		return normalizeOverrides(o)
	}
	flat := map[string]json.RawMessage{}
	for k, v := range ns.Desktop {
		flat[k] = v
	}
	for k, v := range ns.WM.Hyprland {
		flat[k] = v
	}
	if v, ok := flat["input"]; ok && len(v) > 0 && string(v) != "null" {
		o.inputSaved = true
	}
	if merged, err := json.Marshal(flat); err == nil {
		_ = json.Unmarshal(merged, &o)
	}
	o.Cursor.Theme = wm.ResolveCursorTheme(o.Cursor.Theme)
	return normalizeOverrides(o)
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

// normalizeOverrides replaces the nil slices a partial store leaves behind so
// the generators never range a nil.
func normalizeOverrides(o Overrides) Overrides {
	if o.Env == nil {
		o.Env = []EnvVar{}
	}
	if o.WindowRules == nil {
		o.WindowRules = []WindowRule{}
	}
	if o.AppOverrides == nil {
		o.AppOverrides = []AppOverride{}
	}
	if o.Autostart == nil {
		o.Autostart = []Autostart{}
	}
	if o.Keybinds == nil {
		o.Keybinds = []Keybind{}
	}
	if o.Anim.Items == nil {
		o.Anim.Items = []AnimItem{}
	}
	if o.Anim.Curves == nil {
		o.Anim.Curves = []AnimCurve{}
	}
	if o.LayerRules == nil {
		o.LayerRules = []LayerRule{}
	}
	return o
}

// splitStore renders an Overrides as the namespaced desktop.json tree.
func splitStore(o Overrides) (neutralStore, error) {
	b, err := json.Marshal(o)
	if err != nil {
		return neutralStore{}, err
	}
	var flat map[string]json.RawMessage
	if err := json.Unmarshal(b, &flat); err != nil {
		return neutralStore{}, err
	}
	var ns neutralStore
	ns.Desktop = map[string]json.RawMessage{}
	ns.WM.Hyprland = map[string]json.RawMessage{}
	for k, v := range flat {
		if wmHyprlandFields[k] {
			ns.WM.Hyprland[k] = v
		} else {
			ns.Desktop[k] = v
		}
	}
	return ns, nil
}

// --- paths ----------------------------------------------------------------

func configHome() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return base
}

func ryokuConfigDir() string { return filepath.Join(configHome(), "ryoku") }

// desktopStorePath is the neutral settings store the Hub writes and the provider
// reads.
func desktopStorePath() string { return filepath.Join(ryokuConfigDir(), "desktop.json") }

func hyprConfigDir() string { return filepath.Join(configHome(), "hypr") }

// userEditsHyprDir is the hypr slice of the user overlay tree. The generated Lua
// lives here so it survives an update as a user edit; writeOverlayLua reflects
// the same bytes into the live hypr dir so a reload picks them up at once.
func userEditsHyprDir() string {
	return filepath.Join(ryokuConfigDir(), "user_edits", "hypr")
}

func shellStorePath() string { return filepath.Join(ryokuConfigDir(), "shell.json") }

func themeStatePath() string { return filepath.Join(ryokuConfigDir(), "theme.json") }

// --- theme state ----------------------------------------------------------

// paletteDriven reports whether the window colours come from a live palette (the
// wallpaper-follow master or a fixed named scheme) rather than the user's fixed
// border choice. When true the generated config omits the solid col.active_border
// so decoration.lua's palette border wins.
func paletteDriven() bool {
	return loadFollowWallpaper() || staticThemeActive()
}

// borderFollowsPalette reports whether the window border should track the live
// palette: the theme drives colours (paletteDriven) AND the user has not pinned
// a fixed border colour in the store. A false store value pins the fixed
// col.active_border in settings.lua even while the rest of the theme follows the
// wallpaper, so a chosen border colour is exactly what the user gets.
func borderFollowsPalette(o Overrides) bool {
	return paletteDriven() && o.Appearance.BorderFollowsPalette
}

// loadFollowWallpaper reads theme.json's master, defaulting to follow on a
// missing or blank file (the shipped default look).
func loadFollowWallpaper() bool {
	s := struct {
		FollowWallpaper bool `json:"followWallpaper"`
	}{FollowWallpaper: true}
	if b, err := os.ReadFile(themeStatePath()); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s.FollowWallpaper
}

// staticThemeActive reports whether shell.json names a fixed catalog palette
// (not the dynamic Default/Wallpaper variants), whose palette drives the border.
func staticThemeActive() bool {
	b, err := os.ReadFile(shellStorePath())
	if err != nil {
		return false
	}
	var s struct {
		Theme struct {
			Theme string `json:"theme"`
		} `json:"theme"`
	}
	if json.Unmarshal(b, &s) != nil {
		return false
	}
	switch s.Theme.Theme {
	case "", "Default", "Wallpaper":
		return false
	}
	return true
}

// --- disk + lua helpers ---------------------------------------------------

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

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

// writeOverlayLua authors a generated file in the user_edits tree (kept across
// updates) and reflects the same bytes into the live hypr dir so a reload picks
// them up without a full materialize.
func writeOverlayLua(name string, body []byte) error {
	if err := atomicWrite(filepath.Join(userEditsHyprDir(), name), body, 0o644); err != nil {
		return err
	}
	return atomicWrite(filepath.Join(hyprConfigDir(), name), body, 0o644)
}

func luaStr(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", "\\n", "\r", "\\r")
	return "\"" + r.Replace(s) + "\""
}

func luaNum(f float64) string {
	s := fmt.Sprintf("%g", f)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// luaRGB: "#rrggbb" (or "rrggbb") -> Hyprland's rgb(rrggbb) form.
func luaRGB(hex string) string {
	h := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	return "rgb(" + h + ")"
}

func parseFloat(s string, fallback float64) float64 {
	var f float64
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%g", &f); err != nil {
		return fallback
	}
	return f
}

// parseWxH: "1500x850" (or "1500 850" / "1500,850") -> two ints.
func parseWxH(s string) (int, int) {
	s = strings.NewReplacer("x", " ", "X", " ", ",", " ").Replace(s)
	var a, b int
	fmt.Sscan(s, &a, &b)
	return a, b
}
