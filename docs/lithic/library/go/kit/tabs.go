package kit

import (
	"image"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Tabs is a bounded horizontal resource-kind selector.
type Tabs struct {
	Labels   []string
	Selected int
	Preview  State
	tabs     []Click
	hover    []anim.Tween
	focus    []anim.Tween
	widths   []int
	x        anim.Tween
	width    anim.Tween
	changed  int
	pending  bool
}

// Changed reports and clears the selected-index change latch.
func (t *Tabs) Changed() (int, bool) {
	if !t.pending {
		return 0, false
	}
	changed := t.changed
	t.pending = false
	return changed, true
}

// Layout draws and operates the tabs.
func (t *Tabs) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	count := min(len(t.Labels), int(tokens.SizeTabsMaxCount))
	t.ensure(count)
	if count == 0 {
		return layout.Dimensions{}
	}
	t.Selected = min(max(t.Selected, 0), count-1)
	for i := range count {
		if t.tabs[i].Clicked(gtx) && i != t.Selected {
			t.Selected = i
			t.changed, t.pending = i, true
		}
		for {
			event, ok := gtx.Event(
				key.Filter{Focus: &t.tabs[i].Clickable, Name: key.NameLeftArrow},
				key.Filter{Focus: &t.tabs[i].Clickable, Name: key.NameRightArrow},
			)
			if !ok {
				break
			}
			ke, ok := event.(key.Event)
			if !ok || ke.State != key.Press || !gtx.Source.Enabled() {
				continue
			}
			next := i
			if ke.Name == key.NameLeftArrow {
				next = max(0, i-1)
			} else {
				next = min(count-1, i+1)
			}
			if next != i {
				t.Selected = next
				t.changed, t.pending = next, true
				gtx.Execute(key.FocusCmd{Tag: &t.tabs[next].Clickable})
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
	previewIndex := 0
	if t.Selected == 0 && count > 1 {
		previewIndex = 1
	}

	height := gtx.Dp(unit.Dp(tokens.SizeTabsHeight))
	total := 0
	for i, label := range t.Labels[:count] {
		measureGTX := gtx
		measureGTX.Constraints.Min = image.Point{}
		record := op.Record(gtx.Ops)
		dims := Label{Style: th.T.Label, Color: th.R.Fg, MaxLines: 1}.Layout(measureGTX, th, label)
		record.Stop()
		t.widths[i] = dims.Size.X + gtx.Dp(unit.Dp(tokens.SizeTabsLabelExtra))
		total += t.widths[i]
	}
	targetX := 0
	for i := range t.Selected {
		targetX += t.widths[i]
	}
	t.x.Duration, t.width.Duration = th.Motion.Select, th.Motion.Select
	t.x.Ease, t.width.Ease = anim.EaseChisel, anim.EaseChisel
	t.x.Set(gtx, float32(targetX))
	t.width.Set(gtx, float32(t.widths[t.Selected]))
	indicatorX := int(t.x.Value(gtx) + .5)
	indicatorWidth := int(t.width.Value(gtx) + .5)

	baseline := max(1, gtx.Dp(unit.Dp(tokens.SizeTabsBaseline)))
	FillCut(gtx.Ops, image.Rect(0, height-baseline, total, height), 0, disabledInk(gtx, th.R.Vein))
	bladeHeight := max(1, gtx.Dp(unit.Dp(tokens.SizeTabsBlade)))
	bladeBounds := image.Rect(indicatorX, height-bladeHeight, indicatorX+indicatorWidth, height)
	points := tabBladePoints(bladeBounds)
	paint.FillShape(gtx.Ops, disabledInk(gtx, th.R.Fg), clip.Outline{Path: pathFromPoints(gtx.Ops, points[:])}.Op())

	x := 0
	for i, label := range t.Labels[:count] {
		width := t.widths[i]
		offset := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
		itemPreview := StateRest
		if i == previewIndex {
			itemPreview = t.Preview
		}
		dims := t.tabs[i].Layout(exactContext(gtx, image.Pt(width, height)), func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(label).Add(gtx.Ops)
			semantic.SelectedOp(i == t.Selected).Add(gtx.Ops)
			semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
			hover := previewTransition(gtx, &t.hover[i], itemPreview, StateHover, t.tabs[i].Hovered() && i != t.Selected, th.Motion.HoverIn, th.Motion.HoverOut)
			ink := th.R.Fg3
			if i == t.Selected {
				ink = th.R.Fg
			} else {
				ink = palette.Mix(th.R.Fg3, th.R.Fg2, hover)
			}
			if itemPreview == StatePressed || t.tabs[i].Pressed() {
				FillCut(gtx.Ops, image.Rect(0, 0, width, height), 0, disabledInk(gtx, palette.WithAlpha(th.R.Fg, tokens.AlphaPressRow)))
			}
			return layout.Center.Layout(gtx, rawText(th, th.T.Label, disabledInk(gtx, ink), label))
		})
		focus := previewTransition(gtx, &t.focus[i], itemPreview, StateFocus, gtx.Focused(&t.tabs[i].Clickable), th.Motion.FocusIn, th.Motion.FocusOut)
		FocusRingAlpha(gtx, th, image.Rectangle{Max: dims.Size}, 0, focus)
		offset.Pop()
		x += width
	}
	return layout.Dimensions{Size: constrainSize(gtx, image.Pt(total, height))}
}

func (t *Tabs) ensure(count int) {
	for len(t.tabs) < count {
		t.tabs = append(t.tabs, Click{})
		t.hover = append(t.hover, anim.Tween{})
		t.focus = append(t.focus, anim.Tween{})
	}
	t.tabs = t.tabs[:count]
	t.hover = t.hover[:count]
	t.focus = t.focus[:count]
	if cap(t.widths) < count {
		t.widths = make([]int, count)
	} else {
		t.widths = t.widths[:count]
	}
}

func tabBladePoints(bounds image.Rectangle) [4]f32.Point {
	cut := min(max(0, bounds.Dy()), max(0, bounds.Dx()/2))
	return [4]f32.Point{
		f32.Pt(float32(bounds.Min.X+cut), float32(bounds.Min.Y)),
		f32.Pt(float32(bounds.Max.X), float32(bounds.Min.Y)),
		f32.Pt(float32(bounds.Max.X-cut), float32(bounds.Max.Y)),
		f32.Pt(float32(bounds.Min.X), float32(bounds.Max.Y)),
	}
}
