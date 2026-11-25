package anim

import (
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// RevealGroups is how many groups of a page rise as it opens; later ones stand
// (SettingsPage.qml:773-777).
const RevealGroups = 5

// Reveal is a group's entrance, ryoku.dev's "rise" as the Hub's Reveal.qml plays it: the group
// fades in and settles from a few pixels below, once, a stagger after the group before it.
// Only opacity and a translate move, so nothing lays out again.
//
// Order is the group's place on the page from 0. Group n waits (n + 1) staggers, as the Hub
// gives section n the order n + 1; only the first RevealGroups groups rise. A Reveal plays on
// its first Step and never re-arms: keep it beside the group and a later rebuild moves nothing.
// Under reduced motion it never arms and the group simply stands.
type Reveal struct {
	Order int

	start time.Time
	state revealState
}

type revealState uint8

const (
	revealIdle revealState = iota
	revealPlaying
	revealDone
)

// Step returns the group's opacity and its downward offset, in rise's unit, at gtx.Now. d is
// the move (the Hub uses th.Motion.Navigate, 200 ms), stagger the step between groups
// (th.Motion.RevealStagger, 40 ms) and rise the starting offset (tokens.SizeWorkPaneRise,
// 8 dp, in pixels). The move runs on the chisel curve. It requests a frame for the moment the
// group starts and every frame while it moves, and none once it stands.
func (reveal *Reveal) Step(gtx layout.Context, d, stagger time.Duration, rise float32) (opacity, offset float32) {
	switch reveal.state {
	case revealDone:
		return 1, 0
	case revealIdle:
		if Reduced() || reveal.Order < 0 || reveal.Order >= RevealGroups || d <= 0 {
			reveal.state = revealDone
			return 1, 0
		}
		reveal.start, reveal.state = gtx.Now, revealPlaying
	}
	if Reduced() {
		reveal.state = revealDone
		return 1, 0
	}
	begin := reveal.start.Add(time.Duration(reveal.Order+1) * stagger)
	if gtx.Now.Before(begin) {
		gtx.Execute(op.InvalidateCmd{At: begin})
		return 0, rise
	}
	progress := float32(gtx.Now.Sub(begin)) / float32(d)
	if progress >= 1 {
		reveal.state = revealDone
		return 1, 0
	}
	gtx.Execute(op.InvalidateCmd{})
	eased := EaseChisel(progress)
	return eased, rise * (1 - eased)
}

// Done reports whether the group stands: it has risen, or never will.
func (reveal *Reveal) Done() bool { return reveal.state == revealDone }

// Rise is the Hub's page change (Hub.qml:1004-1022): the new page fades in while it slides in
// from the right to its place, both on OutCubic. Zero fields take the Hub's values: a 240 ms
// fade (Tone.move), a 300 ms slide (Tone.move + 60) and a 14 dp shift. Under reduced motion it
// does not play.
type Rise struct {
	Fade  time.Duration
	Slide time.Duration
	Shift unit.Dp

	start time.Time
}

// Hub values for Rise.
const (
	RiseFade  = 240 * time.Millisecond
	RiseSlide = 300 * time.Millisecond
	RiseShift = unit.Dp(14)
)

// Start plays the rise from gtx.Now, for a page that has just opened.
func (rise *Rise) Start(gtx layout.Context) {
	if Reduced() {
		rise.start = time.Time{}
		return
	}
	rise.start = gtx.Now
	gtx.Execute(op.InvalidateCmd{})
}

// Value returns the page's opacity and its rightward offset in pixels at gtx.Now, requesting
// frames only while it moves.
func (rise *Rise) Value(gtx layout.Context) (opacity, offset float32) {
	if rise.start.IsZero() {
		return 1, 0
	}
	if Reduced() {
		rise.start = time.Time{}
		return 1, 0
	}
	fade, slide, shift := rise.Fade, rise.Slide, rise.Shift
	if fade <= 0 {
		fade = RiseFade
	}
	if slide <= 0 {
		slide = RiseSlide
	}
	if shift == 0 {
		shift = RiseShift
	}
	elapsed := gtx.Now.Sub(rise.start)
	if elapsed >= fade && elapsed >= slide {
		rise.start = time.Time{}
		return 1, 0
	}
	gtx.Execute(op.InvalidateCmd{})
	opacity = EaseOutCubic(float32(elapsed) / float32(fade))
	offset = float32(gtx.Dp(shift)) * (1 - EaseOutCubic(float32(elapsed)/float32(slide)))
	return opacity, offset
}

// Active reports whether the rise is playing.
func (rise *Rise) Active() bool { return !rise.start.IsZero() }
