package main

// The effects Ryoku paints itself, through a device's per-LED mode.
//
// A keyboard often ignores half the effects its firmware advertises (an ASUS
// N-KEY board runs Static, Breathing, Flashing and Spectrum Cycle and nothing
// else), and OpenRGB's own client sends the same bytes, so drawing them is the
// only way to make them work. `ryoku-hub lighting animate` renders until nothing
// wants painting, then exits.

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"time"
)

const (
	fxUnit = "ryoku-lighting-fx"
	fxFPS  = 30
)

// fxEffect = one effect Ryoku can draw. `spatial` marks the ones that travel
// across the device, so they are only offered where the LEDs have positions.
type fxEffect struct {
	ID    string
	Label string
	Speed bool
}

// the catalogue, in the order the Hub lists it.
var fxEffects = []fxEffect{
	{ID: "solid", Label: "Solid", Speed: false},
	{ID: "breathe", Label: "Breathe", Speed: true},
	{ID: "pulse", Label: "Pulse", Speed: true},
	{ID: "spectrum", Label: "Spectrum", Speed: true},
	{ID: "wave", Label: "Rainbow Wave", Speed: true},
	{ID: "comet", Label: "Comet", Speed: true},
	{ID: "scanner", Label: "Scanner", Speed: true},
}

func fxByID(id string) (fxEffect, bool) {
	for _, e := range fxEffects {
		if e.ID == id {
			return e, true
		}
	}
	return fxEffect{}, false
}

// paintMode: the mode to sit a device in while Ryoku paints it. Named the way
// OpenRGB names them, most direct first.
func paintMode(d orgbDevice) (int, orgbMode, bool) {
	for _, want := range []string{"Direct", "Custom", "Static"} {
		if idx, m, ok := d.mode(want); ok && m.has(modeHasPerLEDColor) {
			return idx, m, true
		}
	}
	for i, m := range d.Modes {
		if m.has(modeHasPerLEDColor) {
			return i, m, true
		}
	}
	return 0, orgbMode{}, false
}

// canPaint reports whether Ryoku's own effects are available on this device.
func canPaint(d orgbDevice) bool {
	_, _, ok := paintMode(d)
	return ok
}

// ledPositions: where each LED sits across the device, 0 at one edge and 1 at
// the other. A keyboard reports a grid, so an effect travels left to right the
// way it looks; a device without one falls back to LED order.
func ledPositions(d orgbDevice) []float64 {
	pos := make([]float64, d.LEDCount)
	for i := range pos {
		if d.LEDCount > 1 {
			pos[i] = float64(i) / float64(d.LEDCount-1)
		}
	}
	for _, z := range d.Zones {
		if len(z.Map) == 0 || z.W < 2 {
			continue
		}
		for row := range int(z.H) {
			for col := range int(z.W) {
				led := z.Map[row*int(z.W)+col]
				if led == 0xFFFFFFFF || int(led) >= len(pos) {
					continue
				}
				pos[led] = float64(col) / float64(z.W-1)
			}
		}
	}
	return pos
}

// fxFrame renders one frame: `t` is seconds since the effect started, `base` the
// colour it is built from, `bri` a 0..1 scale, `speed` a 0..1 rate.
func fxFrame(id string, pos []float64, t, speed, bri float64, base uint32) []uint32 {
	out := make([]uint32, len(pos))
	r, g, b := unpackRGB(base)
	h, s, _ := rgbToHSV(r, g, b)
	if s < 0.05 {
		s = 1 // a white or grey pick still cycles hue for the rainbow effects
	}
	// 0.25x .. 2x of a one-second beat, so the slow end is calm and the fast end
	// is quick without becoming a strobe.
	rate := 0.25 + speed*1.75

	switch id {
	case "breathe":
		k := 0.15 + 0.85*(0.5-0.5*math.Cos(2*math.Pi*t*rate*0.5))
		return fill(out, scaleRGB(base, k*bri))
	case "pulse":
		k := 0.1
		if math.Mod(t*rate, 1.0) < 0.5 {
			k = 1
		}
		return fill(out, scaleRGB(base, k*bri))
	case "spectrum":
		hue := math.Mod(h+t*rate*0.15, 1)
		return fill(out, scaleRGB(packHSV(hue, s, 1), bri))
	case "wave":
		for i, p := range pos {
			hue := math.Mod(h+t*rate*0.15+p*0.9, 1)
			out[i] = scaleRGB(packHSV(hue, s, 1), bri)
		}
		return out
	case "comet":
		head := math.Mod(t*rate*0.4, 1.3) - 0.15
		for i, p := range pos {
			d := head - p
			k := 0.0
			if d >= 0 && d < 0.35 {
				k = 1 - d/0.35
			}
			out[i] = scaleRGB(base, k*k*bri)
		}
		return out
	case "scanner":
		// a band that runs to one edge and back, so a keyboard sweeps rather
		// than jumping to the far side every cycle.
		phase := math.Mod(t*rate*0.4, 2)
		head := phase
		if phase > 1 {
			head = 2 - phase
		}
		for i, p := range pos {
			d := math.Abs(head - p)
			k := 0.0
			if d < 0.18 {
				k = 1 - d/0.18
			}
			out[i] = scaleRGB(base, k*k*bri)
		}
		return out
	default: // solid
		return fill(out, scaleRGB(base, bri))
	}
}

