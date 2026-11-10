package kit

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// RailDestination describes Fleet, Activity, or Settings without app-specific types.
type RailDestination struct {
	Label string
	Icon  *widget.Icon
	Notch bool
}

type railItemState struct {
	Click
	hover, focus, notch anim.Tween
}

// Rail owns Home, the three destinations, and the sole gold New plaque.
type Rail struct {
	Preview State
	NewOpen bool

	home        railItemState
	newPlaque   NewPlaque
	items       []railItemState
	selection   anim.Tween
	homePending bool
	newPending  bool
	pending     []bool
}

func (r *Rail) prepare(destinations []RailDestination) {
	if len(r.items) != len(destinations) {
		items := make([]railItemState, len(destinations))
		copy(items, r.items)
		r.items = items
	}
	if len(r.pending) != len(destinations) {
		pending := make([]bool, len(destinations))
		copy(pending, r.pending)
		r.pending = pending
	}
}

func (r *Rail) poll(gtx layout.Context) {
	if r.home.Clicked(gtx) {
		r.homePending = true
		r.NewOpen = false
	}
	if r.newPlaque.Clicked(gtx) {
		r.newPending = true
		r.NewOpen = !r.NewOpen
	}
	for index := range r.items {
		if r.items[index].Clicked(gtx) {
			r.pending[index] = true
			r.NewOpen = false
		}
	}
}

// Layout draws a vertical rail or the compact five-column bottom rail.
func (r *Rail) Layout(gtx layout.Context, th *theme.Theme, destinations []RailDestination, selected int, class FrameClass) layout.Dimensions {
	r.prepare(destinations)
	r.poll(gtx)
	if class == FrameCompact {
		return r.layoutBottom(gtx, th, destinations, selected)
	}
	return r.layoutVertical(gtx, th, destinations, selected, class)
}

func (r *Rail) layoutVertical(gtx layout.Context, th *theme.Theme, destinations []RailDestination, selected int, class FrameClass) layout.Dimensions {
	width, height := gtx.Constraints.Max.X, gtx.Constraints.Max.Y
	surface := clip.Rect{Max: image.Pt(width, height)}.Push(gtx.Ops)
	paint.Fill(gtx.Ops, th.R.Stone1)
	tile := gtx.Dp(tokens.SizeRailDestinationExpanded)
	start := gtx.Dp(tokens.SizeRailStartExpanded)
	homeSize := gtx.Dp(tokens.SizeRailHomeExpanded)
	if class == FrameMedium {
		tile = gtx.Dp(tokens.SizeRailDestinationMedium)
		start = gtx.Dp(tokens.SizeRailStartMedium)
		homeSize = gtx.Dp(tokens.SizeRailHomeMedium)
	}
	gap := gtx.Dp(tokens.SizeRailTileGap)
	positions := make([]int, len(destinations))
	for index := range positions {
		positions[index] = start + index*(tile+gap)
		if index > 0 {
			positions[index] += tile + gap
		}
	}
	if len(positions) > 0 {
		positions[len(positions)-1] = height - gtx.Dp(tokens.SizeRailItemBottomInset) - tile
	}
	if selected >= 0 && selected < len(positions) {
		r.selection.Duration = th.Motion.Select
		r.selection.Ease = anim.EaseChisel
		r.selection.Set(gtx, float32(positions[selected]))
		y := int(r.selection.Value(gtx) + .5)
		x := (width - tile) / 2
		offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		FillCut(gtx.Ops, image.Rect(0, 0, tile, tile), gtx.Dp(th.Cut.Mid), th.R.Sel)
		offset.Pop()
	}
	layoutAt(gtx, image.Pt((width-homeSize)/2, gtx.Dp(tokens.SizeRailHomeTop)), image.Pt(homeSize, homeSize), func(gtx layout.Context) layout.Dimensions {
		return r.layoutHome(gtx, th, false)
	})
	for index, destination := range destinations {
		index, destination := index, destination
		layoutAt(gtx, image.Pt((width-tile)/2, positions[index]), image.Pt(tile, tile), func(gtx layout.Context) layout.Dimensions {
			return r.layoutDestination(gtx, th, &r.items[index], destination, index == selected, class == FrameExpanded)
		})
	}
	newY := start + tile + gap
	newCompact := class != FrameExpanded
	newWidth, newHeight := gtx.Dp(tokens.SizeNewPlaqueWidth), gtx.Dp(tokens.SizeNewPlaqueHeight)
	if newCompact {
		newWidth, newHeight = gtx.Dp(tokens.SizeNewPlaqueWidthCompact), gtx.Dp(tokens.SizeNewPlaqueHeightCompact)
	}
	layoutAt(gtx, image.Pt((width-newWidth)/2, newY+(tile-newHeight)/2), image.Pt(newWidth, newHeight), func(gtx layout.Context) layout.Dimensions {
		r.newPlaque.Preview = r.Preview
		return r.newPlaque.Layout(gtx, th, newCompact, r.NewOpen)
	})
	r.poll(gtx)
	surface.Pop()
	return layout.Dimensions{Size: image.Pt(width, height)}
}

