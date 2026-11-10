package kit

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// SectionIndex tracks settings sections as rows or medium-width chips.
type SectionIndex struct {
	Preview State

	rows    []Click
	chips   []Plaque
	hover   []anim.Tween
	focus   []anim.Tween
	pending []bool
	strip   layout.List
}

func (index *SectionIndex) prepare(labels []string) {
	if len(index.rows) != len(labels) {
		rows := make([]Click, len(labels))
		copy(rows, index.rows)
		index.rows = rows
	}
	if len(index.chips) != len(labels) {
		chips := make([]Plaque, len(labels))
		copy(chips, index.chips)
		index.chips = chips
	}
	if len(index.hover) != len(labels) {
		hover := make([]anim.Tween, len(labels))
		copy(hover, index.hover)
		index.hover = hover
	}
	if len(index.focus) != len(labels) {
		focus := make([]anim.Tween, len(labels))
		copy(focus, index.focus)
		index.focus = focus
	}
	if len(index.pending) != len(labels) {
		pending := make([]bool, len(labels))
		copy(pending, index.pending)
		index.pending = pending
	}
	index.strip.Axis = layout.Horizontal
}

// Layout draws the expanded index, medium chip strip, or nothing at compact width.
func (index *SectionIndex) Layout(gtx layout.Context, th *theme.Theme, labels []string, current int, class FrameClass) layout.Dimensions {
	index.prepare(labels)
	for item := range labels {
		if class == FrameExpanded && index.rows[item].Clicked(gtx) {
			index.pending[item] = true
		}
		if class == FrameMedium && index.chips[item].Clicked(gtx) {
			index.pending[item] = true
		}
	}
	switch class {
	case FrameCompact:
		return layout.Dimensions{}
	case FrameMedium:
		return index.layoutChips(gtx, th, labels, current)
	default:
		return index.layoutRows(gtx, th, labels, current)
	}
}

func (index *SectionIndex) layoutChips(gtx layout.Context, th *theme.Theme, labels []string, current int) layout.Dimensions {
	return index.strip.Layout(gtx, len(labels), func(gtx layout.Context, item int) layout.Dimensions {
		return layout.Inset{Right: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			chip := &index.chips[item]
			chip.Kind = PlaqueQuiet
			if item == current {
				chip.Kind = PlaqueOn
			}
			chip.Small = true
			chip.Label = labels[item]
			chip.Preview = index.Preview
			return chip.Layout(gtx, th)
		})
	})
}

func (index *SectionIndex) layoutRows(gtx layout.Context, th *theme.Theme, labels []string, current int) layout.Dimensions {
	width := min(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeSectionIndexWidth))
	gtx.Constraints.Min.X = width
	gtx.Constraints.Max.X = width
	children := make([]layout.FlexChild, 0, len(labels)*2-1)
	for item, label := range labels {
		item, label := item, label
		if item > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(tokens.SizeSectionIndexRowGap)}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dimensions := index.rows[item].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeSectionIndexRow))
				selected := item == current
				ink := th.R.Fg2
				if selected {
					FillCut(gtx.Ops, image.Rectangle{Max: size}, gtx.Dp(th.Cut.Mid), th.R.Sel)
					ink = th.R.OnSel
				} else {
					hovered := index.rows[item].Hovered() || index.Preview == StateHover || index.Preview == StatePressed
					hover := hoverValue(gtx, th, &index.hover[item], hovered)
					if hover > 0 {
						FillCut(gtx.Ops, image.Rectangle{Max: size}, gtx.Dp(th.Cut.Mid), palette.Mix(th.R.Stone2, th.R.Stone3, hover))
					}
				}
				semantic.Button.Add(gtx.Ops)
				semantic.LabelOp(label).Add(gtx.Ops)
				semantic.SelectedOp(selected).Add(gtx.Ops)
				semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
				labelContext := gtx
				labelContext.Constraints.Min = image.Point{}
				labelContext.Constraints.Max = image.Pt(max(0, size.X-2*gtx.Dp(tokens.SizeSectionIndexInset)), size.Y)
				record := op.Record(gtx.Ops)
				labelDimensions := Text(th, th.T.Row, ink, label)(labelContext)
				call := record.Stop()
				x := gtx.Dp(tokens.SizeSectionIndexInset)
				y := max(0, (size.Y-labelDimensions.Size.Y)/2)
				offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
				call.Add(gtx.Ops)
				offset.Pop()
				return layout.Dimensions{Size: size, Baseline: y + labelDimensions.Baseline}
			})
			focused := gtx.Focused(&index.rows[item].Clickable) || index.Preview == StateFocus
			focus := focusValue(gtx, th, &index.focus[item], focused)
			FocusRingAlpha(gtx, th, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Mid), focus)
			return dimensions
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// Selected reports one activated section.
func (index *SectionIndex) Selected() (int, bool) {
	for item, pending := range index.pending {
		if pending {
			index.pending[item] = false
			return item, true
		}
	}
	return 0, false
}
