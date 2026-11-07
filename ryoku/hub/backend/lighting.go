package main

// `ryoku-hub lighting ...`, the control plane for RGB devices: the wallpaper
// picker's Lighting settings drive it through the Ryogami daemon, the shell
// calls `accent` on a palette change, and every compositor's autostart calls
// `apply` once a session.
//
// Every path is opt-in. OpenRGB can reconfigure every controller while probing,
// and native providers write firmware-backed laptop lighting directly. Off means
// no detection and no writes; a device is written only once adopted; the mode it
// was found on is recorded so handing it back restores it; and its own memory is
// written only on request. Settings live in ~/.config/ryoku/lighting.json, which
// no update or shell restart touches.

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	lightingUnit = "ryoku-openrgb"
	// a device with no accent of its own falls back to the Ryoku signature.
	lightingFallback = "#F25623"
	// a palette this dark reads as "off" on a keyboard, so the accent is lifted
	// until its brightest channel clears this.
	lightingMinChannel = 96
	// a bright accent whose channels are bunched near the top washes out to
	// near-white on an LED; punchColor pulls the non-dominant channels down
	// until the hue shows. Thresholds tuned on real keyboard hardware.
	lightingWashMax    = 180 // only channels above this wash out
	lightingWashSpread = 70  // channels within this of the top read as grey
	lightingWashGain   = 2   // pull-down factor for the non-dominant channels
)

type lightingProviders uint8

const (
	lightingOpenRGB lightingProviders = 1 << iota
	lightingAura
	lightingQMK
	lightingAll = lightingOpenRGB | lightingAura | lightingQMK
)

// ── state: ~/.config/ryoku/lighting.json ────────────────────────────────────

// lightingSettings = what the user chose for one device. Percentages are stored
// device-independently and scaled into whatever range the mode advertises; -1
// means "leave the device's own value alone", which is the default so adopting a
// device does not silently change knobs the user never touched.
type lightingSettings struct {
	Name    string `json:"name"`
	Managed bool   `json:"managed"`
	// exactly one of these drives the device: Effect is one Ryoku paints (and
	// needs Ryoku running), Mode is one of the device's own (which keeps running
	// without it). Picking one clears the other.
	Mode       string            `json:"mode"`
	Effect     string            `json:"effect,omitempty"`
	Source     string            `json:"source"` // accent | fixed
	Colors     []string          `json:"colors,omitempty"`
	ZoneColors map[string]string `json:"zoneColors,omitempty"`
	Brightness int               `json:"brightness"`
	Speed      int               `json:"speed"`
	Direction  string            `json:"direction,omitempty"`
	Restore    string            `json:"restore,omitempty"`
}

type lightingState struct {
	Enabled bool                         `json:"enabled"`
	Devices map[string]*lightingSettings `json:"devices,omitempty"`
}

func lightingPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "ryoku", "lighting.json")
}

// loadLighting: a missing or unreadable file is the off state, never an error.
// Lighting must fail closed; a broken file cannot become "write to everything".
func loadLighting() lightingState {
	s := lightingState{Devices: map[string]*lightingSettings{}}
	b, err := os.ReadFile(lightingPath())
	if err != nil {
		return s
	}
	if json.Unmarshal(b, &s) != nil {
		return lightingState{Devices: map[string]*lightingSettings{}}
	}
	if s.Devices == nil {
		s.Devices = map[string]*lightingSettings{}
	}
	return s
}

// saveLighting writes atomically, so a crash mid-write cannot leave a half file
// the next load would read as "lighting off, nothing adopted".
func saveLighting(s lightingState) error {
	p := lightingPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), "lighting-*.json")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, p)
}

// deviceKey: an identity that survives a reboot. The USB path and the SDK index
// both move, so the name plus the serial is the stable pair. An SMBus device
// (RAM sticks, some boards and graphics cards) reports no serial, but its
// adapter and address are fixed by the slot it sits in, so that pair tells
// identical sticks apart. Two identical devices with neither share one entry,
// which is honest: nothing about them distinguishes one from the other between
// sessions.
func deviceKey(d orgbDevice) string {
	if d.Serial != "" {
		return d.Name + "#" + d.Serial
	}
	if slot, ok := smbusSlot(d.Location); ok {
		return d.Name + "#" + slot
	}
	return d.Name
}

var (
	smbusLocation = regexp.MustCompile(`^I2C: (.*), address (0x[0-9A-Fa-f]+)$`)
	// the bus number follows driver load order, so it can move between boots
	// while the adapter name and the address stay put.
	smbusBusNode = regexp.MustCompile(`\s*\(?/dev/i2c-\d+\)?`)
)

// smbusSlot reads "smbus:<adapter>@<address>" out of an OpenRGB I2C location.
func smbusSlot(location string) (string, bool) {
	m := smbusLocation.FindStringSubmatch(location)
	if m == nil {
		return "", false
	}
	adapter := strings.TrimSpace(smbusBusNode.ReplaceAllString(m[1], ""))
	return "smbus:" + adapter + "@" + strings.ToLower(m[2]), true
}

// legacyKeys: the keys this device's settings may still be stored under. An
// SMBus device was once keyed by its bare name, shared by every identical stick.
func legacyKeys(d orgbDevice) []string {
	keys := d.Aliases
	if d.Serial == "" {
		if _, ok := smbusSlot(d.Location); ok {
			keys = append(append([]string(nil), keys...), d.Name)
		}
	}
	return keys
}

func cloneSettings(s *lightingSettings) *lightingSettings {
	c := *s
	c.Colors = append([]string(nil), s.Colors...)
	if s.ZoneColors != nil {
		c.ZoneColors = make(map[string]string, len(s.ZoneColors))
		for z, hex := range s.ZoneColors {
			c.ZoneColors[z] = hex
		}
	}
	return &c
}

// ── the wire shape the Hub renders ──────────────────────────────────────────

