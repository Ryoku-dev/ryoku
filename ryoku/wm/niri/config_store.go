package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	wm "ryoku-wm"
)

// The neutral settings store as niri reads it: the desktop.* fields niri can
// express, the wm.niri.* exclusives that have no neutral key, the desktop.json
// codec that fills them, and the niri defaults the Hub overlays user values on.
// Only the leaves niri honours are modelled; apply reports everything else in
// the store as unhonored rather than parsing it.
//
// settings.kdl carries the full effective config, not a diff, because the shipped
// niri tree has no defaults module: config.kdl plus five comment-only seeds, so
// settings.kdl and rebinds.kdl are the only writers of real config. A leaf left
// out here would boot as stock niri, not Ryoku.

// Appearance: the window-frame, gap, shadow and opacity leaves niri can express.
// Window blur is deliberately absent: niri 26.04's forced background-effect
// renders translucent windows opaque instead of frosted on real hardware, so
// Ryoku does not model it and the neutral blur keys are reported unhonored. The
// rest of the neutral appearance model (glow, per-window dim) has no niri setting
// and is reported unhonored too.
type Appearance struct {
	GapsIn         int    `json:"gapsIn"`
	GapsOut        int    `json:"gapsOut"`
	BorderSize     int    `json:"borderSize"`
	Rounding       int    `json:"rounding"`
	ActiveBorder   string `json:"activeBorder"`
	InactiveBorder string `json:"inactiveBorder"`
	// BorderFollowsPalette: true takes the border's active and inactive colours
	// from the live palette (the file the border act records), false uses the
	// ActiveBorder/InactiveBorder above verbatim. The niri twin of Hyprland's
	// same neutral key, so the Hub toggle reaches both compositors.
	BorderFollowsPalette bool    `json:"borderFollowsPalette"`
	Animations           bool    `json:"animations"`
	ActiveOpacity        float64 `json:"activeOpacity"`
	InactiveOpacity      float64 `json:"inactiveOpacity"`
	ShadowEnabled        bool    `json:"shadowEnabled"`
	ShadowRange          int     `json:"shadowRange"`
	ShadowColor          string  `json:"shadowColor"`
	ShadowSpread         int     `json:"shadowSpread"`
	ShadowOffsetX        int     `json:"shadowOffsetX"`
	ShadowOffsetY        int     `json:"shadowOffsetY"`
}

// Input: the keyboard, pointer and touchpad leaves niri's input block covers.
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
	TouchScrollFactor  float64 `json:"touchScrollFactor"`
	TapToClick         bool    `json:"tapToClick"`
	TapAndDrag         bool    `json:"tapAndDrag"`
	Clickfinger        bool    `json:"clickfinger"`
	MiddleEmulation    bool    `json:"middleEmulation"`
	DisableWhileTyping bool    `json:"disableWhileTyping"`
	RepeatRate         int     `json:"repeatRate"`
	RepeatDelay        int     `json:"repeatDelay"`
}

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

// WindowRule = one user rule: optional app-id/title match + one action.
type WindowRule struct {
	Class  string `json:"class"`
	Title  string `json:"title"`
	Action string `json:"action"`
	Value  string `json:"value"`
}

// AppOverride: per-app appearance. Numeric fields use -1 for "inherit"; niri can
// express opacity, corner radius, border width and forced blur, nothing else
// here.
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

// Keybind = a user shortcut. action "exec" runs Value; the window actions take no
// value.
type Keybind struct {
	Keys    string `json:"keys"`
	Action  string `json:"action"`
	Value   string `json:"value"`
	Release bool   `json:"release,omitempty"`
}

// Windows is the neutral window-behaviour block. TameMaximizeOnOpen resets a
// window an app opens maximised back to an ordinary tile with the gaps kept, so
// it never lands edge to edge; off, apps open themselves maximised.
type Windows struct {
	TameMaximizeOnOpen bool `json:"tameMaximizeOnOpen"`
}

type Struts struct {
	Left   int `json:"left"`
	Right  int `json:"right"`
	Top    int `json:"top"`
	Bottom int `json:"bottom"`
}

