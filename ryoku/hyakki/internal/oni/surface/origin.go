package surface

import (
	"image"

	"ryoku-rashin/internal/oni/spec"
)

// The sprite is two surfaces: the layer surface, which places it and takes
// the pointer, and a subsurface with the pixels. Pointer events are relative
// to the layer surface, but compositors disagree on which origin of a moving
// one: niri reports every event of a press against where the surface was
// when the button went down, others against where it is now, and a moved
// layer surface settles at the commit on some and only at the next draw on
// others. Reading an event against the wrong origin fed each drag step back
// into the next until the character ran into the screen corner. So while the
// button is held the layer surface does not move at all, and only the
// subsurface follows the pointer; any other move of the layer surface is
// confirmed by a frame callback before pointer positions are trusted again.

// spriteOrigin is the layer surface's origin (output-local logical px) as the
// compositor has it.
type spriteOrigin struct {
	applied image.Point
	pending image.Point
	moving  bool
}

// place asks for the layer surface where presentation p puts the sprite. It
// reports whether the next commit carries new margins; that commit must also
// request the frame callback whose done calls rendered. A held press keeps
// the layer surface where it is, and a move while another is unconfirmed
// waits; the subsurface puts the pixels where they belong either way.
func (o *spriteOrigin) place(p presentation) bool {
	if p.grab || o.moving || p.origin == o.applied {
		return false
	}
	o.pending, o.moving = p.origin, true
	return true
}

// rendered is the done of the frame callback committed with a move.
func (o *spriteOrigin) rendered() {
	o.applied, o.moving = o.pending, false
}

// requested is the origin the latest commit asked for.
func (o *spriteOrigin) requested() image.Point {
	if o.moving {
		return o.pending
	}
	return o.applied
}

// global maps a pointer position relative to the layer surface to global
// logical px on an output whose top-left is outMin; ok is false while a move
// is unconfirmed.
func (o *spriteOrigin) global(outMin image.Point, x, y float64) (p spec.Point, ok bool) {
	if o.moving {
		return spec.Point{}, false
	}
	return spec.Point{X: float64(outMin.X+o.applied.X) + x, Y: float64(outMin.Y+o.applied.Y) + y}, true
}

// layerSize is the layer surface's size for a sprite of size sprite whose
// layer surface is at at (output-local logical px) on an output of size out,
// while parts (the strand and the perch overlays, its subsurfaces, as
// output-local rects; empty when not out) are shown. The protocol does not
// clip a subsurface to its parent, but Hyprland squeezes the part of one
// that passes its layer surface's right or bottom edge into that edge (past
// the left or top edge it draws as it should). So while any part is out the
// layer surface reaches the output's bottom-right corner and every part's,
// which keeps its size steady while the strand grows; with none out it is
// sprite-sized. Its buffer is transparent and its input region stays the
// character's, so the larger surface takes no pointer events.
func layerSize(sprite, at, out image.Point, parts ...image.Rectangle) image.Point {
	size, shown := sprite, false
	for _, r := range parts {
		if r.Empty() {
			continue
		}
		shown = true
		size.X = max(size.X, r.Max.X-at.X)
		size.Y = max(size.Y, r.Max.Y-at.Y)
	}
	if shown {
		size.X = max(size.X, out.X-at.X)
		size.Y = max(size.Y, out.Y-at.Y)
	}
	return size
}
