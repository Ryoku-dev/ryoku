package kit

import (
	"image"
	"strconv"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

type filterChip struct {
	Click
	press anim.Tween
	hover anim.Tween
	focus anim.Tween
}

// Filters is a row of mutually exclusive filter chips. Chips at rest are
// tonal plates; the selection glides between them as a sel fill with the place
// blade along its bottom.
type Filters struct {
	Labels   []string
	Counts   []int
	Selected int
	Preview  State
	chips    []filterChip
	widths   []int
	plates   []op.CallOp
	children []layout.FlexChild
	x        anim.Tween
	width    anim.Tween
	changed  int
	pending  bool
}

// Changed reports and clears the selected-index change latch.
func (f *Filters) Changed() (int, bool) {
	if !f.pending {
		return 0, false
	}
	changed := f.changed
	f.pending = false
	return changed, true
}

// Layout draws and operates the filter row.
func (f *Filters) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	gtx.Constraints.Min = image.Point{}
	f.ensure(len(f.Labels))
	for i := range f.chips {
		if f.chips[i].Clicked(gtx) && i != f.Selected {
			f.Selected = i
			f.changed, f.pending = i, true
		}
	}
	if len(f.Labels) == 0 {
		return layout.Dimensions{}
	}
	f.Selected = min(max(f.Selected, 0), len(f.Labels)-1)
	gap := gtx.Dp(unit.Dp(tokens.SizeFilterGap))
	previewIndex := 0
	if f.Selected == 0 && len(f.Labels) > 1 {
		previewIndex = 1
	}
	record := op.Record(gtx.Ops)
	for i, label := range f.Labels {
		i, label := i, label
		f.children[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			cut := gtx.Dp(th.Cut.Small)
			itemPreview := StateRest
			if i == previewIndex {
				itemPreview = f.Preview
			}
			dims := f.chips[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.Button.Add(gtx.Ops)
				semantic.LabelOp(label).Add(gtx.Ops)
				semantic.SelectedOp(i == f.Selected).Add(gtx.Ops)
				semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
				pressed := previewTransition(gtx, &f.chips[i].press, itemPreview, StatePressed, f.chips[i].Pressed(), th.Motion.Press, th.Motion.Press)
				cut = gtx.Dp(th.Cut.Small) + int(float32(gtx.Dp(unit.Dp(tokens.SizeControlPressCutGrow)))*pressed+.5)
				sink := int(float32(gtx.Dp(unit.Dp(tokens.SizeControlPressSink)))*pressed + .5)
				hover := previewTransition(gtx, &f.chips[i].hover, itemPreview, StateHover, f.chips[i].Hovered() && i != f.Selected, th.Motion.HoverIn, th.Motion.HoverOut)
				ink := disabledInk(gtx, th.R.Fg2)
				if i == f.Selected {
					ink = disabledInk(gtx, th.R.OnSel)
				}

				contentGTX := gtx
				contentGTX.Constraints.Min = image.Point{}
				contentGTX.Constraints.Max.Y = gtx.Dp(unit.Dp(tokens.SizeFilterHeight))
				contentRecord := op.Record(gtx.Ops)
				content := layout.Inset{Left: unit.Dp(tokens.SizeFilterInset), Right: unit.Dp(tokens.SizeFilterInset)}.Layout(contentGTX, func(gtx layout.Context) layout.Dimensions {
					var parts [3]layout.FlexChild
					count := 0
					parts[count] = layout.Rigid(rawText(th, th.T.Label, ink, label))
					count++
					if i < len(f.Counts) {
						parts[count] = layout.Rigid(layout.Spacer{Width: unit.Dp(tokens.SizeFilterCountGap)}.Layout)
						count++
						parts[count] = layout.Rigid(rawText(th, th.T.MonoSm, ink, strconv.Itoa(f.Counts[i])))
						count++
					}
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, parts[:count]...)
				})
				call := contentRecord.Stop()
				size := constrainSize(gtx, image.Pt(content.Size.X, gtx.Dp(unit.Dp(tokens.SizeFilterHeight))))
				bounds := image.Rectangle{Max: size}
				plate := op.Record(gtx.Ops)
				if i != f.Selected {
					FillCut(gtx.Ops, bounds, cut, disabledInk(gtx, palette.Mix(th.R.Field, th.R.Fg, tokens.AlphaHoverPlane*min(hover, 1))))
				}
				f.plates[i] = plate.Stop()
				if i == f.Selected && pressed > 0 {
					FillCut(gtx.Ops, bounds, cut, palette.WithAlpha(th.R.OnSel, tokens.AlphaPressRow*pressed))
				}
				offset := op.Offset(image.Pt((size.X-content.Size.X)/2, (size.Y-content.Size.Y)/2+sink)).Push(gtx.Ops)
				call.Add(gtx.Ops)
				offset.Pop()
				return layout.Dimensions{Size: size}
			})
			f.widths[i] = dims.Size.X
			focus := previewTransition(gtx, &f.chips[i].focus, itemPreview, StateFocus, gtx.Focused(&f.chips[i].Clickable), th.Motion.FocusIn, th.Motion.FocusOut)
			FocusRingAlpha(gtx, th, image.Rectangle{Max: dims.Size}, cut, focus)
			return dims
		})
	}
	dims := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gap}.Layout(gtx, f.children...)
	content := record.Stop()

	height := gtx.Dp(unit.Dp(tokens.SizeFilterHeight))
	plateX, targetX := 0, 0
	for i := range f.plates {
		if i == f.Selected {
			targetX = plateX
		}
		offset := op.Offset(image.Pt(plateX, (dims.Size.Y-height)/2)).Push(gtx.Ops)
		f.plates[i].Add(gtx.Ops)
		offset.Pop()
		plateX += f.widths[i] + gap
	}
	f.x.Duration, f.width.Duration = th.Motion.Select, th.Motion.Select
	f.x.Ease, f.width.Ease = anim.EaseChisel, anim.EaseChisel
	f.x.Set(gtx, float32(targetX))
	f.width.Set(gtx, float32(f.widths[f.Selected]))
	x := int(f.x.Value(gtx) + .5)
	width := int(f.width.Value(gtx) + .5)
	selection := image.Rect(x, (dims.Size.Y-height)/2, x+width, (dims.Size.Y+height)/2)
	selectionCut := gtx.Dp(th.Cut.Small)
	FillCut(gtx.Ops, selection, selectionCut, disabledInk(gtx, th.R.Sel))
	PlaceBlade(gtx, th, selection, selectionCut, layout.Horizontal)
	content.Add(gtx.Ops)
	return dims
}

func (f *Filters) ensure(count int) {
	for len(f.chips) < count {
		f.chips = append(f.chips, filterChip{})
	}
	f.chips = f.chips[:count]
	if cap(f.widths) < count {
		f.widths = make([]int, count)
	} else {
		f.widths = f.widths[:count]
	}
	if cap(f.plates) < count {
		f.plates = make([]op.CallOp, count)
	} else {
		f.plates = f.plates[:count]
	}
	if cap(f.children) < count {
		f.children = make([]layout.FlexChild, count)
	} else {
		f.children = f.children[:count]
	}
}
