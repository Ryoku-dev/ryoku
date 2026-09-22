package main

import (
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
)

// transitionDurationMs is the shared wall-clock length of every wallpaper reveal.
// Recovered from the wallpaper-daemon era (the old shared transitionDuration of
// "2.2" seconds): every Super+W / Super+Shift+W switch runs the same duration and
// only the shape (kind / easing / edge) varies, so the desktop feels consistent
// regardless of which preset is picked.
const transitionDurationMs = 2200

// transitionKinds is the closed set of reveal geometries the backdrop's shader
// understands. The first seven are recovered from the daemon's --transition-type
// values; the rest are the expressive set ported from the upstream ii transitions,
// which warp the sample coordinates rather than sweep a mask (see reveal.frag).
var transitionKinds = map[string]bool{
	"fade": true, "wipe": true, "wave": true,
	"center": true, "grow": true, "any": true, "outer": true,
	"pixelate": true, "dissolve": true, "ripple": true, "shatter": true, "glitch": true,
	"crt": true, "stripes": true, "melt": true, "peel": true,
}

// transitionPreset is one named wallpaper reveal. The in-shell backdrop reveals the
// new image over the old through a GPU mask whose geometry is `kind`, whose timing
// is the cubic-bezier `bezier`, and whose boundary is feathered by `edgeSoftness`,
// all over the shared transitionDurationMs. The 22 presets below are 13 recovered
// from the wallpaper daemon (re-typed from its --transition-* flags) plus 9 ports
// of the upstream ii expressive set; the recovered flag mapping is:
//
//	--transition-type  -> kind        --transition-wave "<w>,<h>" -> waveAmp (h/500)
//	--transition-angle -> angle       --transition-pos            -> pos
//	--transition-bezier -> bezier     --transition-step           -> edgeSoftness
//
// where the daemon's step is edge softness only (low = feathered band, high =
// crisp), so it maps inversely as edgeSoftness = (120 - step) / 300; 'fade' ignored
// step, so its edgeSoftness stays 0.
type transitionPreset struct {
	name string
	// kind is the reveal geometry: one of fade, wipe, wave, center, grow, any,
	// outer, or an expressive kind (pixelate, dissolve, ripple, shatter, glitch,
	// crt, stripes, melt, peel); see transitionKinds.
	kind string
	// angle is the sweep direction in degrees for wipe / wave (0 sweeps left to
	// right, 90 top to bottom). Unused by the radial and fade kinds.
	angle float64
	// waveAmp is the wave-boundary amplitude as a fraction of the sweep extent, for
	// the wave kind only (0 otherwise).
	waveAmp float64
	// pos is the origin anchor for the grow kind, one of the eight compass points;
	// "" means the surface centre. center / any resolve their own origin, so they
	// leave it empty.
	pos string
	// bezier is the cubic-bezier easing (x1,y1,x2,y2) shaping the reveal's timing:
	// recovered verbatim for the daemon set, reused from that same vocabulary for the
	// expressive ports. All 22 stay monotonic (y in [0,1]) so no reveal wraps.
	bezier [4]float64
	// edgeSoftness is the feathered width of the reveal boundary as a fraction of
	// the reveal coordinate: 0 = crisp, larger = a softer band.
	edgeSoftness float64
}