// lightMode = one mode reduced to the knobs it actually offers, so the UI can
// draw exactly the controls the device supports and nothing else.
type lightMode struct {
	Name       string   `json:"name"`
	Speed      bool     `json:"speed"`
	Brightness bool     `json:"brightness"`
	Colors     int      `json:"colors"` // mode-specific colour slots
	PerLED     bool     `json:"perLed"`
	CanSave    bool     `json:"canSave"`
	Directions []string `json:"directions,omitempty"`
	// what the device itself is set to, as a share of the range it reports, so
	// an untouched knob shows the device's own value instead of a made-up one.
	SpeedPct int `json:"speedPct"`
	BriPct   int `json:"briPct"`
}

type lightZone struct {
	Name string `json:"name"`
	LEDs int    `json:"leds"`
}

// lightFx = one effect Ryoku paints, listed only for devices that can be
// painted at all.
type lightFx struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Speed bool   `json:"speed"`
}

type lightDevice struct {
	Key         string            `json:"key"`
	Name        string            `json:"name"`
	Vendor      string            `json:"vendor,omitempty"`
	Type        string            `json:"type,omitempty"`
	Description string            `json:"description,omitempty"`
	Location    string            `json:"location,omitempty"`
	Online      bool              `json:"online"`
	Active      string            `json:"active,omitempty"` // mode the device is on now
	Modes       []lightMode       `json:"modes,omitempty"`
	Effects     []lightFx         `json:"effects,omitempty"`
	Zones       []lightZone       `json:"zones,omitempty"`
	Managed     bool              `json:"managed"`
	Following   bool              `json:"following"`
	Mode        string            `json:"mode,omitempty"`
	Effect      string            `json:"effect,omitempty"`
	Source      string            `json:"source,omitempty"`
	Colors      []string          `json:"colors,omitempty"`
	ZoneColors  map[string]string `json:"zoneColors,omitempty"`
	Brightness  int               `json:"brightness"`
	Speed       int               `json:"speed"`
	Direction   string            `json:"direction,omitempty"`
	Restore     string            `json:"restore,omitempty"`
}

type lightReport struct {
	Available bool          `json:"available"` // at least one lighting provider is installed
	Enabled   bool          `json:"enabled"`
	Server    bool          `json:"server"` // at least one provider is reachable
	Accent    string        `json:"accent"`
	Devices   []lightDevice `json:"devices"`
	Error     string        `json:"error,omitempty"`
}

func openrgbInstalled() bool {
	_, err := exec.LookPath("openrgb")
	return err == nil
}

func lightingAvailable() bool {
	return openrgbInstalled() || auraInstalled() || qmkInstalled()
}

func runLighting(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("lighting needs a subcommand: state|scan|enable|disable|set|follow|apply|accent|save|release|animate")
	}
	switch args[0] {
	case "state":
		return printJSON(lightingStateReport())
	case "scan":
		return printJSON(lightingScan())
	case "enable":
		return lightingEnable(true)
	case "disable":
		return lightingEnable(false)
	case "set":
		if len(args) < 3 {
			return fmt.Errorf("lighting set needs <key> <json>")
		}
		return lightingSet(args[1], args[2])
	case "follow":
		if len(args) != 3 {
			return fmt.Errorf("lighting follow needs <key>|--all on|off")
		}
		switch args[2] {
		case "on":
			return lightingFollow(args[1], true)
		case "off":
			return lightingFollow(args[1], false)
		default:
			return fmt.Errorf("lighting follow needs on or off")
		}
	case "apply":
		return lightingApply("")
	case "accent":
		accent := ""
		if len(args) > 1 {
			accent = args[1]
		}
		return lightingApply(accent)
	case "save":
		if len(args) < 2 {
			return fmt.Errorf("lighting save needs <key>")
		}
		return lightingSaveToDevice(args[1])
	case "release":
		if len(args) < 2 {
			return fmt.Errorf("lighting release needs <key>")
		}
		return lightingRelease(args[1])
	case "animate":
		return runAnimate()
	default:
		return fmt.Errorf("unknown lighting subcommand: %s", args[0])
	}
}

// lightingStateReport answers without touching hardware: what the user chose
// and whether any provider is installed and running. This is what the Hub opens
// with, so opening the page never enumerates a device.
func lightingStateReport() lightReport {
	st := loadLighting()
	rep := lightReport{
		Available: lightingAvailable(),
		Enabled:   st.Enabled,
		Server:    auraInstalled() && auraServerUp() || serverUp() || qmkInstalled(),
		Accent:    accentColor(),
	}
	for key, s := range st.Devices {
		rep.Devices = append(rep.Devices, savedView(key, s))
	}
	sortDevices(rep.Devices)
	return rep
}

// lightingScan looks for devices, bringing OpenRGB up if it is not already
// running and reading any active native provider. Enumeration happens only
// after the user switched lighting on or pressed Rescan.
func lightingScan() lightReport {
	rep := lightingStateReport()
	if !rep.Enabled {
		return rep
	}
	if !rep.Available {
		rep.Error = "No supported lighting provider is installed."
		return rep
	}
	err := access(true, true, lightingAll, func(o *orgbConn, live []orgbDevice) error {
		rep = liveReport(rememberFirstSight(live), live)
		return nil
	})
	if err != nil {
		rep.Error = err.Error()
	}
	return rep
}

// rememberFirstSight records, once per device, the mode it was already on, so
// handing it back later is a restore and not a guess.
func rememberFirstSight(live []orgbDevice) lightingState {
	st := loadLighting()
	dirty := false
	// a legacy entry is dropped only after every device it stood for has taken
	// its own copy, so identical sticks that shared one all keep the look.
	claimed := map[string]bool{}
	for _, d := range live {
		key := deviceKey(d)
		s := st.Devices[key]
		for _, alias := range legacyKeys(d) {
			old := st.Devices[alias]
			if old == nil || alias == key {
				continue
			}
			if s == nil || firstSightDefaults(s) {
				moved := cloneSettings(old)
				moved.Name = d.Name
				st.Devices[key] = moved
				s = moved
				if d.Provider == auraProvider {
					if moved.Effect != "" {
						moved.Mode = auraModeForEffect(moved.Effect, d)
						moved.Effect = ""
					} else {
						moved.Mode = auraModeForLegacyMode(moved.Mode, d)
					}
				}
			}
			claimed[alias] = true
			dirty = true
		}
		if s == nil {
			s = &lightingSettings{Name: d.Name, Brightness: -1, Speed: -1, Source: "accent"}
			st.Devices[key] = s
			dirty = true
		}
		if _, _, ok := d.mode(s.Restore); !ok && len(d.Modes) > 0 {
			s.Restore = d.Modes[d.ActiveMode].Name
			dirty = true
		}
	}
	for alias := range claimed {
		if !liveKey(live, alias) {
			delete(st.Devices, alias)
		}
	}
	if dirty {
		_ = saveLighting(st)
	}
	return st
}

