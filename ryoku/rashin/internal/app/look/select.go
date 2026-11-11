package look

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

type SelectForm uint8

const (
	SelectField SelectForm = iota
	SelectQuiet
)

// detail is a second line under the label in the opened list; a disabled option
// is shown at alpha.disabled, can't be picked and reads as unavailable
type SelectOption struct {
	Label, Detail string
	Disabled      bool
}

// the menu opens in window coordinates so it works inside clipped and scrolling parents
type Select struct {
	Label   string
	Form    SelectForm
	Icon    *widget.Icon
	Preview kit.State
	// takes the keyboard when the menu closes: nil is the select itself, one in a composer hands it to the text
	Return any
	// a quiet select whose value is itself a state worth seeing, such as one that stops asking
	Tone palette.Tone

	click   kit.Click
	menu    kit.Menu
	picker  picker
	hover   anim.Tween
	focus   anim.Tween
	size    image.Point
	value   int
	changed bool
}

func (selection *Select) Changed() (int, bool) {
	changed := selection.changed
	selection.changed = false
	return selection.value, changed
}

func (selection *Select) Layout(
	gtx layout.Context,
	th *Theme,
	overlays *Overlays,
	options []string,
	selected int,
) layout.Dimensions {
	described := make([]SelectOption, len(options))
	for index, option := range options {
		described[index].Label = option
	}
	return selection.LayoutOptions(gtx, th, overlays, described, selected)
}

func (selection *Select) LayoutOptions(
	gtx layout.Context,
	th *Theme,
	overlays *Overlays,
	options []SelectOption,
	selected int,
) layout.Dimensions {
	tag := &selection.click.Clickable
	if selection.click.Clicked(gtx) && overlays != nil {
		if overlays.IsOpen(tag) {
			overlays.Close(tag)
		} else {
			selection.open(gtx, th, overlays, options, selected)
		}
	}
	label := ""
	if selected >= 0 && selected < len(options) {
		label = options[selected].Label
	}
	open := overlays != nil && overlays.IsOpen(tag)
	anchor := func(gtx layout.Context) layout.Dimensions {
		original := gtx
		focused := original.Focused(tag) || selection.Preview == kit.StateFocus
		focus := transition(original, &selection.focus, focused, th.Motion.FocusIn, th.Motion.FocusOut)
		dimensions := selection.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			description := label
			if selection.Label != "" {
				description = selection.Label + ": " + label
			}
			semantic.LabelOp(description).Add(gtx.Ops)
			semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
			if selection.Form == SelectQuiet {
				return selection.layoutQuiet(gtx, th, label, open)
			}
			return selection.layoutField(gtx, th, label, open, focus)
		})
		selection.size = dimensions.Size
		kit.FocusRingAlpha(original, th.Theme, image.Rectangle{Max: dimensions.Size}, gtx.Dp(selection.cut(th)), focus)
		return dimensions
	}
	if selection.Label == "" || selection.Form == SelectQuiet {
		return anchor(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(kit.Text(th.Theme, th.T.Detail, th.R.Fg2, selection.Label)),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(anchor),
	)
}

func (selection *Select) cut(th *Theme) unit.Dp {
	if selection.Form == SelectQuiet {
		return th.Cut.Small
	}
	return th.Cut.Mid
}

func (selection *Select) open(gtx layout.Context, th *Theme, overlays *Overlays, options []SelectOption, selected int) {
	tag := &selection.click.Clickable
	placement := Placement{Return: tag}
	if selection.Return != nil {
		placement.Return = selection.Return
	}
	listed := len(options) >= pickerSearchFrom
	for _, option := range options {
		listed = listed || option.Detail != ""
	}
	if listed {
		selection.picker.open()
		placement.Frame, placement.Width = true, 340
		placement.Focus = selection.picker.focus(len(options) >= pickerSearchFrom)
		overlays.Open(tag, overlays.AnchorOf(tag, selection.size), placement, func(gtx layout.Context) layout.Dimensions {
			dimensions, picked := selection.picker.Layout(gtx, th, options, selected)
			if picked >= 0 {
				selection.value, selection.changed = picked, true
				overlays.Close(tag)
			}
			return dimensions
		})
		return
	}
	items := make([]kit.MenuItem, len(options))
	for index, option := range options {
		items[index] = kit.MenuItem{Label: option.Label, Checked: index == selected, Disabled: option.Disabled}
	}
	content := func(gtx layout.Context) layout.Dimensions {
		selection.menu.Preview = selection.Preview
		dimensions := selection.menu.Layout(gtx, th.Theme, items)
		if index, ok := selection.menu.Clicked(gtx); ok {
			selection.value, selection.changed = index, true
			overlays.Close(tag)
			gtx.Execute(op.InvalidateCmd{})
		}
		return dimensions
	}
	if selection.Form == SelectField {
		placement.MinWidth = unit.Dp(float32(selection.size.X) / gtx.Metric.PxPerDp)
	}
	overlays.Open(tag, overlays.AnchorOf(tag, selection.size), placement, content)
}

