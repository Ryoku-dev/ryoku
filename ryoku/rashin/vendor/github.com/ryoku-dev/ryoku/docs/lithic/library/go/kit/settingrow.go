package kit

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

var iconRestart = Icon(symbols.RestartAlt)

// SettingRow is one setting in a settings slab: a wrapping title and detail
// with the control at the right. Rows stack directly; each draws the vein
// line above itself unless First.
type SettingRow struct {
	Title       string
	Detail      string
	Changed     bool
	Unavailable string
	Highlight   float32
	Activatable bool
	First       bool
	Reset       Click
	Row         Click
	Preview     State

	reset               IconPlaque
	hoverArea           gesture.Hover
	hovered             bool
	hover, press, focus anim.Tween
	resetShow           anim.Tween
}

// Hovered reports whether the pointer is over the row.
func (r *SettingRow) Hovered() bool { return r.hovered }

// Layout draws the row with control in its control slot. Read Reset.Clicked
// for a reset request and Row.Clicked for an activation.
func (r *SettingRow) Layout(gtx layout.Context, th *theme.Theme, control layout.Widget) layout.Dimensions {
	r.hovered = r.hoverArea.Update(gtx.Source)
	width := gtx.Constraints.Max.X
	gap := 0
	if !r.First {
		gap = gtx.Dp(tokens.SizeSettingsRowGap)
	}
	textGap := gtx.Dp(th.Sp.S4)
	resetGap := gtx.Dp(th.Sp.S2)
	resetSize := gtx.Dp(tokens.SizeIconPlaqueSmall)
	reserve := 0
	if r.Changed {
		reserve = resetSize + resetGap
	}

	controlGTX := gtx
	if r.Unavailable != "" {
		controlGTX = controlGTX.Disabled()
	}
	controlGTX.Constraints = layout.Constraints{Max: image.Pt(max(0, min(gtx.Dp(tokens.SizeSettingsControlMax), width-textGap-reserve)), gtx.Constraints.Max.Y)}
	controlRecord := op.Record(gtx.Ops)
	controlDims := layout.Dimensions{}
	if control != nil {
		controlDims = control(controlGTX)
	}
	controlCall := controlRecord.Stop()

	textWidth := max(0, width-controlDims.Size.X-textGap-reserve)
	textRecord := op.Record(gtx.Ops)
	textDims := r.layoutText(gtx, th, textWidth)
	textCall := textRecord.Stop()

	height := max(gtx.Dp(tokens.SizeFactsRowHeight), textDims.Size.Y, controlDims.Size.Y, resetSize)
	box := image.Rect(0, gap, width, gap+height)
	bleed := image.Rect(box.Min.X-gtx.Dp(th.Sp.S2), box.Min.Y-gtx.Dp(th.Sp.S1), box.Max.X+gtx.Dp(th.Sp.S2), box.Max.Y+gtx.Dp(th.Sp.S1))
	cut := gtx.Dp(th.Cut.Small)
	if gap > 0 {
		line := max(1, gtx.Dp(unit.Dp(tokens.SizeHairline)))
		y := gap / 2
		FillCut(gtx.Ops, image.Rect(0, y, width, y+line), 0, th.R.Vein)
	}

	area := clip.Rect(bleed).Push(gtx.Ops)
	r.hoverArea.Add(gtx.Ops)

	hovered := r.hovered || r.Preview == StateHover
	hover := previewTransition(gtx, &r.hover, r.Preview, StateHover, r.Activatable && r.hovered, th.Motion.HoverIn, th.Motion.HoverOut)
	press := previewTransition(gtx, &r.press, r.Preview, StatePressed, r.Activatable && r.Row.Pressed(), th.Motion.Press, th.Motion.Press)
	if !r.Activatable || r.Unavailable != "" {
		hover, press = 0, 0
	}
	plane := max(tokens.AlphaHoverPlane*hover, tokens.AlphaHoverPlane*min(max(r.Highlight, 0), 1))
	plane = max(plane, tokens.AlphaPressRow*press)
	FillCut(gtx.Ops, bleed, cut, palette.WithAlpha(th.R.Fg, plane))

	rowFocused := false
	if r.Activatable {
		rowGTX := gtx
		if r.Unavailable != "" {
			rowGTX = rowGTX.Disabled()
		}
		rowGTX.Constraints = layout.Exact(bleed.Size())
		offset := op.Offset(bleed.Min).Push(gtx.Ops)
		r.Row.Layout(rowGTX, func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(r.Title).Add(gtx.Ops)
			if detail := r.detailText(); detail != "" {
				semantic.DescriptionOp(detail).Add(gtx.Ops)
			}
			semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		})
		offset.Pop()
		rowFocused = gtx.Focused(&r.Row.Clickable)
	}

	offset := op.Offset(image.Pt(0, gap+(height-textDims.Size.Y)/2)).Push(gtx.Ops)
	textCall.Add(gtx.Ops)
	offset.Pop()

	controlX := width - controlDims.Size.X
	if r.Changed {
		focusWithin := rowFocused || gtx.Focused(&r.Reset.Clickable) || r.Preview == StateFocus
		show := previewTransition(gtx, &r.resetShow, r.Preview, StateHover, hovered || focusWithin, th.Motion.HoverIn, th.Motion.HoverOut)
		if focusWithin {
			show = 1
		}
		r.reset.Kind = PlaqueQuiet
		r.reset.Small = true
		r.reset.Icon = iconRestart
		r.reset.Description = "Reset " + r.Title
		offset := op.Offset(image.Pt(controlX-resetGap-resetSize, gap+(height-resetSize)/2)).Push(gtx.Ops)
		fade := paint.PushOpacity(gtx.Ops, show)
		r.reset.layoutWith(exactContext(gtx, image.Pt(resetSize, resetSize)), th, &r.Reset)
		fade.Pop()
		offset.Pop()
	}

	offset = op.Offset(image.Pt(controlX, gap+(height-controlDims.Size.Y)/2)).Push(gtx.Ops)
	controlCall.Add(gtx.Ops)
	offset.Pop()
	area.Pop()

	if r.Activatable {
		focused := rowFocused || r.Preview == StateFocus
		focus := focusValue(gtx, th, &r.focus, focused)
		FocusRingAlpha(gtx, th, bleed, cut, focus)
	}
	return layout.Dimensions{Size: image.Pt(width, gap+height)}
}