func firstSightDefaults(s *lightingSettings) bool {
	return !s.Managed && s.Mode == "" && s.Effect == "" &&
		(s.Source == "" || s.Source == "accent") && len(s.Colors) == 0 &&
		len(s.ZoneColors) == 0 && s.Brightness < 0 && s.Speed < 0 && s.Direction == ""
}

// liveReport: the whole picture from a device list already in hand, so acting on
// a device answers from the same connection instead of scanning a second time.
// A fresh scan per click is what makes a settings page flicker.
func liveReport(st lightingState, live []orgbDevice) lightReport {
	return lightReport{
		Available: true,
		Enabled:   st.Enabled,
		Server:    true,
		Accent:    accentColor(),
		Devices:   mergeViews(live, st),
	}
}

// afterWrite re-reads every reachable provider, so the Hub is told what the
// device is on now rather than what it was on a moment ago. The old list stands
// if neither provider can be refreshed.
func afterWrite(o *orgbConn, live []orgbDevice) []orgbDevice {
	var openrgb []orgbDevice
	if o != nil {
		openrgb, _ = o.Devices()
	}
	aura, _ := readAuraDevices()
	if fresh := combineLightingDevices(openrgb, aura); len(fresh) > 0 {
		return fresh
	}
	return live
}

// mergeViews joins what the hardware reports with what the user chose, and lists
// saved-but-absent devices as offline so unplugging a keyboard does not silently
// drop its settings.
func mergeViews(live []orgbDevice, st lightingState) []lightDevice {
	seen := map[string]bool{}
	var out []lightDevice
	for _, d := range live {
		key := deviceKey(d)
		seen[key] = true
		s := st.Devices[key]
		if s == nil {
			s = &lightingSettings{Brightness: -1, Speed: -1}
		}
		v := savedView(key, s)
		v.Following = v.Managed && v.Source == "accent"
		v.Name = d.Name
		v.Vendor = d.Vendor
		v.Type = d.Type
		v.Description = d.Description
		v.Location = d.Location
		v.Online = true
		if len(d.Modes) > 0 {
			v.Active = d.Modes[d.ActiveMode].Name
		}
		if v.Mode == "" {
			v.Mode = v.Active
		}
		for _, m := range d.Modes {
			v.Modes = append(v.Modes, lightMode{
				Name:       m.Name,
				Speed:      m.has(modeHasSpeed),
				Brightness: m.has(modeHasBrightness),
				Colors:     int(m.ColorsMax),
				PerLED:     m.has(modeHasPerLEDColor),
				CanSave:    m.CanSave(),
				Directions: m.Directions(),
				SpeedPct:   asPercent(m.Speed, m.SpeedMin, m.SpeedMax),
				BriPct:     asPercent(m.Bri, m.BriMin, m.BriMax),
			})
		}
		for _, z := range d.Zones {
			v.Zones = append(v.Zones, lightZone{Name: z.Name, LEDs: int(z.LEDs)})
		}
		// Ryoku's own effects, but only where the device can be painted at all.
		if canPaint(d) {
			for _, e := range fxEffects {
				v.Effects = append(v.Effects, lightFx{ID: e.ID, Label: e.Label, Speed: e.Speed})
			}
		}
		out = append(out, v)
	}
	for key, s := range st.Devices {
		if !seen[key] {
			out = append(out, savedView(key, s))
		}
	}
	sortDevices(out)
	return out
}

func savedView(key string, s *lightingSettings) lightDevice {
	name := s.Name
	if name == "" {
		name = strings.SplitN(key, "#", 2)[0]
	}
	src := s.Source
	if src == "" {
		src = "accent"
	}
	return lightDevice{
		Key:        key,
		Name:       name,
		Managed:    s.Managed,
		Following:  s.Managed && src == "accent",
		Mode:       s.Mode,
		Effect:     s.Effect,
		Source:     src,
		Colors:     s.Colors,
		ZoneColors: s.ZoneColors,
		Brightness: s.Brightness,
		Speed:      s.Speed,
		Direction:  s.Direction,
		Restore:    s.Restore,
	}
}

// sortDevices: online devices first, then by name, so the list a user sees does
// not reshuffle between scans. Identical sticks fall back to their key.
func sortDevices(ds []lightDevice) {
	sort.SliceStable(ds, func(i, j int) bool {
		if ds[i].Online != ds[j].Online {
			return ds[i].Online
		}
		if ds[i].Name != ds[j].Name {
			return ds[i].Name < ds[j].Name
		}
		return ds[i].Key < ds[j].Key
	})
}

// lightingEnable flips the master switch. Turning it off hands every adopted
// device back to the mode it was on before Ryoku touched it and stops the
// server Ryoku started, so "off" means the desktop is not in the loop at all.
// The per-device choices are kept, so turning it back on restores the look.
func lightingEnable(on bool) error {
	st := loadLighting()
	if st.Enabled == on {
		if on {
			return printJSON(lightingScan())
		}
		return printJSON(lightingStateReport())
	}
	if !on {
		releaseAll(st)
		stopServer()
	}
	st.Enabled = on
	if err := saveLighting(st); err != nil {
		return err
	}
	if on {
		return printJSON(lightingScan())
	}
	return printJSON(lightingStateReport())
}