// transitionPresets is the 22-preset table: the recovered 13 (one crossfade, three
// directional sweeps, five circle reveals, and four Material 3 expressive-motion
// ports) plus nine coordinate-warping ports of the upstream ii set (block, noise,
// wave, shatter, glitch, scanline, stripe, melt, peel). The per-preset intent
// comments are the recovered originals; the ii ports carry fresh one-line intents.
var transitionPresets = []transitionPreset{
	// crossfade
	{name: "silk_fade", kind: "fade", // crossfade, easeInOutCubic
		bezier: [4]float64{0.65, 0, 0.35, 1}},
	// directional sweeps (wipe / wave)
	{name: "diagonal_silk", kind: "wipe", angle: 30, // 30deg wipe, fast launch then glide, easeOutExpo
		bezier: [4]float64{0.16, 1, 0.3, 1}, edgeSoftness: (120 - 110) / 300.0},
	{name: "dream_curtain", kind: "wipe", angle: 90, // top-down curtain, soft feathered edge, easeInOutQuint
		bezier: [4]float64{0.83, 0, 0.17, 1}, edgeSoftness: (120 - 35) / 300.0},
	{name: "liquid_ribbon", kind: "wave", angle: 45, waveAmp: 35 / 500.0, // diagonal rolling waves, easeInOutQuart
		bezier: [4]float64{0.76, 0, 0.24, 1}, edgeSoftness: (120 - 90) / 300.0},
	// circle reveals (center / grow / outer / any)
	{name: "iris_open", kind: "center", // iris bloom from dead center, easeOutQuint
		bezier: [4]float64{0.22, 1, 0.36, 1}, edgeSoftness: (120 - 100) / 300.0},
	{name: "corner_bloom", kind: "grow", pos: "bottom-left", // blooms from bottom-left, easeOutExpo
		bezier: [4]float64{0.16, 1, 0.3, 1}, edgeSoftness: (120 - 90) / 300.0},
	{name: "spotlight_rise", kind: "grow", pos: "bottom", // swells up from bottom-center, easeOutCirc
		bezier: [4]float64{0, 0.55, 0.45, 1}, edgeSoftness: (120 - 90) / 300.0},
	{name: "wander_iris", kind: "any", // bloom from a random on-screen point, easeOutQuart
		bezier: [4]float64{0.25, 1, 0.5, 1}, edgeSoftness: (120 - 100) / 300.0},
	{name: "vignette_close", kind: "outer", // new image seals from edges to center, easeInOutCubic
		bezier: [4]float64{0.65, 0, 0.35, 1}, edgeSoftness: (120 - 90) / 300.0},
	// Material 3 expressive-motion ports: the daemon rode these signature curves
	// on its sweeps; celeste_veil is the wallpaper crossfade itself. The springy
	// overshoot (y>1) and two-segment emphasized curves were left out because a
	// single monotonic bezier cannot carry them.
	{name: "celeste_veil", kind: "fade", // expressive slow-effects wallpaper crossfade
		bezier: [4]float64{0.34, 0.88, 0.34, 1}},
	{name: "comet_streak", kind: "wipe", angle: 135, // fast-launch, long-glide sweep, emphasizedDecel
		bezier: [4]float64{0.05, 0.7, 0.1, 1}, edgeSoftness: (120 - 100) / 300.0},
	{name: "aurora_ripple", kind: "wave", angle: 120, waveAmp: 30 / 500.0, // snappy front-loaded wavy sweep, expressiveFastEffects
		bezier: [4]float64{0.31, 0.94, 0.34, 1}, edgeSoftness: (120 - 80) / 300.0},
	{name: "starfall_bloom", kind: "grow", pos: "top", // iris blooming down from the top, M3 standard
		bezier: [4]float64{0.2, 0, 0, 1}, edgeSoftness: (120 - 100) / 300.0},
	// coordinate-warping ports of the upstream ii set: these distort how the two
	// frames are sampled (mosaics, ripples, flying shards, screen melt) rather than
	// sweep a mask, so the five that draw their own hard edge leave edgeSoftness 0,
	// exactly as fade does.
	{name: "mosaic_swell", kind: "pixelate", // block grid swells to a mosaic at the midpoint then resolves, easeInOutCubic
		bezier: [4]float64{0.65, 0, 0.35, 1}},
	{name: "ember_burn", kind: "dissolve", // noise threshold burns through behind a warm edge, easeInOutQuart
		bezier: [4]float64{0.76, 0, 0.24, 1}, edgeSoftness: (120 - 60) / 300.0},
	{name: "pond_wake", kind: "ripple", // a wave front wells the new image up from the centre, easeOutQuint
		bezier: [4]float64{0.22, 1, 0.36, 1}, edgeSoftness: (120 - 85) / 300.0},
	{name: "glass_scatter", kind: "shatter", // old frame breaks into shards that spin and fly off, emphasizedDecel
		bezier: [4]float64{0.05, 0.7, 0.1, 1}},
	{name: "signal_tear", kind: "glitch", // scanline tears and RGB split peak mid-switch, easeInOutCubic
		bezier: [4]float64{0.65, 0, 0.35, 1}},
	{name: "cathode_wink", kind: "crt", // old frame collapses to a bright line, new frame reopens, easeInOutQuint
		bezier: [4]float64{0.83, 0, 0.17, 1}},
	{name: "shutter_sweep", kind: "stripes", angle: 24, // angled stripes comb in from alternating sides, easeOutQuart
		bezier: [4]float64{0.25, 1, 0.5, 1}, edgeSoftness: (120 - 90) / 300.0},
	{name: "wax_descent", kind: "melt", // old frame melts downward in ragged columns (Doom melt), easeInOutQuint
		bezier: [4]float64{0.83, 0, 0.17, 1}},
	{name: "page_turn", kind: "peel", // diagonal front peels the old sheet away to the new, easeOutExpo
		bezier: [4]float64{0.16, 1, 0.3, 1}, edgeSoftness: (120 - 100) / 300.0},
}

