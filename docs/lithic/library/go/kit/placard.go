package kit

import (
	"image"
	"time"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

type placardEntry struct {
	id      int
	text    string
	action  string
	expires time.Time
	button  Click
	enter   anim.Tween
	hover   anim.Tween
	started bool
	pending bool
}

// Placard owns a bottom-edge stack of transient notices.
type Placard struct {
	Preview State

	next    int
	entries []*placardEntry
}

// Push adds a notice and returns its action identity.
func (p *Placard) Push(now time.Time, text, action string) int {
	p.next++
	p.entries = append(p.entries, &placardEntry{
		id: p.next, text: text, action: action, expires: now.Add(tokens.MotionPlacardLife),
	})
	if len(p.entries) > tokens.SizePlacardStackCount {
		p.entries = p.entries[len(p.entries)-tokens.SizePlacardStackCount:]
	}
	return p.next
}

// Dismiss removes a notice by identity.
func (p *Placard) Dismiss(id int) {
	for index, entry := range p.entries {
		if entry.id == id {
			p.entries = append(p.entries[:index], p.entries[index+1:]...)
			return
		}
	}
}

func prunePlacards(entries []*placardEntry, now time.Time) []*placardEntry {
	kept := entries[:0]
	for _, entry := range entries {
		if now.Before(entry.expires) {
			kept = append(kept, entry)
		}
	}
	return kept
}

// Layout draws up to three deferred placards and their draining life blades.
func (p *Placard) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	p.entries = prunePlacards(p.entries, gtx.Now)
	window := gtx.Constraints.Max
	if len(p.entries) == 0 {
		return layout.Dimensions{Size: window}
	}
	earliest := p.entries[0].expires
	for _, entry := range p.entries[1:] {
		if entry.expires.Before(earliest) {
			earliest = entry.expires
		}
	}
	gtx.Execute(op.InvalidateCmd{At: earliest})
	if !anim.Reduced() {
		gtx.Execute(op.InvalidateCmd{})
	}

	placardContext := overlayContentContext(gtx)
	placardContext.Constraints.Min = image.Point{}
	placardContext.Constraints.Max.X = min(window.X, gtx.Dp(tokens.SizePlacardWidthMax))
	placardContext.Constraints.Max.Y = window.Y
	record := op.Record(gtx.Ops)
	children := make([]layout.FlexChild, 0, len(p.entries)*2-1)
	for index, entry := range p.entries {
		entry := entry
		if entry.action != "" && entry.button.Clicked(gtx) {
			entry.pending = true
		}
		if !entry.started {
			entry.enter = anim.Tween{Duration: tokens.MotionPlacardIn, Ease: anim.EaseChisel}
			entry.enter.Set(gtx, 0)
			entry.enter.Set(gtx, 1)
			entry.started = true
		}
		progress := entry.enter.Value(gtx)
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(tokens.SizePlacardStackGap)}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			height := gtx.Dp(tokens.SizePlacardHeight)
			width := gtx.Constraints.Max.X
			gtx.Constraints.Min = image.Pt(width, height)
			gtx.Constraints.Max = image.Pt(width, height)
			opacity := paint.PushOpacity(gtx.Ops, progress)
			rise := int(float32(gtx.Dp(tokens.SizePlacardRise)) * (1 - progress))
			offset := op.Offset(image.Pt(0, rise)).Push(gtx.Ops)
			bounds := image.Rect(0, 0, width, height)
			FillCut(gtx.Ops, bounds, gtx.Dp(tokens.CutPlacard), th.R.Sel)
			StrokeCut(gtx.Ops, bounds, gtx.Dp(tokens.CutPlacard), max(1, gtx.Dp(tokens.SizeHairline)), th.R.Vein)
			layout.Inset{Left: unit.Dp(tokens.SizePlacardInset), Right: unit.Dp(tokens.SizePlacardTrailingInset)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return centerY(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, rawText(th, th.T.Body, th.R.OnSel, entry.text)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if entry.action == "" {
								return layout.Dimensions{}
							}
							return layout.Inset{Left: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return entry.button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									semantic.Button.Add(gtx.Ops)
									semantic.LabelOp(entry.action).Add(gtx.Ops)
									semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
									actionHeight := gtx.Dp(tokens.SizePlacardActionHeight)
									labelContext := gtx
									labelContext.Constraints.Min = image.Point{}
									labelContext.Constraints.Max.X = max(0, labelContext.Constraints.Max.X-2*gtx.Dp(tokens.SizePlacardActionInset))
									labelContext.Constraints.Max.Y = actionHeight
									labelRecord := op.Record(gtx.Ops)
									labelDimensions := rawText(th, th.T.Label, th.R.OnSel, entry.action)(labelContext)
									labelCall := labelRecord.Stop()
									size := gtx.Constraints.Constrain(image.Pt(labelDimensions.Size.X+2*gtx.Dp(tokens.SizePlacardActionInset), actionHeight))
									actionBounds := image.Rectangle{Max: size}
									hovered := entry.button.Hovered() || p.Preview == StateHover || p.Preview == StatePressed
									hover := hoverValue(gtx, th, &entry.hover, hovered)
									if hover > 0 {
										FillCut(gtx.Ops, actionBounds, gtx.Dp(th.Cut.Small), palette.WithAlpha(th.R.OnSel, tokens.AlphaHoverInk*hover))
									}
									StrokeCut(gtx.Ops, actionBounds, gtx.Dp(th.Cut.Small), max(1, gtx.Dp(tokens.SizeHairline)), th.R.OnSel)
									labelOffset := op.Offset(image.Pt((size.X-labelDimensions.Size.X)/2, (size.Y-labelDimensions.Size.Y)/2)).Push(gtx.Ops)
									labelCall.Add(gtx.Ops)
									labelOffset.Pop()
									return layout.Dimensions{Size: size}
								})
							})
						}),
					)
				})
			})
			remaining := float64(entry.expires.Sub(gtx.Now)) / float64(tokens.MotionPlacardLife)
			remaining = max(0, min(1, remaining))
			bladeWidth := int(float64(width) * remaining)
			if bladeWidth > 0 {
				Blade(gtx.Ops, image.Rect(0, height-max(1, gtx.Dp(tokens.SizePlacardBlade)), bladeWidth, height), th.R.OnSel)
			}
			offset.Pop()
			opacity.Pop()
			return layout.Dimensions{Size: bounds.Max}
		}))
	}
	dimensions := layout.Flex{Axis: layout.Vertical}.Layout(placardContext, children...)
	call := record.Stop()

	overlay := op.Record(gtx.Ops)
	position := image.Pt((window.X-dimensions.Size.X)/2, max(0, window.Y-dimensions.Size.Y-gtx.Dp(tokens.SizePlacardBottom)))
	offset := op.Offset(position).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	presentOverlay(gtx, overlay.Stop(), p.Preview != StateRest, window)
	return layout.Dimensions{Size: window}
}

// Action returns and removes one activated notice action.
func (p *Placard) Action(gtx layout.Context) (int, bool) {
	for index, entry := range p.entries {
		clicked := entry.pending
		entry.pending = false
		if !clicked && entry.action != "" {
			clicked = entry.button.Clicked(gtx)
		}
		if clicked {
			id := entry.id
			p.entries = append(p.entries[:index], p.entries[index+1:]...)
			return id, true
		}
	}
	return 0, false
}