// lightingSet merges one device's settings and applies them at once, the way
// every other live control in the Hub behaves. Adopting a device (managed true)
// applies what is stored; dropping it hands the device back.
func lightingSet(key, patch string) error {
	var p struct {
		Managed    *bool              `json:"managed"`
		Mode       *string            `json:"mode"`
		Effect     *string            `json:"effect"`
		Source     *string            `json:"source"`
		Colors     *[]string          `json:"colors"`
		ZoneColors *map[string]string `json:"zoneColors"`
		Brightness *int               `json:"brightness"`
		Speed      *int               `json:"speed"`
		Direction  *string            `json:"direction"`
	}
	if err := json.Unmarshal([]byte(patch), &p); err != nil {
		return fmt.Errorf("lighting set: %w", err)
	}
	if p.Managed != nil && !*p.Managed {
		return lightingRelease(key)
	}

	st := loadLighting()
	s := st.Devices[key]
	if s == nil {
		s = &lightingSettings{Brightness: -1, Speed: -1, Source: "accent"}
		st.Devices[key] = s
	}
	if s.Name == "" {
		s.Name = strings.SplitN(key, "#", 2)[0]
	}
	if p.Managed != nil {
		s.Managed = *p.Managed
	}
	// a device runs one thing at a time: picking its own effect drops Ryoku's,
	// and picking Ryoku's leaves the device mode as what to fall back to.
	if p.Mode != nil {
		s.Mode = *p.Mode
		s.Effect = ""
	}
	if p.Effect != nil {
		if _, ok := fxByID(*p.Effect); ok || *p.Effect == "" {
			s.Effect = *p.Effect
		}
	}
	if p.Source != nil && (*p.Source == "accent" || *p.Source == "fixed") {
		s.Source = *p.Source
	}
	if p.Colors != nil {
		s.Colors = normalizeHexes(*p.Colors)
	}
	if p.ZoneColors != nil {
		s.ZoneColors = map[string]string{}
		for z, c := range *p.ZoneColors {
			if h, ok := normalizeHex(c); ok {
				s.ZoneColors[z] = h
			}
		}
	}
	if p.Brightness != nil {
		s.Brightness = clampPercent(*p.Brightness)
	}
	if p.Speed != nil {
		s.Speed = clampPercent(*p.Speed)
	}
	if p.Direction != nil {
		s.Direction = *p.Direction
	}
	if err := saveLighting(st); err != nil {
		return err
	}
	// a Ryoku-painted effect needs the painter running; it exits by itself once
	// no device wants one, so this is the only place that has to ask.
	ensureAnimator(st)
	// one connection: apply, then answer from the device list already in hand.
	// The Hub redraws from this reply, so a second scan here would rebuild the
	// page under the pointer on every click.
	if !st.Enabled {
		return printJSON(lightingStateReport())
	}
	rep := lightingStateReport()
	err := access(true, false, providersForKey(key), func(o *orgbConn, live []orgbDevice) error {
		if s.Managed {
			if err := applyOne(o, live, key, s, accentColor()); err != nil {
				return err
			}
			live = afterWrite(o, live)
		}
		rep = liveReport(st, live)
		return nil
	})
	if err != nil {
		rep.Error = err.Error()
	}
	return printJSON(rep)
}

// lightingFollow adopts a device into palette following or hands it back. The
// all-device forms share one provider pass so a batch cannot interleave with a
// palette update or repeatedly reconnect to the SDK server.
func lightingFollow(key string, on bool) error {
	if key == "" {
		return fmt.Errorf("lighting follow needs <key>|--all")
	}
	if !on {
		if key == "--all" {
			return lightingReleaseManaged()
		}
		return lightingRelease(key)
	}

	st := loadLighting()
	if !st.Enabled {
		return printJSON(failed(lightingStateReport(), fmt.Errorf("Lighting is off.")))
	}

	rep := lightingStateReport()
	var issues []string
	err := lightingAccess(true, true, followProviders(key), func(o *orgbConn, live []orgbDevice) error {
		st = rememberFirstSight(live)
		var targets []orgbDevice
		if key == "--all" {
			targets = live
		} else if d, ok := findLive(live, key); ok {
			targets = []orgbDevice{d}
		} else {
			issues = append(issues, fmt.Sprintf("%s is not connected", key))
		}

		adopted := make([]orgbDevice, 0, len(targets))
		for _, d := range targets {
			s := st.Devices[deviceKey(d)]
			if s == nil {
				continue
			}
			if !prepareFollow(d, s) {
				issues = append(issues, fmt.Sprintf("%s has no colour-capable mode", d.Name))
				continue
			}
			adopted = append(adopted, d)
		}
		if len(adopted) > 0 {
			if err := saveLighting(st); err != nil {
				return err
			}
			ensureAnimator(st)
		}
		accent := accentColor()
		for _, d := range adopted {
			id := deviceKey(d)
			if err := applyOne(o, live, id, st.Devices[id], accent); err != nil {
				issues = append(issues, fmt.Sprintf("%s: %v", d.Name, err))
			}
		}
		rep = liveReport(st, afterWrite(o, live))
		return nil
	})
	if err != nil {
		issues = append(issues, err.Error())
	}
	if len(issues) > 0 {
		rep.Error = strings.Join(issues, "; ")
	}
	return printJSON(rep)
}

func followProviders(key string) lightingProviders {
	if key == "--all" {
		return lightingAll
	}
	return providersForKey(key)
}

func prepareFollow(d orgbDevice, s *lightingSettings) bool {
	firmwareOnly := d.Provider == auraProvider || d.Provider == qmkProvider
	if s.Effect != "" && !firmwareOnly && canPaint(d) {
		s.Managed = true
		s.Source = "accent"
		return true
	}

	mode, ok := colourMode(d, s.Mode)
	if !ok {
		return false
	}
	s.Managed = true
	s.Source = "accent"
	s.Mode = mode.Name
	s.Effect = ""
	return true
}

