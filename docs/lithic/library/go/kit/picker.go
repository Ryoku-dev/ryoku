package kit

import (
	"image"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

var pickerArrow = Icon(symbols.ExpandMore)

// Picker is a labelled field that opens its options as a popover menu.
type Picker struct {
	Preview State

	button       Click
	popover      Popover
	menu         Menu
	hover, focus anim.Tween
}

// Focus moves keyboard focus to the picker anchor.
func (p *Picker) Focus(gtx layout.Context) {
	gtx.Execute(key.FocusCmd{Tag: &p.button.Clickable})
}

// Open reports whether the option menu is open.
func (p *Picker) Open() bool { return p.popover.Open }

// Show opens the option menu.
func (p *Picker) Show() { p.popover.Open = true }

// Close dismisses the option menu and restores focus.
func (p *Picker) Close(gtx layout.Context) { p.popover.Close(gtx) }

// Layout draws the anchor and its option menu.
func (p *Picker) Layout(gtx layout.Context, th *theme.Theme, label string, options []string, selected int) layout.Dimensions {
	if p.button.Clicked(gtx) {
		p.popover.Open = !p.popover.Open
	}
	for {
		value, ok := gtx.Event(key.Filter{Focus: &p.button.Clickable, Name: key.NameDownArrow})
		if !ok {
			break
		}
		if pressed, ok := value.(key.Event); ok && pressed.State == key.Press && gtx.Source.Enabled() {
			p.popover.Open = true
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	p.popover.Opener = &p.button.Clickable
	p.popover.Preview = p.Preview
	value := ""
	if selected >= 0 && selected < len(options) {
		value = options[selected]
	}
	anchor := func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, 3)
		if label != "" {
			children = append(children,
				layout.Rigid(Text(th, th.T.Detail, th.R.Fg2, label)),
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			)
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			finalCut := gtx.Dp(th.Cut.Mid)
			focused := gtx.Focused(&p.button.Clickable) || p.Preview == StateFocus
			focus := focusValue(gtx, th, &p.focus, focused)
			dimensions := p.button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.Button.Add(gtx.Ops)
				name := value
				if label != "" {
					name = label + ": " + value
				}
				semantic.LabelOp(name).Add(gtx.Ops)
				semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
				height := gtx.Dp(tokens.SizePickerHeight)
				width := max(gtx.Constraints.Min.X, gtx.Dp(tokens.SizePickerWidthMin))
				width = min(width, gtx.Constraints.Max.X)
				bounds := image.Rect(0, 0, width, height)
				hovered := p.button.Hovered() || p.Preview == StateHover
				if p.Preview == StatePressed {
					hovered = true
				}
				hover := hoverValue(gtx, th, &p.hover, hovered)
				drawFieldSurface(gtx, th, bounds, finalCut, palette.Mix(th.R.Field, th.R.Fg, tokens.AlphaHoverPlane*hover), focus, false)
				gtx.Constraints = layout.Exact(bounds.Max)
				return layout.Inset{Left: unit.Dp(tokens.SizePickerInset), Right: unit.Dp(tokens.SizePickerInset)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return centerY(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, rawText(th, th.T.Body, th.R.Fg, value)),
							layout.Rigid(layout.Spacer{Width: unit.Dp(tokens.SizePickerArrowGap)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								size := gtx.Dp(tokens.SizePickerArrow)
								gtx.Constraints = layout.Exact(image.Pt(size, size))
								return pickerArrow.Layout(gtx, th.R.Fg2)
							}),
						)
					})
				})
			})
			FocusRingAlpha(gtx, th, image.Rectangle{Max: dimensions.Size}, finalCut, focus)
			return dimensions
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	content := func(gtx layout.Context) layout.Dimensions {
		items := make([]MenuItem, len(options))
		for index, option := range options {
			items[index] = MenuItem{Label: option, Checked: index == selected}
		}
		// the keyboard starts on the current choice
		p.menu.start = selected
		return p.menu.Layout(gtx, th, items)
	}
	return p.popover.Layout(gtx, anchor, content)
}

// Changed returns the selected option index and closes the menu.
func (p *Picker) Changed(gtx layout.Context) (int, bool) {
	if selected, ok := p.menu.Clicked(gtx); ok {
		p.popover.Close(gtx)
		return selected, true
	}
	return 0, false
}