func fill(out []uint32, c uint32) []uint32 {
	for i := range out {
		out[i] = c
	}
	return out
}

func unpackRGB(c uint32) (float64, float64, float64) {
	return float64(c&0xFF) / 255, float64((c>>8)&0xFF) / 255, float64((c>>16)&0xFF) / 255
}

func scaleRGB(c uint32, k float64) uint32 {
	if k <= 0 {
		return 0
	}
	if k > 1 {
		k = 1
	}
	r := uint32(float64(c&0xFF)*k + 0.5)
	g := uint32(float64((c>>8)&0xFF)*k + 0.5)
	b := uint32(float64((c>>16)&0xFF)*k + 0.5)
	return r | g<<8 | b<<16
}

func rgbToHSV(r, g, b float64) (float64, float64, float64) {
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	d := max - min
	h := 0.0
	switch {
	case d == 0:
	case max == r:
		h = math.Mod((g-b)/d/6+1, 1)
	case max == g:
		h = ((b-r)/d + 2) / 6
	default:
		h = ((r-g)/d + 4) / 6
	}
	s := 0.0
	if max > 0 {
		s = d / max
	}
	return h, s, max
}

func packHSV(h, s, v float64) uint32 {
	h = math.Mod(math.Mod(h, 1)+1, 1) * 6
	i := math.Floor(h)
	f := h - i
	p := v * (1 - s)
	q := v * (1 - s*f)
	t := v * (1 - s*(1-f))
	var r, g, b float64
	switch int(i) % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	default:
		r, g, b = v, p, q
	}
	return uint32(r*255+0.5) | uint32(g*255+0.5)<<8 | uint32(b*255+0.5)<<16
}

// ── the animator ────────────────────────────────────────────────────────────

// runAnimate paints every managed device that asked for a Ryoku effect, and
// exits once none do (the user picked device effects again, handed the devices
// back, or switched lighting off). One process for every device, so a keyboard
// and a mouse stay in step.
func runAnimate() error {
	o, err := orgbDial(3 * time.Second)
	if err != nil {
		return fmt.Errorf("OpenRGB is not running")
	}
	defer o.Close()
	live, err := o.Devices()
	if err != nil {
		return err
	}

	type painted struct {
		dev orgbDevice
		pos []float64
	}
	byKey := map[string]painted{}
	for _, d := range live {
		byKey[deviceKey(d)] = painted{dev: d, pos: ledPositions(d)}
	}

	st := loadLighting()
	stamp := fxStamp()
	accent := accentColor()
	seated := map[string]string{} // key -> the mode we sat the device in
	start := time.Now()
	tick := time.NewTicker(time.Second / fxFPS)
	defer tick.Stop()

	for range tick.C {
		if s := fxStamp(); s != stamp {
			stamp = s
			st = loadLighting()
			accent = accentColor()
		}
		work := 0
		for key, set := range st.Devices {
			if !st.Enabled || !set.Managed || set.Effect == "" {
				delete(seated, key)
				continue
			}
			p, ok := byKey[key]
			if !ok {
				continue
			}
			work++
			// sit the device in its per-LED mode once, then keep painting it.
			if seated[key] != set.Effect {
				idx, m, ok := paintMode(p.dev)
				if !ok {
					continue
				}
				if m.has(modeHasBrightness) {
					m.Bri = m.BriMax
				}
				if err := o.SetMode(p.dev.Index, idx, m); err != nil {
					return err
				}
				seated[key] = set.Effect
			}
			base := uint32(0xFFFFFF)
			if cols := colorValues(set, accent, 1); len(cols) > 0 {
				base = cols[0]
			}
			bri, speed := 1.0, 0.5
			if set.Brightness >= 0 {
				bri = float64(set.Brightness) / 100
			}
			if set.Speed >= 0 {
				speed = float64(set.Speed) / 100
			}
			frame := fxFrame(set.Effect, p.pos, time.Since(start).Seconds(), speed, bri, base)
			if err := o.SetLEDs(p.dev.Index, frame); err != nil {
				return err
			}
		}
		if work == 0 {
			return nil
		}
	}
	return nil
}

// fxStamp: a cheap fingerprint of the two files the painter follows, so a Hub
// change or a wallpaper retint reaches the device within a frame or two without
// watching for signals.
func fxStamp() string {
	out := ""
	for _, p := range []string{lightingPath(), palettePath()} {
		if fi, err := os.Stat(p); err == nil {
			out += fmt.Sprintf("%d/%d;", fi.ModTime().UnixNano(), fi.Size())
		}
	}
	return out
}

// ensureAnimator starts the painter when a device wants a Ryoku effect, and
// leaves it alone otherwise: it exits by itself once nothing needs painting.
func ensureAnimator(st lightingState) {
	want := false
	for _, s := range st.Devices {
		if st.Enabled && s.Managed && s.Effect != "" {
			want = true
			break
		}
	}
	if !want || animatorRunning() {
		return
	}
	self, err := os.Executable()
	if err != nil {
		self = "ryoku-hub"
	}
	_ = exec.Command("ryoku-host", "transient", "start", fxUnit,
		"--prop", "Description=Ryoku lighting effects", "--",
		self, "lighting", "animate").Run()
}

func animatorRunning() bool {
	return exec.Command("ryoku-host", "transient", "is-active", fxUnit).Run() == nil
}