func colourMode(d orgbDevice, stored string) (orgbMode, bool) {
	if _, mode, ok := d.mode(stored); ok && modeShowsColour(mode) {
		return mode, true
	}
	if d.ActiveMode >= 0 && d.ActiveMode < len(d.Modes) {
		if mode := d.Modes[d.ActiveMode]; modeShowsColour(mode) {
			return mode, true
		}
	}
	for _, mode := range d.Modes {
		if mode.Name == "Static" && mode.ColorsMax > 0 {
			return mode, true
		}
	}
	for _, mode := range d.Modes {
		if mode.Name == "Direct" && modeShowsColour(mode) {
			return mode, true
		}
	}
	for _, mode := range d.Modes {
		if mode.Name == "Custom" && mode.has(modeHasPerLEDColor) {
			return mode, true
		}
	}
	for _, mode := range d.Modes {
		if mode.ColorsMax > 0 {
			return mode, true
		}
	}
	for _, mode := range d.Modes {
		if mode.has(modeHasPerLEDColor) {
			return mode, true
		}
	}
	return orgbMode{}, false
}

func modeShowsColour(mode orgbMode) bool {
	return mode.ColorsMax > 0 || mode.has(modeHasPerLEDColor)
}

func lightingReleaseManaged() error {
	st := loadLighting()
	if !anyManaged(st) {
		return printJSON(lightingStateReport())
	}
	if !st.Enabled {
		for _, s := range st.Devices {
			s.Managed = false
		}
		if err := saveLighting(st); err != nil {
			return err
		}
		ensureAnimator(st)
		return printJSON(lightingStateReport())
	}

	rep := lightingStateReport()
	var issues []string
	err := lightingAccess(true, false, providersForState(st), func(o *orgbConn, live []orgbDevice) error {
		keys := make([]string, 0, len(st.Devices))
		for key, s := range st.Devices {
			if s.Managed {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		released := make([]*lightingSettings, 0, len(keys))
		for _, key := range keys {
			s := st.Devices[key]
			if err := restoreDevice(o, live, key, s); err != nil {
				name := s.Name
				if name == "" {
					name = key
				}
				issues = append(issues, fmt.Sprintf("%s: %v", name, err))
				continue
			}
			s.Managed = false
			released = append(released, s)
		}
		if len(released) > 0 {
			if err := saveLighting(st); err != nil {
				for _, s := range released {
					s.Managed = true
				}
				return err
			}
			ensureAnimator(st)
		}
		rep = liveReport(st, afterWrite(o, live))
		return nil
	})
	if err != nil {
		issues = append(issues, err.Error())
	}
	if len(issues) > 0 {
		rep.Error = strings.Join(issues, "; ")
	}
	return printJSON(rep)
}

// lightingApply pushes stored settings at every adopted device. Called with an
// accent by the shell when the palette changes (only accent-following devices
// are touched then) and bare once a session by the Hyprland autostart.
//
// It prints nothing: it runs behind the desktop, and a device that has gone
// missing is not an error worth a log line on every wallpaper change.
func lightingApply(accent string) error {
	st := loadLighting()
	if !st.Enabled || !anyManaged(st) || !lightingAvailable() {
		return nil
	}
	if accent == "" {
		accent = accentColor()
	} else if h, ok := normalizeHex(accent); ok {
		accent = punchColor(liftColor(h))
	} else {
		accent = accentColor()
	}
	// Session restore may bring OpenRGB up; native providers are already
	// supervised by the system. Either way, yield to a user action in flight.
	err := access(false, true, providersForState(st), func(o *orgbConn, live []orgbDevice) error {
		st = applyLiveSettings(o, live, accent)
		return nil
	})
	// at login this is what brings a painted effect back; on a palette change the
	// painter is normally already up and just picks the new accent out of the
	// state it re-reads.
	ensureAnimator(st)
	return err
}

func applyLiveSettings(o *orgbConn, live []orgbDevice, accent string) lightingState {
	st := rememberFirstSight(live)
	if !st.Enabled {
		return st
	}
	for key, s := range st.Devices {
		if s.Managed {
			_ = applyOne(o, live, key, s, accent)
		}
	}
	return st
}

// lightingSaveToDevice stores the current look in the device's own memory, so it
// survives OpenRGB quitting and the machine powering off. Explicit only: on a
// device with onboard profiles this replaces the one in the active slot.
func lightingSaveToDevice(key string) error {
	st := loadLighting()
	s := st.Devices[key]
	if s == nil || !s.Managed {
		return printJSON(failed(lightingStateReport(), fmt.Errorf("%s is not under Ryoku control", key)))
	}
	rep := lightingStateReport()
	err := access(true, false, providersForKey(key), func(o *orgbConn, live []orgbDevice) error {
		d, ok := findLive(live, key)
		if !ok {
			return fmt.Errorf("%s is not connected", key)
		}
		idx, m, ok := resolveMode(d, s.Mode)
		if !ok {
			return fmt.Errorf("%s cannot store a mode", key)
		}
		if !m.CanSave() {
			return fmt.Errorf("%s does not offer saving %s to the device", d.Name, m.Name)
		}
		tuned := tuneMode(m, s, accentColor())
		if err := o.SetMode(d, idx, tuned); err != nil {
			return err
		}
		if err := o.SaveMode(d, idx, tuned); err != nil {
			return err
		}
		rep = liveReport(st, afterWrite(o, live))
		return nil
	})
	return printJSON(failed(rep, err))
}

// failed attaches a reason to a report without dropping the devices it carries,
// so a refused action leaves the page as it was with an explanation on it.
func failed(rep lightReport, cause error) lightReport {
	if cause != nil && rep.Error == "" {
		rep.Error = cause.Error()
	}
	return rep
}

// lightingRelease hands one device back: the mode it was on before Ryoku ever
// wrote to it, then out of the managed set. Its settings stay on disk, so
// adopting it again restores the look the user built.
func lightingRelease(key string) error {
	st := loadLighting()
	s := st.Devices[key]
	if s == nil || !s.Managed {
		return printJSON(lightingStateReport())
	}
	if !st.Enabled {
		s.Managed = false
		if err := saveLighting(st); err != nil {
			return err
		}
		ensureAnimator(st)
		return printJSON(lightingStateReport())
	}

	rep := lightingStateReport()
	err := lightingAccess(true, false, providersForKey(key), func(o *orgbConn, live []orgbDevice) error {
		if err := restoreDevice(o, live, key, s); err != nil {
			return err
		}
		s.Managed = false
		if err := saveLighting(st); err != nil {
			s.Managed = true
			return err
		}
		ensureAnimator(st)
		rep = liveReport(st, afterWrite(o, live))
		return nil
	})
	return printJSON(failed(rep, err))
}

// releaseAll: the master switch going off puts every adopted device back. Best
// effort by design; if the server is already gone there is nothing holding the
// devices and nothing to restore through.
func releaseAll(st lightingState) {
	providers := providersForState(st)
	reachable := providers&lightingAura != 0 && auraInstalled() && auraServerUp()
	if !reachable && providers&lightingOpenRGB != 0 {
		reachable = serverUp()
	}
	if !reachable && providers&lightingQMK != 0 && qmkInstalled() {
		reachable = true
	}
	if !reachable {
		return
	}
	_ = access(true, false, providers, func(o *orgbConn, live []orgbDevice) error {
		for key, s := range st.Devices {
			if s.Managed {
				_ = restoreDevice(o, live, key, s)
			}
		}
		return nil
	})
}

func restoreDevice(o *orgbConn, live []orgbDevice, key string, s *lightingSettings) error {
	d, ok := findLive(live, key)
	if !ok {
		return fmt.Errorf("%s is not connected", key)
	}
	if s.Restore == "" {
		return fmt.Errorf("%s has no previous mode to restore", d.Name)
	}
	idx, m, ok := d.mode(s.Restore)
	if !ok {
		return fmt.Errorf("%s no longer offers %s", d.Name, s.Restore)
	}
	if d.Provider == auraProvider {
		return auraRestore(d.ProviderPath, uint32(m.Value))
	}
	if d.Provider == qmkProvider {
		return nil // the board keeps its last colour; no pre-adoption state was captured to restore
	}
	if o == nil {
		return fmt.Errorf("OpenRGB is not running")
	}
	return o.SetMode(d, idx, m)
}

func anyManaged(st lightingState) bool {
	for _, s := range st.Devices {
		if s.Managed {
			return true
		}
	}
	return false
}

func providersForKey(key string) lightingProviders {
	if strings.Contains(key, "#"+auraProvider+":") {
		return lightingAura
	}
	if strings.Contains(key, "#"+qmkProvider+":") {
		return lightingQMK
	}
	return lightingAll
}

func providersForState(st lightingState) lightingProviders {
	providers := lightingProviders(0)
	for key, s := range st.Devices {
		if !s.Managed {
			continue
		}
		providers |= providersForKey(key)
	}
	return providers
}

func findLive(live []orgbDevice, key string) (orgbDevice, bool) {
	for _, d := range live {
		if deviceKey(d) == key {
			return d, true
		}
	}
	return orgbDevice{}, false
}

func liveKey(live []orgbDevice, key string) bool {
	_, ok := findLive(live, key)
	return ok
}

// ── applying one device ─────────────────────────────────────────────────────

func applyOne(o *orgbConn, live []orgbDevice, key string, s *lightingSettings, accent string) error {
	d, ok := findLive(live, key)
	if !ok {
		return fmt.Errorf("%s is not connected", key)
	}
	if d.Provider == auraProvider {
		if s.Effect != "" {
			return fmt.Errorf("%s offers its firmware effects, not Ryoku per-key effects", d.Name)
		}
		_, mode, ok := resolveMode(d, s.Mode)
		if !ok {
			return fmt.Errorf("%s reports no modes", d.Name)
		}
		return auraWrite(d.ProviderPath, auraEffectFromMode(tuneMode(mode, s, accent)), s.Brightness)
	}
	if d.Provider == qmkProvider {
		if s.Effect != "" {
			return fmt.Errorf("%s offers its firmware effects, not Ryoku per-key effects", d.Name)
		}
		_, mode, ok := resolveMode(d, s.Mode)
		if !ok {
			return fmt.Errorf("%s reports no modes", d.Name)
		}
		return qmkApply(tuneMode(mode, s, accent), s.Brightness)
	}
	if o == nil {
		return fmt.Errorf("OpenRGB is not running")
	}
	// a Ryoku effect: sit the device in its per-LED mode and paint. An animated
	// one is then drawn frame by frame by the painter; a still one is this one
	// write, so a fixed colour needs no process behind it.
	if s.Effect != "" {
		idx, m, ok := paintMode(d)
		if !ok {
			return fmt.Errorf("%s cannot be painted by Ryoku; pick one of its own effects", d.Name)
		}
		if m.has(modeHasBrightness) {
			m.Bri = m.BriMax // the effect scales the colour itself
		}
		if err := o.SetMode(d, idx, m); err != nil {
			return err
		}
		if s.Effect != "solid" {
			return nil
		}
		base := uint32(0xFFFFFF)
		if cols := colorValues(s, accent, 1); len(cols) > 0 {
			base = cols[0]
		}
		bri := 1.0
		if s.Brightness >= 0 {
			bri = float64(s.Brightness) / 100
		}
		return o.SetLEDs(d, fxFrame("solid", make([]float64, d.LEDCount), 0, 0, bri, base))
	}
	idx, m, ok := resolveMode(d, s.Mode)
	if !ok {
		return fmt.Errorf("%s reports no modes", d.Name)
	}
	tuned := tuneMode(m, s, accent)
	if err := o.SetMode(d, idx, tuned); err != nil {
		return err
	}
	if !m.has(modeHasPerLEDColor) {
		return nil
	}
	return paintLEDs(o, d, s, accent)
}

// resolveMode: the stored mode by name, or the mode the device is on now when a
// firmware update renamed or dropped it. A vanished mode must not stop the rest
// of the settings from applying.
func resolveMode(d orgbDevice, name string) (int, orgbMode, bool) {
	if len(d.Modes) == 0 {
		return 0, orgbMode{}, false
	}
	if name != "" {
		if idx, m, ok := d.mode(name); ok {
			return idx, m, true
		}
	}
	return d.ActiveMode, d.Modes[d.ActiveMode], true
}

// tuneMode folds the user's choices into the mode the device reported. Anything
// the user did not set, or the mode does not offer, is echoed back untouched.
func tuneMode(m orgbMode, s *lightingSettings, accent string) orgbMode {
	out := m
	if n := int(m.ColorsMax); n > 0 {
		cols := colorValues(s, accent, n)
		if len(cols) >= int(m.ColorsMin) {
			out.Colors = cols
			out.ColorMode = 2 // MODE_COLORS_MODE_SPECIFIC
		}
	}
	if m.has(modeHasBrightness) && s.Brightness >= 0 {
		out.Bri = scaleTo(s.Brightness, m.BriMin, m.BriMax)
	}
	if m.has(modeHasSpeed) && s.Speed >= 0 {
		out.Speed = scaleTo(s.Speed, m.SpeedMin, m.SpeedMax)
	}
	if v, ok := directionValue(m, s.Direction); ok {
		out.Direction = v
	}
	return out
}

// paintLEDs colours a per-LED mode: each zone separately when the user coloured
// them apart, otherwise the whole device in one colour.
func paintLEDs(o *orgbConn, d orgbDevice, s *lightingSettings, accent string) error {
	base := colorValues(s, accent, 1)
	if len(base) == 0 {
		return nil
	}
	if len(d.Zones) > 1 && len(s.ZoneColors) > 0 {
		for i, z := range d.Zones {
			c := base[0]
			if hex, ok := s.ZoneColors[z.Name]; ok {
				if v, ok := colorValue(hex); ok {
					c = v
				}
			}
			if err := o.SetZoneLEDs(d, i, repeat(c, int(z.LEDs))); err != nil {
				return err
			}
		}
		return nil
	}
	if d.LEDCount == 0 {
		return nil
	}
	return o.SetLEDs(d, repeat(base[0], d.LEDCount))
}

func repeat(c uint32, n int) []uint32 {
	out := make([]uint32, n)
	for i := range out {
		out[i] = c
	}
	return out
}

// colorValues: the colours to send, from the palette accent or the fixed
// colours the user picked, padded to the count the mode wants.
func colorValues(s *lightingSettings, accent string, want int) []uint32 {
	var hexes []string
	if s.Source == "fixed" && len(s.Colors) > 0 {
		hexes = s.Colors
	} else if s.Source == "fixed" {
		hexes = []string{lightingFallback}
	} else {
		hexes = []string{accent}
		hexes = append(hexes, s.Colors...)
	}
	out := make([]uint32, 0, want)
	for _, h := range hexes {
		if len(out) == want {
			break
		}
		if v, ok := colorValue(h); ok {
			out = append(out, v)
		}
	}
	for len(out) < want && len(out) > 0 {
		out = append(out, out[len(out)-1])
	}
	return out
}

// colorValue packs #RRGGBB the way OpenRGB does: red in the low byte.
func colorValue(hex string) (uint32, bool) {
	h, ok := normalizeHex(hex)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseUint(h[1:], 16, 32)
	if err != nil {
		return 0, false
	}
	r := (n >> 16) & 0xFF
	g := (n >> 8) & 0xFF
	b := n & 0xFF
	return uint32(r | g<<8 | b<<16), true
}

func directionValue(m orgbMode, name string) (uint32, bool) {
	for _, d := range orgbDirections {
		if d.Name == name && m.has(d.Flag) {
			return d.Val, true
		}
	}
	return 0, false
}

// scaleTo maps a percentage onto whatever range the mode advertises. Some
// devices report min above max (a bigger raw number is slower), and the same
// arithmetic keeps 0% slowest and 100% fastest on those too.
func scaleTo(pct int, min, max uint32) uint32 {
	pct = clampPercent(pct)
	lo, hi := float64(min), float64(max)
	return uint32(lo + (hi-lo)*float64(pct)/100 + 0.5)
}

// asPercent is scaleTo's inverse, for showing a device's own value as a share of
// its range.
func asPercent(raw, min, max uint32) int {
	if min == max {
		return 100
	}
	p := (float64(raw) - float64(min)) / (float64(max) - float64(min)) * 100
	return clampPercent(int(p + 0.5))
}

func clampPercent(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func normalizeHexes(in []string) []string {
	var out []string
	for _, h := range in {
		if n, ok := normalizeHex(h); ok {
			out = append(out, n)
		}
	}
	return out
}

var hexRe = regexp.MustCompile(`^#?([0-9a-fA-F]{6})$`)

func normalizeHex(s string) (string, bool) {
	m := hexRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", false
	}
	return "#" + strings.ToUpper(m[1]), true
}

// ── the palette accent ──────────────────────────────────────────────────────

var activeRe = regexp.MustCompile(`active\s*=\s*"#?([0-9a-fA-F]{6})"`)

// palettePath: where matugen writes the desktop palette on every wallpaper
// change. The painter watches it too, so a retint reaches the keyboard.
func palettePath() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".cache")
	}
	return filepath.Join(base, "ryoku", "hypr-colors.lua")
}

