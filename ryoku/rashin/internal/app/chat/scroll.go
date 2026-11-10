package chat

import (
	"image"
	"math"
	"os"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
)

var transcriptScrollOnX11 = os.Getenv("WAYLAND_DISPLAY") == ""

const thumbIdle = 600 * time.Millisecond

type transcriptScroller struct {
	layout.List
	tag           struct{}
	fraction      float32
	userScrolled  bool
	follow        bool
	overflowKnown bool
	lastScroll    time.Time
}

// asks for one more frame when the moment ends so the thumb can leave
func (scroller *transcriptScroller) Active(gtx layout.Context) bool {
	until := scroller.lastScroll.Add(thumbIdle)
	if scroller.lastScroll.IsZero() || !gtx.Now.Before(until) {
		return false
	}
	gtx.Execute(op.InvalidateCmd{At: until})
	return true
}

func transcriptScrollDelta(delta, pixelsPerDP float32) float32 {
	if transcriptScrollOnX11 {
		notches := float32(math.Round(float64(delta / 10)))
		if notches != 0 && float32(math.Abs(float64(delta-notches*10))) <= .5 {
			return notches * 48 * pixelsPerDP
		}
	}
	return delta * pixelsPerDP
}

func addTranscriptOffset(position *layout.Position, fraction *float32, delta float32) int {
	if delta < 0 {
		position.BeforeEnd = true
	}
	total := *fraction + delta
	whole := int(total)
	*fraction = total - float32(whole)
	position.Offset += whole
	return whole
}

func (scroller *transcriptScroller) updateInput(gtx layout.Context) {
	const limit = 1 << 30
	for {
		raw, ok := gtx.Event(pointer.Filter{
			Target:  &scroller.tag,
			Kinds:   pointer.Scroll,
			ScrollY: pointer.ScrollRange{Min: -limit, Max: limit},
		})
		if !ok {
			return
		}
		scroll, ok := raw.(pointer.Event)
		if !ok || scroll.Scroll.Y == 0 {
			continue
		}
		delta := transcriptScrollDelta(scroll.Scroll.Y, gtx.Metric.PxPerDp)
		addTranscriptOffset(&scroller.Position, &scroller.fraction, delta)
		scroller.userScrolled, scroller.lastScroll = true, gtx.Now
	}
}

func (scroller *transcriptScroller) Layout(gtx layout.Context, count int, element layout.ListElement) layout.Dimensions {
	scroller.Axis = layout.Vertical
	scroller.ScrollToEnd = scroller.follow && scroller.overflowKnown
	scroller.updateInput(gtx)

	source := gtx.Source
	listGTX := gtx.Disabled()
	length := 0
	list := func() (layout.Dimensions, op.CallOp) {
		length = 0
		record := op.Record(gtx.Ops)
		dimensions := scroller.List.Layout(listGTX, count, func(child layout.Context, index int) layout.Dimensions {
			child.Source = source
			dimensions := element(child, index)
			length += dimensions.Size.Y
			return dimensions
		})
		return dimensions, record.Stop()
	}
	dimensions, call := list()
	switch whole := scroller.Position.First == 0 && scroller.Position.Count == count; {
	// a short transcript stays at the top, so the first frame that overflows is laid out twice and I never show it from the top
	case !scroller.overflowKnown && count > 0 && (scroller.Position.Count < count || scroller.Position.BeforeEnd):
		scroller.overflowKnown = true
		if scroller.follow {
			scroller.ScrollToEnd, scroller.Position.BeforeEnd = true, false
			dimensions, call = list()
		}
	// its room can be small for a moment, as while the composer travels down the page; once all of it fits again it goes back to the top rather than hang from the end
	case scroller.overflowKnown && whole && length <= gtx.Constraints.Max.Y:
		scroller.overflowKnown, scroller.ScrollToEnd, scroller.Position = false, false, layout.Position{}
		dimensions, call = list()
	}
	call.Add(gtx.Ops)

	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops)
	event.Op(gtx.Ops, &scroller.tag)
	area.Pop()
	pass.Pop()
	return dimensions
}

func (scroller *transcriptScroller) UserScrolled() bool {
	scrolled := scroller.userScrolled
	scroller.userScrolled = false
	return scrolled
}

func (scroller *transcriptScroller) AtEnd() bool {
	return !scroller.overflowKnown || !scroller.Position.BeforeEnd
}

func (scroller *transcriptScroller) SetFollow(follow bool) {
	scroller.follow = follow
}

func (scroller *transcriptScroller) JumpToEnd() {
	scroller.follow = true
	scroller.Position.BeforeEnd = false
	scroller.fraction = 0
}
