package kit

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Aside lays out the quiet live summary beside a draft.
func Aside(gtx layout.Context, th *theme.Theme, title string, rows ...layout.Widget) layout.Dimensions {
	width := min(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeDraftAsideMax))
	minimum := min(width, gtx.Dp(tokens.SizeDraftAsideMin))
	gtx.Constraints.Max.X = width
	gtx.Constraints.Min.X = max(minimum, min(gtx.Constraints.Min.X, width))
	return SlabContent(gtx, th, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := []layout.FlexChild{layout.Rigid(Text(th, th.T.Title, th.R.Fg, title))}
		for _, row := range rows {
			if row == nil {
				continue
			}
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
				layout.Rigid(row),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// AsideFact is one labelled literal value.
func AsideFact(th *theme.Theme, label, value string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		labelWidth := min(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeFactsLabelWidth))
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = labelWidth, labelWidth
				return Text(th, th.T.Body, th.R.Fg2, label)(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return Label{Style: th.T.Mono, Color: th.R.Fg, MaxLines: 2}.Layout(gtx, th, value)
			}),
		)
	}
}

// AsideNote is supporting draft copy.
func AsideNote(th *theme.Theme, value string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return Label{Style: th.T.Detail, Color: th.R.Fg2, MaxLines: 4}.Layout(gtx, th, value)
	}
}

// AsidePreview displays literal text on an inner field slab.
func AsidePreview(th *theme.Theme, value string) layout.Widget {
	lines := strings.Split(strings.TrimRight(value, "\n"), "\n")
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return themedSlab(gtx, th, th.R.Field, th.Cut.Small, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(th.Sp.S3).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, len(lines))
				for index, line := range lines {
					line := line
					children[index] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if line == "" {
							return layout.Dimensions{Size: image.Pt(0, gtx.Sp(th.T.MonoSm.Line))}
						}
						return Label{Style: th.T.MonoSm, Color: th.R.Fg, MaxLines: 1}.Layout(gtx, th, line)
					})
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		})
	}
}

// themedSlab is Slab in th's design: the cut plane behind widget.
func themedSlab(gtx layout.Context, th *theme.Theme, ink color.NRGBA, cut unit.Dp, widget layout.Widget) layout.Dimensions {
	record := op.Record(gtx.Ops)
	dimensions := widget(gtx)
	call := record.Stop()
	Fill(gtx, th, image.Rectangle{Max: dimensions.Size}, gtx.Dp(cut), ink)
	call.Add(gtx.Ops)
	return dimensions
}