// accentColor: the desktop's current accent, lifted to something a keyboard can
// actually show.
func accentColor() string {
	b, err := os.ReadFile(palettePath())
	if err != nil {
		return lightingFallback
	}
	m := activeRe.FindSubmatch(b)
	if m == nil {
		return lightingFallback
	}
	return punchColor(liftColor("#" + strings.ToUpper(string(m[1]))))
}

// liftColor scales a very dark accent up until its brightest channel is visible.
// A near-black palette would otherwise read as a broken keyboard rather than a
// dark theme.
func liftColor(hex string) string {
	h, ok := normalizeHex(hex)
	if !ok {
		return lightingFallback
	}
	n, err := strconv.ParseUint(h[1:], 16, 32)
	if err != nil {
		return lightingFallback
	}
	r, g, b := int((n>>16)&0xFF), int((n>>8)&0xFF), int(n&0xFF)
	max := r
	if g > max {
		max = g
	}
	if b > max {
		max = b
	}
	if max == 0 {
		return lightingFallback
	}
	if max < lightingMinChannel {
		r = r * lightingMinChannel / max
		g = g * lightingMinChannel / max
		b = b * lightingMinChannel / max
	}
	return fmt.Sprintf("#%02X%02X%02X", min(r, 255), min(g, 255), min(b, 255))
}