// pickedTransition is a preset resolved for one switch: the preset fields plus a
// concrete origin (the pos anchor, or a fresh random point for the `any` kind), the
// shared duration, and a fresh 0..1 seed the noise kinds (dissolve / shatter /
// glitch / melt) vary their pattern on. It is what the daemon publishes on the
// wallpaper topic for the backdrop's reveal shader to consume; the json tags match
// the QML frame.
type pickedTransition struct {
	Name         string     `json:"name"`
	Kind         string     `json:"kind"`
	Angle        float64    `json:"angle"`
	WaveAmp      float64    `json:"waveAmp"`
	OriginX      float64    `json:"originX"`
	OriginY      float64    `json:"originY"`
	Bezier       [4]float64 `json:"bezier"`
	EdgeSoftness float64    `json:"edgeSoftness"`
	DurationMs   int        `json:"durationMs"`
	Seed         float64    `json:"seed"`
	// Shader names a skwd catalog transition; when set, the backdrop loads
	// skwd/<shader>.frag.qsb and ignores the mask-kind fields above.
	Shader string `json:"shader,omitempty"`
}

// pickTransition returns a random preset resolved for one switch, never repeating
// the immediately previous pick so consecutive switches never feel samey. The last
// index is persisted on the daemon (guarded by wallMu, the wallpaper hot-path lock,
// so the bare field needs no extra mutex), mirroring the recovered picker.
func (d *daemon) pickTransition() *pickedTransition {
	n := len(transitionPresets)
	if n == 0 {
		return nil
	}
	i := rand.IntN(n)
	if n > 1 && i == d.lastTransition {
		i = (i + 1 + rand.IntN(n-1)) % n
	}
	d.lastTransition = i
	return resolveTransition(transitionPresets[i])
}

