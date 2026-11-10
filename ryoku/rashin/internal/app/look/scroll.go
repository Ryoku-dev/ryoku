package look

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

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
)

const (
	x11Notch   = 10
	x11NotchDP = 48
	thumbIdle  = 600 * time.Millisecond
)

var scrollOnX11 = os.Getenv("WAYLAND_DISPLAY") == ""

type Scroller struct {
	layout.List
	Preview kit.State

	tag          struct{}
	fraction     float32
	hovered      bool
	lastScroll   time.Time
	thumb        anim.Tween
	thumbOn      bool
	thumbReady   bool
	userScrolled bool
	listGesture  bool

	inputOps     *op.Ops
	inputAt      time.Time
	inputReady   bool
	inputBefore  layout.Position
	inputApplied int

	probeOps        op.Ops
	itemSizes       []image.Point
	itemCount       int
	cacheReady      bool
	listConstraints layout.Constraints
	programmatic    bool

	spring        anim.Spring
	springActive  bool
	springApplied float32
}

func mapScrollDelta(delta, pixelsPerDP float32, x11 bool) float32 {
	if x11 {
		notches := float32(math.Round(float64(delta / x11Notch)))
		if notches != 0 && float32(math.Abs(float64(delta-notches*x11Notch))) <= .5 {
			return notches * x11NotchDP * pixelsPerDP
		}
	}
	return delta * pixelsPerDP
}

func addListOffset(position *layout.Position, fraction *float32, delta float32) int {
	if delta < 0 {
		position.BeforeEnd = true
	}
	total := *fraction + delta
	whole := int(total)
	*fraction = total - float32(whole)
	position.Offset += whole
	return whole
}

func sameListPosition(a, b layout.Position) bool {
	return a.First == b.First && a.Offset == b.Offset
}

func itemExtent(sizes []image.Point, index, fallback int, axis layout.Axis) int {
	if index >= 0 && index < len(sizes) {
		size := axis.Convert(sizes[index]).X
		if size > 0 {
			return size
		}
	}
	return max(1, fallback)
}

func listPositionDelta(
	before, after layout.Position,
	sizes []image.Point,
	fallback, gap int,
	axis layout.Axis,
) int {
	delta := after.Offset - before.Offset
	if after.First > before.First {
		for index := before.First; index < after.First; index++ {
			delta += itemExtent(sizes, index, fallback, axis) + gap
		}
	} else {
		for index := after.First; index < before.First; index++ {
			delta -= itemExtent(sizes, index, fallback, axis) + gap
		}
	}
	return delta
}

func scrollThumb(viewport, content, first, offset, count, minimum int) (top, height int, ok bool) {
	if viewport <= 0 || content <= viewport || count <= 0 {
		return 0, 0, false
	}
	height = max(minimum, viewport*viewport/content)
	height = min(viewport, height)
	travel := viewport - height
	average := float64(content) / float64(count)
	scrolled := float64(first)*average + float64(offset)
	maximum := float64(content - viewport)
	if maximum > 0 {
		top = int(math.Round(float64(travel) * min(max(scrolled/maximum, 0), 1)))
	}
	return top, height, true
}

func (scroller *Scroller) setThumb(gtx layout.Context, th *Theme, shown bool) float32 {
	duration := th.Motion.HoverOut
	if shown {
		duration = th.Motion.HoverIn
	}
	scroller.thumb.Duration = duration
	scroller.thumb.Ease = anim.EaseChisel
	if !scroller.thumbReady {
		scroller.thumb.Set(gtx, 0)
		scroller.thumbReady = true
	}
	if shown != scroller.thumbOn {
		scroller.thumbOn = shown
		if shown {
			scroller.thumb.Set(gtx, 1)
		} else {
			scroller.thumb.Set(gtx, 0)
		}
	}
	return scroller.thumb.Value(gtx)
}

func (scroller *Scroller) stopSpring() {
	if scroller.springActive {
		scroller.spring.Snap(scroller.springApplied)
		scroller.springActive = false
	}
}

func (scroller *Scroller) updateHover(gtx layout.Context) {
	for {
		raw, ok := gtx.Event(pointer.Filter{Target: &scroller.tag, Kinds: pointer.Enter | pointer.Leave})
		if !ok {
			return
		}
		e, ok := raw.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Enter:
			scroller.hovered = true
		case pointer.Leave:
			scroller.hovered = false
		}
	}
}

func (scroller *Scroller) probeInput(gtx layout.Context) int {
	if !scroller.cacheReady || scroller.itemCount <= 0 {
		return 0
	}
	before := scroller.Position
	fallback := max(1, before.Length/max(scroller.itemCount, 1))
	scroller.probeOps.Reset()
	probe := gtx
	probe.Ops = &scroller.probeOps
	probe.Constraints = scroller.listConstraints
	scroller.List.Layout(probe, scroller.itemCount, func(_ layout.Context, index int) layout.Dimensions {
		size := image.Point{}
		if index >= 0 && index < len(scroller.itemSizes) {
			size = scroller.itemSizes[index]
		}
		if size.Y <= 0 {
			size.Y = fallback
		}
		return layout.Dimensions{Size: size}
	})
	after := scroller.Position
	if sameListPosition(before, after) {
		if !scroller.List.Dragging() {
			scroller.listGesture = false
		}
		return 0
	}
	raw := float32(listPositionDelta(before, after, scroller.itemSizes, fallback, scroller.Gap, layout.Vertical))
	dragging := scroller.List.Dragging()
	if !dragging && !scroller.listGesture {
		raw = mapScrollDelta(raw, gtx.Metric.PxPerDp, scrollOnX11)
	}
	scroller.Position = before
	addListOffset(&scroller.Position, &scroller.fraction, raw)
	scroller.listGesture = dragging || scroller.listGesture
	if !scroller.programmatic {
		scroller.stopSpring()
		scroller.userScrolled = true
		scroller.lastScroll = gtx.Now
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(thumbIdle)})
	}
	return 1
}

