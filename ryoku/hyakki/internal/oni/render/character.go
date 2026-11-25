// Package render draws an Oni character from its profile artwork, posed by
// the shared motion engine: one software raster every renderer uses (the
// native surface directly, render/gio for in-app previews). It reads no
// system state; colours come in as a Look built from the desktop's roles.
package render

import (
	"errors"
	"fmt"
	"image"
	"math"
	"slices"
	"sync"

	"ryoku-rashin/internal/oni/motion"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/spec"
)

// FallbackUnsupported is a Fallback reason for artwork the validator admits
// but this renderer cannot draw; the slot names the file and the feature.
const FallbackUnsupported = "unsupported-svg"

// What an unsupported part of an SVG uses instead.
const (
	UsedNotDrawn = "not-drawn"
)

// Character is one loaded character ready to draw. Draw may be called from
// one goroutine at a time per Character; it reuses its scratch buffers.
type Character struct {
	id        string
	framePx   float64
	buttonPx  float64
	flatSq    float64
	squash    spec.Range
	glow      spec.Glow
	lap       spec.Lap
	outlinePx float64

	// anchor is where the strand leaves the art, fractions of its viewBox;
	// follow is how much of the pose it rides.
	anchor point
	follow spec.AnchorFollow

	base     *art
	states   map[string]*art
	gestures map[string]*art
	spec     map[string]bool

	fallbacks []profile.Fallback

	mu   sync.Mutex
	d    drawer
	main canvas
	out  canvas
}

// Load builds a character from its profile and the validated bytes of the
// files it declares, keyed by the profile-relative path. Oni's own artwork
// (and the neutral silhouette, which is Oni's static image) wears the Look's
// roles; another character's artwork keeps the colours it was drawn in.
func Load(p profile.Profile, files map[string][]byte, s *spec.OniSpec) (*Character, error) {
	if s == nil {
		return nil, errors.New("render: no motion spec")
	}
	c := &Character{
		id:        p.ID,
		framePx:   s.Character.FramePx,
		buttonPx:  s.Character.ButtonPx,
		flatSq:    s.Poses.Flat.Squash,
		squash:    s.Springs.PoseRanges.Squash,
		glow:      s.Colour.Glow,
		lap:       s.Colour.Lap,
		outlinePx: s.Character.OutlinePx,
		anchor:    point{s.Strand.Anchor.X, s.Strand.Anchor.Y},
		follow:    s.Strand.Anchor.Follow,
		states:    map[string]*art{},
		gestures:  map[string]*art{},
		spec:      map[string]bool{},
	}
	biggest := 1.0
	for _, f := range s.Placement.Sizes {
		biggest = max(biggest, f)
	}
	maxPx := s.Character.ButtonPx * biggest * s.Character.DPRCap * s.Springs.PoseRanges.Scale[1] * (1 + s.Springs.PoseRanges.Squash[1])

	ink, inkOK := parseColour(s.Character.Ink)
	white, whiteOK := parseColour(s.Character.White)
	if !inkOK || !whiteOK {
		return nil, errors.New("render: the motion spec's ink or white is not a colour")
	}
	near := func(a, b rgba) bool {
		return math.Abs(a.R-b.R) < 1.5/255 && math.Abs(a.G-b.G) < 1.5/255 && math.Abs(a.B-b.B) < 1.5/255
	}
	roles := func(silhouette bool) func(rgba) colour {
		return func(x rgba) colour {
			switch {
			case near(x, ink):
				return colour{c: x, role: roleInk}
			case silhouette || near(x, white):
				return colour{c: x, role: roleWhite}
			}
			return colour{c: x, role: roleBody}
		}
	}
	authored := func(x rgba) colour { return colour{c: x} }
	own := authored
	if p.Shipped && p.ID == profile.ShippedID {
		own = roles(false)
	}

	compiled := map[string]*art{}
	compile := func(slot, file string) *art {
		if a, ok := compiled[file]; ok {
			return a
		}
		data, ok := files[file]
		if !ok {
			return nil
		}
		reported := map[string]bool{}
		a, err := compileArt(file, data, own, maxPx, func(what string) {
			if !reported[what] {
				reported[what] = true
				c.fallbacks = append(c.fallbacks, profile.Fallback{Slot: file + ": " + what, Reason: FallbackUnsupported, Used: UsedNotDrawn})
			}
		})
		if err != nil {
			c.fallbacks = append(c.fallbacks, profile.Fallback{Slot: slot, Reason: FallbackUnsupported, Used: profile.UsedStaticPose})
			return nil
		}
		compiled[file] = a
		return a
	}

	var baseSlot, baseFile string
	switch p.Artwork.Kind {
	case profile.ArtworkParametric:
		baseSlot, baseFile = "artwork.fallback", p.Artwork.Fallback
	case profile.ArtworkSVG:
		baseSlot, baseFile = "artwork.image", p.Artwork.Image
	default:
		baseSlot = "artwork"
	}
	if baseFile != "" {
		if _, ok := files[baseFile]; !ok {
			c.fallbacks = append(c.fallbacks, profile.Fallback{Slot: baseSlot, Reason: profile.FallbackAssetMissing, Used: profile.UsedSilhouette})
		} else {
			c.base = compile(baseSlot, baseFile)
		}
	} else {
		c.fallbacks = append(c.fallbacks, profile.Fallback{Slot: baseSlot, Reason: profile.FallbackMissing, Used: profile.UsedSilhouette})
	}
	if c.base != nil {
		if an := p.Artwork.Anchor; an != nil {
			c.anchor = point{an.X, an.Y}
		}
	} else {
		// the profile's anchor is a point on the artwork it could not draw
		sil, err := silhouette(roles(true), maxPx)
		if err != nil {
			return nil, err
		}
		c.base = sil
	}

	slots := func(group string, set map[string]string, ids []string, used string, into map[string]*art) {
		for _, id := range ids {
			slot := "animations." + group + "." + id
			file, ok := set[id]
			switch {
			case !ok || file == "":
				c.fallbacks = append(c.fallbacks, profile.Fallback{Slot: slot, Reason: profile.FallbackMissing, Used: used})
			case file == profile.PoseFromSpec:
				if p.Artwork.Kind == profile.ArtworkParametric {
					c.spec[group+"."+id] = true
				} else {
					c.fallbacks = append(c.fallbacks, profile.Fallback{Slot: slot, Reason: profile.FallbackMissing, Used: used})
				}
			default:
				if _, present := files[file]; !present {
					c.fallbacks = append(c.fallbacks, profile.Fallback{Slot: slot, Reason: profile.FallbackAssetMissing, Used: used})
				} else if a := compile(slot, file); a != nil {
					into[id] = a
				}
			}
		}
	}
	slots("states", p.Animations.States, spec.StateIDs, profile.UsedStaticPose, c.states)
	slots("gestures", p.Animations.Gestures, spec.GestureIDs, profile.UsedBasePose, c.gestures)
	return c, nil
}

