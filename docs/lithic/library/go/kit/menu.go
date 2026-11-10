package kit

import (
	"image"

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

// MenuItem describes one command, disabled command, danger command, or divider.
type MenuItem struct {
	Icon            *widget.Icon
	Label, Shortcut string
	Danger          bool
	Disabled        bool
	Divider         bool
	Preview         State
}

// Menu is a cut command list with latched item activation.
type Menu struct {
	Preview State

	clicks              []Click
	disabled, divider   []bool
	pending             []bool
	hover, press, focus []anim.Tween
}

func (m *Menu) prepare(items []MenuItem) {
	if len(m.clicks) != len(items) {
		clicks := make([]Click, len(items))
		copy(clicks, m.clicks)
		m.clicks = clicks
	}
	if len(m.disabled) != len(items) {
		m.disabled = make([]bool, len(items))
	}
	if len(m.divider) != len(items) {
		m.divider = make([]bool, len(items))
	}
	if len(m.pending) != len(items) {
		m.pending = make([]bool, len(items))
	}
	if len(m.hover) != len(items) {
		hover := make([]anim.Tween, len(items))
		copy(hover, m.hover)
		m.hover = hover
	}
	if len(m.press) != len(items) {
		press := make([]anim.Tween, len(items))
		copy(press, m.press)
		m.press = press
	}
	if len(m.focus) != len(items) {
		focus := make([]anim.Tween, len(items))
		copy(focus, m.focus)
		m.focus = focus
	}
	for index, item := range items {
		m.disabled[index] = item.Disabled
		m.divider[index] = item.Divider
		if item.Disabled || item.Divider {
			m.pending[index] = false
		}
	}
}

// Layout draws the menu and its items.
func (m *Menu) Layout(gtx layout.Context, th *theme.Theme, items []MenuItem) layout.Dimensions {
	m.prepare(items)
	for index := range m.clicks {
		if !m.disabled[index] && !m.divider[index] && m.clicks[index].Clicked(gtx) {
			m.pending[index] = true
		}
	}
	width := menuWidth(gtx, th, items)
	gtx.Constraints.Min.X = width
	gtx.Constraints.Max.X = width

	content := op.Record(gtx.Ops)
	dimensions := layout.Inset{Top: unit.Dp(tokens.SizeMenuPaddingVertical), Bottom: unit.Dp(tokens.SizeMenuPaddingVertical)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(items))
		for index, item := range items {
			index, item := index, item
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if item.Divider {
					return layout.Inset{Top: th.Sp.S2, Bottom: th.Sp.S2, Left: unit.Dp(tokens.SizeMenuDividerInset), Right: unit.Dp(tokens.SizeMenuDividerInset)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						width := gtx.Constraints.Max.X
						height := max(1, gtx.Dp(tokens.SizeHairline))
						FillCut(gtx.Ops, image.Rect(0, 0, width, height), 0, th.R.Vein)
						return layout.Dimensions{Size: image.Pt(width, height)}
					})
				}
				row := func(gtx layout.Context) layout.Dimensions {
					height := gtx.Dp(tokens.SizeMenuItem)
					gtx.Constraints.Min.Y = height
					gtx.Constraints.Max.Y = height
					preview := item.Preview
					if preview == StateRest {
						preview = m.Preview
					}
					hovered := m.clicks[index].Hovered() || preview == StateHover
					pressed := m.clicks[index].Pressed() || preview == StatePressed
					hover := hoverValue(gtx, th, &m.hover[index], hovered && !item.Disabled)
					press := transitionValue(gtx, &m.press[index], pressed && !item.Disabled, th.Motion.Press, th.Motion.Press)
					if hover > 0 || press > 0 {
						amount := max(tokens.AlphaHoverPlane*hover, tokens.AlphaPressRow*press)
						fill := palette.Mix(th.R.Stone4, th.R.Fg, amount)
						FillCut(gtx.Ops, image.Rect(0, 0, gtx.Constraints.Max.X, height), 0, fill)
					}
					semantic.Button.Add(gtx.Ops)
					semantic.LabelOp(item.Label).Add(gtx.Ops)
					semantic.EnabledOp(!item.Disabled).Add(gtx.Ops)
					return layout.Inset{Left: unit.Dp(tokens.SizeMenuInset), Right: unit.Dp(tokens.SizeMenuInset)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return centerY(gtx, func(gtx layout.Context) layout.Dimensions {
							ink := th.R.Fg
							if item.Danger {
								ink = th.R.Jasper
							}
							if item.Disabled {
								ink = th.R.Fg3
							}
							children := make([]layout.FlexChild, 0, 4)
							if item.Icon != nil {
								children = append(children,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										size := gtx.Dp(tokens.SizeMenuIcon)
										gtx.Constraints = layout.Exact(image.Pt(size, size))
										return item.Icon.Layout(gtx, ink)
									}),
									layout.Rigid(layout.Spacer{Width: unit.Dp(tokens.SizeMenuIconGap)}.Layout),
								)
							}
							children = append(children, layout.Flexed(1, rawText(th, th.T.Label, ink, item.Label)))
							if item.Shortcut != "" {
								children = append(children,
									layout.Rigid(layout.Spacer{Width: unit.Dp(tokens.SizeMenuShortcutGap)}.Layout),
									layout.Rigid(rawText(th, th.T.MonoSm, th.R.Fg3, item.Shortcut)),
								)
							}
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
						})
					})
				}
				var dims layout.Dimensions
				if item.Disabled {
					dims = m.clicks[index].Layout(gtx.Disabled(), row)
				} else {
					dims = m.clicks[index].Layout(gtx, row)
				}
				preview := item.Preview
				if preview == StateRest {
					preview = m.Preview
				}
				focused := gtx.Focused(&m.clicks[index].Clickable) || preview == StateFocus
				focus := focusValue(gtx, th, &m.focus[index], focused && !item.Disabled)
				FocusRingAlpha(gtx, th, image.Rectangle{Max: dims.Size}, 0, focus)
				return dims
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
	contentCall := content.Stop()

	overlay := op.Record(gtx.Ops)
	bounds := image.Rectangle{Max: dimensions.Size}
	FillCut(gtx.Ops, bounds, gtx.Dp(tokens.CutMenu), th.R.Stone4)
	StrokeCut(gtx.Ops, bounds, gtx.Dp(tokens.CutMenu), max(1, gtx.Dp(tokens.SizeHairline)), th.R.Vein)
	contentCall.Add(gtx.Ops)
	presentOverlay(gtx, overlay.Stop(), m.Preview != StateRest, dimensions.Size)
	return dimensions
}

