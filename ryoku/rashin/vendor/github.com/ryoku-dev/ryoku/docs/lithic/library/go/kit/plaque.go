package kit

import (
	"image"
	"image/color"
	"time"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// PlaqueKind selects the plaque's semantic surface treatment.
type PlaqueKind uint8

const (
	PlaqueGold PlaqueKind = iota
	PlaqueLine
	PlaqueQuiet
	PlaqueOn
	PlaqueDanger
	PlaqueDangerLine
)

type plaqueColors struct {
	fill, ink, outline color.NRGBA
	stroke             bool
}

// Plaque is a labelled cut action.
type Plaque struct {
	Click
	Kind      PlaqueKind
	Icon      *widget.Icon
	Label     string
	Small     bool
	Busy      bool
	Preview   State
	press     anim.Tween
	hover     anim.Tween
	focus     anim.Tween
	lastWidth int
}

// Clicked reports one activation unless the plaque is busy.
func (p *Plaque) Clicked(gtx layout.Context) bool {
	if !p.Busy {
		return p.Click.Clicked(gtx)
	}
	for p.Click.Clicked(gtx) {
	}
	return false
}

// Layout draws the plaque and its interaction states.
func (p *Plaque) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	height := unit.Dp(tokens.SizePlaqueHeight)
	side := unit.Dp(tokens.SizePlaqueInset)
	iconSize := unit.Dp(tokens.SizePlaqueIcon)
	gap := unit.Dp(tokens.SizePlaqueGap)
	baseCut := th.Cut.Mid
	if p.Small {
		height = unit.Dp(tokens.SizePlaqueHeightSmall)
		side = unit.Dp(tokens.SizePlaqueInsetSmall)
		iconSize = unit.Dp(tokens.SizePlaqueIconSmall)
		gap = unit.Dp(tokens.SizePlaqueGapSmall)
		baseCut = th.Cut.Small
	}

	original := gtx
	interaction := gtx
	if p.Busy {
		interaction = interaction.Disabled()
	}
	h := gtx.Dp(height)
	finalCut := gtx.Dp(baseCut)
	dims := p.Click.Layout(interaction, func(gtx layout.Context) layout.Dimensions {
		gtx.Source = original.Source
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(p.Label).Add(gtx.Ops)
		semantic.EnabledOp(original.Source.Enabled() && !p.Busy).Add(gtx.Ops)

		pressed := previewTransition(original, &p.press, p.Preview, StatePressed, p.Pressed() && !p.Busy, th.Motion.Press, th.Motion.Press)
		finalCut = gtx.Dp(baseCut) + int(float32(gtx.Dp(unit.Dp(tokens.SizeControlPressCutGrow)))*pressed+.5)
		sink := int(float32(gtx.Dp(unit.Dp(tokens.SizeControlPressSink)))*pressed + .5)
		hover := previewTransition(original, &p.hover, p.Preview, StateHover, p.Hovered() && !p.Busy, th.Motion.HoverIn, th.Motion.HoverOut)
		colors := interactivePlaqueColors(original, th, p.Kind, hover)

		contentGTX := gtx
		contentGTX.Constraints.Min = image.Point{}
		contentGTX.Constraints.Max.Y = h
		record := op.Record(gtx.Ops)
		content := layout.Inset{Left: side, Right: side}.Layout(contentGTX, func(gtx layout.Context) layout.Dimensions {
			var children [3]layout.FlexChild
			count := 0
			if p.Busy {
				children[count] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return Flake(gtx, th, palette.Becoming, float64(tokens.AlphaWorkingBase), iconSize)
				})
				count++
				children[count] = layout.Rigid(layout.Spacer{Width: gap}.Layout)
				count++
			} else if p.Icon != nil {
				children[count] = layout.Rigid(iconLayout(p.Icon, iconSize, colors.ink))
				count++
				children[count] = layout.Rigid(layout.Spacer{Width: gap}.Layout)
				count++
			}
			children[count] = layout.Rigid(rawText(th, th.T.Label, colors.ink, p.Label))
			count++
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children[:count]...)
		})
		call := record.Stop()

		desiredWidth := content.Size.X
		if p.Busy && p.lastWidth > 0 {
			desiredWidth = p.lastWidth
		}
		size := constrainSize(gtx, image.Pt(desiredWidth, h))
		if !p.Busy {
			p.lastWidth = size.X
		}
		bounds := image.Rectangle{Max: size}
		drawPlaqueSurface(gtx, th, bounds, finalCut, p.Kind, hover)
		offset := op.Offset(image.Pt((size.X-content.Size.X)/2, (size.Y-content.Size.Y)/2+sink)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		return layout.Dimensions{Size: size, Baseline: content.Baseline + (size.Y-content.Size.Y)/2 + sink}
	})
	focus := previewTransition(original, &p.focus, p.Preview, StateFocus, original.Focused(&p.Click.Clickable), th.Motion.FocusIn, th.Motion.FocusOut)
	FocusRingAlpha(original, th, image.Rectangle{Max: dims.Size}, finalCut, focus)
	return dims
}