// silhouette is the neutral figure for a character without artwork: Oni's
// own static image in ink and white.
func silhouette(recolour func(rgba) colour, maxPx float64) (*art, error) {
	d := profile.Default()
	data, ok := d.Files[d.Profile.Artwork.Fallback]
	if !d.Valid || !ok {
		return nil, errors.New("render: the shipped oni has no static image for the silhouette")
	}
	a, err := compileArt(d.Profile.Artwork.Fallback, data, recolour, maxPx, func(string) {})
	if err != nil {
		return nil, fmt.Errorf("render: the silhouette: %w", err)
	}
	return a, nil
}

// Fallbacks is every slot this character draws with something else, and
// every part of its artwork it cannot draw.
func (c *Character) Fallbacks() []profile.Fallback { return slices.Clone(c.fallbacks) }

// HasGesture is whether gesture id has its own drawing (a pose image, or the
// motion spec's pose for parametric artwork) rather than the base pose.
func (c *Character) HasGesture(id string) bool {
	return c.gestures[id] != nil || c.spec["gestures."+id]
}

// Extent is the square, in pixels at device scale 1, that holds the
// character at size factor scale in every pose, accent glow included.
func (c *Character) Extent(scale float64) image.Point {
	n := int(math.Ceil(c.framePx * max(scale, 0)))
	return image.Pt(n, n)
}

// Anchor is where the strand leaves the character drawn in pose at size
// factor scale: logical px from the top-left of its Extent square. The
// point (strand.anchor, or the profile's artwork.anchor) rides each part of
// the pose by its strand.anchor.follow weight, so at full weight it stays on
// the same spot of the drawing as the art scales, squashes, leans and floats.
func (c *Character) Anchor(pose motion.Pose, scale float64) spec.Point {
	ext := c.Extent(scale)
	centre := point{float64(ext.X) / 2, float64(ext.Y) / 2}
	if !(scale > 0) {
		return spec.Point{X: centre.X, Y: centre.Y}
	}
	a := c.pick(pose)
	sc := pose.Scale
	if sc == 0 {
		sc = 1
	}
	f := c.follow
	followed := motion.Pose{
		Scale:  1 + (sc-1)*f.Scale,
		Squash: pose.Squash * f.Squash,
		Flat:   pose.Flat * f.Squash,
		LeanX:  pose.LeanX * f.Lean,
		LeanY:  pose.LeanY * f.Lift,
		HoverY: pose.HoverY * f.Hover,
	}
	vb := a.vb
	p := c.matrix(a, followed, centre, scale).apply(point{vb[0] + c.anchor.X*vb[2], vb[1] + c.anchor.Y*vb[3]})
	return spec.Point{X: p.X, Y: p.Y}
}