func menuWidth(gtx layout.Context, th *theme.Theme, items []MenuItem) int {
	maximum := min(gtx.Dp(tokens.SizeMenuWidthMax), gtx.Constraints.Max.X)
	minimum := min(gtx.Dp(tokens.SizeMenuWidthMin), maximum)
	widest := minimum
	var operations op.Ops
	for _, item := range items {
		if item.Divider {
			continue
		}
		fixed := gtx.Dp(2 * tokens.SizeMenuInset)
		if item.Icon != nil {
			fixed += gtx.Dp(tokens.SizeMenuIcon + tokens.SizeMenuIconGap)
		}
		if item.Shortcut != "" {
			measure := gtx
			measure.Constraints.Min = image.Point{}
			measure.Constraints.Max.X = max(0, maximum-fixed-gtx.Dp(tokens.SizeMenuShortcutGap))
			operations.Reset()
			measure.Ops = &operations
			dims := Label{Style: th.T.MonoSm, Color: th.R.Fg3, MaxLines: 1}.Layout(measure, th, item.Shortcut)
			fixed += gtx.Dp(tokens.SizeMenuShortcutGap) + dims.Size.X
		}
		measure := gtx
		measure.Constraints.Min = image.Point{}
		measure.Constraints.Max.X = max(0, maximum-fixed)
		operations.Reset()
		measure.Ops = &operations
		dims := Label{Style: th.T.Label, Color: th.R.Fg, MaxLines: 1}.Layout(measure, th, item.Label)
		widest = max(widest, min(maximum, fixed+dims.Size.X))
	}
	return widest
}

// Clicked returns the activated item index.
func (m *Menu) Clicked(gtx layout.Context) (int, bool) {
	for index := range m.clicks {
		if m.pending[index] {
			m.pending[index] = false
			return index, true
		}
		if !m.disabled[index] && !m.divider[index] && m.clicks[index].Clicked(gtx) {
			return index, true
		}
	}
	return 0, false
}

func centerY(gtx layout.Context, widget layout.Widget) layout.Dimensions {
	original := gtx.Constraints
	gtx.Constraints.Min.Y = 0
	record := op.Record(gtx.Ops)
	dimensions := widget(gtx)
	call := record.Stop()
	size := original.Constrain(image.Pt(dimensions.Size.X, max(original.Min.Y, dimensions.Size.Y)))
	y := (size.Y - dimensions.Size.Y) / 2
	offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: size, Baseline: dimensions.Baseline + size.Y - dimensions.Size.Y - y}
}
