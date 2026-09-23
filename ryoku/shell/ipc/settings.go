package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	wm "ryoku-wm"
)

// settings.go is the shell's typed configuration, owned by the daemon and served
// to QML and Ryoku Hub over the "settings" state topic. It is the single writer
// of ~/.config/ryoku/shell.json: readers subscribe for a live frame, and every
// change is a settings.patch call, so two writers can never race on the file (a
// setting silently reverting was the hazard of the Hub and the daemon both
// writing it directly).
//
// The daemon owns a typed schema for six top-level namespaces (general, theme,
// bars, menus, notifications, wallpaper). Those are validated: enum membership
// and integer ranges are rejected, the float strength/contrast/opacity values
// clamp into range. Every other top-level key in the file (the Ryoku-native look
// knobs frameRadius, frameBars, weatherLocation, sidebar panes, language, and the
// rest) is carried through verbatim as passthrough: echoed in the frame and
// merged on patch, but not validated, because the daemon has no schema for them.
// Passthrough keeps the daemon the sole writer without having to model keys other
// surfaces own.
//
// The frame is the whole file as one JSON object, so a subscriber sees both the
// validated schema and the passthrough keys and can drop its own file reader.
//
// State flows out through the topic; intent flows in through the calls:
//
//	subscribe settings                       the current file as JSON, re-pushed on any change
//	call settings.patch {"path":..,"value":..} set one leaf (validated for schema keys)
//	call settings.reset {"path":..}            restore a schema key to its shipped default

// contractKeys is the set of top-level namespaces the daemon owns a schema for.
// A patch whose first path segment is one of these is validated; anything else is
// passthrough.
var contractKeys = map[string]bool{
	"general":       true,
	"theme":         true,
	"bars":          true,
	"menus":         true,
	"notifications": true,
	"wallpaper":     true,
}

// Value domains, in the display order of contract 14 section 8. Stored as stable
// identifiers (no spaces); the label a user sees is the UI's concern. None name
// the reference project: the widget ids reference Hyprland (the compositor) and
// generic desktop concepts only.
var (
	positionValues        = []string{"Left", "Right", "Top", "TopLeft", "TopRight", "Bottom", "BottomLeft", "BottomRight"}
	notificationPosValues = []string{"Left", "Right", "Center"}
	menuExpansionValues   = []string{"AlwaysExpanded", "ExpandBothWays", "ExpandUp", "ExpandDown"}
	tempUnitValues        = []string{"Metric", "Imperial"}
	contentFitValues      = []string{"Contain", "Cover", "Fill", "ScaleDown", "Center", "Tile"}
	videoEngineValues     = []string{"ryogami", "in_shell"}
	quickSettingsIconVals = []string{"Arch", "Fedora", "Hyprland", "Nix"}
	matugenPrefValues     = []string{"Darkness", "Lightness", "Saturation", "LessSaturation", "Value"}
	matugenTypeValues     = []string{"Content", "Expressive", "Fidelity", "FruitSalad", "Monochrome", "Neutral", "Rainbow", "TonalSpot", "Vibrant"}
	matugenModeValues     = []string{"Light", "Dark"}
	orientationValues     = []string{"Horizontal", "Vertical"}
	locQueryTypeValues    = []string{"Coordinates", "City"}

	// 23 bar widgets, all unit variants (no per-item config).
	barWidgetValues = []string{
		"AudioInput", "AudioOutput", "Battery", "Bluetooth", "Clipboard", "Clock",
		"HyprlandDock", "HyprlandLayoutSwitcher", "HyprlandWorkspaces", "HyprPicker",
		"Lock", "Logout", "Network", "Notifications", "PowerProfile", "QuickSettings",
		"Reboot", "RecordingIndicator", "Screenshot", "Shutdown", "Tray", "VpnIndicator",
		"Wallpaper",
	}
	// 17 menu widgets. AppLauncher is deliberately absent: Ryoku's launcher is its
	// own standalone surface, not an in-frame menu widget, so it has no renderer
	// here (user correction).
	menuWidgetValues = []string{
		"AudioInput", "AudioOutput", "Bluetooth", "Calendar", "Clipboard", "Clock",
		"Container", "Divider", "MediaPlayer", "Network", "Notifications", "PowerProfiles",
		"QuickActions", "Spacer", "ThemePicker",
		"Wallpaper", "Weather",
	}
	// 10 quick actions.
	quickActionValues = []string{
		"AirplaneMode", "DoNotDisturb", "ColorPicker", "IdleInhibitor", "Lock",
		"Logout", "NightLight", "Reboot", "Settings", "Shutdown",
	}

	// theme.theme accepts the two dynamic variants (which carry no static palette)
	// plus every static catalog name (themes_gen.go). An unknown name is rejected.
	themeThemeValues = append([]string{"Default", "Wallpaper"}, themeCatalogNames...)
)

// settings mirrors the reference configuration schema (contract 14), minus the
// keys Ryoku has no consumer for (see the slice report): the icon-theme group,
// the custom-CSS file, and the app-launcher menu.
type settings struct {
	General       generalSettings       `json:"general"`
	Theme         themeSettings         `json:"theme"`
	Bars          barsSettings          `json:"bars"`
	Menus         menusSettings         `json:"menus"`
	Notifications notificationsSettings `json:"notifications"`
	Wallpaper     wallpaperSettings     `json:"wallpaper"`
}

type generalSettings struct {
	ClockFormat24H       bool          `json:"clock_format_24_h"`
	WeatherLocationQuery locationQuery `json:"weather_location_query"`
	TemperatureUnit      string        `json:"temperature_unit"`
}