func (r *Rail) layoutBottom(gtx layout.Context, th *theme.Theme, destinations []RailDestination, selected int) layout.Dimensions {
	width, height := gtx.Constraints.Max.X, gtx.Constraints.Max.Y
	surface := clip.Rect{Max: image.Pt(width, height)}.Push(gtx.Ops)
	paint.Fill(gtx.Ops, th.R.Stone1)
	columns := tokens.SizeRailBottomColumns
	columnWidth := width / columns
	tileHeight := gtx.Dp(tokens.SizeRailDestinationCompact)
	y := (height - tileHeight) / 2
	position := func(index int) int {
		if index == 0 {
			return columnWidth
		}
		return (index + 2) * columnWidth
	}
	if selected >= 0 && selected < len(destinations) {
		r.selection.Duration = th.Motion.Select
		r.selection.Ease = anim.EaseChisel
		r.selection.Set(gtx, float32(position(selected)))
		x := int(r.selection.Value(gtx) + .5)
		offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		FillCut(gtx.Ops, image.Rect(0, 0, columnWidth, tileHeight), gtx.Dp(th.Cut.Small), th.R.Sel)
		offset.Pop()
	}
	layoutAt(gtx, image.Pt(0, y), image.Pt(columnWidth, tileHeight), func(gtx layout.Context) layout.Dimensions {
		return r.layoutHome(gtx, th, true)
	})
	for index, destination := range destinations {
		index, destination := index, destination
		layoutAt(gtx, image.Pt(position(index), y), image.Pt(columnWidth, tileHeight), func(gtx layout.Context) layout.Dimensions {
			return r.layoutDestination(gtx, th, &r.items[index], destination, index == selected, true)
		})
	}
	layoutAt(gtx, image.Pt(2*columnWidth, y), image.Pt(columnWidth, tileHeight), func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			r.newPlaque.Preview = r.Preview
			return r.newPlaque.Layout(gtx, th, true, r.NewOpen)
		})
	})
	r.poll(gtx)
	surface.Pop()
	return layout.Dimensions{Size: image.Pt(width, height)}
}

func (r *Rail) layoutHome(gtx layout.Context, th *theme.Theme, label bool) layout.Dimensions {
	cut := gtx.Dp(th.Cut.Mid)
	dimensions := r.home.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp("Home").Add(gtx.Ops)
		semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
		hovered := r.home.Hovered() || r.Preview == StateHover || r.Preview == StatePressed
		hover := hoverValue(gtx, th, &r.home.hover, hovered)
		if hover > 0 {
			FillCut(gtx.Ops, image.Rectangle{Max: gtx.Constraints.Max}, cut, palette.Mix(th.R.Stone1, th.R.Stone3, hover))
		}
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			markSize := unit.Dp(tokens.SizeBrandMark)
			if label {
				markSize = unit.Dp(tokens.SizeBrandMarkCompact)
			}
			mark := layout.Rigid(BrandMark(th, markSize))
			if !label {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, mark)
			}
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				mark,
				layout.Rigid(layout.Spacer{Height: unit.Dp(tokens.SizeRailItemLabelGap)}.Layout),
				layout.Rigid(Text(th, th.T.Caption, th.R.Fg2, "Home")),
			)
		})
	})
	focused := gtx.Focused(&r.home.Clickable) || r.Preview == StateFocus
	focus := focusValue(gtx, th, &r.home.focus, focused)
	FocusRingAlpha(gtx, th, image.Rectangle{Max: dimensions.Size}, cut, focus)
	return dimensions
}

