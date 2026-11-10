package chat

import (
	"image"
	"math"
	"os"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

var transcriptScrollOnX11 = os.Getenv("WAYLAND_DISPLAY") == ""

type transcriptScroller struct {
	layout.List
	tag          struct{}
	fraction     float32
	userScrolled bool
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
		scroller.userScrolled = true
	}
}

func (scroller *transcriptScroller) Layout(gtx layout.Context, count int, element layout.ListElement) layout.Dimensions {
	scroller.Axis = layout.Vertical
	scroller.ScrollToEnd = true
	scroller.updateInput(gtx)

	source := gtx.Source
	listGTX := gtx.Disabled()
	dimensions := scroller.List.Layout(listGTX, count, func(child layout.Context, index int) layout.Dimensions {
		child.Source = source
		return element(child, index)
	})

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
	return !scroller.Position.BeforeEnd
}

func (scroller *transcriptScroller) JumpToEnd() {
	scroller.Position.BeforeEnd = false
	scroller.fraction = 0
}
