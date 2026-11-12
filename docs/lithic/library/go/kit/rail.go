package kit

import (
	"image"
	"time"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
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

// Rail owns Home, the destinations, and the sole gold New plaque.
// Destinations before FootIndex stack from the top, with New after the first
// one; destinations from FootIndex on stay in reading order as one group
// anchored to the foot. A zero FootIndex means the last destination alone.
// Brand, when set, draws the host's mark inside Home's selection square in
// place of the octagon.
type Rail struct {
	Preview   State
	NewOpen   bool
	HideNew   bool
	FootIndex int
	Brand     BrandMarkFunc

	home      railItemState
	newPlaque NewPlaque
	items     []railItemState
	selection anim.Tween
	// selectionShown records whether the last frame drew a selection
	selectionShown bool
	homePending    bool
	newPending     bool
	pending        []bool
	heights        []int
	wrapped        []bool
	measure        op.Ops
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
	labels := class == FrameExpanded
	tileWidth := tile
	r.sizeTiles(len(destinations), tile)
	room := 0
	if labels {
		pad := gtx.Dp(th.Sp.S1)
		widest := 0
		for _, destination := range destinations {
			widest = max(widest, r.labelWidth(gtx, th, destination.Label))
		}
		tileWidth = railTileWidth(widest, tile, width, pad)
		room = tileWidth - 2*pad
		r.wrapLabels(gtx, th, destinations, room, tile)
	}
	gap := gtx.Dp(tokens.SizeRailTileGap)
	positions := railPositions(r.heights, r.FootIndex, !r.HideNew, start, tile, gap, height-gtx.Dp(tokens.SizeRailItemBottomInset))
	x := (width - tileWidth) / 2
	shown := selected >= 0 && selected < len(positions)
	if shown {
		y := int(r.placeSelection(gtx, th.Motion.Select, float32(positions[selected])) + .5)
		offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		tileBounds, tileCut := image.Rect(0, 0, tileWidth, r.heights[selected]), gtx.Dp(th.Cut.Mid)
		FillCut(gtx.Ops, tileBounds, tileCut, th.R.Sel)

		offset.Pop()
	}
	r.selectionShown = shown
	layoutAt(gtx, image.Pt((width-homeSize)/2, gtx.Dp(tokens.SizeRailHomeTop)), image.Pt(homeSize, homeSize), func(gtx layout.Context) layout.Dimensions {
		return r.layoutHome(gtx, th, false)
	})
	for index, destination := range destinations {
		index, destination := index, destination
		layoutAt(gtx, image.Pt(x, positions[index]), image.Pt(tileWidth, r.heights[index]), func(gtx layout.Context) layout.Dimensions {
			wrap := 0
			if r.wrapped[index] {
				wrap = room
			}
			return r.layoutDestination(gtx, th, &r.items[index], destination, index == selected, labels, wrap)
		})
	}
	if !r.HideNew {
		newY := start + tile + gap
		if len(r.heights) > 0 {
			newY = start + r.heights[0] + gap
		}
		newCompact := class != FrameExpanded
		newWidth, newHeight := gtx.Dp(tokens.SizeNewPlaqueWidth), gtx.Dp(tokens.SizeNewPlaqueHeight)
		if newCompact {
			newWidth, newHeight = gtx.Dp(tokens.SizeNewPlaqueWidthCompact), gtx.Dp(tokens.SizeNewPlaqueHeightCompact)
		}
		layoutAt(gtx, image.Pt((width-newWidth)/2, newY+(tile-newHeight)/2), image.Pt(newWidth, newHeight), func(gtx layout.Context) layout.Dimensions {
			r.newPlaque.Preview = r.Preview
			return r.newPlaque.Layout(gtx, th, newCompact, r.NewOpen)
		})
	}
	r.poll(gtx)
	surface.Pop()
	return layout.Dimensions{Size: image.Pt(width, height)}
}

// placeSelection returns the selection fill's position for target. After a
// frame with no selection the fill appears in place; otherwise it glides.
func (r *Rail) placeSelection(gtx layout.Context, duration time.Duration, target float32) float32 {
	if !r.selectionShown {
		r.selection = anim.Tween{}
	}
	r.selection.Duration = duration
	r.selection.Ease = anim.EaseChisel
	r.selection.Set(gtx, target)
	return r.selection.Value(gtx)
}

// railTileWidth is the shared expanded tile width: the widest label plus pad
// on each side, never narrower than floor nor wider than the rail less pad on
// each side. A label wider than the result less its pads wraps.
func railTileWidth(widest, floor, railWidth, pad int) int {
	ceiling := max(floor, railWidth-2*pad)
	return min(max(widest+2*pad, floor), ceiling)
}

func (r *Rail) sizeTiles(count, tile int) {
	if cap(r.heights) < count {
		r.heights = make([]int, count)
		r.wrapped = make([]bool, count)
	}
	r.heights = r.heights[:count]
	r.wrapped = r.wrapped[:count]
	for index := range r.heights {
		r.heights[index], r.wrapped[index] = tile, false
	}
}

func (r *Rail) labelWidth(gtx layout.Context, th *theme.Theme, label string) int {
	r.measure.Reset()
	measure := gtx
	measure.Ops = &r.measure
	measure.Constraints = layout.Constraints{Max: image.Pt(1<<20, gtx.Constraints.Max.Y)}
	return Label{Style: th.T.Caption, Color: th.R.Fg2, MaxLines: 1}.Layout(measure, th, label).Size.X
}

// wrapLabels marks labels that need a second line inside room and grows
// their tiles by one caption line.
func (r *Rail) wrapLabels(gtx layout.Context, th *theme.Theme, destinations []RailDestination, room, tile int) {
	line := gtx.Sp(th.T.Caption.Line)
	for index, destination := range destinations {
		if r.labelWidth(gtx, th, destination.Label) > room {
			r.wrapped[index] = true
			r.heights[index] = tile + line
		}
	}
}

// railPositions places destinations of the given heights in a rail whose
// foot edge is foot: those before footIndex from start, leaving a slot of
// newHeight after the first one for New, and the rest as one reading-order
// group ending at foot.
func railPositions(heights []int, footIndex int, newSlot bool, start, newHeight, gap, foot int) []int {
	count := len(heights)
	if footIndex <= 0 || footIndex > count {
		footIndex = count - 1
	}
	positions := make([]int, count)
	y := start
	for index := range footIndex {
		positions[index] = y
		y += heights[index] + gap
		if index == 0 && newSlot {
			y += newHeight + gap
		}
	}
	y = foot
	for index := count - 1; index >= max(footIndex, 0); index-- {
		y -= heights[index]
		positions[index] = y
		y -= gap
	}
	return positions
}

func (r *Rail) layoutBottom(gtx layout.Context, th *theme.Theme, destinations []RailDestination, selected int) layout.Dimensions {
	width, height := gtx.Constraints.Max.X, gtx.Constraints.Max.Y
	surface := clip.Rect{Max: image.Pt(width, height)}.Push(gtx.Ops)
	paint.Fill(gtx.Ops, th.R.Stone1)
	newSlot := 1
	if r.HideNew {
		newSlot = 0
	}
	// home, each destination and New share equal columns; three destinations give the token's five
	columnWidth := width / (1 + len(destinations) + newSlot)
	tileHeight := gtx.Dp(tokens.SizeRailDestinationCompact)
	y := (height - tileHeight) / 2
	position := func(index int) int {
		if index == 0 {
			return columnWidth
		}
		return (index + 1 + newSlot) * columnWidth
	}
	shown := selected >= 0 && selected < len(destinations)
	if shown {
		x := int(r.placeSelection(gtx, th.Motion.Select, float32(position(selected))) + .5)
		offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		tileBounds, tileCut := image.Rect(0, 0, columnWidth, tileHeight), gtx.Dp(th.Cut.Small)
		FillCut(gtx.Ops, tileBounds, tileCut, th.R.Sel)

		offset.Pop()
	}
	r.selectionShown = shown
	layoutAt(gtx, image.Pt(0, y), image.Pt(columnWidth, tileHeight), func(gtx layout.Context) layout.Dimensions {
		return r.layoutHome(gtx, th, true)
	})
	for index, destination := range destinations {
		index, destination := index, destination
		layoutAt(gtx, image.Pt(position(index), y), image.Pt(columnWidth, tileHeight), func(gtx layout.Context) layout.Dimensions {
			return r.layoutDestination(gtx, th, &r.items[index], destination, index == selected, true, 0)
		})
	}
	if !r.HideNew {
		layoutAt(gtx, image.Pt(2*columnWidth, y), image.Pt(columnWidth, tileHeight), func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				r.newPlaque.Preview = r.Preview
				return r.newPlaque.Layout(gtx, th, true, r.NewOpen)
			})
		})
	}
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
			mark := layout.Rigid(BrandMarkWith(th, markSize, r.Brand))
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

// layoutDestination draws one tile; a positive wrap is the width a label
// wraps to on two centred lines.
func (r *Rail) layoutDestination(gtx layout.Context, th *theme.Theme, state *railItemState, destination RailDestination, selected, labels bool, wrap int) layout.Dimensions {
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
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if wrap <= 0 {
						return Text(th, th.T.Caption, ink, destination.Label)(gtx)
					}
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = wrap, wrap
					return Label{Style: th.T.Caption, Color: ink, MaxLines: 2, Align: text.Middle}.Layout(gtx, th, destination.Label)
				}),
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