func (r *Rail) layoutDestination(gtx layout.Context, th *theme.Theme, state *railItemState, destination RailDestination, selected, labels bool) layout.Dimensions {
	cut := gtx.Dp(th.Cut.Mid)
	dimensions := state.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(destination.Label).Add(gtx.Ops)
		semantic.SelectedOp(selected).Add(gtx.Ops)
		semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
		hovered := state.Hovered() || r.Preview == StateHover || r.Preview == StatePressed
		hover := hoverValue(gtx, th, &state.hover, hovered && !selected)
		if hover > 0 {
			FillCut(gtx.Ops, image.Rectangle{Max: gtx.Constraints.Max}, cut, palette.Mix(th.R.Stone1, th.R.Stone3, hover))
		}
		state.notch.Duration = th.Motion.Notch
		state.notch.Ease = anim.EaseChisel
		if destination.Notch {
			state.notch.Set(gtx, 1)
		} else {
			state.notch.Set(gtx, 0)
		}
		notchSize := int(float32(gtx.Dp(th.Cut.Notch))*state.notch.Value(gtx) + .5)
		if notchSize > 0 {
			Notch(gtx.Ops, image.Point{}, notchSize, th.R.Amber)
		}
		ink := th.R.Fg2
		if selected {
			ink = th.R.OnSel
		}
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			iconWidget := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				size := gtx.Dp(tokens.SizeRailDestinationIcon)
				gtx.Constraints = layout.Exact(image.Pt(size, size))
				if destination.Icon == nil {
					return layout.Dimensions{Size: image.Pt(size, size)}
				}
				return destination.Icon.Layout(gtx, ink)
			})
			if !labels {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, iconWidget)
			}
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				iconWidget,
				layout.Rigid(layout.Spacer{Height: unit.Dp(tokens.SizeRailItemLabelGap)}.Layout),
				layout.Rigid(Text(th, th.T.Caption, ink, destination.Label)),
			)
		})
	})
	focused := gtx.Focused(&state.Clickable) || r.Preview == StateFocus
	focus := focusValue(gtx, th, &state.focus, focused)
	FocusRingAlpha(gtx, th, image.Rectangle{Max: dimensions.Size}, cut, focus)
	return dimensions
}

// HomeClicked reports a Home activation.
func (r *Rail) HomeClicked() bool {
	clicked := r.homePending
	r.homePending = false
	return clicked
}

// NewClicked reports a New activation.
func (r *Rail) NewClicked() bool {
	clicked := r.newPending
	r.newPending = false
	return clicked
}

// Selected returns one destination activation.
func (r *Rail) Selected() (int, bool) {
	for index, pending := range r.pending {
		if pending {
			r.pending[index] = false
			return index, true
		}
	}
	return 0, false
}

func layoutAt(gtx layout.Context, at, size image.Point, widget layout.Widget) layout.Dimensions {
	size.X = max(0, size.X)
	size.Y = max(0, size.Y)
	offset := op.Offset(at).Push(gtx.Ops)
	area := clip.Rect{Max: size}.Push(gtx.Ops)
	child := gtx
	child.Constraints = layout.Exact(size)
	widget(child)
	area.Pop()
	offset.Pop()
	return layout.Dimensions{Size: size}
}