func (selection *Select) hoverValue(gtx layout.Context, th *Theme, open bool) float32 {
	hovered := selection.click.Hovered() || selection.click.Pressed() || open
	if selection.Preview == kit.StateHover || selection.Preview == kit.StatePressed {
		hovered = true
	}
	return transition(gtx, &selection.hover, hovered && gtx.Source.Enabled(), th.Motion.HoverIn, th.Motion.HoverOut)
}

func arrow(open bool) *widget.Icon {
	if open {
		return Icon.ExpandLess
	}
	return Icon.ExpandMore
}

func (selection *Select) layoutField(gtx layout.Context, th *Theme, label string, open bool, focus float32) layout.Dimensions {
	hover := selection.hoverValue(gtx, th, open)
	width := min(max(gtx.Constraints.Min.X, gtx.Dp(160)), gtx.Constraints.Max.X)
	bounds := image.Rect(0, 0, width, min(gtx.Dp(44), gtx.Constraints.Max.Y))
	cut := gtx.Dp(th.Cut.Mid)
	fill := palette.Mix(th.R.Field, th.R.Fg, tokens.AlphaHoverPlane*hover)
	line := palette.Mix(th.R.Edge, th.R.Fg2, focus)
	if !gtx.Source.Enabled() {
		fill, line = palette.WithAlpha(fill, tokens.AlphaDisabled), palette.WithAlpha(line, tokens.AlphaDisabled)
	}
	kit.FillCut(gtx.Ops, bounds, cut, fill)
	kit.StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), line)
	left, right := gtx.Dp(14), gtx.Dp(10)
	content := gtx
	content.Constraints = layout.Constraints{Max: image.Pt(width-left-right, bounds.Max.Y)}
	MiddleLeft(content, left, bounds.Max.Y, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, kit.Text(th.Theme, th.T.Body, th.R.Fg, label)),
			layout.Rigid(layout.Spacer{Width: 8}.Layout),
			layout.Rigid(IconWidget(arrow(open), 20, th.R.Fg2)),
		)
	})
	return layout.Dimensions{Size: bounds.Max}
}

func (selection *Select) layoutQuiet(gtx layout.Context, th *Theme, label string, open bool) layout.Dimensions {
	hover := selection.hoverValue(gtx, th, open)
	height := gtx.Dp(32)
	ink := palette.Mix(th.R.Fg2, th.R.Fg, hover)
	if selection.Tone != palette.Idle {
		ink = th.R.Stone(selection.Tone)
	}
	record := op.Record(gtx.Ops)
	content := gtx
	content.Constraints = layout.Constraints{Max: image.Pt(gtx.Constraints.Max.X, height)}
	dimensions := layout.Inset{Left: 10, Right: 6}.Layout(content, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, 5)
		if selection.Icon != nil {
			children = append(children, layout.Rigid(IconWidget(selection.Icon, 16, ink)), layout.Rigid(layout.Spacer{Width: 6}.Layout))
		}
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(200))
				return kit.RawText(th.Theme, th.T.Label, ink, label)(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: 2}.Layout),
			layout.Rigid(IconWidget(arrow(open), 18, palette.Mix(th.R.Fg3, th.R.Fg2, hover))),
		)
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	})
	call := record.Stop()
	bounds := image.Rect(0, 0, dimensions.Size.X, height)
	if hover > 0 {
		kit.FillCut(gtx.Ops, bounds, gtx.Dp(th.Cut.Small), palette.WithAlpha(th.R.Fg, tokens.AlphaHoverFill*hover))
	}
	offset := op.Offset(image.Pt(0, (height-dimensions.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: bounds.Max}
}
