package kit

import (
	"image"

	"gioui.org/f32"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget"
	"golang.org/x/image/math/fixed"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// BecomingName fills a machine name from left to right as its job advances.
type BecomingName struct {
	Name     string
	Fraction float64
	Tone     palette.Tone
	Compact  bool
}

// Layout draws the outlined name and clipped solid fill.
func (name BecomingName) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	style := th.T.Name
	if name.Compact {
		style = th.T.Sentence
	}
	lineInk := th.R.Fg
	if name.Name == "" {
		return layout.Dimensions{}
	}
	measure := op.Record(gtx.Ops)
	dimensions := widget.Label{MaxLines: 1, Truncator: "…", LineHeight: style.Line, LineHeightScale: 1}.Layout(
		gtx, th.Shaper, style.Face, style.Size, name.Name, textMaterial(gtx.Ops, th.R.Fg),
	)
	measure.Stop()
	th.Shaper.LayoutString(text.Parameters{
		Font: style.Face, PxPerEm: fixed.I(gtx.Sp(style.Size)), MaxLines: 1, Truncator: "…",
		MaxWidth: gtx.Constraints.Max.X, MinWidth: gtx.Constraints.Min.X, Locale: gtx.Locale,
		LineHeight: fixed.I(gtx.Sp(style.Line)), LineHeightScale: 1,
	}, name.Name)
	var storage [128]text.Glyph
	glyphs := storage[:0]
	for glyph, ok := th.Shaper.NextGlyph(); ok; glyph, ok = th.Shaper.NextGlyph() {
		glyphs = append(glyphs, glyph)
		if glyph.Flags&text.FlagLineBreak != 0 {
			break
		}
	}
	if len(glyphs) == 0 {
		return dimensions
	}
	semantic.LabelOp(name.Name).Add(gtx.Ops)
	path := th.Shaper.Shape(glyphs)
	baseline := f32.Pt(float32(glyphs[0].X)/64, float32(glyphs[0].Y))
	filled := int(float64(dimensions.Size.X)*min(max(name.Fraction, 0), 1) + .5)
	if filled > 0 {
		fill := clip.Rect(image.Rect(0, 0, filled, dimensions.Size.Y)).Push(gtx.Ops)
		move := op.Affine(f32.AffineId().Offset(baseline)).Push(gtx.Ops)
		shape := clip.Outline{Path: path}.Op().Push(gtx.Ops)
		paint.ColorOp{Color: th.R.Fg}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		shape.Pop()
		move.Pop()
		fill.Pop()
	}
	if filled < dimensions.Size.X {
		outlineClip := clip.Rect(image.Rect(filled, 0, dimensions.Size.X, dimensions.Size.Y)).Push(gtx.Ops)
		move := op.Affine(f32.AffineId().Offset(baseline)).Push(gtx.Ops)
		paint.FillShape(gtx.Ops, lineInk, clip.Stroke{Path: path, Width: float32(max(1, gtx.Dp(tokens.SizeHairline)))}.Op())
		move.Pop()
		outlineClip.Pop()
	}
	return dimensions
}