// Proportions is a list of column-width proportions. It carries as a plain
// comma-separated string ("0.33, 0.5, 0.67") rather than a JSON array so the Hub
// edits it as one text field and compares it as one scalar; niri still gets a
// proportion line per entry. Unmarshal also tolerates the array form and a
// trailing percent, so an older store or a "50%" entry still parses.
type Proportions []float64

func (p Proportions) MarshalJSON() ([]byte, error) {
	parts := make([]string, len(p))
	for i, v := range p {
		parts[i] = kdlNum(v)
	}
	return json.Marshal(strings.Join(parts, ", "))
}

func (p *Proportions) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*p = nil
		return nil
	}
	if b[0] == '[' {
		var nums []float64
		if err := json.Unmarshal(b, &nums); err != nil {
			return err
		}
		*p = nums
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*p = parseProportions(s)
	return nil
}

func parseProportions(s string) Proportions {
	var out Proportions
	for _, tok := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
		scale := 1.0
		if strings.HasSuffix(tok, "%") {
			tok = strings.TrimSuffix(tok, "%")
			scale = 0.01
		}
		if v, err := strconv.ParseFloat(tok, 64); err == nil {
			out = append(out, v*scale)
		}
	}
	return out
}

// AnimSpec is one niri animation's tuning: a mode plus the easing and spring
// fields the chosen mode reads. Mode "default" emits nothing so niri keeps its
// own animation for that kind; "off" disables it; "ease" and "spring" carry the
// two families niri's parser accepts.
type AnimSpec struct {
	Mode         string  `json:"mode"`
	DurationMs   int     `json:"durationMs"`
	Curve        string  `json:"curve"`
	DampingRatio float64 `json:"dampingRatio"`
	Stiffness    int     `json:"stiffness"`
	Epsilon      float64 `json:"epsilon"`
}

// NiriAnim is the per-kind animation tree. The kinds mirror niri's animation
// nodes one for one, so a Hub row named wm.niri.anim.<kind>.<field> reaches the
// right block.
type NiriAnim struct {
	WorkspaceSwitch             AnimSpec `json:"workspaceSwitch"`
	WindowOpen                  AnimSpec `json:"windowOpen"`
	WindowClose                 AnimSpec `json:"windowClose"`
	HorizontalViewMovement      AnimSpec `json:"horizontalViewMovement"`
	WindowMovement              AnimSpec `json:"windowMovement"`
	WindowResize                AnimSpec `json:"windowResize"`
	ConfigNotificationOpenClose AnimSpec `json:"configNotificationOpenClose"`
	ScreenshotUiOpen            AnimSpec `json:"screenshotUiOpen"`
	OverviewOpenClose           AnimSpec `json:"overviewOpenClose"`
}

// LayerRule is one user layer-shell rule: a namespace match plus the fields niri
// can set on a matching surface. Opacity -1 and cornerRadius -1 leave niri's own
// value alone; blur and shadow "inherit" do the same.
type LayerRule struct {
	Namespace    string  `json:"namespace"`
	Opacity      float64 `json:"opacity"`
	CornerRadius int     `json:"cornerRadius"`
	Blur         string  `json:"blur"`
	Shadow       string  `json:"shadow"`
	BlockOut     bool    `json:"blockOut"`
	BabaIsFloat  bool    `json:"babaIsFloat"`
}