func plaquePalette(th *theme.Theme, kind PlaqueKind) plaqueColors {
	switch kind {
	case PlaqueLine:
		return plaqueColors{ink: th.R.GoldInk, outline: th.R.GoldInk, stroke: true}
	case PlaqueQuiet:
		return plaqueColors{ink: th.R.Fg2, outline: th.R.Vein, stroke: true}
	case PlaqueOn:
		return plaqueColors{fill: th.R.Sel, ink: th.R.OnSel}
	case PlaqueDanger:
		return plaqueColors{fill: th.R.Jasper, ink: palette.InkOn(th.R.Jasper)}
	case PlaqueDangerLine:
		return plaqueColors{ink: th.R.Jasper, outline: th.R.Jasper, stroke: true}
	default:
		return plaqueColors{fill: th.R.Gold, ink: th.R.OnGold}
	}
}

func interactivePlaqueColors(gtx layout.Context, th *theme.Theme, kind PlaqueKind, hover float32) plaqueColors {
	colors := plaquePalette(th, kind)
	if hover > 0 {
		amount := tokens.AlphaHoverFill * min(hover, 1)
		if colors.fill.A == 0 {
			colors.fill = palette.WithAlpha(colors.ink, amount)
		} else {
			colors.fill = palette.Mix(colors.fill, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, amount)
		}
	}
	colors.fill = disabledInk(gtx, colors.fill)
	colors.ink = disabledInk(gtx, colors.ink)
	colors.outline = disabledInk(gtx, colors.outline)
	return colors
}

func drawPlaqueSurface(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, cut int, kind PlaqueKind, hover float32) color.NRGBA {
	colors := interactivePlaqueColors(gtx, th, kind, hover)
	FillCut(gtx.Ops, bounds, cut, colors.fill)
	if colors.stroke {
		StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), colors.outline)
	}
	return colors.ink
}

func previewTransition(gtx layout.Context, tween *anim.Tween, preview, forced State, active bool, in, out time.Duration) float32 {
	if preview == forced {
		return 1
	}
	return transitionValue(gtx, tween, active, in, out)
}

func exactContext(gtx layout.Context, size image.Point) layout.Context {
	size = constrainSize(gtx, size)
	gtx.Constraints = layout.Exact(size)
	return gtx
}

func constrainSize(gtx layout.Context, size image.Point) image.Point {
	size.X = min(max(0, size.X), gtx.Constraints.Max.X)
	size.Y = min(max(0, size.Y), gtx.Constraints.Max.Y)
	return size
}

func iconLayout(icon *widget.Icon, size unit.Dp, ink color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if icon == nil {
			return layout.Dimensions{}
		}
		sz := gtx.Dp(size)
		return icon.Layout(exactContext(gtx, image.Pt(sz, sz)), ink)
	}
}

func lerp32(a, b, t float32) float32 {
	return a + (b-a)*t
}
