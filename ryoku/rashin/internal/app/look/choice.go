package look

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	material "golang.org/x/exp/shiny/materialdesign/icons"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

var choiceArrow = kit.Icon(material.NavigationExpandMore)

// Choice is a selection field whose menu lives in window coordinates.
type Choice struct {
	// Label is an optional caption above the selection anchor.
	Label string

	click kit.Click
	menu  kit.Menu
	size  image.Point
	value int
	ready bool
}

func (choice *Choice) Layout(gtx layout.Context, th *Theme, overlays *Overlays, options []string, selected int) layout.Dimensions {
	if selected >= 0 && selected < len(options) && !choice.ready {
		choice.value = selected
	}
	if choice.click.Clicked(gtx) && overlays != nil {
		tag := &choice.click.Clickable
		if overlays.IsOpen(tag) {
			overlays.Close(tag)
		} else {
			items := make([]kit.MenuItem, len(options))
			for index, option := range options {
				items[index] = kit.MenuItem{Label: option}
			}
			content := func(gtx layout.Context) layout.Dimensions {
				dimensions := choice.menu.Layout(gtx, th.Theme, items)
				if index, ok := choice.menu.Clicked(gtx); ok {
					choice.value, choice.ready = index, true
					overlays.Close(tag)
					gtx.Execute(op.InvalidateCmd{})
				}
				return dimensions
			}
			width := unit.Dp(float32(choice.size.X) / gtx.Metric.PxPerDp)
			overlays.Open(tag, overlays.AnchorOf(&choice.click.Clickable, choice.size), Placement{MinWidth: width}, content)
		}
	}
	label := ""
	if selected >= 0 && selected < len(options) {
		label = options[selected]
	}
	if choice.ready && choice.value >= 0 && choice.value < len(options) {
		label = options[choice.value]
	}
	anchor := func(gtx layout.Context) layout.Dimensions {
		dimensions := choice.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			description := label
			if choice.Label != "" {
				description = choice.Label + ": " + label
			}
			semantic.LabelOp(description).Add(gtx.Ops)
			semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
			height := gtx.Dp(tokens.SizePickerHeight)
			width := max(gtx.Constraints.Min.X, gtx.Dp(tokens.SizePickerWidthMin))
			width = min(width, gtx.Constraints.Max.X)
			bounds := image.Rect(0, 0, width, height)
			fill := th.R.Stone3
			if choice.click.Hovered() || choice.click.Pressed() {
				fill = palette.Mix(fill, th.R.Fg, tokens.AlphaHoverPlane)
			}
			kit.FillCut(gtx.Ops, bounds, gtx.Dp(th.Cut.Mid), fill)
			kit.StrokeCut(gtx.Ops, bounds, gtx.Dp(th.Cut.Mid), max(1, gtx.Dp(1)), th.R.Vein)
			gtx.Constraints = layout.Exact(bounds.Max)
			return layout.Inset{Left: unit.Dp(tokens.SizePickerInset), Right: unit.Dp(tokens.SizePickerInset)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, kit.Text(th.Theme, th.T.Body, th.R.Fg, label)),
						layout.Rigid(layout.Spacer{Width: unit.Dp(tokens.SizePickerArrowGap)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							size := gtx.Dp(tokens.SizePickerArrow)
							gtx.Constraints = layout.Exact(image.Pt(size, size))
							return choiceArrow.Layout(gtx, th.R.Fg2)
						}),
					)
				})
			})
		})
		choice.size = dimensions.Size
		return dimensions
	}
	if choice.Label == "" {
		return anchor(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, choice.Label)),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(anchor),
	)
}

func (choice *Choice) Changed() (int, bool) {
	value, ready := choice.value, choice.ready
	choice.ready = false
	return value, ready
}
