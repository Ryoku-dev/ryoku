package surface

import (
	"image"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

// simLayer is a compositor holding the layer surface. It moves the surface
// as it processes the commit or only when it next draws, and reports pointer
// events against where it shows the surface or, like niri during a press,
// against where the surface was when the button went down.
type simLayer struct {
	movesWhenDrawn, pressOrigin bool
	shown, latest, pressedAt    image.Point
	pressed                     bool
}

func (c *simLayer) commit(p image.Point) {
	c.latest = p
	if !c.movesWhenDrawn {
		c.shown = p
	}
}

func (c *simLayer) draw() { c.shown = c.latest }

func (c *simLayer) press(down bool) { c.pressed, c.pressedAt = down, c.shown }

// local is the surface-local position the compositor reports for a pointer
// at global g.
func (c *simLayer) local(outMin, g image.Point) (float64, float64) {
	origin := c.shown
	if c.pressOrigin && c.pressed {
		origin = c.pressedAt
	}
	l := g.Sub(outMin).Sub(origin)
	return float64(l.X), float64(l.Y)
}

func TestDragFollowsThePointerWhicheverWayTheCompositorReportsIt(t *testing.T) {
	for _, tc := range []struct {
		name               string
		drawn, pressOrigin bool
	}{
		{"moves at the commit", false, false},
		{"moves when it draws", true, false},
		{"reports a press against where it began", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outMin := image.Pt(1920, 0)
			start, size := image.Pt(1700, 860), image.Pt(180, 180)
			grab := image.Pt(90, 90)
			from := outMin.Add(start).Add(grab)
			to := outMin.Add(image.Pt(612, 450))
			want := to.Sub(grab).Sub(outMin)

			comp := &simLayer{movesWhenDrawn: tc.drawn, pressOrigin: tc.pressOrigin, shown: start, latest: start}
			o := spriteOrigin{applied: start}
			var g grip
			now := time.Unix(0, 0)
			target := start
			event := func(pt image.Point) {
				x, y := comp.local(outMin, pt)
				p, ok := o.global(outMin, x, y)
				if !ok {
					return
				}
				now = now.Add(4 * time.Millisecond)
				if _, dragging, pos := g.move(p, now); dragging {
					target = pos.Sub(outMin)
				}
			}
			// The host commits as the frame callback arrives, so the pointer
			// events of a frame land between its commit and the next draw.
			frame := func() {
				comp.draw()
				if o.moving {
					o.rendered()
				}
				if o.place(presentation{origin: target, size: size, grab: g.pressed}) {
					comp.commit(target)
				}
			}

			x, y := comp.local(outMin, from)
			p, ok := o.global(outMin, x, y)
			if !ok {
				t.Fatal("the pointer on a sprite at rest was not mapped")
			}
			comp.press(true)
			g.press(outMin.Add(start), p, now)
			lo, hi := image.Pt(min(start.X, want.X), min(start.Y, want.Y)), image.Pt(max(start.X, want.X), max(start.Y, want.Y))
			const steps = 24
			for i := 1; i <= steps; i++ {
				event(from.Add(to.Sub(from).Mul(i).Div(steps)))
				frame()
				if r := target; r.X < lo.X || r.Y < lo.Y || r.X > hi.X || r.Y > hi.Y {
					t.Fatalf("step %d: the sprite went to %v, outside the drag from %v to %v", i, r, start, want)
				}
			}
			event(to)
			if !g.release() {
				t.Fatal("the drag was taken for a click")
			}
			comp.press(false)
			if target != want {
				t.Fatalf("dropped at %v, want %v under the pointer", target, want)
			}
			frame()
			frame()
			if got := o.requested(); got != want {
				t.Fatalf("after the drop the layer surface is at %v, want %v", got, want)
			}
			x, y = comp.local(outMin, to)
			if p, ok := o.global(outMin, x, y); !ok || p != (spec.Point{X: float64(to.X), Y: float64(to.Y)}) {
				t.Fatalf("after the drop the pointer maps to %v %v, want %v", p, ok, to)
			}
		})
	}
}