// Niri holds the wm.niri.* exclusives: niri behaviours with no neutral key. This
// is the niri twin of wm.hyprland.*, so the Hub can surface them without every
// other compositor pretending to have them.
type Niri struct {
	PreferNoCSD          bool        `json:"preferNoCsd"`
	HotkeyOverlaySkip    bool        `json:"hotkeyOverlaySkip"`
	HotkeyOverlayHide    bool        `json:"hotkeyOverlayHideNotBound"`
	ScreenshotPath       string      `json:"screenshotPath"`
	DefaultColumnWidth   float64     `json:"defaultColumnWidth"`
	PresetColumnWidths   Proportions `json:"presetColumnWidths"`
	PresetWindowHeights  Proportions `json:"presetWindowHeights"`
	DefaultColumnDisplay string      `json:"defaultColumnDisplay"`
	CenterFocused        string      `json:"centerFocusedColumn"`
	AlwaysCenterSingle   bool        `json:"alwaysCenterSingleColumn"`
	EmptyWorkspaceAbove  bool        `json:"emptyWorkspaceAboveFirst"`
	BackgroundColor      string      `json:"backgroundColor"`
	Frame                string      `json:"frame"`
	BorderGradient       bool        `json:"borderGradient"`
	GradientFrom         string      `json:"gradientFrom"`
	GradientTo           string      `json:"gradientTo"`
	GradientAngle        int         `json:"gradientAngle"`
	GradientRelativeTo   string      `json:"gradientRelativeTo"`
	UrgentColor          string      `json:"urgentColor"`
	TabIndicatorWidth    int         `json:"tabIndicatorWidth"`
	TabIndicatorHide     bool        `json:"tabIndicatorHideSingle"`
	InsertHint           bool        `json:"insertHint"`
	AnimationSlowdown    float64     `json:"animationSlowdown"`
	Anim                 NiriAnim    `json:"anim"`
	BlockOutApps         string      `json:"blockOutApps"`
	Struts               Struts      `json:"struts"`
	HotCorners           bool        `json:"hotCorners"`
	OverviewZoom         float64     `json:"overviewZoom"`
	BackdropColor        string      `json:"backdropColor"`
	WorkspaceShadow      bool        `json:"workspaceShadow"`
	WorkspaceShadowSoft  int         `json:"workspaceShadowSoftness"`
	WorkspaceShadowSprd  int         `json:"workspaceShadowSpread"`
	WorkspaceShadowY     int         `json:"workspaceShadowOffsetY"`
	WorkspaceShadowColor string      `json:"workspaceShadowColor"`
	RecentWindows        bool        `json:"recentWindows"`
	WarpMouseToFocus     string      `json:"warpMouseToFocus"`
	FocusFollowScroll    int         `json:"focusFollowsMouseScroll"`
	WorkspaceBackForth   bool        `json:"workspaceAutoBackAndForth"`
	ModKey               string      `json:"modKey"`
	ModKeyNested         string      `json:"modKeyNested"`
	DisablePowerKey      bool        `json:"disablePowerKey"`
	DndViewTriggerWidth  int         `json:"dndEdgeViewScrollTriggerWidth"`
	DndViewDelayMs       int         `json:"dndEdgeViewScrollDelayMs"`
	DndViewMaxSpeed      int         `json:"dndEdgeViewScrollMaxSpeed"`
	DndWsTriggerHeight   int         `json:"dndEdgeWorkspaceTriggerHeight"`
	DndWsDelayMs         int         `json:"dndEdgeWorkspaceDelayMs"`
	DndWsMaxSpeed        int         `json:"dndEdgeWorkspaceMaxSpeed"`
	LayerRules           []LayerRule `json:"layerRules"`
}

// niriStore is the typed store the generator consumes. Niri stays out of the flat
// desktop marshalling (json:"-") because splitStore routes it under wm.niri.
type niriStore struct {
	Appearance     Appearance        `json:"appearance"`
	Input          Input             `json:"input"`
	Cursor         Cursor            `json:"cursor"`
	Windows        Windows           `json:"windows"`
	Env            []EnvVar          `json:"env"`
	WindowRules    []WindowRule      `json:"windowRules"`
	AppOverrides   []AppOverride     `json:"appOverrides"`
	Autostart      []Autostart       `json:"autostart"`
	Keybinds       []Keybind         `json:"keybinds"`
	Apps           map[string]string `json:"apps,omitempty"`
	KeybindRebinds map[string]string `json:"keybindRebinds,omitempty"`
	Unbinds        []string          `json:"unbinds,omitempty"`
	Niri           Niri              `json:"-"`
}

// neutralStore is desktop.json on disk: { "desktop": {...}, "wm": { "<name>":
// {...} } }. WM is keyed by compositor so apply can see, and preserve, another
// compositor's exclusives without parsing them.
type neutralStore struct {
	Desktop map[string]json.RawMessage `json:"desktop"`
	WM      map[string]json.RawMessage `json:"wm"`
}