func (r *SettingRow) detailText() string {
	if r.Unavailable != "" {
		return r.Unavailable
	}
	return r.Detail
}

func (r *SettingRow) layoutText(gtx layout.Context, th *theme.Theme, width int) layout.Dimensions {
	gtx.Constraints = layout.Constraints{Max: image.Pt(width, gtx.Constraints.Max.Y)}
	titleDims := r.layoutTitle(gtx, th, width)
	detail, ink := r.Detail, th.R.Fg2
	if r.Unavailable != "" {
		detail, ink = r.Unavailable, th.R.Fg3
	}
	if detail == "" {
		return titleDims
	}
	y := titleDims.Size.Y + gtx.Dp(tokens.SizeSettingsDetailGap)
	offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
	detailGTX := gtx
	detailGTX.Constraints.Max.Y = max(0, gtx.Constraints.Max.Y-y)
	detailDims := Label{Style: th.T.Detail, Color: ink}.Layout(detailGTX, th, detail)
	offset.Pop()
	return layout.Dimensions{Size: image.Pt(max(titleDims.Size.X, detailDims.Size.X), y+detailDims.Size.Y)}
}

func (r *SettingRow) layoutTitle(gtx layout.Context, th *theme.Theme, width int) layout.Dimensions {
	if !r.Changed {
		return Label{Style: th.T.Row, Color: th.R.Fg}.Layout(gtx, th, r.Title)
	}
	markRecord := op.Record(gtx.Ops)
	markGTX := gtx
	markGTX.Constraints.Min = image.Point{}
	markDims := Label{Style: th.T.Detail, Color: th.R.Fg3, MaxLines: 1}.Layout(markGTX, th, "Changed")
	markCall := markRecord.Stop()
	space := gtx.Dp(th.Sp.S3)

	titleGTX := gtx
	titleGTX.Constraints.Max.X = max(0, width-markDims.Size.X-space)
	titleDims := Label{Style: th.T.Row, Color: th.R.Fg}.Layout(titleGTX, th, r.Title)

	baseline := titleDims.Size.Y - titleDims.Baseline
	markY := baseline - (markDims.Size.Y - markDims.Baseline)
	offset := op.Offset(image.Pt(titleDims.Size.X+space, markY)).Push(gtx.Ops)
	markCall.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{
		Size:     image.Pt(titleDims.Size.X+space+markDims.Size.X, max(titleDims.Size.Y, markY+markDims.Size.Y)),
		Baseline: titleDims.Baseline,
	}
}
