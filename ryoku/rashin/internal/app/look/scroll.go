package look

import (
	"image"
	"math"
	"os"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
)

const scrollRangeLimit = 1 << 30

var scrollOnX11 = os.Getenv("WAYLAND_DISPLAY") == ""

// Scroller virtualises a vertical list and reapplies every physical scroll
// delta immediately. Measured item sizes keep mixed-height rows exact.
type Scroller struct {
	layout.List
	tag             struct{}
	fraction        float32
	probeOps        op.Ops
	itemSizes       []image.Point
	itemCount       int
	cacheReady      bool
	listConstraints layout.Constraints
	inputOps        *op.Ops
	inputReady      bool
	inputAt         int64
	userScrolled    bool
}

func mapScrollDelta(delta, pixelsPerDP float32, x11 bool) float32 {
	if x11 {
		notches := float32(math.Round(float64(delta / 10)))
		if notches != 0 && float32(math.Abs(float64(delta-notches*10))) <= .5 {
			return notches * 48 * pixelsPerDP
		}
	}
	return delta * pixelsPerDP
}

func verticalScrollRange(position layout.Position) pointer.ScrollRange {
	var bounds pointer.ScrollRange
	if position.First > 0 || position.Offset > 0 {
		bounds.Min = -scrollRangeLimit
	}
	if position.BeforeEnd {
		bounds.Max = scrollRangeLimit
	}
	return bounds
}

func addOffset(position *layout.Position, fraction *float32, delta float32) {
	total := *fraction + delta
	whole := int(total)
	*fraction = total - float32(whole)
	position.Offset += whole
	if delta < 0 {
		position.BeforeEnd = true
	}
}

func itemExtent(sizes []image.Point, index, fallback int) int {
	if index >= 0 && index < len(sizes) && sizes[index].Y > 0 {
		return sizes[index].Y
	}
	return max(1, fallback)
}

func positionDelta(before, after layout.Position, sizes []image.Point, fallback, gap int) int {
	delta := after.Offset - before.Offset
	if after.First > before.First {
		for index := before.First; index < after.First; index++ {
			delta += itemExtent(sizes, index, fallback) + gap
		}
	} else {
		for index := after.First; index < before.First; index++ {
			delta -= itemExtent(sizes, index, fallback) + gap
		}
	}
	return delta
}

func samePosition(left, right layout.Position) bool {
	return left.First == right.First && left.Offset == right.Offset
}

func (s *Scroller) updateInput(gtx layout.Context) {
	stamp := gtx.Now.UnixNano()
	if s.inputReady && s.inputOps == gtx.Ops && s.inputAt == stamp {
		return
	}
	s.inputReady, s.inputOps, s.inputAt = true, gtx.Ops, stamp
	if !s.cacheReady || s.itemCount == 0 {
		return
	}
	before := s.Position
	fallback := max(1, before.Length/max(s.itemCount, 1))
	s.probeOps.Reset()
	probe := gtx
	probe.Ops = &s.probeOps
	probe.Constraints = s.listConstraints
	s.List.Layout(probe, s.itemCount, func(_ layout.Context, index int) layout.Dimensions {
		size := image.Pt(1, fallback)
		if index < len(s.itemSizes) && s.itemSizes[index].Y > 0 {
			size = s.itemSizes[index]
		}
		return layout.Dimensions{Size: size}
	})
	after := s.Position
	if samePosition(before, after) {
		return
	}
	delta := float32(positionDelta(before, after, s.itemSizes, fallback, s.Gap))
	if !s.List.Dragging() {
		delta = mapScrollDelta(delta, gtx.Metric.PxPerDp, scrollOnX11)
	}
	s.Position = before
	addOffset(&s.Position, &s.fraction, delta)
	s.userScrolled = true
}

func (s *Scroller) Layout(gtx layout.Context, th *Theme, count int, element layout.ListElement) layout.Dimensions {
	s.Axis = layout.Vertical
	s.updateInput(gtx)
	s.listConstraints = gtx.Constraints
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, &s.tag)
	area.Pop()
	pass.Pop()
	if len(s.itemSizes) < count {
		s.itemSizes = append(s.itemSizes, make([]image.Point, count-len(s.itemSizes))...)
	}
	s.itemCount = count
	dimensions := s.List.Layout(gtx, count, func(gtx layout.Context, index int) layout.Dimensions {
		dimensions := element(gtx, index)
		s.itemSizes[index] = dimensions.Size
		return dimensions
	})
	s.cacheReady = true
	if dimensions.Size.Y > 0 && s.Position.Length > dimensions.Size.Y {
		viewport, content := dimensions.Size.Y, s.Position.Length
		height := max(gtx.Dp(32), viewport*viewport/content)
		travel := viewport - height
		average := float64(content) / float64(max(count, 1))
		scrolled := float64(s.Position.First)*average + float64(s.Position.Offset)
		maximum := float64(content - viewport)
		top := 0
		if maximum > 0 {
			top = int(float64(travel) * min(max(scrolled/maximum, 0), 1))
		}
		width := max(1, gtx.Dp(3))
		kit.FillCut(gtx.Ops, image.Rect(max(0, dimensions.Size.X-width-4), top, max(0, dimensions.Size.X-4), top+height), min(width/2, height/2), th.R.Vein)
	}
	return dimensions
}

func (s *Scroller) ScrollTo(index int) {
	s.Position.First = max(0, index)
	s.Position.Offset = 0
	s.Position.BeforeEnd = true
	s.fraction = 0
}

func (s *Scroller) UserScrolled() bool {
	value := s.userScrolled
	s.userScrolled = false
	return value
}
