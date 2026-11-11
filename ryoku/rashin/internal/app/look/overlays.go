package look

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
)

type Side uint8

const (
	Below Side = iota
	Above
)

type Align uint8

const (
	Start Align = iota
	End
)

type Placement struct {
	Prefer   Side
	Align    Align
	Gap      unit.Dp
	Width    unit.Dp
	MinWidth unit.Dp
	Passive  bool
	Return   any
	// what holds the keyboard while it is open: nil is the overlay layer, a list with a field names the field
	Focus any
	// a slab and hairline around content that brings none, a menu frames itself
	Frame bool
}

type overlayEntry struct {
	tag       any
	anchor    image.Rectangle
	placement Placement
	content   layout.Widget
	rect      image.Rectangle
	desired   bool
	tip       bool
	tipFrame  uint64
	motion    anim.Tween
	motionSet bool
}

// Overlays records nested controls and replays their popovers in full-window
// coordinates after the frame.
type Overlays struct {
	Preview kit.State

	window, pointer, press  image.Point
	tracker, outside, focus struct{}
	items                   []*overlayEntry
	marks                   map[any]image.Rectangle
	tipFrame                uint64
	lastTipClosed           int64
}

func (overlays *Overlays) Window() image.Point  { return overlays.window }
func (overlays *Overlays) Pointer() image.Point { return overlays.pointer }
func (overlays *Overlays) Press() image.Point   { return overlays.press }

func (overlays *Overlays) Mark(tag any, rectangle image.Rectangle) {
	if overlays.marks == nil {
		overlays.marks = make(map[any]image.Rectangle)
	}
	overlays.marks[tag] = rectangle
}

func (overlays *Overlays) Rect(tag any) (image.Rectangle, bool) {
	rectangle, ok := overlays.marks[tag]
	return rectangle, ok
}

func (overlays *Overlays) AnchorOf(clickable *widget.Clickable, size image.Point) image.Rectangle {
	history := clickable.History()
	if len(history) > 0 {
		position := history[len(history)-1].Position
		minimum := overlays.press.Sub(image.Pt(int(position.X), int(position.Y)))
		return image.Rectangle{Min: minimum, Max: minimum.Add(size)}
	}
	if rectangle, ok := overlays.Rect(clickable); ok {
		return rectangle
	}
	return image.Rectangle{}
}

func (overlays *Overlays) Open(tag any, anchor image.Rectangle, placement Placement, content layout.Widget) {
	overlays.open(tag, anchor, placement, content, false)
}

func (overlays *Overlays) open(
	tag any,
	anchor image.Rectangle,
	placement Placement,
	content layout.Widget,
	tip bool,
) {
	if placement.Gap == 0 {
		placement.Gap = 8
	}
	if tip {
		for _, entry := range overlays.items {
			if entry.tip && entry.tag != tag {
				entry.desired = false
			}
		}
	}
	for index, entry := range overlays.items {
		if entry.tag != tag {
			continue
		}
		entry.anchor, entry.placement, entry.content = anchor, placement, content
		entry.desired, entry.tip = true, tip
		if tip {
			entry.tipFrame = overlays.tipFrame
		}
		if index != len(overlays.items)-1 {
			copy(overlays.items[index:], overlays.items[index+1:])
			overlays.items[len(overlays.items)-1] = entry
		}
		return
	}
	overlays.items = append(overlays.items, &overlayEntry{
		tag: tag, anchor: anchor, placement: placement, content: content,
		desired: true, tip: tip, tipFrame: overlays.tipFrame,
	})
}

func (overlays *Overlays) OpenAt(tag any, at image.Point, content layout.Widget) {
	overlays.Open(tag, image.Rectangle{Min: at, Max: at}, Placement{}, content)
}

func (overlays *Overlays) Close(tag any) bool {
	entry := overlays.entry(tag)
	if entry == nil {
		return false
	}
	entry.desired = false
	return true
}

func (overlays *Overlays) DismissTop() bool {
	if tag, ok := overlays.Top(); ok {
		return overlays.Close(tag)
	}
	return false
}

func (overlays *Overlays) IsOpen(tag any) bool { return overlays.entry(tag) != nil }

