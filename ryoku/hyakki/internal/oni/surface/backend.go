package surface

import (
	"context"
	"image"
	"time"

	"ryoku-rashin/internal/oni/placement"
	"ryoku-rashin/internal/oni/render"
	"ryoku-rashin/internal/oni/spec"
)

// Namespace is the layer-shell namespace compositors see for Oni.
const Namespace = "hyakki-oni"

// glowDPR is the most device px per logical px the Hands glow draws at: a
// soft gradient scaled up by the compositor looks the same and costs a
// quarter of the pixels at each breath.
const glowDPR = 0.5

// Protocols the surface looks for, in --globals order. The first four are
// required.
var Protocols = []string{
	"wl_compositor", "wl_shm", "zwlr_layer_shell_v1", "wl_subcompositor",
	"wl_seat", "wl_output", "zxdg_output_manager_v1",
	"wp_viewporter", "wp_fractional_scale_manager_v1",
	"zwlr_foreign_toplevel_manager_v1",
}

// Globals is what --globals prints: the advertised version of every protocol
// the surface uses, and the ones the compositor lacks.
type Globals struct {
	Display string            `json:"display"`
	Bound   map[string]uint32 `json:"bound"`
	Missing []string          `json:"missing"`
}

// presentation is one commit of the sprite surface.
type presentation struct {
	output string
	// origin is the surface's top-left, output-local logical px.
	origin image.Point
	// size is the surface size in logical px.
	size image.Point
	// img is the buffer in device px; nil moves the surface without new
	// pixels.
	img *image.NRGBA
	// input is the input region, surface-local logical px.
	input image.Rectangle
	// arm requests a frame callback.
	arm bool
	// grab holds the layer surface where it is while the button is held on
	// the character (see spriteOrigin).
	grab bool
}

// strandPresentation is one commit of the strand surface: a subsurface of the
// sprite's layer surface below the sprite's pixels, free to extend past the
// layer surface's size, that never takes the pointer.
type strandPresentation struct {
	output string
	// rect is the surface's place and size, output-local logical px.
	rect image.Rectangle
	// img is the buffer in device px.
	img *image.NRGBA
	// arm requests the frame callback unless one is already pending.
	arm bool
}

// overlayPresentation is one commit of an overlay surface (the perch chip,
// the drag highlight or the guide card): a subsurface of the sprite's layer
// surface, placed on the output like the strand.
type overlayPresentation struct {
	output string
	// rect is the surface's place and size, output-local logical px.
	rect image.Rectangle
	// img is the buffer in device px.
	img *image.NRGBA
	// input is the input region, surface-local logical px; empty lets the
	// pointer through.
	input image.Rectangle
}

// glowPresentation is one commit of the Hands glow on one output: its four
// strips (render.GlowTop and on), each its own input-less layer surface on
// the overlay layer, at rects (output-local logical px) with pixels imgs
// in device px. An empty rect has no surface; a nil image keeps a strip's
// pixels.
type glowPresentation struct {
	output string
	rects  [render.GlowStrips]image.Rectangle
	imgs   [render.GlowStrips]*image.NRGBA
}

// Overlay surfaces.
const (
	overlayChip overlay = iota
	overlayHighlight
	overlayCard
	// overlayPill is Hands' pill and overlayHandoff its handoff card.
	overlayPill
	overlayHandoff
	overlayNotice
	overlayCount
)

type overlay int

// backend is the display connection. Every method but post runs on the loop
// goroutine, which also delivers every sink call.
type backend interface {
	run(ctx context.Context) error
	post(fn func())
	outputs() []placement.Output
	// scale is device px per logical px for a surface on output.
	scale(output string) float64
	// present commits p and returns the origin the surface now has (a move
	// waits while the previous one is unconfirmed) and whether it committed:
	// a surface the compositor has not configured yet takes no buffer.
	present(p presentation) (origin image.Point, committed bool, err error)
	hide()
	covered(output string) bool
	foreignToplevels() bool
	close()
	// strandScale is device px per logical px for the strand on output,
	// at most limit (the spec's strand dpr cap; integer scales only when
	// the compositor has no viewporter).
	strandScale(output string, limit float64) float64
	// presentStrand commits p; it reports false, with no error, while the
	// sprite is not configured on that output.
	presentStrand(p strandPresentation) (committed bool, err error)
	// hideStrand unmaps the strand and frees its buffers; a frame callback
	// pending on it moves to the sprite.
	hideStrand()
	// presentOverlay commits p on surface which; false, with no error,
	// while the sprite is not configured on that output. It never arms a
	// frame callback: the sprite's or the strand's paces every step.
	presentOverlay(which overlay, p overlayPresentation) (committed bool, err error)
	// hideOverlay unmaps which and frees its buffers.
	hideOverlay(which overlay)
	// setPick maps one transparent input surface over each of outputs, on
	// the layer above everything, and unmaps the rest; an empty list maps
	// none. It never arms a frame callback.
	setPick(outputs []string)
	// glowScale is device px per logical px for the Hands glow on output:
	// at most glowDPR, since a soft gradient needs no more, and whole when
	// the compositor has no viewporter.
	glowScale(output string) float64
	// presentGlow commits p, making the output's glow surfaces first; it
	// reports false, with no error, until the compositor configured them
	// (sink.glowConfigured follows). Glow surfaces never take the pointer
	// or the keyboard and never arm a frame callback.
	presentGlow(p glowPresentation) (committed bool, err error)
	// hideGlow destroys the glow surfaces of every output not in keep; an
	// empty keep takes the whole glow down.
	hideGlow(keep []string)
}

// sink receives display events on the loop goroutine.
type sink interface {
	outputsChanged()
	scaleChanged()
	configured()
	frameDone(now time.Time)
	// pointerMotion is the pointer in global logical px.
	pointerMotion(p spec.Point)
	pointerLeave()
	button(pressed bool)
	coveredChanged()
	surfaceClosed()
	// chipPointer is the pointer over the chip, chip-local logical px; nil
	// when it left.
	chipPointer(p *spec.Point)
	// chipButton is the left button over the chip.
	chipButton(pressed bool)
	// cardPointer is the pointer over the guide card, card-local logical
	// px; nil when it left.
	cardPointer(p *spec.Point)
	// cardButton is the left button over the guide card.
	cardButton(pressed bool)
	// pickClick is one left click on the pick surface of output, at p,
	// output-local logical px.
	pickClick(output string, p spec.Point)
	// pickCancel is a right click on a pick surface.
	pickCancel()
	// handsPointer is the pointer over Hands' pill or handoff card
	// (which), surface-local logical px; nil when it left.
	handsPointer(which overlay, p *spec.Point)
	// handsButton is the left button over the pill or the handoff card.
	handsButton(which overlay, pressed bool)
	// glowConfigured says the glow surfaces on output can take pixels.
	glowConfigured(output string)
}
