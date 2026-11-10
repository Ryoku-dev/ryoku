package look

import (
	"image"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
)

type Tip struct {
	Preview kit.State

	tag         struct{}
	hovered     bool
	shown       bool
	pendingRect bool
	since       time.Time
	local       image.Point
	rect        image.Rectangle
}

func (tip *Tip) Layout(
	gtx layout.Context,
	th *Theme,
	overlays *Overlays,
	text string,
	child layout.Widget,
) layout.Dimensions {
	if overlays != nil {
		overlays.touchTip(&tip.tag)
	}
	if tip.shown && overlays != nil && !overlays.desired(&tip.tag) {
		tip.reset()
	}
	sawPosition := false
	for {
		raw, ok := gtx.Event(pointer.Filter{
			Target: &tip.tag,
			Kinds:  pointer.Enter | pointer.Leave | pointer.Move,
		})
		if !ok {
			break
		}
		e := raw.(pointer.Event)
		switch e.Kind {
		case pointer.Enter:
			tip.hovered = true
			tip.since = gtx.Now
			if overlays != nil && overlays.recentTip(gtx.Now.UnixNano()) {
				tip.since = gtx.Now.Add(-th.Motion.TooltipDelay)
			}
		case pointer.Move:
			tip.local = e.Position.Round()
			tip.pendingRect, sawPosition = true, true
		case pointer.Leave:
			tip.dismiss(overlays)
		}
	}
	dimensions := child(gtx)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops)
	// inside the area, or the words describe whatever pane the control stands in
	semantic.DescriptionOp(text).Add(gtx.Ops)
	event.Op(gtx.Ops, &tip.tag)
	area.Pop()
	pass.Pop()
	if tip.pendingRect && sawPosition && overlays != nil {
		minimum := overlays.Pointer().Sub(tip.local)
		tip.rect = image.Rectangle{Min: minimum, Max: minimum.Add(dimensions.Size)}
		tip.pendingRect = false
	}
	if sawPosition && tip.pendingRect {
		gtx.Execute(op.InvalidateCmd{})
	}
	if tip.Preview == kit.StateHover || tip.Preview == kit.StatePressed || tip.Preview == kit.StateFocus {
		tip.hovered = true
		tip.since = gtx.Now.Add(-th.Motion.TooltipDelay)
		if tip.rect.Empty() {
			tip.rect = image.Rectangle{Max: dimensions.Size}
		}
	}
	if overlays == nil || text == "" || !tip.hovered || tip.rect.Empty() {
		return dimensions
	}
	ready := tip.since.Add(th.Motion.TooltipDelay)
	if gtx.Now.Before(ready) {
		gtx.Execute(op.InvalidateCmd{At: ready})
		return dimensions
	}
	overlays.open(
		&tip.tag,
		tip.rect,
		Placement{Prefer: Below, Align: Start, Passive: true, Gap: 8},
		kit.Text(th.Theme, th.T.Caption, th.R.Fg, text),
		true,
	)
	tip.shown = true
	return dimensions
}

func (tip *Tip) dismiss(overlays *Overlays) {
	tip.hovered = false
	if overlays != nil {
		overlays.Close(&tip.tag)
	}
}

func (tip *Tip) reset() {
	tip.hovered, tip.shown, tip.pendingRect = false, false, false
	tip.rect = image.Rectangle{}
}