func (overlays *Overlays) Top() (any, bool) {
	for index := len(overlays.items) - 1; index >= 0; index-- {
		entry := overlays.items[index]
		if entry.desired && !entry.tip && !entry.placement.Passive {
			return entry.tag, true
		}
	}
	return nil, false
}

func (overlays *Overlays) Track(gtx layout.Context) {
	for {
		raw, ok := gtx.Event(pointer.Filter{
			Target: &overlays.tracker,
			Kinds:  pointer.Press | pointer.Move | pointer.Drag,
		})
		if !ok {
			break
		}
		e := raw.(pointer.Event)
		overlays.pointer = e.Position.Round()
		if e.Kind == pointer.Press {
			overlays.press = e.Position.Round()
		}
	}
	// dialogs draw deferred, after everything else; the tracker defers too so
	// it lies above them and a press in a dialog still places that dialog's menus
	layer := op.Record(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: overlays.window}).Push(gtx.Ops)
	event.Op(gtx.Ops, &overlays.tracker)
	area.Pop()
	pass.Pop()
	op.Defer(gtx.Ops, layer.Stop())
}

func (overlays *Overlays) Layout(gtx layout.Context, th *Theme) {
	overlays.window = gtx.Constraints.Max
	for _, entry := range overlays.items {
		if entry.tip && entry.desired && entry.tipFrame != overlays.tipFrame {
			entry.desired = false
		}
	}
	if tag, ok := overlays.Top(); ok {
		entry := overlays.entry(tag)
		for {
			raw, found := gtx.Event(pointer.Filter{Target: &overlays.outside, Kinds: pointer.Press})
			if !found {
				break
			}
			e := raw.(pointer.Event)
			if entry == nil || !e.Position.Round().In(entry.rect) {
				overlays.Close(tag)
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
	overlays.Track(gtx)
	top, hasTop := overlays.Top()
	if hasTop {
		event.Op(gtx.Ops, &overlays.focus)
		var holder any = &overlays.focus
		if entry := overlays.entry(top); entry != nil && entry.placement.Focus != nil {
			holder = entry.placement.Focus
		}
		if !gtx.Focused(holder) {
			gtx.Execute(key.FocusCmd{Tag: holder})
		}
	}

	kept := overlays.items[:0]
	for _, entry := range overlays.items {
		entry.motion.Duration = th.Motion.MenuIn
		if entry.tip && entry.desired {
			entry.motion.Duration = th.Motion.TooltipIn
		}
		target := float32(1)
		if !entry.desired {
			entry.motion.Duration = th.Motion.MenuOut
			target = 0
		}
		if !entry.motionSet {
			entry.motion.Ease = anim.EaseChisel
			entry.motion.Set(gtx, 0)
			entry.motionSet = true
		}
		entry.motion.Set(gtx, target)
		progress := entry.motion.Value(gtx)
		if overlays.Preview != kit.StateRest {
			progress = 1
		}
		if !entry.desired && progress <= 0 {
			if entry.tip {
				overlays.lastTipClosed = gtx.Now.UnixNano()
			}
			// the last menu to leave hands the keyboard back; one that closes under another leaves it there
			if !hasTop {
				if target := entry.focusReturn(); target != nil {
					gtx.Execute(key.FocusCmd{Tag: target})
				}
			}
			continue
		}
		kept = append(kept, entry)
		layer := op.Record(gtx.Ops)
		if hasTop && entry.tag == top {
			area := clip.Rect(image.Rectangle{Max: overlays.window}).Push(gtx.Ops)
			event.Op(gtx.Ops, &overlays.outside)
			area.Pop()
		}
		overlays.layoutEntry(gtx, th, entry, progress)
		op.Defer(gtx.Ops, layer.Stop())
	}
	overlays.items = kept
	overlays.tipFrame++
}

func (entry *overlayEntry) focusReturn() any {
	if entry.tip || entry.placement.Passive {
		return nil
	}
	if entry.placement.Return != nil {
		return entry.placement.Return
	}
	return entry.tag
}

func (overlays *Overlays) entry(tag any) *overlayEntry {
	for _, entry := range overlays.items {
		if entry.tag == tag {
			return entry
		}
	}
	return nil
}

func (overlays *Overlays) desired(tag any) bool {
	entry := overlays.entry(tag)
	return entry != nil && entry.desired
}

func (overlays *Overlays) touchTip(tag any) {
	if entry := overlays.entry(tag); entry != nil && entry.tip {
		entry.tipFrame = overlays.tipFrame
	}
}

func (overlays *Overlays) recentTip(now int64) bool {
	const adjacent = int64(300_000_000)
	return overlays.lastTipClosed != 0 && now-overlays.lastTipClosed <= adjacent
}

func (overlays *Overlays) layoutEntry(
	gtx layout.Context,
	th *Theme,
	entry *overlayEntry,
	progress float32,
) {
	measure := gtx
	measure.Constraints.Min = image.Point{}
	measure.Constraints.Max = image.Pt(max(0, overlays.window.X-32), max(0, overlays.window.Y-32))
	framed := entry.tip || entry.placement.Frame
	inset := layout.Inset{}
	switch {
	case entry.tip:
		measure.Constraints.Max.X = min(measure.Constraints.Max.X, gtx.Dp(320))
		inset = layout.Inset{Top: 6, Right: 8, Bottom: 6, Left: 8}
	case framed:
		inset = layout.UniformInset(8)
	}
	if entry.placement.Width > 0 {
		width := min(measure.Constraints.Max.X, gtx.Dp(entry.placement.Width))
		measure.Constraints.Min.X, measure.Constraints.Max.X = width, width
	} else if entry.placement.MinWidth > 0 {
		measure.Constraints.Min.X = min(measure.Constraints.Max.X, gtx.Dp(entry.placement.MinWidth))
	}
	record := op.Record(gtx.Ops)
	dimensions := inset.Layout(measure, entry.content)
	call := record.Stop()
	position, actual := placeOverlay(
		overlays.window,
		entry.anchor,
		dimensions.Size,
		gtx.Dp(entry.placement.Gap),
		8,
		entry.placement.Prefer,
		entry.placement.Align,
	)
	travel := float32(gtx.Dp(6))
	if entry.tip {
		travel = float32(gtx.Dp(8))
	} else if !entry.desired {
		travel = float32(gtx.Dp(4))
	}
	direction := float32(-1)
	if actual == Above {
		direction = 1
	}
	position.Y += int(direction * travel * (1 - progress))
	offset := op.Offset(position).Push(gtx.Ops)
	opacity := paint.PushOpacity(gtx.Ops, progress)
	bounds := image.Rectangle{Max: dimensions.Size}
	entry.rect = image.Rectangle{Min: position, Max: position.Add(dimensions.Size)}
	if framed {
		cut := th.Cut.Menu
		if entry.tip {
			cut = th.Cut.Tooltip
		}
		kit.FillCut(gtx.Ops, bounds, gtx.Dp(cut), th.R.Stone4)
		kit.StrokeCut(gtx.Ops, bounds, gtx.Dp(cut), max(1, gtx.Dp(1)), th.R.Vein)
	}
	call.Add(gtx.Ops)
	opacity.Pop()
	offset.Pop()
}

func placeOverlay(
	window image.Point,
	anchor image.Rectangle,
	content image.Point,
	gap, margin int,
	prefer Side,
	align Align,
) (image.Point, Side) {
	x := anchor.Min.X
	if align == End {
		x = anchor.Max.X - content.X
	}
	x = min(max(x, margin), max(margin, window.X-margin-content.X))
	below := anchor.Max.Y + gap
	above := anchor.Min.Y - gap - content.Y
	fitsBelow := below+content.Y <= window.Y-margin
	fitsAbove := above >= margin
	actual := prefer
	if prefer == Below && !fitsBelow && fitsAbove {
		actual = Above
	} else if prefer == Above && !fitsAbove && fitsBelow {
		actual = Below
	}
	y := below
	if actual == Above {
		y = above
	}
	y = min(max(y, margin), max(margin, window.Y-margin-content.Y))
	return image.Pt(x, y), actual
}

func place(
	window image.Point,
	anchor image.Rectangle,
	content image.Point,
	gap int,
	prefer Side,
	align Align,
) image.Point {
	position, _ := placeOverlay(window, anchor, content, gap, 8, prefer, align)
	return position
}

func AnyOpen(dialogs ...*kit.Dialog) bool {
	for _, dialog := range dialogs {
		if dialog.Open {
			return true
		}
	}
	return false
}