func (c *Character) pick(pose motion.Pose) *art {
	if a := c.gestures[pose.Gesture]; pose.Gesture != "" && a != nil {
		return a
	}
	if a := c.states[pose.State]; a != nil {
		return a
	}
	return c.base
}

// Draw clears dst, draws the character centred in it at size factor scale
// on a device scale of dpr, and returns the bounds of what it painted. A
// zero Scale or Bright in pose reads as 1.
func (c *Character) Draw(dst *image.NRGBA, pose motion.Pose, look Look, scale, dpr float64) image.Rectangle {
	c.mu.Lock()
	defer c.mu.Unlock()
	b := dst.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		o := dst.PixOffset(b.Min.X, y)
		clear(dst.Pix[o : o+b.Dx()*4])
	}
	k := scale * dpr
	if b.Empty() || !(k > 0) {
		return image.Rectangle{}
	}
	a := c.pick(pose)
	centre := point{float64(b.Min.X) + float64(b.Dx())/2, float64(b.Min.Y) + float64(b.Dy())/2}
	m := c.matrix(a, pose, centre, k)

	d := &c.d
	d.look = look
	d.depth, d.mdepth = 0, 0
	c.main.setRect(b)
	d.node(a.root, &c.main, m)

	bright := float32(pose.Bright)
	if bright == 0 {
		bright = 1
	}
	if bright != 1 {
		for i := 0; i < len(c.main.pix); i += 4 {
			al := c.main.pix[i+3]
			for ch := range 3 {
				c.main.pix[i+ch] = min(c.main.pix[i+ch]*bright, al)
			}
		}
	}

	result := &c.main
	if acc, on := lapAccent(pose.Lap, look); on {
		c.out.setRect(b)
		vb := a.vb
		ac := m.apply(point{vb[0] + vb[2]/2, vb[1] + vb[3]/2})
		d.drawAccent(&c.out, &c.main, acc, pose.Spin, ac, k, c.glow.Spread, c.outlinePx, c.lap.Core, c.lap.HeadWidth)
		composite(&c.out, &c.main, fullPlane(d, b))
		d.mdepth--
		result = &c.out
	}

	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		o := dst.PixOffset(b.Min.X, y)
		src := result.offset(b.Min.X, y)
		for x := b.Min.X; x < b.Max.X; x++ {
			al := result.pix[src+3]
			if al > 0.5/255 {
				inv := 1 / al
				dst.Pix[o] = channel(result.pix[src] * inv)
				dst.Pix[o+1] = channel(result.pix[src+1] * inv)
				dst.Pix[o+2] = channel(result.pix[src+2] * inv)
				dst.Pix[o+3] = channel(al)
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x+1), max(maxY, y+1)
			}
			o += 4
			src += 4
		}
	}
	if maxX <= minX {
		return image.Rectangle{}
	}
	return image.Rect(minX, minY, maxX, maxY)
}

func fullPlane(d *drawer, r image.Rectangle) *mask {
	m := d.plane(r)
	for i := range m.pix {
		m.pix[i] = 1
	}
	return m
}

func channel(v float32) uint8 {
	return uint8(min(max(v, 0), 1)*255 + 0.5)
}

// matrix maps the art's viewBox to device pixels: fitted into the button
// square at the frame's centre, then posed about the art's base (its bottom
// centre): scaled and squashed, leaned, raised by LeanY and floated by the
// hover offset.
func (c *Character) matrix(a *art, pose motion.Pose, centre point, k float64) affine {
	vb := a.vb
	s0 := c.buttonPx * k / max(vb[2], vb[3])
	fit := translate(centre.X, centre.Y).mul(scaling(s0, s0)).mul(translate(-(vb[0] + vb[2]/2), -(vb[1] + vb[3]/2)))
	base := point{centre.X, centre.Y + vb[3]*s0/2}

	sc := pose.Scale
	if sc == 0 {
		sc = 1
	}
	sq := min(max(pose.Squash+pose.Flat*c.flatSq, c.squash[0]), c.squash[1])
	lift := pose.HoverY*k + pose.LeanY*vb[3]*s0/4
	posed := translate(base.X, base.Y+lift).
		mul(rotation(pose.LeanX)).
		mul(scaling(sc*(1+sq), sc*(1-sq))).
		mul(translate(-base.X, -base.Y))
	return posed.mul(fit)
}
