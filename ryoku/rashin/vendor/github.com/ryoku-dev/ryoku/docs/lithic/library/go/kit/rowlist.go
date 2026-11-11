package kit

import (
	"image"
	"math"
	"os"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

const (
	scrollRangeLimit = 1 << 30
	notchWayland     = 150
	notchX11         = 10
	notchTolerance   = .5
	notchDecay       = 5
	scrollEpsilon    = .001
)

var rowListOnX11 = os.Getenv("WAYLAND_DISPLAY") == ""

type lithicList struct {
	widget.List
	scrollTag  uint8
	pending    float32
	fraction   float32
	lastDrain  time.Time
	drainUntil time.Time
}

func mapRowListScroll(delta, pixelsPerDP float32, x11 bool) (float32, bool) {
	notchSize := float32(notchWayland)
	if x11 {
		notchSize = notchX11
	}
	notches := float32(math.Round(float64(delta / notchSize)))
	if notches != 0 && float32(math.Abs(float64(delta-notches*notchSize))) <= notchTolerance {
		return notches * float32(tokens.SizeRowListWheelNotch) * pixelsPerDP, true
	}
	return delta * pixelsPerDP, false
}

func (list *lithicList) scrollRange() pointer.ScrollRange {
	var bounds pointer.ScrollRange
	if list.Position.First > 0 || list.Position.Offset > 0 {
		bounds.Min = -scrollRangeLimit
	}
	if list.Position.BeforeEnd {
		bounds.Max = scrollRangeLimit
	}
	return bounds
}

func (list *lithicList) addOffset(delta float32) int {
	if delta < 0 {
		list.Position.BeforeEnd = true
	}
	total := list.fraction + delta
	whole := int(total)
	list.fraction = total - float32(whole)
	list.Position.Offset += whole
	return whole
}

func (list *lithicList) clearPending() {
	list.pending = 0
	list.lastDrain = time.Time{}
	list.drainUntil = time.Time{}
}

func (list *lithicList) queueNotch(gtx layout.Context, pixels float32, duration time.Duration) {
	list.pending += pixels
	if float32(math.Abs(float64(list.pending))) < scrollEpsilon {
		list.clearPending()
		return
	}
	if list.lastDrain.IsZero() {
		list.lastDrain = gtx.Now
	}
	list.drainUntil = gtx.Now.Add(duration)
}

func (list *lithicList) drainNotches(gtx layout.Context, duration time.Duration) int {
	if list.pending == 0 {
		return 0
	}
	if anim.Reduced() || !gtx.Now.Before(list.drainUntil) {
		delta := list.pending
		list.clearPending()
		return list.addOffset(delta)
	}
	if list.lastDrain.IsZero() || !gtx.Now.After(list.lastDrain) {
		list.lastDrain = gtx.Now
		gtx.Execute(op.InvalidateCmd{})
		return 0
	}
	elapsed := gtx.Now.Sub(list.lastDrain)
	tau := float64(duration) / notchDecay
	portion := float32(1 - math.Exp(-float64(elapsed)/tau))
	delta := list.pending * portion
	list.pending -= delta
	list.lastDrain = gtx.Now
	applied := list.addOffset(delta)
	if list.pending != 0 {
		gtx.Execute(op.InvalidateCmd{})
	}
	return applied
}

func (list *lithicList) updateScroll(gtx layout.Context, duration time.Duration) (layout.Position, int) {
	before := list.Position
	filters := pointer.Filter{Target: &list.scrollTag, ScrollY: list.scrollRange()}
	applied := 0
	for {
		event, ok := gtx.Event(filters)
		if !ok {
			break
		}
		scroll, ok := event.(pointer.Event)
		if !ok || scroll.Kind != pointer.Scroll {
			continue
		}
		pixels, notch := mapRowListScroll(scroll.Scroll.Y, gtx.Metric.PxPerDp, rowListOnX11)
		if notch && !anim.Reduced() {
			list.queueNotch(gtx, pixels, duration)
			continue
		}
		applied += list.addOffset(pixels)
	}
	applied += list.drainNotches(gtx, duration)
	return before, applied
}

func sameRowListPosition(a, b layout.Position) bool {
	return a.First == b.First && a.Offset == b.Offset
}

func rowListThumb(viewport, content, first, offset, count, minimum int) (top, height int, ok bool) {
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

func (list *lithicList) Layout(gtx layout.Context, th *theme.Theme, count int, element layout.ListElement) layout.Dimensions {
	list.Axis = layout.Vertical
	before, applied := list.updateScroll(gtx, th.Motion.Wheel)
	dimensions := list.List.Layout(gtx, count, element)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops)
	event.Op(gtx.Ops, &list.scrollTag)
	area.Pop()
	pass.Pop()
	if applied != 0 && sameRowListPosition(before, list.Position) && list.pending != 0 {
		list.clearPending()
		list.fraction = 0
	}
	top, height, ok := rowListThumb(dimensions.Size.Y, list.Position.Length, list.Position.First, list.Position.Offset, count, gtx.Dp(tokens.SizeScrollThumbMin))
	if ok {
		width := max(1, gtx.Dp(tokens.SizeScrollThumb))
		x := dimensions.Size.X - width
		FillCut(gtx.Ops, image.Rect(x, top, x+width, top+height), min(width/2, height/2), th.R.Vein)
	}
	return dimensions
}

// RowList owns Lithic scrolling, selection motion and keyboard traversal.
type RowList struct {
	Preview State

	list          lithicList
	heights       []int
	offsets       []int
	defaultHeight int
	slabY         anim.Tween
	slabTarget    int
	selected      int
	selectedReady bool
	slabVisible   bool
	keyTag        uint8
	changed       int
	changedReady  bool
	activated     int
	activeReady   bool
	focus         anim.Tween
}

func fillRowOffsets(destination, heights []int, gap, fallback int) []int {
	if cap(destination) < len(heights) {
		destination = make([]int, len(heights))
	} else {
		destination = destination[:len(heights)]
	}
	y := 0
	for index, height := range heights {
		destination[index] = y
		if height <= 0 {
			height = fallback
		}
		y += height
		if index+1 < len(heights) {
			y += gap
		}
	}
	return destination
}

func (list *RowList) resize(count, fallback int) {
	if list.defaultHeight != fallback {
		list.heights, list.offsets = nil, nil
		list.defaultHeight = fallback
	}
	if cap(list.heights) < count {
		next := make([]int, count)
		copy(next, list.heights)
		list.heights = next
	} else {
		old := len(list.heights)
		list.heights = list.heights[:count]
		if count > old {
			clear(list.heights[old:])
		}
	}
}

func (list *RowList) handleKeys(gtx layout.Context, count, selected int) int {
	list.changedReady, list.activeReady = false, false
	current := selected
	for {
		event, ok := gtx.Event(
			key.Filter{Focus: &list.keyTag, Name: key.NameUpArrow},
			key.Filter{Focus: &list.keyTag, Name: key.NameDownArrow},
			key.Filter{Focus: &list.keyTag, Name: key.NameHome},
			key.Filter{Focus: &list.keyTag, Name: key.NameEnd},
			key.Filter{Focus: &list.keyTag, Name: key.NameReturn},
			key.Filter{Focus: &list.keyTag, Name: key.NameEnter},
		)
		if !ok {
			break
		}
		pressed, ok := event.(key.Event)
		if !ok || pressed.State != key.Press || count == 0 {
			continue
		}
		switch pressed.Name {
		case key.NameUpArrow:
			current = max(0, current-1)
		case key.NameDownArrow:
			current = min(count-1, max(0, current+1))
		case key.NameHome:
			current = 0
		case key.NameEnd:
			current = count - 1
		case key.NameReturn, key.NameEnter:
			list.activated, list.activeReady = max(0, current), true
		}
		if current != selected {
			list.changed, list.changedReady = current, true
		}
	}
	return current
}

// Focus moves keyboard focus to the list.
func (list *RowList) Focus(gtx layout.Context) {
	gtx.Execute(key.FocusCmd{Tag: &list.keyTag})
}

// Changed reports the most recent keyboard selection change.
func (list *RowList) Changed() (int, bool) { return list.changed, list.changedReady }

// Activated reports the most recent keyboard activation.
func (list *RowList) Activated() (int, bool) { return list.activated, list.activeReady }

// Layout draws a scrolling row list and its gliding selection slab.
func (list *RowList) Layout(gtx layout.Context, th *theme.Theme, count, selected int, row layout.ListElement) layout.Dimensions {
	count = max(0, count)
	selected = list.handleKeys(gtx, count, selected)
	if list.changedReady {
		position := &list.list.Position
		if selected < position.First {
			position.First, position.Offset = selected, 0
		} else if position.Count > 0 && selected >= position.First+position.Count {
			position.First, position.Offset = max(0, selected-position.Count+1), 0
		}
	}
	gap := gtx.Dp(tokens.SizeRowListGap)
	fallback := gtx.Dp(th.RowHeight())
	list.resize(count, fallback)
	list.list.Gap = gap
	list.slabY.Duration, list.slabY.Ease = th.Motion.Select, anim.EaseChisel
	record := op.Record(gtx.Ops)
	dimensions := list.list.Layout(gtx, th, count, func(gtx layout.Context, index int) layout.Dimensions {
		dimensions := row(gtx, index)
		if index >= 0 && index < len(list.heights) {
			list.heights[index] = dimensions.Size.Y
		}
		return dimensions
	})
	content := record.Stop()
	list.offsets = fillRowOffsets(list.offsets, list.heights, gap, fallback)
	visible := false
	targetY, slabHeight := 0, fallback
	if selected >= 0 && selected < len(list.offsets) && list.list.Position.First >= 0 && list.list.Position.First < len(list.offsets) {
		targetY = list.offsets[selected] - list.offsets[list.list.Position.First] - list.list.Position.Offset
		slabHeight = list.heights[selected]
		if slabHeight <= 0 {
			slabHeight = fallback
		}
		visible = targetY < dimensions.Size.Y && targetY+slabHeight > 0
	}
	changed := !list.selectedReady || list.selected != selected
	if visible {
		switch {
		case !list.selectedReady || !list.slabVisible:
			list.slabY = anim.Tween{Duration: th.Motion.Select, Ease: anim.EaseChisel}
			list.slabY.Set(gtx, float32(targetY))
		case changed:
			list.slabY.Set(gtx, float32(targetY))
		case list.slabTarget != targetY:
			list.slabY = anim.Tween{Duration: th.Motion.Select, Ease: anim.EaseChisel}
			list.slabY.Set(gtx, float32(targetY))
		}
		list.slabTarget = targetY
		y := int(list.slabY.Value(gtx) + .5)
		stack := clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops)
		slab := image.Rect(0, y, dimensions.Size.X, y+slabHeight)
		FillCut(gtx.Ops, slab, 0, th.R.Sel)
		PlaceBlade(gtx, th, slab, 0, layout.Vertical)
		stack.Pop()
	}
	list.selected, list.selectedReady, list.slabVisible = selected, true, visible
	content.Add(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops)
	event.Op(gtx.Ops, &list.keyTag)
	area.Pop()
	focused := gtx.Focused(&list.keyTag) || list.Preview == StateFocus
	focus := focusValue(gtx, th, &list.focus, focused)
	FocusRingAlpha(gtx, th, image.Rectangle{Max: dimensions.Size}, 0, focus)
	return dimensions
}
