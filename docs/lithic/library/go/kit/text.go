package kit

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

// Label lays out text with a Lithic text style.
type Label struct {
	Style    theme.TextStyle
	Color    color.NRGBA
	MaxLines int
	Align    text.Alignment
}

// Layout renders value with the theme shaper.
func (label Label) Layout(gtx layout.Context, th *theme.Theme, value string) layout.Dimensions {
	truncator := ""
	if label.MaxLines > 0 {
		truncator = "…"
	}
	widgetLabel := widget.Label{
		Alignment: label.Align, MaxLines: label.MaxLines, Truncator: truncator,
		LineHeight: label.Style.Line, LineHeightScale: 1,
	}
	return widgetLabel.Layout(gtx, th.Shaper, label.Style.Face, label.Style.Size, value, textMaterial(gtx.Ops, disabledInk(gtx, label.Color)))
}

// Text returns a one-line Lithic text widget that respects disabled state.
func Text(th *theme.Theme, style theme.TextStyle, ink color.NRGBA, value string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return Label{Style: style, Color: ink, MaxLines: 1}.Layout(gtx, th, value)
	}
}

// RawText returns a one-line text widget without disabled-state tinting.
func RawText(th *theme.Theme, style theme.TextStyle, ink color.NRGBA, value string) layout.Widget {
	return rawText(th, style, ink, value)
}

func rawText(th *theme.Theme, style theme.TextStyle, ink color.NRGBA, value string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		child := gtx
		child.Constraints.Min.Y = 0
		record := op.Record(gtx.Ops)
		label := widget.Label{MaxLines: 1, Truncator: "…", LineHeight: style.Line, LineHeightScale: 1}
		dimensions := label.Layout(child, th.Shaper, style.Face, style.Size, value, textMaterial(gtx.Ops, ink))
		call := record.Stop()
		lineHeight := max(dimensions.Size.Y, gtx.Sp(style.Line))
		size := gtx.Constraints.Constrain(image.Pt(dimensions.Size.X, lineHeight))
		y := max(0, (size.Y-dimensions.Size.Y)/2)
		offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		return layout.Dimensions{Size: size, Baseline: dimensions.Baseline + y}
	}
}

func textMaterial(ops *op.Ops, ink color.NRGBA) op.CallOp {
	record := op.Record(ops)
	paint.ColorOp{Color: ink}.Add(ops)
	return record.Stop()
}