// defaultAnim is the neutral baseline for one animation kind. Mode "default"
// leaves niri's own animation in place; the easing and spring fields carry
// middle-of-the-road values so a user switching to "ease" or "spring" starts
// from something sensible rather than zero.
func defaultAnim() AnimSpec {
	return AnimSpec{
		Mode:         "default",
		DurationMs:   250,
		Curve:        "ease-out-cubic",
		DampingRatio: 1.0,
		Stiffness:    800,
		Epsilon:      0.0001,
	}
}

// defaultStore is niri's baseline. It differs from Hyprland where niri has its
// own opinion (gaps 16, border width 4, no rounding); the pointer and keyboard
// leaves match the recommended niri session; the wm.niri block is the Ryoku look
// for the settings niri owns alone.
func defaultStore() niriStore {
	return niriStore{
		Appearance: Appearance{
			GapsIn: 16, GapsOut: 16, BorderSize: 4, Rounding: 0,
			ActiveBorder: "#e0563b", InactiveBorder: "#313a4d", BorderFollowsPalette: true, Animations: true,
			ActiveOpacity: 1, InactiveOpacity: 1,
			ShadowEnabled: true, ShadowRange: 45, ShadowColor: "#000000",
			ShadowSpread: 0, ShadowOffsetX: 0, ShadowOffsetY: 5,
		},
		Windows: Windows{TameMaximizeOnOpen: true},
		Input: Input{
			KbLayout: "us", NumlockByDefault: false, FollowMouse: 1,
			Sensitivity: 0, AccelProfile: "", LeftHanded: false,
			MouseNaturalScroll: false, MouseScrollFactor: 1, MiddleClickPaste: true,
			NaturalScroll: true, TouchScrollFactor: 1,
			TapToClick: true, TapAndDrag: true, Clickfinger: false,
			MiddleEmulation: false, DisableWhileTyping: true,
			RepeatRate: 25, RepeatDelay: 600,
		},
		Cursor:       Cursor{Theme: "Bibata-Modern-Ice", Size: 24, InactiveTimeout: 0, HideOnKeyPress: false},
		Env:          []EnvVar{},
		WindowRules:  []WindowRule{},
		AppOverrides: []AppOverride{},
		Autostart:    []Autostart{},
		Keybinds:     []Keybind{},
		Niri: Niri{
			PreferNoCSD:          true,
			HotkeyOverlaySkip:    true,
			HotkeyOverlayHide:    false,
			ScreenshotPath:       "~/Pictures/Screenshots/Screenshot from %Y-%m-%d %H-%M-%S.png",
			DefaultColumnWidth:   0.5,
			PresetColumnWidths:   []float64{0.33333, 0.5, 0.66667},
			PresetWindowHeights:  []float64{0.33333, 0.5, 0.66667},
			DefaultColumnDisplay: "normal",
			CenterFocused:        "never",
			AlwaysCenterSingle:   true,
			EmptyWorkspaceAbove:  false,
			BackgroundColor:      "",
			Frame:                "border",
			BorderGradient:       false,
			GradientFrom:         "#e0563b",
			GradientTo:           "#9b3226",
			GradientAngle:        180,
			GradientRelativeTo:   "window",
			UrgentColor:          "#9b0000",
			TabIndicatorWidth:    4,
			TabIndicatorHide:     true,
			InsertHint:           true,
			AnimationSlowdown:    1,
			Anim: NiriAnim{
				WorkspaceSwitch:             defaultAnim(),
				WindowOpen:                  defaultAnim(),
				WindowClose:                 defaultAnim(),
				HorizontalViewMovement:      defaultAnim(),
				WindowMovement:              defaultAnim(),
				WindowResize:                defaultAnim(),
				ConfigNotificationOpenClose: defaultAnim(),
				ScreenshotUiOpen:            defaultAnim(),
				OverviewOpenClose:           defaultAnim(),
			},
			BlockOutApps:         "",
			Struts:               Struts{},
			HotCorners:           false,
			OverviewZoom:         0.5,
			BackdropColor:        "",
			WorkspaceShadow:      true,
			WorkspaceShadowSoft:  40,
			WorkspaceShadowSprd:  10,
			WorkspaceShadowY:     10,
			WorkspaceShadowColor: "",
			RecentWindows:        true,
			WarpMouseToFocus:     "off",
			FocusFollowScroll:    0,
			WorkspaceBackForth:   false,
			ModKey:               "Super",
			ModKeyNested:         "Super",
			DisablePowerKey:      false,
			DndViewTriggerWidth:  30,
			DndViewDelayMs:       100,
			DndViewMaxSpeed:      1500,
			DndWsTriggerHeight:   50,
			DndWsDelayMs:         100,
			DndWsMaxSpeed:        1500,
			LayerRules:           []LayerRule{},
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

// loadStore fills the niri defaults, then overlays the store's desktop.* and
// wm.niri.* leaves. An absent leaf keeps its default, so the generated config is
// always the full effective look, not a partial one.
func loadStore(path string) niriStore {
	s := defaultStore()
	ns, ok := readNeutralStore(path)
	if !ok {
		return s
	}
	if b, err := json.Marshal(ns.Desktop); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	s.Cursor.Theme = wm.ResolveCursorTheme(s.Cursor.Theme)
	if raw, ok := ns.WM["niri"]; ok {
		_ = json.Unmarshal(raw, &s.Niri)
	}
	// A preset-width cycle needs at least two widths; with one, Super+R has
	// nothing to step to and does nothing. Fall back to the default so the bind
	// always works. unhonored names the substitution so the user sees it.
	if len(s.Niri.PresetColumnWidths) < 2 {
		s.Niri.PresetColumnWidths = defaultStore().Niri.PresetColumnWidths
	}
	return s
}

// splitStore renders the store as the namespaced desktop.json tree: desktop.* is
// the flat neutral fields, wm.niri.* is the exclusives, so the Hub learns both
// the namespace name and its sections from the defaults it reads.
func splitStore(s niriStore) (map[string]any, error) {
	db, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var desktop map[string]json.RawMessage
	if err := json.Unmarshal(db, &desktop); err != nil {
		return nil, err
	}
	nb, err := json.Marshal(s.Niri)
	if err != nil {
		return nil, err
	}
	var niri map[string]json.RawMessage
	if err := json.Unmarshal(nb, &niri); err != nil {
		return nil, err
	}
	return map[string]any{
		"desktop": desktop,
		"wm":      map[string]any{"niri": niri},
	}, nil
}

// --- paths ----------------------------------------------------------------

func configHome() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	return filepath.Join(os.Getenv("HOME"), ".config")
}

// storePath is the neutral settings store this provider reads for the runtime
// behaviours that never reach the config file, like taming a maximised open.
func storePath() string {
	return filepath.Join(configHome(), "ryoku", "desktop.json")
}

// borderPalettePath is where the border act records the live palette's active
// and inactive colours. writeFrame reads it so the border tracks the wallpaper
// the way Hyprland's decoration.lua does; the last session's file is still there
// at login, so the first render is already themed rather than the store colour.
func borderPalettePath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "ryoku", "niri-border-palette.json")
}

// borderPaletteColors reads the active and inactive colours the border act last
// wrote from the live palette. ok is false when the file is absent or holds no
// usable colour, so writeFrame falls back to the store colours.
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
	if kdlColor(p.Active) == "" && kdlColor(p.Inactive) == "" {
		return "", "", false
	}
	return p.Active, p.Inactive, true
}

// userEditsNiriDir is the niri slice of the user overlay tree. The generated KDL
// lives here so it survives an update as a user edit; writeOverlayKdl reflects
// the same bytes into the live niri dir so a reload picks them up at once.
func userEditsNiriDir() string {
	return filepath.Join(configHome(), "ryoku", "user_edits", "niri")
}

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

// writeOverlayKdl authors a generated file in the user_edits tree first, then
// reflects the same bytes into the live niri dir. The order matters: if the live
// write fails the overlay still holds the new content, so a materialize re-lays
// it rather than resurrecting the old one.
func writeOverlayKdl(name string, body []byte) error {
	if err := atomicWrite(filepath.Join(userEditsNiriDir(), name), body, 0o644); err != nil {
		return err
	}
	return atomicWrite(filepath.Join(niriConfigDir(), name), body, 0o644)
}
