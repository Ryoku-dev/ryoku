package look

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

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
}

type entry struct {
	tag       any
	anchor    image.Rectangle
	placement Placement
	content   layout.Widget
	rect      image.Rectangle
}

// Overlays records nested controls and replays their popovers in full-window
// coordinates after the frame.
type Overlays struct {
	window, pointer, press image.Point
	tracker, outside       struct{}
	items                  []*entry
	marks                  map[any]image.Rectangle
}

func (o *Overlays) Window() image.Point  { return o.window }
func (o *Overlays) Pointer() image.Point { return o.pointer }
func (o *Overlays) Press() image.Point   { return o.press }

func (o *Overlays) Mark(tag any, rectangle image.Rectangle) {
	if o.marks == nil {
		o.marks = make(map[any]image.Rectangle)
	}
	o.marks[tag] = rectangle
}

func (o *Overlays) Rect(tag any) (image.Rectangle, bool) {
	rectangle, ok := o.marks[tag]
	return rectangle, ok
}

func (o *Overlays) AnchorOf(clickable *widget.Clickable, size image.Point) image.Rectangle {
	history := clickable.History()
	if len(history) > 0 {
		position := history[len(history)-1].Position
		minimum := o.press.Sub(image.Pt(int(position.X), int(position.Y)))
		return image.Rectangle{Min: minimum, Max: minimum.Add(size)}
	}
	if rectangle, ok := o.Rect(clickable); ok {
		return rectangle
	}
	return image.Rectangle{}
}

func (o *Overlays) Open(tag any, anchor image.Rectangle, placement Placement, content layout.Widget) {
	if placement.Gap == 0 {
		placement.Gap = 8
	}
	for index, item := range o.items {
		if item.tag == tag {
			item.anchor, item.placement, item.content = anchor, placement, content
			if index != len(o.items)-1 {
				copy(o.items[index:], o.items[index+1:])
				o.items[len(o.items)-1] = item
			}
			return
		}
	}
	o.items = append(o.items, &entry{tag: tag, anchor: anchor, placement: placement, content: content})
}

func (o *Overlays) Reanchor(tag any, anchor image.Rectangle) {
	for _, item := range o.items {
		if item.tag == tag {
			item.anchor = anchor
			return
		}
	}
}

func (o *Overlays) OpenAt(tag any, at image.Point, content layout.Widget) {
	o.Open(tag, image.Rectangle{Min: at, Max: at}, Placement{}, content)
}

func (o *Overlays) Close(tag any) bool {
	for index := len(o.items) - 1; index >= 0; index-- {
		if o.items[index].tag == tag {
			o.items = append(o.items[:index], o.items[index+1:]...)
			return true
		}
	}
	return false
}

func (o *Overlays) DismissTop() bool {
	for index := len(o.items) - 1; index >= 0; index-- {
		if !o.items[index].placement.Passive {
			o.items = append(o.items[:index], o.items[index+1:]...)
			return true
		}
	}
	return false
}

func (o *Overlays) IsOpen(tag any) bool {
	for _, item := range o.items {
		if item.tag == tag {
			return true
		}
	}
	return false
}

func (o *Overlays) Top() (any, bool) {
	for index := len(o.items) - 1; index >= 0; index-- {
		if !o.items[index].placement.Passive {
			return o.items[index].tag, true
		}
	}
	return nil, false
}

func (o *Overlays) Track(gtx layout.Context) {
	for {
		raw, ok := gtx.Event(pointer.Filter{Target: &o.tracker, Kinds: pointer.Press | pointer.Move | pointer.Drag})
		if !ok {
			break
		}
		e := raw.(pointer.Event)
		o.pointer = e.Position.Round()
		if e.Kind == pointer.Press {
			o.press = e.Position.Round()
		}
	}
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: o.window}).Push(gtx.Ops)
	event.Op(gtx.Ops, &o.tracker)
	area.Pop()
	pass.Pop()
}

// Layout draws every open overlay after the window body. The outside catcher
// is recorded behind overlay content so controls remain interactive.
func (o *Overlays) Layout(gtx layout.Context, th *Theme) {
	o.window = gtx.Constraints.Max
	o.Track(gtx)
	if tag, ok := o.Top(); ok {
		for {
			raw, found := gtx.Event(pointer.Filter{Target: &o.outside, Kinds: pointer.Press})
			if !found {
				break
			}
			e := raw.(pointer.Event)
			item := o.find(tag)
			if item == nil || !e.Position.Round().In(item.rect) {
				o.Close(tag)
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}

	for _, item := range o.items {
		layer := op.Record(gtx.Ops)
		if tag, ok := o.Top(); ok && tag == item.tag {
			area := clip.Rect(image.Rectangle{Max: o.window}).Push(gtx.Ops)
			event.Op(gtx.Ops, &o.outside)
			area.Pop()
		}
		o.layoutEntry(gtx, th, item)
		op.Defer(gtx.Ops, layer.Stop())
	}
}

func (o *Overlays) HandleEscape(gtx layout.Context, focus any) bool {
	for {
		raw, ok := gtx.Event(key.Filter{Focus: focus, Name: key.NameEscape})
		if !ok {
			return false
		}
		if raw.(key.Event).State == key.Press {
			return o.DismissTop()
		}
	}
}

func (o *Overlays) find(tag any) *entry {
	for _, item := range o.items {
		if item.tag == tag {
			return item
		}
	}
	return nil
}

func (o *Overlays) layoutEntry(gtx layout.Context, th *Theme, item *entry) {
	measure := gtx
	measure.Constraints.Min = image.Point{}
	measure.Constraints.Max = image.Pt(max(0, o.window.X-32), max(0, o.window.Y-32))
	if item.placement.Width > 0 {
		width := min(measure.Constraints.Max.X, gtx.Dp(item.placement.Width))
		measure.Constraints.Min.X, measure.Constraints.Max.X = width, width
	} else if item.placement.MinWidth > 0 {
		measure.Constraints.Min.X = min(measure.Constraints.Max.X, gtx.Dp(item.placement.MinWidth))
	}
	record := op.Record(gtx.Ops)
	dimensions := layout.UniformInset(8).Layout(measure, item.content)
	call := record.Stop()
	position := place(o.window, item.anchor, dimensions.Size, gtx.Dp(item.placement.Gap), item.placement.Prefer, item.placement.Align)
	item.rect = image.Rectangle{Min: position, Max: position.Add(dimensions.Size)}
	offset := op.Offset(position).Push(gtx.Ops)
	kit.FillCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Menu), th.R.Stone4)
	kit.StrokeCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Menu), max(1, gtx.Dp(1)), th.R.Vein)
	call.Add(gtx.Ops)
	offset.Pop()
}

func place(window image.Point, anchor image.Rectangle, size image.Point, gap int, prefer Side, align Align) image.Point {
	x := anchor.Min.X
	if align == End {
		x = anchor.Max.X - size.X
	}
	y := anchor.Max.Y + gap
	if prefer == Above {
		y = anchor.Min.Y - gap - size.Y
	}
	if y+size.Y > window.Y-8 {
		y = anchor.Min.Y - gap - size.Y
	}
	if y < 8 {
		y = min(window.Y-size.Y-8, anchor.Max.Y+gap)
	}
	x = max(8, min(x, window.X-size.X-8))
	y = max(8, y)
	return image.Pt(x, y)
}