func (scroller *Scroller) UpdateInput(gtx layout.Context) {
	if scroller.inputReady && scroller.inputOps == gtx.Ops && scroller.inputAt == gtx.Now {
		return
	}
	scroller.inputOps, scroller.inputAt, scroller.inputReady = gtx.Ops, gtx.Now, true
	scroller.inputBefore = scroller.Position
	scroller.updateHover(gtx)
	scroller.inputApplied = scroller.probeInput(gtx)
	scroller.inputApplied += scroller.advanceSpring(gtx)
}

func (scroller *Scroller) advanceSpring(gtx layout.Context) int {
	if !scroller.springActive {
		return 0
	}
	value := scroller.spring.Value(gtx)
	delta := value - scroller.springApplied
	scroller.springApplied = value
	applied := addListOffset(&scroller.Position, &scroller.fraction, delta)
	if !scroller.spring.Moving() {
		scroller.springActive = false
	}
	return applied
}

func (scroller *Scroller) Layout(
	gtx layout.Context,
	th *Theme,
	count int,
	element layout.ListElement,
) layout.Dimensions {
	scroller.Axis = layout.Vertical
	scroller.UpdateInput(gtx)
	scroller.listConstraints = gtx.Constraints
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, &scroller.tag)
	area.Pop()
	pass.Pop()

	beforeList := scroller.Position
	if len(scroller.itemSizes) < count {
		scroller.itemSizes = append(scroller.itemSizes, make([]image.Point, count-len(scroller.itemSizes))...)
	}
	scroller.itemCount = count
	dimensions := scroller.List.Layout(gtx, count, func(gtx layout.Context, index int) layout.Dimensions {
		dimensions := element(gtx, index)
		scroller.itemSizes[index] = dimensions.Size
		return dimensions
	})
	scroller.cacheReady = true
	dragging := scroller.List.Dragging()
	movedByList := !sameListPosition(beforeList, scroller.Position)
	if !scroller.programmatic && movedByList && (dragging || scroller.listGesture) {
		scroller.stopSpring()
		scroller.userScrolled = true
		scroller.lastScroll = gtx.Now
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(thumbIdle)})
	}
	if dragging {
		scroller.listGesture = true
	} else if !movedByList {
		scroller.listGesture = false
	}
	if scroller.inputApplied != 0 && sameListPosition(scroller.inputBefore, scroller.Position) {
		scroller.fraction = 0
		scroller.stopSpring()
	}
	scroller.programmatic = false

	shown := scroller.hovered || (!scroller.lastScroll.IsZero() && gtx.Now.Before(scroller.lastScroll.Add(thumbIdle)))
	if scroller.Preview == kit.StateHover || scroller.Preview == kit.StatePressed {
		shown = true
	}
	alpha := scroller.setThumb(gtx, th, shown)
	if alpha <= 0 {
		return dimensions
	}
	DrawThumb(gtx, th, dimensions.Size, scroller.Position, count, alpha)
	return dimensions
}

func DrawThumb(gtx layout.Context, th *Theme, size image.Point, position layout.Position, count int, alpha float32) {
	top, height, ok := scrollThumb(size.Y, position.Length, position.First, position.Offset, count, gtx.Dp(12))
	if !ok || alpha <= 0 {
		return
	}
	width := max(1, gtx.Dp(3))
	x := max(0, size.X-gtx.Dp(4)-width)
	kit.FillCut(gtx.Ops, image.Rect(x, top, x+width, top+height), min(width/2, height/2), palette.WithAlpha(th.R.Vein, alpha))
}

func (scroller *Scroller) AnchorTo(index, offsetPx int) {
	scroller.stopSpring()
	scroller.Position.First = max(index, 0)
	scroller.Position.Offset = -offsetPx
	scroller.Position.BeforeEnd = true
	scroller.fraction = 0
	scroller.programmatic = true
}

func (scroller *Scroller) ScrollTo(index int) { scroller.AnchorTo(index, 0) }

func (scroller *Scroller) SpringBy(deltaPx int) {
	scroller.stopSpring()
	if anim.Reduced() {
		addListOffset(&scroller.Position, &scroller.fraction, float32(deltaPx))
		scroller.programmatic = true
		return
	}
	scroller.spring = anim.Spring{Omega: 4 / .240, Rest: .25}
	scroller.spring.Snap(0)
	scroller.spring.Set(float32(deltaPx))
	scroller.springApplied = 0
	scroller.springActive = scroller.spring.Moving()
	scroller.programmatic = true
}

func (scroller *Scroller) UserScrolled() bool {
	scrolled := scroller.userScrolled
	scroller.userScrolled = false
	return scrolled
}

func (scroller *Scroller) Offset() (first, px int) {
	return scroller.Position.First, scroller.Position.Offset
}