// transitionFor is the animation a wallpaper op reveals with. Two paths must
// not animate and always return nil: init, which paints the saved wallpaper
// onto a fresh backdrop at login, and live-reload, which relaunches the current
// clip after a settings change. A user-driven switch follows the picker's
// transition block (the skwd keys in ryogami-wall/config.json): off means a
// plain cut, "random" is a no-repeat pick over the 39-shader skwd catalog, a
// catalog name pins that shader, and the "ryoku" sentinel (or an unknown value)
// falls back to the shell's 22-preset reveal engine, which
// wallpaper.transition_preset in shell.json can pin further.
func (d *daemon) transitionFor(mode string) *pickedTransition {
	if mode == "init" || mode == "live-reload" {
		return nil
	}
	prefs := readWallUITransition()
	if !prefs.Enabled {
		return nil
	}
	switch {
	case prefs.Shader == transitionRandom:
		return d.pickAnyTransition(prefs.DurationMs)
	case knownSkwdShader(prefs.Shader):
		return &pickedTransition{
			Name:       "skwd",
			Kind:       "skwd",
			Shader:     prefs.Shader,
			DurationMs: prefs.DurationMs,
			Seed:       rand.Float64(),
		}
	}
	// A picker selection may name a reveal preset as well as a skwd shader: the
	// two engines share one pool in the UI, so resolve a preset by that name here.
	if p, okPreset := lookupTransitionPreset(prefs.Shader); okPreset {
		return resolveTransition(p)
	}
	// Legacy path: shell.json wallpaper.transition_preset pins a reveal preset.
	if p, okPreset := lookupTransitionPreset(wallpaperTransitionPreset()); okPreset {
		return resolveTransition(p)
	}
	return d.pickTransition()
}

// pickAnyTransition is the no-repeat random pick over the merged pool: every skwd
// shader and every reveal preset is one candidate, so "Random" rotates the whole
// transition set the picker shows rather than only the shader half. The skwd
// candidates carry the picker's duration; the reveal candidates keep the engine's
// shared reveal duration, exactly as when each is pinned.
func (d *daemon) pickAnyTransition(durationMs int) *pickedTransition {
	total := len(skwdShaders) + len(transitionPresets)
	if total == 0 {
		return nil
	}
	i := rand.IntN(total)
	if total > 1 && i == d.lastTransition {
		i = (i + 1 + rand.IntN(total-1)) % total
	}
	d.lastTransition = i
	if i < len(skwdShaders) {
		return &pickedTransition{
			Name:       "skwd",
			Kind:       "skwd",
			Shader:     skwdShaders[i],
			DurationMs: durationMs,
			Seed:       rand.Float64(),
		}
	}
	return resolveTransition(transitionPresets[i-len(skwdShaders)])
}

// transitionRandom is the sentinel wallpaper.transition_preset value: a fresh
// no-repeat pick per switch, the shipped default and the behaviour from before
// the preference existed.
const transitionRandom = "random"

// transitionPresetNames lists every preset name in table order. The settings
// enum (settings.go) prepends transitionRandom to build the value domain, and
// the reveal pickers in the Hub and the studio list these by name.
func transitionPresetNames() []string {
	names := make([]string, len(transitionPresets))
	for i := range transitionPresets {
		names[i] = transitionPresets[i].name
	}
	return names
}

// lookupTransitionPreset resolves a stored preference to a preset. The random
// sentinel, the empty string (no key), and any name absent from the table all
// return ok=false, so transitionFor falls back to the no-repeat picker.
func lookupTransitionPreset(name string) (transitionPreset, bool) {
	if name == "" || name == transitionRandom {
		return transitionPreset{}, false
	}
	for i := range transitionPresets {
		if transitionPresets[i].name == name {
			return transitionPresets[i], true
		}
	}
	return transitionPreset{}, false
}

// wallpaperTransitionPreset reads wallpaper.transition_preset from shell.json,
// mirroring wallpaperContentFit: the key is formalised in settings.go, but the
// switch reads it per apply so a changed preference takes effect on the next
// switch with no live plumbing. A missing file, key, or unreadable value is the
// random sentinel, so a switch always resolves.
func wallpaperTransitionPreset() string {
	dir := ryokuConfigDir()
	if dir == "" {
		return transitionRandom
	}
	b, err := os.ReadFile(filepath.Join(dir, "shell.json"))
	if err != nil {
		return transitionRandom
	}
	var m struct {
		Wallpaper struct {
			TransitionPreset string `json:"transition_preset"`
		} `json:"wallpaper"`
	}
	if json.Unmarshal(b, &m) != nil {
		return transitionRandom
	}
	if m.Wallpaper.TransitionPreset == "" {
		return transitionRandom
	}
	return m.Wallpaper.TransitionPreset
}