// punchColor restores saturation in a washed-out bright accent so an LED shows
// the hue, not near-white. It pulls the non-dominant channels away from the top
// one, lifting chroma while keeping the hue (the gaps scale together) and the
// brightness (the top channel is fixed). A near-grey accent passes through.
func punchColor(hex string) string {
	h, ok := normalizeHex(hex)
	if !ok {
		return lightingFallback
	}
	n, err := strconv.ParseUint(h[1:], 16, 32)
	if err != nil {
		return lightingFallback
	}
	r, g, b := int((n>>16)&0xFF), int((n>>8)&0xFF), int(n&0xFF)
	hi, lo := r, r
	if g > hi {
		hi = g
	}
	if b > hi {
		hi = b
	}
	if g < lo {
		lo = g
	}
	if b < lo {
		lo = b
	}
	if hi > lightingWashMax && hi-lo < lightingWashSpread {
		r = max(0, min(hi-(hi-r)*lightingWashGain, 255))
		g = max(0, min(hi-(hi-g)*lightingWashGain, 255))
		b = max(0, min(hi-(hi-b)*lightingWashGain, 255))
	}
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// ── the OpenRGB server ──────────────────────────────────────────────────────

// serverUp: is an SDK server listening now? A user running the OpenRGB GUI has
// one, and Ryoku uses it rather than starting a second.
func serverUp() bool {
	c, err := net.DialTimeout("tcp", orgbAddr, 400*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// connectServer: use the running server, or start Ryoku's own as a transient
// host job so it is supervised, logged, and stoppable by name. The second
// return says Ryoku started it, which matters: OpenRGB accepts connections
// before it has finished finding devices, so a cold start needs a moment more
// before the empty device list can be believed.
var openrgbDial = orgbDial

func connectServer() (*orgbConn, bool, error) {
	if o, err := openrgbDial(2 * time.Second); err == nil {
		return o, false, nil
	}
	if err := startServer(); err != nil {
		return nil, false, err
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		if o, err := openrgbDial(2 * time.Second); err == nil {
			return o, true, nil
		}
		if time.Now().After(deadline) {
			return nil, true, fmt.Errorf("OpenRGB did not answer on %s", orgbAddr)
		}
		time.Sleep(400 * time.Millisecond)
	}
}

// startServer runs OpenRGB headless, bound to loopback: no window, no tray, and
// nothing reachable from the network.
func startServer() error {
	out, err := exec.Command("ryoku-host", "transient", "start", lightingUnit,
		"--prop", "Description=OpenRGB SDK server for Ryoku lighting",
		"--env", "QT_QPA_PLATFORM=offscreen", "--",
		"openrgb", "--server", "--server-host", "127.0.0.1").CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not start OpenRGB: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// stopServer stops only the unit Ryoku starts. A server the user runs themselves
// is theirs; Ryoku never takes it down.
func stopServer() {
	_ = exec.Command("ryoku-host", "transient", "stop", lightingUnit).Run()
}

var lightingAccess = access

// access runs one serialized job against the requested lighting providers.
//
//	wait     hold for another lighting change to finish, instead of skipping.
//	mayStart bring OpenRGB up when nothing is listening.
func access(wait, mayStart bool, providers lightingProviders, fn func(*orgbConn, []orgbDevice) error) error {
	unlock, err := lightingLock(wait)
	if err != nil {
		if !wait {
			return nil
		}
		return err
	}
	defer unlock()

	var o *orgbConn
	var openrgb []orgbDevice
	var openrgbErr error
	if providers&lightingOpenRGB != 0 && openrgbInstalled() {
		started := false
		if mayStart {
			o, started, openrgbErr = connectServer()
		} else {
			o, openrgbErr = openrgbDial(2 * time.Second)
			if openrgbErr != nil {
				openrgbErr = fmt.Errorf("OpenRGB is not running. Press Rescan devices to start it")
			}
		}
		if openrgbErr == nil {
			defer o.Close()
			openrgb, openrgbErr = o.Devices()
			if openrgbErr == nil && started && len(openrgb) == 0 {
				deadline := time.Now().Add(25 * time.Second)
				for len(openrgb) == 0 && !o.listChanged && time.Now().Before(deadline) {
					time.Sleep(500 * time.Millisecond)
					if openrgb, openrgbErr = o.Devices(); openrgbErr != nil {
						break
					}
				}
				if openrgbErr == nil && len(openrgb) == 0 && o.listChanged {
					openrgb, openrgbErr = o.Devices()
				}
			}
		}
	}

	var aura []orgbDevice
	var auraErr error
	if providers&lightingAura != 0 {
		aura, auraErr = readAuraDevices()
	}
	var qmk []orgbDevice
	var qmkErr error
	if providers&lightingQMK != 0 {
		qmk, qmkErr = readQMKDevices()
	}
	live := append(combineLightingDevices(openrgb, aura), qmk...)
	if len(live) == 0 {
		if openrgbErr != nil {
			return openrgbErr
		}
		if auraErr != nil {
			return auraErr
		}
		if qmkErr != nil {
			return qmkErr
		}
	}
	return fn(o, live)
}

// lightingLock serialises device writes across the Hub, the palette hook and
// the session restore, so two of them cannot interleave on one keyboard.
func lightingLock(wait bool) (func(), error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	f, err := os.OpenFile(filepath.Join(dir, "ryoku-lighting.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return func() {}, err
	}
	how := syscall.LOCK_EX
	if !wait {
		how |= syscall.LOCK_NB
	}
	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		f.Close()
		return func() {}, fmt.Errorf("another lighting change is in flight")
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
