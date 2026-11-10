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

// Tip adds delayed pointer detail without constraining it to the caller's slot.
type Tip struct {
	tag     struct{}
	hovered bool
	since   time.Time
}

func (tip *Tip) Layout(gtx layout.Context, th *Theme, overlays *Overlays, text string, body layout.Widget) layout.Dimensions {
	for {
		raw, ok := gtx.Event(pointer.Filter{Target: &tip.tag, Kinds: pointer.Enter | pointer.Leave | pointer.Move})
		if !ok {
			break
		}
		e := raw.(pointer.Event)
		switch e.Kind {
		case pointer.Enter:
			tip.hovered = true
			tip.since = gtx.Now
		case pointer.Leave:
			tip.hovered = false
			if overlays != nil {
				overlays.Close(&tip.tag)
			}
		}
	}
	dimensions := body(gtx)
	semantic.DescriptionOp(text).Add(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops)
	event.Op(gtx.Ops, &tip.tag)
	area.Pop()
	pass.Pop()
	if overlays == nil || text == "" || !tip.hovered {
		return dimensions
	}
	ready := tip.since.Add(th.Motion.TooltipDelay)
	if gtx.Now.Before(ready) {
		gtx.Execute(op.InvalidateCmd{At: ready})
		return dimensions
	}
	anchor := image.Rectangle{Min: overlays.Pointer(), Max: overlays.Pointer()}
	overlays.Open(&tip.tag, anchor, Placement{Passive: true, Gap: 8}, kit.Text(th.Theme, th.T.Caption, th.R.Fg, text))
	return dimensions
}