// resolveTransition binds a preset to a concrete origin, a fresh per-switch seed,
// and the shared duration.
func resolveTransition(p transitionPreset) *pickedTransition {
	ox, oy := originForPreset(p)
	return &pickedTransition{
		Name:         p.name,
		Kind:         p.kind,
		Angle:        p.angle,
		WaveAmp:      p.waveAmp,
		OriginX:      ox,
		OriginY:      oy,
		Bezier:       p.bezier,
		EdgeSoftness: p.edgeSoftness,
		DurationMs:   transitionDurationMs,
		Seed:         rand.Float64(),
	}
}

// originForPreset resolves the reveal origin in surface coordinates (0,0 top-left
// to 1,1 bottom-right). The `any` kind blooms from a fresh random on-screen point;
// grow reads its compass `pos`; every other kind blooms from (or, for outer, seals
// toward) the centre.
func originForPreset(p transitionPreset) (x, y float64) {
	if p.kind == "any" {
		return rand.Float64(), rand.Float64()
	}
	switch p.pos {
	case "top":
		return 0.5, 0.0
	case "bottom":
		return 0.5, 1.0
	case "left":
		return 0.0, 0.5
	case "right":
		return 1.0, 0.5
	case "top-left":
		return 0.0, 0.0
	case "top-right":
		return 1.0, 0.0
	case "bottom-left":
		return 0.0, 1.0
	case "bottom-right":
		return 1.0, 1.0
	}
	return 0.5, 0.5
}

// The skwd shader catalog: the 39 GLSL transitions ported from skwd-paper,
// compiled beside the shell's Backdrop (modules/wallpaper/skwd/<name>.frag.qsb).
// Names match upstream's catalog verbatim, so the picker's shader dropdown and a
// config.json written for skwd both keep meaning; crossfade is the clean cross
// dissolve carried over from skwd-wall v2's transition.wgsl default branch.
var skwdShaders = []string{
	"pixelate", "iris", "liquid-ripple", "wave-warp", "glitch",
	"voronoi-shatter", "heat-melt", "plasma-flow", "ink-splash", "smoke",
	"chromatic-bloom", "inkwell-drop", "pixelfade-wave", "soft-warp-fade",
	"zoom-blur-pull", "flyeye", "mosaic-tumble", "crosswarp", "morph",
	"bounce", "circle-crop", "colour-distance", "crossfade", "crazy-parametric",
	"directional", "directional-scaled", "edge-transition", "glitch-displace",
	"overexposure", "polka-dots-curtain", "puzzle-right", "static-fade",
	"crosshatch", "directional-wipe", "fadecolor", "parametric-glitch",
	"perlin", "polar-function", "randomsquares",
}

// wallUITransition is the picker's transition preference block from
// ~/.config/ryogami-wall/config.json, the same keys skwd-wall writes
// (transition.enabled / transition.shader / transition.durationMs).
type wallUITransition struct {
	Enabled    bool
	Shader     string
	DurationMs int
}

func readWallUITransition() wallUITransition {
	out := wallUITransition{Enabled: true, Shader: "random", DurationMs: 600}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(home(), ".config")
	}
	var m struct {
		Transition struct {
			Enabled    *bool  `json:"enabled"`
			Shader     string `json:"shader"`
			DurationMs int    `json:"durationMs"`
		} `json:"transition"`
	}
	loadJSON(filepath.Join(base, "ryogami-wall", "config.json"), &m)
	if m.Transition.Enabled != nil {
		out.Enabled = *m.Transition.Enabled
	}
	if m.Transition.Shader != "" {
		out.Shader = m.Transition.Shader
	}
	if m.Transition.DurationMs >= 50 && m.Transition.DurationMs <= 10000 {
		out.DurationMs = m.Transition.DurationMs
	}
	return out
}

func knownSkwdShader(name string) bool {
	for _, s := range skwdShaders {
		if s == name {
			return true
		}
	}
	return false
}