func TestPointerIsTrustedOnlyOnceAMoveIsDrawn(t *testing.T) {
	o := spriteOrigin{applied: image.Pt(10, 20)}
	if p, ok := o.global(image.Pt(100, 0), 5, 6); !ok || p != (spec.Point{X: 115, Y: 26}) {
		t.Fatalf("at rest: %v %v", p, ok)
	}
	if o.place(presentation{origin: image.Pt(40, 50), grab: true}) || o.requested() != image.Pt(10, 20) {
		t.Fatal("the layer surface moved while the button was held")
	}
	if !o.place(presentation{origin: image.Pt(40, 50)}) {
		t.Fatal("a new origin sent no margins")
	}
	if o.place(presentation{origin: image.Pt(60, 70)}) {
		t.Fatal("a second move went out before the first was drawn")
	}
	if _, ok := o.global(image.Pt(100, 0), 5, 6); ok {
		t.Fatal("a pointer position was trusted while the move was in flight")
	}
	if o.requested() != image.Pt(40, 50) {
		t.Fatalf("requested = %v", o.requested())
	}
	o.rendered()
	if p, ok := o.global(image.Pt(100, 0), 5, 6); !ok || p != (spec.Point{X: 145, Y: 56}) {
		t.Fatalf("after the draw: %v %v", p, ok)
	}
	if o.place(presentation{origin: image.Pt(40, 50)}) {
		t.Fatal("asking for the origin it has sent margins")
	}
}

// squishedRect is where a compositor like Hyprland shows a subsurface at r
// (output-local) of a layer surface at at with size size: the part of it
// past the layer surface's right or bottom edge is squeezed into that edge,
// and a subsurface wholly past it shows nothing.
func squishedRect(at, size image.Point, r image.Rectangle) image.Rectangle {
	l := r.Sub(at)
	w, h := l.Dx(), l.Dy()
	if l.Min.X+w > size.X {
		w = size.X - l.Min.X
	}
	if l.Min.Y+h > size.Y {
		h = size.Y - l.Min.Y
	}
	if w <= 0 || h <= 0 {
		return image.Rectangle{}
	}
	return image.Rect(l.Min.X, l.Min.Y, l.Min.X+w, l.Min.Y+h).Add(at)
}

// The layer surface always covers the strand and the perch overlays, so a
// compositor that squeezes a subsurface into its parent shows every part
// where it was drawn; with no part out it is the sprite's size.
func TestEveryPartShowsWhereItIsOnACompositorThatSquishesSubsurfaces(t *testing.T) {
	out, sprite := image.Pt(1920, 1080), image.Pt(180, 180)
	for _, tc := range []struct {
		name  string
		at    image.Point
		parts []image.Rectangle
		// oldRight is where a sprite-sized layer surface showed the first
		// part's right edge (its own when nothing was cut); the Hyprland lab
		// measured the highlight's and the perch curl's
		oldRight int
	}{
		// the drag highlight under a carried character: the lab drew its
		// outline's right edge at x 1606 instead of 1766
		{"highlight wider than the sprite", image.Pt(1426, 457), []image.Rectangle{image.Rect(1030, 110, 1770, 590)}, 1606},
		// a pointing strand down to Save notes and the chip below the sprite
		{"strand below and right, chip below", image.Pt(1552, 126), []image.Rectangle{image.Rect(1618, 78, 1802, 486), image.Rect(1527, 312, 1758, 335)}, 1732},
		{"perch curl past the sprite's right edge", image.Pt(1552, 126), []image.Rectangle{image.Rect(1618, 78, 1802, 274)}, 1732},
		{"strand up and to the left only", image.Pt(1700, 860), []image.Rectangle{image.Rect(900, 300, 1780, 900)}, 1780},
		{"curl past the output's right edge", image.Pt(1730, 40), []image.Rectangle{image.Rect(1700, 10, 1935, 200)}, 1910},
	} {
		if cut := squishedRect(tc.at, sprite, tc.parts[0]); cut.Max.X != tc.oldRight {
			t.Fatalf("%s: a sprite-sized layer surface shows the part to x %d, want %d", tc.name, cut.Max.X, tc.oldRight)
		}
		size := layerSize(sprite, tc.at, out, tc.parts...)
		for _, r := range tc.parts {
			if got := squishedRect(tc.at, size, r); got != r {
				t.Fatalf("%s: part %v shows as %v under a %v layer surface", tc.name, r, got, size)
			}
		}
		if size.X < sprite.X || size.Y < sprite.Y {
			t.Fatalf("%s: layer surface %v is smaller than the sprite", tc.name, size)
		}
	}
	at := image.Pt(1552, 126)
	if got := layerSize(sprite, at, out); got != sprite {
		t.Fatalf("nothing out: %v, want the sprite's %v", got, sprite)
	}
	if got := layerSize(sprite, at, out, image.Rectangle{}, image.Rectangle{}); got != sprite {
		t.Fatalf("parts gone: %v, want the sprite's %v", got, sprite)
	}
	// a strand growing inside the output keeps the layer surface's size, so
	// it is not resized on every frame of a reach
	short := layerSize(sprite, at, out, image.Rect(1600, 200, 1640, 240))
	long := layerSize(sprite, at, out, image.Rect(1200, 60, 1900, 1000))
	if short != long || short != image.Pt(1920-1552, 1080-126) {
		t.Fatalf("a growing strand resized the layer surface: %v then %v", short, long)
	}
}