// locationQuery is the reference's tagged Coordinates|City union flattened to one
// object: the discriminator plus every variant's fields, so both variants round
// trip and the dialog fields (lat, lon, name, country) are each patchable.
type locationQuery struct {
	Type    string  `json:"type"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Name    string  `json:"name"`
	Country string  `json:"country"`
}

type themeSettings struct {
	Theme      string           `json:"theme"`
	Matugen    matugenSettings  `json:"matugen"`
	Attributes attributesConfig `json:"attributes"`
	Motion     motionConfig     `json:"motion"`
}

type matugenSettings struct {
	Preference string  `json:"preference"`
	SchemeType string  `json:"scheme_type"`
	Mode       string  `json:"mode"`
	Contrast   float64 `json:"contrast"`
}

// motionConfig is the shell-UI animation timing, read live by ryoku/ui Tokens.
type motionConfig struct {
	Scale  float64 `json:"scale"`  // 0.25..3.0 global speed (1 = shipped timing)
	Reduce bool    `json:"reduce"` // collapse animations to instant (accessibility)
}

type attributesConfig struct {
	Font          fontConfig   `json:"font"`
	Sizing        sizingConfig `json:"sizing"`
	WindowOpacity float64      `json:"window_opacity"`
}

type fontConfig struct {
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
	Tertiary  string `json:"tertiary"`
}

type sizingConfig struct {
	RadiusWidget int `json:"radius_widget"`
	RadiusWindow int `json:"radius_window"`
	BorderWidth  int `json:"border_width"`
}

type barsSettings struct {
	Frame     frameConfig      `json:"frame"`
	Widgets   barWidgetsConfig `json:"widgets"`
	TopBar    horizontalBar    `json:"top_bar"`
	BottomBar horizontalBar    `json:"bottom_bar"`
	LeftBar   verticalBar      `json:"left_bar"`
	RightBar  verticalBar      `json:"right_bar"`
}

type frameConfig struct {
	EnableFrame   bool     `json:"enable_frame"`
	MonitorFilter []string `json:"monitor_filter"`
}

type barWidgetsConfig struct {
	QuickSettings quickSettingsWidget `json:"quick_settings"`
}

type quickSettingsWidget struct {
	Icon string `json:"icon"`
}

// horizontalBar is a top or bottom bar: height, and start/center/end widget lists
// named left/center/right to match the reference.
type horizontalBar struct {
	MinimumHeight   int      `json:"minimum_height"`
	RevealByDefault bool     `json:"reveal_by_default"`
	LeftWidgets     []string `json:"left_widgets"`
	CenterWidgets   []string `json:"center_widgets"`
	RightWidgets    []string `json:"right_widgets"`
}

// verticalBar is a left or right bar: width, and start/center/end widget lists
// named top/center/bottom.
type verticalBar struct {
	MinimumWidth    int      `json:"minimum_width"`
	RevealByDefault bool     `json:"reveal_by_default"`
	TopWidgets      []string `json:"top_widgets"`
	CenterWidgets   []string `json:"center_widgets"`
	BottomWidgets   []string `json:"bottom_widgets"`
}

type menusSettings struct {
	ClockMenu              menuConfig `json:"clock_menu"`
	ClipboardMenu          menuConfig `json:"clipboard_menu"`
	QuickSettingsMenu      menuConfig `json:"quick_settings_menu"`
	NotificationMenu       menuConfig `json:"notification_menu"`
	WallpaperMenu          menuConfig `json:"wallpaper_menu"`
	LeftMenuExpansionType  string     `json:"left_menu_expansion_type"`
	RightMenuExpansionType string     `json:"right_menu_expansion_type"`
}

type menuConfig struct {
	Position     string       `json:"position"`
	MinimumWidth int          `json:"minimum_width"`
	Widgets      []menuWidget `json:"widgets"`
}

// menuWidget is the reference's tagged menu-widget union. Unit variants serialize
// as {"type":..}; the three configured variants carry their fields, kept as
// pointers so a unit variant never emits a spurious zero. QuickActions holds a
// quick-action id list under "actions", distinct from Container's nested widget
// list under "widgets".
type menuWidget struct {
	Type         string       `json:"type"`
	Size         *int         `json:"size,omitempty"`
	Spacing      *int         `json:"spacing,omitempty"`
	MinimumWidth *int         `json:"minimum_width,omitempty"`
	Orientation  *string      `json:"orientation,omitempty"`
	Widgets      []menuWidget `json:"widgets,omitempty"`
	Actions      []string     `json:"actions,omitempty"`
}

type notificationsSettings struct {
	NotificationPosition string `json:"notification_position"`
	PopupWindowMargins   int    `json:"popup_window_margins"`
}

type wallpaperSettings struct {
	ContentFit          string `json:"content_fit"`
	TransitionPreset    string `json:"transition_preset"`
	VideoEngine         string `json:"video_engine"`
	VideoEnabled        bool   `json:"video_enabled"`
	VideoTranscode      bool   `json:"video_transcode"`
	VideoTranscodeFps   int    `json:"video_transcode_fps"`
	VideoTranscodeWidth int    `json:"video_transcode_width"`
}

func ip(n int) *int { return &n }

// mw builds a unit menu-widget; spc a spacer; qa a quick-actions group. Used only
// to spell the default widget lists.
func mw(kind string) menuWidget       { return menuWidget{Type: kind} }
func spc(size int) menuWidget         { return menuWidget{Type: "Spacer", Size: ip(size)} }
func qa(actions ...string) menuWidget { return menuWidget{Type: "QuickActions", Actions: actions} }

// defaultSettings is the shipped schema default, matching contract 14 section 8
// value for value. Slices are non-nil so the frame is always a defined array,
// never null.
func defaultSettings() *settings {
	menu := func(pos string, width int, widgets ...menuWidget) menuConfig {
		return menuConfig{Position: pos, MinimumWidth: width, Widgets: widgets}
	}
	return &settings{
		General: generalSettings{
			ClockFormat24H:       false,
			WeatherLocationQuery: locationQuery{Type: "Coordinates", Lat: 0, Lon: 0, Name: "", Country: ""},
			TemperatureUnit:      "Metric",
		},
		Theme: themeSettings{
			Theme:   "Default",
			Matugen: matugenSettings{Preference: "Darkness", SchemeType: "TonalSpot", Mode: "Dark", Contrast: 0},
			Motion:  motionConfig{Scale: 1, Reduce: false},
			Attributes: attributesConfig{
				Font:          fontConfig{Primary: "", Secondary: "", Tertiary: ""},
				Sizing:        sizingConfig{RadiusWidget: 8, RadiusWindow: 8, BorderWidth: 2},
				WindowOpacity: 1,
			},
		},
		Bars: barsSettings{
			Frame:     frameConfig{EnableFrame: true, MonitorFilter: []string{}},
			Widgets:   barWidgetsConfig{QuickSettings: quickSettingsWidget{Icon: "Arch"}},
			TopBar:    horizontalBar{MinimumHeight: 0, RevealByDefault: true, LeftWidgets: []string{}, CenterWidgets: []string{}, RightWidgets: []string{}},
			BottomBar: horizontalBar{MinimumHeight: 0, RevealByDefault: true, LeftWidgets: []string{}, CenterWidgets: []string{}, RightWidgets: []string{}},
			LeftBar: verticalBar{
				MinimumWidth:    0,
				RevealByDefault: true,
				TopWidgets:      []string{"QuickSettings", "HyprlandWorkspaces"},
				CenterWidgets:   []string{"HyprlandDock"},
				BottomWidgets: []string{
					"RecordingIndicator", "Tray", "Screenshot", "Wallpaper", "Clipboard",
					"Notifications", "AudioInput", "AudioOutput", "Bluetooth", "Network",
					"Battery", "Clock",
				},
			},
			RightBar: verticalBar{MinimumWidth: 0, RevealByDefault: true, TopWidgets: []string{}, CenterWidgets: []string{}, BottomWidgets: []string{}},
		},
		Menus: menusSettings{
			ClockMenu:     menu("Left", 410, mw("Calendar"), spc(20), mw("Weather")),
			ClipboardMenu: menu("Left", 410, mw("Clipboard")),
			QuickSettingsMenu: menu("Left", 410,
				mw("Clock"), mw("Network"), mw("Bluetooth"), mw("AudioOutput"), mw("AudioInput"),
				mw("PowerProfiles"), mw("MediaPlayer"),
				spc(20), qa("AirplaneMode", "NightLight", "ColorPicker", "Settings"),
				spc(20), qa("Logout", "Lock", "Reboot", "Shutdown"),
			),
			NotificationMenu:       menu("Left", 410, mw("Notifications")),
			WallpaperMenu:          menu("Bottom", 1200, mw("ThemePicker"), mw("Wallpaper")),
			LeftMenuExpansionType:  "AlwaysExpanded",
			RightMenuExpansionType: "AlwaysExpanded",
		},
		Notifications: notificationsSettings{NotificationPosition: "Right", PopupWindowMargins: 0},
		Wallpaper:     wallpaperSettings{ContentFit: "Cover", TransitionPreset: "random", VideoEngine: "ryogami", VideoEnabled: true, VideoTranscodeFps: 24, VideoTranscodeWidth: 1920},
	}
}

// validator accumulates the first schema error while normalising in place. strict
// distinguishes the two entry points: a patch (strict) rejects an out-of-range
// integer, a file load (lenient) clamps it so a hand-edited file still loads.
// Float strengths always clamp (the reference newtypes clamp); enums and empty
// required strings always error (a bad enum makes a whole file malformed, exactly
// as the reference's deserialize fails).
type validator struct {
	strict bool
	err    error
}

func inSet(set []string, v string) bool {
	for _, x := range set {
		if x == v {
			return true
		}
	}
	return false
}

func (v *validator) enum(path, val string, set []string) {
	if v.err != nil {
		return
	}
	if !inSet(set, val) {
		v.err = fmt.Errorf("%s: %q is not one of %s", path, val, strings.Join(set, ", "))
	}
}

func (v *validator) nonEmpty(path, val string) {
	if v.err != nil {
		return
	}
	if val == "" {
		v.err = fmt.Errorf("%s: must not be empty", path)
	}
}

func (v *validator) clampF(p *float64, lo, hi float64) {
	if *p < lo {
		*p = lo
	} else if *p > hi {
		*p = hi
	}
}

func (v *validator) rangeI(path string, p *int, lo, hi int) {
	if v.err != nil {
		return
	}
	if *p >= lo && *p <= hi {
		return
	}
	if v.strict {
		v.err = fmt.Errorf("%s: %d is out of range [%d, %d]", path, *p, lo, hi)
		return
	}
	if *p < lo {
		*p = lo
	} else {
		*p = hi
	}
}

func (v *validator) barWidgets(path string, ws []string) {
	for i, w := range ws {
		v.enum(fmt.Sprintf("%s[%d]", path, i), w, barWidgetValues)
	}
}

func (s *settings) normalize(strict bool) error {
	v := &validator{strict: strict}
	s.General.normalize(v)
	s.Theme.normalize(v)
	s.Bars.normalize(v)
	s.Menus.normalize(v)
	s.Notifications.normalize(v)
	s.Wallpaper.normalize(v)
	return v.err
}

func (g *generalSettings) normalize(v *validator) {
	v.enum("general.temperature_unit", g.TemperatureUnit, tempUnitValues)
	v.enum("general.weather_location_query.type", g.WeatherLocationQuery.Type, locQueryTypeValues)
}

func (t *themeSettings) normalize(v *validator) {
	v.nonEmpty("theme.theme", t.Theme)
	// theme.theme validates against the live set (dynamic variants + static
	// catalog + installed library ids). A patch (strict) rejects an unknown
	// name as enum would; a file load (lenient) coerces it to "Wallpaper" so a
	// user whose saved theme was retired from the catalog keeps the rest of
	// their schema instead of the whole load resetting to defaults.
	if v.err == nil {
		set := effectiveThemeThemeValues()
		if !inSet(set, t.Theme) {
			if v.strict {
				v.err = fmt.Errorf("theme.theme: %q is not one of %s", t.Theme, strings.Join(set, ", "))
			} else {
				t.Theme = "Wallpaper"
			}
		}
	}
	v.enum("theme.matugen.preference", t.Matugen.Preference, matugenPrefValues)
	v.enum("theme.matugen.scheme_type", t.Matugen.SchemeType, matugenTypeValues)
	v.enum("theme.matugen.mode", t.Matugen.Mode, matugenModeValues)
	v.clampF(&t.Matugen.Contrast, -1, 1)
	v.rangeI("theme.attributes.sizing.radius_widget", &t.Attributes.Sizing.RadiusWidget, 0, 1000)
	v.rangeI("theme.attributes.sizing.radius_window", &t.Attributes.Sizing.RadiusWindow, 0, 1000)
	v.rangeI("theme.attributes.sizing.border_width", &t.Attributes.Sizing.BorderWidth, 0, 20)
	v.clampF(&t.Attributes.WindowOpacity, 0, 1)
	v.clampF(&t.Motion.Scale, 0.25, 3)
}

func (b *barsSettings) normalize(v *validator) {
	v.enum("bars.widgets.quick_settings.icon", b.Widgets.QuickSettings.Icon, quickSettingsIconVals)
	b.TopBar.normalize(v, "bars.top_bar")
	b.BottomBar.normalize(v, "bars.bottom_bar")
	b.LeftBar.normalize(v, "bars.left_bar")
	b.RightBar.normalize(v, "bars.right_bar")
}

func (hb *horizontalBar) normalize(v *validator, path string) {
	v.rangeI(path+".minimum_height", &hb.MinimumHeight, 0, 500)
	v.barWidgets(path+".left_widgets", hb.LeftWidgets)
	v.barWidgets(path+".center_widgets", hb.CenterWidgets)
	v.barWidgets(path+".right_widgets", hb.RightWidgets)
}

func (vb *verticalBar) normalize(v *validator, path string) {
	v.rangeI(path+".minimum_width", &vb.MinimumWidth, 0, 500)
	v.barWidgets(path+".top_widgets", vb.TopWidgets)
	v.barWidgets(path+".center_widgets", vb.CenterWidgets)
	v.barWidgets(path+".bottom_widgets", vb.BottomWidgets)
}

func (m *menusSettings) normalize(v *validator) {
	m.ClockMenu.normalize(v, "menus.clock_menu")
	m.ClipboardMenu.normalize(v, "menus.clipboard_menu")
	m.QuickSettingsMenu.normalize(v, "menus.quick_settings_menu")
	m.NotificationMenu.normalize(v, "menus.notification_menu")
	m.WallpaperMenu.normalize(v, "menus.wallpaper_menu")
	v.enum("menus.left_menu_expansion_type", m.LeftMenuExpansionType, menuExpansionValues)
	v.enum("menus.right_menu_expansion_type", m.RightMenuExpansionType, menuExpansionValues)
}

func (mc *menuConfig) normalize(v *validator, path string) {
	v.enum(path+".position", mc.Position, positionValues)
	v.rangeI(path+".minimum_width", &mc.MinimumWidth, 0, 10000)
	for i := range mc.Widgets {
		mc.Widgets[i].normalize(v, fmt.Sprintf("%s.widgets[%d]", path, i))
	}
}

// normalize validates a menu widget and canonicalises it: a variant carries only
// its own fields, so the non-applicable ones are cleared. A missing spacer size
// defaults to 16, the value the reference's add-widget menu inserts.
func (w *menuWidget) normalize(v *validator, path string) {
	if v.err != nil {
		return
	}
	v.enum(path+".type", w.Type, menuWidgetValues)
	if v.err != nil {
		return
	}
	switch w.Type {
	case "Spacer":
		if w.Size == nil {
			w.Size = ip(16)
		}
		v.rangeI(path+".size", w.Size, 0, 500)
		w.Spacing, w.MinimumWidth, w.Orientation, w.Widgets, w.Actions = nil, nil, nil, nil, nil
	case "Container":
		if w.Spacing == nil {
			w.Spacing = ip(0)
		}
		if w.MinimumWidth == nil {
			w.MinimumWidth = ip(0)
		}
		if w.Orientation == nil {
			o := "Horizontal"
			w.Orientation = &o
		}
		v.rangeI(path+".spacing", w.Spacing, 0, 100)
		v.rangeI(path+".minimum_width", w.MinimumWidth, 0, 2000)
		v.enum(path+".orientation", *w.Orientation, orientationValues)
		for i := range w.Widgets {
			w.Widgets[i].normalize(v, fmt.Sprintf("%s.widgets[%d]", path, i))
		}
		w.Size, w.Actions = nil, nil
	case "QuickActions":
		for i, a := range w.Actions {
			v.enum(fmt.Sprintf("%s.actions[%d]", path, i), a, quickActionValues)
		}
		w.Size, w.Spacing, w.MinimumWidth, w.Orientation, w.Widgets = nil, nil, nil, nil, nil
	default:
		w.Size, w.Spacing, w.MinimumWidth, w.Orientation, w.Widgets, w.Actions = nil, nil, nil, nil, nil, nil
	}
}

func (n *notificationsSettings) normalize(v *validator) {
	v.enum("notifications.notification_position", n.NotificationPosition, notificationPosValues)
	v.rangeI("notifications.popup_window_margins", &n.PopupWindowMargins, 0, 1000)
}

func (w *wallpaperSettings) normalize(v *validator) {
	v.enum("wallpaper.content_fit", w.ContentFit, contentFitValues)
	// video_engine: "ryogami" (C player, default) or "in_shell" (QtMultimedia).
	// An empty value reads as the default, so a shell.json with no key keeps
	// the shipped engine.
	if w.VideoEngine == "" {
		w.VideoEngine = "ryogami"
	}
	v.enum("wallpaper.video_engine", w.VideoEngine, videoEngineValues)
	// Clamp the transcode caps so a hand-edited value never runs unbounded.
	v.rangeI("wallpaper.video_transcode_fps", &w.VideoTranscodeFps, 1, 120)
	v.rangeI("wallpaper.video_transcode_width", &w.VideoTranscodeWidth, 640, 7680)
	// wallpaper.transition_preset is Ryogami's now: it reads the key from
	// shell.json per-apply and falls back to random on an unknown name, so
	// ryoku-shell keeps the key but no longer duplicates the preset name list.
}

// splitPath breaks a dotted patch path into segments, rejecting an empty path or
// any empty segment (a malformed path such as "a." or "a..b").
func splitPath(path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	segs := strings.Split(path, ".")
	for _, s := range segs {
		if s == "" {
			return nil, fmt.Errorf("malformed path %q: empty segment", path)
		}
	}
	return segs, nil
}

// setByPath sets the leaf at segs in m to the JSON value. When create is false
// (a schema path) every segment must already exist, so an unknown path is
// rejected rather than silently creating a key the schema would then ignore. When
// create is true (passthrough) missing objects are created. Descending through a
// non-object (for example an array index) is always an error.
func setByPath(m map[string]any, segs []string, value json.RawMessage, create bool) error {
	var v any
	if err := json.Unmarshal(value, &v); err != nil {
		return fmt.Errorf("value: %w", err)
	}
	cur := m
	for i := 0; i < len(segs)-1; i++ {
		next, ok := cur[segs[i]]
		if !ok {
			if !create {
				return fmt.Errorf("unknown setting %q", strings.Join(segs, "."))
			}
			nm := map[string]any{}
			cur[segs[i]] = nm
			cur = nm
			continue
		}
		nm, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("cannot descend into %q: not an object", segs[i])
		}
		cur = nm
	}
	last := segs[len(segs)-1]
	if !create {
		if _, ok := cur[last]; !ok {
			return fmt.Errorf("unknown setting %q", strings.Join(segs, "."))
		}
	}
	// Passthrough whole-object replacement preserves absent subtrees. When a
	// single-segment passthrough patch replaces a top-level object with another
	// object, overlay the incoming keys onto the existing value instead of
	// swapping the whole map, so a writer that ships a value missing a subtree it
	// never touched (a stale or partial frameBars, the dock-pinning path, a hand
	// edit) can never drop that subtree from the store. Present keys are replaced,
	// so edits and removals still apply; only keys absent from the patch survive.
	// Nested passthrough paths and scalar or array values keep plain replace.
	if create && len(segs) == 1 {
		if existing, ok := cur[last].(map[string]any); ok {
			if incoming, ok := v.(map[string]any); ok {
				merged := make(map[string]any, len(existing)+len(incoming))
				for k, val := range existing {
					merged[k] = val
				}
				for k, val := range incoming {
					merged[k] = val
				}
				v = merged
			}
		}
	}
	cur[last] = v
	return nil
}

// getByPath returns the value at segs, or an error if any segment is missing or
// descends through a non-object.
func getByPath(m map[string]any, segs []string) (any, error) {
	var cur any = m
	for _, seg := range segs {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path %q: %q is not an object", strings.Join(segs, "."), seg)
		}
		v, ok := obj[seg]
		if !ok {
			return nil, fmt.Errorf("unknown setting %q", strings.Join(segs, "."))
		}
		cur = v
	}
	return cur, nil
}

// boolAt reads a boolean leaf ("clipboard.pruneWeekly") out of the loaded file --
// schema keys and passthrough alike, since a key the daemon carries verbatim is
// still a key it has to act on. A missing or non-boolean value is false.
func (s *settingsStore) boolAt(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := getByPath(s.raw, strings.Split(path, "."))
	if err != nil {
		return false
	}
	b, _ := v.(bool)
	return b
}

// deepCopyMap clones a decoded JSON object so a patch can be validated on a copy
// and only committed if it holds together.
func deepCopyMap(m map[string]any) map[string]any {
	b, _ := json.Marshal(m)
	var c map[string]any
	_ = json.Unmarshal(b, &c)
	if c == nil {
		c = map[string]any{}
	}
	return c
}

func settingsToMap(s *settings) map[string]any {
	b, _ := json.Marshal(s)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

// writeContract overlays the six schema namespaces of ns onto full, so a clamped
// value from normalize is what gets persisted.
func writeContract(full map[string]any, ns *settings) {
	for k, v := range settingsToMap(ns) {
		full[k] = v
	}
}

// resolveThemePalette syncs the derived state of the active theme: the passthrough
// themePalette key here, and theme.json's followWallpaper next door.
//
// the selected static theme's palette (role token -> hex) is copied in, or the key
// is removed for the two dynamic variants (Default, Wallpaper), which have none.
// Called wherever raw and its theme selection are rebuilt (load, patch, and reset
// through patch), so the pill's Theme.namedScheme always tracks the chosen theme.
func resolveThemePalette(full map[string]any, themeName string) {
	// The live path is gated on theme.json, so the same selection has to reach it.
	syncFollowWallpaper(themeName)
	pal, ok := lookupThemePalette(themeName)
	if !ok {
		delete(full, "themePalette")
		return
	}
	m := make(map[string]any, len(pal))
	for k, v := range pal {
		m[k] = v
	}
	full["themePalette"] = m
}

// buildSettings reads the six schema namespaces out of a decoded file, defaulting
// any that are absent, then normalises. A namespace whose shape does not fit the
// schema, or a value that fails normalisation, is an error the caller treats as a
// malformed file (lenient) or a rejected patch (strict).
func buildSettings(raw map[string]any, strict bool) (*settings, error) {
	s := defaultSettings()
	dst := map[string]any{
		"general":       &s.General,
		"theme":         &s.Theme,
		"bars":          &s.Bars,
		"menus":         &s.Menus,
		"notifications": &s.Notifications,
		"wallpaper":     &s.Wallpaper,
	}
	for k, ptr := range dst {
		raw, ok := raw[k]
		if !ok {
			continue
		}
		b, err := json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		if err := json.Unmarshal(b, ptr); err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
	}
	if err := s.normalize(strict); err != nil {
		return nil, err
	}
	return s, nil
}

// settingsStore owns the file in memory: raw is the whole file (schema keys
// normalised, passthrough keys verbatim), cur is the typed view of the schema
// keys. onChange, when set, delivers a fresh frame to subscribers. caps and
// deadKeys ride in every frame so the Hub gates its settings surface from the
// same stream it reads values from: caps is behavioural gating (every
// capability an explicit boolean, all-false when no provider answers, so a
// missing key never reads as supported), deadKeys is provider store leaves some
// installed provider models but the active one does not, which nothing would
// write. Both are fixed for the daemon's life (the active provider does not
// change without a re-login), set once before the first publish.
type settingsStore struct {
	mu       sync.Mutex
	path     string
	raw      map[string]any
	cur      *settings
	mtime    time.Time
	onChange func([]byte)
	caps     map[string]bool
	deadKeys []string
	// windowRuleActions is the active provider's window-rule action list, ridden
	// in the frame beside caps/deadKeys so the Hub's window-rules editor offers
	// only ids this compositor can apply. Empty when no provider answers.
	windowRuleActions []string
}

func newSettingsStore(path string) *settingsStore {
	s := &settingsStore{path: path}
	raw, cur, mtime := loadSettingsFile(path)
	s.raw, s.cur, s.mtime = raw, cur, mtime
	return s
}

// loadSettingsFile reads the file into the in-memory pair. A missing file yields
// defaults; an existing file that will not parse is retried, then backed up to
// <path>.corrupt before falling back to defaults, so a transient or corrupt read
// never silently drops the passthrough keys. A parseable file whose schema portion does not validate keeps its
// passthrough keys but resets the schema to defaults, rather than discarding the
// native look knobs over a bad enum.
func loadSettingsFile(path string) (map[string]any, *settings, time.Time) {
	def := defaultSettings()
	b, err := os.ReadFile(path)
	if err != nil {
		return settingsToMap(def), def, time.Time{}
	}
	var mtime time.Time
	if fi, e := os.Stat(path); e == nil {
		mtime = fi.ModTime()
	}
	var raw map[string]any
	parsed := json.Unmarshal(b, &raw) == nil && raw != nil
	// never reduce an existing file to defaults over a transient/corrupt read:
	// that drops qsbar and the other passthrough knobs on the next patch.
	for i := 0; !parsed && i < 3; i++ {
		time.Sleep(50 * time.Millisecond)
		if b2, e := os.ReadFile(path); e == nil {
			b = b2
			parsed = json.Unmarshal(b, &raw) == nil && raw != nil
		}
	}
	if !parsed {
		if len(b) > 0 {
			_ = os.WriteFile(path+".corrupt", b, 0o644)
		}
		return settingsToMap(def), def, mtime
	}
	cur, err := buildSettings(raw, false)
	if err != nil {
		cur = def
	}
	writeContract(raw, cur)
	resolveThemePalette(raw, cur.Theme.Theme)
	return raw, cur, mtime
}
func loadSettingsPatchBase(path string, fallback map[string]any) (map[string]any, *settings, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		raw := deepCopyMap(fallback)
		cur, buildErr := buildSettings(raw, false)
		if buildErr != nil {
			return nil, nil, buildErr
		}
		return raw, cur, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil || raw == nil {
		if err == nil {
			err = fmt.Errorf("root must be an object")
		}
		return nil, nil, fmt.Errorf("parse settings: %w", err)
	}
	cur, err := buildSettings(raw, false)
	if err != nil {
		return nil, nil, fmt.Errorf("validate settings: %w", err)
	}
	writeContract(raw, cur)
	resolveThemePalette(raw, cur.Theme.Theme)
	return raw, cur, nil
}

// frameLocked marshals the whole file for a subscriber, plus the runtime gating
// keys the Hub reads off the same stream. caps and deadKeys are not part of the
// file (they are never persisted), so they overlay a shallow copy rather than
// s.raw itself. Before the daemon sets caps (a bare store in a test) the frame
// is the file alone, unchanged. Map marshalling sorts keys, so the frame is
// byte-stable and the topic suppresses no-op re-pushes.
func (s *settingsStore) frameLocked() []byte {
	if s.caps == nil {
		b, _ := json.Marshal(s.raw)
		return b
	}
	out := make(map[string]any, len(s.raw)+3)
	for k, v := range s.raw {
		out[k] = v
	}
	out["caps"] = s.caps
	dead := s.deadKeys
	if dead == nil {
		dead = []string{}
	}
	out["deadKeys"] = dead
	wra := s.windowRuleActions
	if wra == nil {
		wra = []string{}
	}
	out["windowRuleActions"] = wra
	b, _ := json.Marshal(out)
	return b
}

func (s *settingsStore) notify(frame []byte) {
	if s.onChange != nil {
		s.onChange(frame)
	}
}

// themeName reports the applied colour scheme (theme.theme). Empty when settings
// have not loaded; used to reset the applied scheme before its files are removed.
func (s *settingsStore) themeName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cur == nil {
		return ""
	}
	return s.cur.Theme.Theme
}

func lockSettingsFile(path string) (func(), error) {
	if path == "" {
		return nil, fmt.Errorf("no config path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}

const barStyleTransactionKey = "ryoStoreBarStyleTransaction"

// patch sets one leaf. A schema path is validated and clamped; any other path is
// passthrough (merged, not validated). The change is persisted before it is
// committed in memory, so a write failure drops the change rather than leaving
// memory and disk disagreeing.
func (s *settingsStore) patch(path string, value json.RawMessage) error {
	segs, err := splitPath(path)
	if err != nil {
		return err
	}
	if len(value) == 0 {
		return fmt.Errorf("missing value")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockSettingsFile(s.path)
	if err != nil {
		return err
	}
	defer unlock()

	diskRaw, diskCur, err := loadSettingsPatchBase(s.path, s.raw)
	if err != nil {
		return err
	}
	full := deepCopyMap(diskRaw)
	contract := contractKeys[segs[0]]
	if err := setByPath(full, segs, value, !contract); err != nil {
		return err
	}
	if len(segs) == 1 && segs[0] == "barStyle" {
		delete(full, barStyleTransactionKey)
	}
	newCur := diskCur
	if contract {
		ns, err := buildSettings(full, true)
		if err != nil {
			return err
		}
		writeContract(full, ns)
		resolveThemePalette(full, ns.Theme.Theme)
		newCur = ns
	}
	if err := s.persistLocked(full); err != nil {
		return err
	}
	s.raw = full
	s.cur = newCur
	s.notify(s.frameLocked())
	return nil
}

// reset restores a schema key to its shipped default. Passthrough keys have no
// shipped default here, so resetting one is an error; a caller resets a native
// key by patching its known default value.
func (s *settingsStore) reset(path string) error {
	segs, err := splitPath(path)
	if err != nil {
		return err
	}
	if !contractKeys[segs[0]] {
		return fmt.Errorf("reset %q: passthrough key has no shipped default; patch its value instead", path)
	}
	val, err := getByPath(settingsToMap(defaultSettings()), segs)
	if err != nil {
		return err
	}
	vb, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return s.patch(path, vb)
}

// persistLocked writes the whole file atomically: a temp file in the same
// directory, then a rename over the target, so a reader (or the poll watcher)
// never sees a half-written file.
func (s *settingsStore) persistLocked(full map[string]any) error {
	if s.path == "" {
		return fmt.Errorf("no config path")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(full, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if fi, e := os.Stat(s.path); e == nil {
		s.mtime = fi.ModTime()
	}
	return nil
}

// reload re-reads the file after an external edit. A malformed file (unparseable,
// or a schema value that does not validate) keeps the last-good state; a good
// file replaces it and pushes the change.
func (s *settingsStore) reload() {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var raw map[string]any
	badFile := json.Unmarshal(b, &raw) != nil || raw == nil
	var cur *settings
	if !badFile {
		cur, err = buildSettings(raw, false)
		if err != nil {
			badFile = true
		}
	}
	s.mu.Lock()
	if fi, e := os.Stat(s.path); e == nil {
		s.mtime = fi.ModTime()
	}
	if badFile {
		s.mu.Unlock()
		return
	}
	writeContract(raw, cur)
	resolveThemePalette(raw, cur.Theme.Theme)
	s.raw, s.cur = raw, cur
	frame := s.frameLocked()
	s.mu.Unlock()
	s.notify(frame)
}

// settingsPollInterval is how often the watcher checks the file's mtime. The file
// changes rarely (only hand edits, since the daemon is otherwise the sole
// writer), so a poll is simpler and more robust across editor rename-and-replace
// than an inotify watch, at a negligible idle cost.
const settingsPollInterval = 500 * time.Millisecond

// watch reloads the file whenever its mtime changes from what the store last
// wrote or read, so an external hand-edit retunes the shell live. It runs until
// the daemon quits.
func (s *settingsStore) watch(quit <-chan struct{}) {
	for {
		select {
		case <-quit:
			return
		case <-time.After(settingsPollInterval):
		}
		fi, err := os.Stat(s.path)
		if err != nil {
			continue
		}
		s.mu.Lock()
		same := fi.ModTime().Equal(s.mtime)
		s.mu.Unlock()
		if !same {
			s.reload()
		}
	}
}

// startSettings brings the settings topic and its calls up: the daemon becomes
// the sole writer of shell.json, publishing the current file and re-publishing on
// every patch, reset, or external edit.
func (d *daemon) startSettings() {
	store := newSettingsStore(filepath.Join(ryokuConfigDir(), "shell.json"))
	d.settings = store
	t := d.registerTopic("settings")

	// The Hub gates its settings surface off this frame, so the gating state
	// rides with it. Both are fixed for the daemon's life, so the providers are
	// probed once here, not on every emission.
	caps := d.capsMap()
	dead := d.deadSettingKeys()
	wra := d.windowRuleActions()

	store.mu.Lock()
	store.caps = caps
	store.deadKeys = dead
	store.windowRuleActions = wra
	frame := store.frameLocked()
	// Nudge the paint worker whenever the theme keys the matugen pipeline reads
	// (the active theme and the scheme knobs) change, so a knob patch retunes the
	// desktop the same way a wallpaper change does. The publish is unchanged; this
	// only layers the re-theme trigger on top of it.
	lastThemeSig := matugenThemeSig(frame)
	lastFontSig := fontSig(frame)
	store.onChange = func(f []byte) {
		t.publish(f)
		if sig := matugenThemeSig(f); sig != lastThemeSig {
			lastThemeSig = sig
			d.scheduleTheme()
		}
		if fs := fontSig(f); fs != lastFontSig {
			lastFontSig = fs
			go applyFont(f)
		}
	}
	store.mu.Unlock()
	t.publish(frame)
	// Apply the configured system font on startup so a fresh login matches the
	// saved choice without waiting for a change (this replaces the old autostart
	// hardcode). Off the hot path, best-effort.
	go applyFont(frame)

	d.registerCall("settings.patch", func(raw json.RawMessage) (any, error) {
		var a struct {
			Path  string          `json:"path"`
			Value json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, store.patch(a.Path, a.Value)
	})
	d.registerCall("settings.reset", func(raw json.RawMessage) (any, error) {
		var a struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, store.reset(a.Path)
	})

	go store.watch(d.quit)
}

// capsMap is every capability with an explicit boolean for the active provider,
// so the Hub never reads a missing key as supported. It reuses the client's
// cached probe (the wm topic's source), so it never forks the provider a second
// time; a failed or absent probe yields all-false, the safe default for gating.
func (d *daemon) capsMap() map[string]bool {
	caps, _ := d.wmc.Caps()
	all := wm.All()
	m := make(map[string]bool, len(all))
	for _, c := range all {
		m[string(c)] = caps.Has(c)
	}
	return m
}

// windowRuleActions is the active provider's window-rule action list, ridden in
// the settings frame beside caps/deadKeys so the Hub's window-rules editor
// offers only ids this compositor's config writer can apply. It reuses the
// cached probe; a failed or absent probe yields an empty list, the safe default
// that offers no action.
func (d *daemon) windowRuleActions() []string {
	caps, err := d.wmc.Caps()
	if err != nil || caps.WindowRuleActions == nil {
		return []string{}
	}
	return caps.WindowRuleActions
}

// deadSettingKeys are the provider store leaves that some installed provider
// models but the active one does not: nothing would write them, so the Hub
// drops a row keyed on one rather than show a control with no writer. A leaf no
// installed provider models is Hub-owned (the Hub acts on it itself) and never
// appears here, which is why the owned set is the union across providers, not
// the active one alone. Sorted, so the frame stays byte-stable.
func (d *daemon) deadSettingKeys() []string {
	activeName := ""
	if caps, err := d.wmc.Caps(); err == nil {
		activeName = caps.Name
	}
	owned := map[string]bool{}
	active := map[string]bool{}
	for _, name := range wm.Providers() {
		c := wm.OpenNamed(name)
		if !c.Available() {
			continue
		}
		leaves := providerLeafKeys(c)
		for k := range leaves {
			owned[k] = true
		}
		if name == activeName {
			active = leaves
		}
	}
	dead := make([]string, 0, len(owned))
	for k := range owned {
		if !active[k] {
			dead = append(dead, k)
		}
	}
	sort.Strings(dead)
	return dead
}

// providerLeafKeys flattens a provider's default subtree to the dotted paths of
// the leaves it models (desktop.appearance.rounding, wm.niri.overviewZoom). A
// list is a whole-collection affordance a list editor owns, so it counts as one
// key at its own path and is not descended into: a provider that models
// wm.<name>.layerRules owns that collection, and the other provider's page for
// it must read as dead. A failed or unparseable probe yields the empty set, so
// the active provider then models nothing and every provider-owned row is
// treated as dead, the safe default.
func providerLeafKeys(c *wm.Client) map[string]bool {
	out := map[string]bool{}
	b, err := c.Defaults()
	if err != nil {
		return out
	}
	var tree map[string]any
	if json.Unmarshal(b, &tree) != nil {
		return out
	}
	flattenLeaves("", tree, out)
	return out
}

// flattenLeaves records every scalar leaf and every list under v as a dotted
// path in out.
func flattenLeaves(prefix string, v map[string]any, out map[string]bool) {
	for k, child := range v {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		if obj, ok := child.(map[string]any); ok {
			flattenLeaves(path, obj, out)
			continue
		}
		out[path] = true
	}
}
