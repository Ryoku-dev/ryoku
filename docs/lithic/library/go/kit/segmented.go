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
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// segmentedMaxOptions is the most options one segmented control offers.
const segmentedMaxOptions = 5

type segmentState struct {
	Click
	hover, focus anim.Tween
}

// Segmented is a single choice among two to five equal-width options on a
// field track. The selection glides between options and carries the place
// blade along its bottom.
type Segmented struct {
	Labels   []string
	Selected int
	Preview  State
	segments []segmentState
	position anim.Tween
	changed  int
	pending  bool
}

// Changed reports and clears the selected-index change latch.
func (s *Segmented) Changed() (int, bool) {
	if !s.pending {
		return 0, false
	}
	s.pending = false
	return s.changed, true
}

// Layout draws and operates the segmented control. Its natural width fits the
// widest label in every segment; a caller widens it through Constraints.Min.X.
func (s *Segmented) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	count := min(len(s.Labels), segmentedMaxOptions)
	s.ensure(count)
	if count == 0 {
		return layout.Dimensions{}
	}
	s.Selected = min(max(s.Selected, 0), count-1)
	for i := range count {
		if s.segments[i].Clicked(gtx) {
			s.choose(i)
		}
		for {
			event, ok := gtx.Event(
				key.Filter{Focus: &s.segments[i].Clickable, Name: key.NameLeftArrow},
				key.Filter{Focus: &s.segments[i].Clickable, Name: key.NameRightArrow},
				key.Filter{Focus: &s.segments[i].Clickable, Name: key.NameUpArrow},
				key.Filter{Focus: &s.segments[i].Clickable, Name: key.NameDownArrow},
				key.Filter{Focus: &s.segments[i].Clickable, Name: key.NameHome},
				key.Filter{Focus: &s.segments[i].Clickable, Name: key.NameEnd},
			)
			if !ok {
				break
			}
			ke, ok := event.(key.Event)
			if !ok || ke.State != key.Press || !gtx.Source.Enabled() {
				continue
			}
			next := i
			switch ke.Name {
			case key.NameLeftArrow, key.NameUpArrow:
				next = max(0, i-1)
			case key.NameRightArrow, key.NameDownArrow:
				next = min(count-1, i+1)
			case key.NameHome:
				next = 0
			case key.NameEnd:
				next = count - 1
			}
			if next != i {
				s.choose(next)
				gtx.Execute(key.FocusCmd{Tag: &s.segments[next].Clickable})
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}
	previewIndex := 0
	if s.Selected == 0 && count > 1 {
		previewIndex = 1
	}

	inset := gtx.Dp(unit.Dp(tokens.SizeSegmentInset))
	height := gtx.Dp(unit.Dp(tokens.SizeSegmentHeight))
	widest := 0
	for _, label := range s.Labels[:count] {
		measure := gtx
		measure.Constraints.Min = image.Point{}
		record := op.Record(gtx.Ops)
		dims := Label{Style: th.T.Label, Color: th.R.Fg2, MaxLines: 1}.Layout(measure, th, label)
		record.Stop()
		widest = max(widest, dims.Size.X)
	}
	natural := count*(widest+2*gtx.Dp(th.Sp.S4)) + 2*inset
	size := gtx.Constraints.Constrain(image.Pt(max(natural, gtx.Constraints.Min.X), height))
	inner := max(0, size.X-2*inset)
	edge := func(index int) int { return inset + inner*index/count }

	track := image.Rectangle{Max: size}
	trackCut := gtx.Dp(th.Cut.Mid)
	FillCut(gtx.Ops, track, trackCut, disabledInk(gtx, th.R.Field))
	StrokeCut(gtx.Ops, track, trackCut, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), disabledInk(gtx, th.R.Vein))

	s.position.Duration, s.position.Ease = th.Motion.Select, anim.EaseChisel
	s.position.Set(gtx, float32(s.Selected))
	x := inset + int(float32(inner)*s.position.Value(gtx)/float32(count)+.5)
	segmentCut := gtx.Dp(th.Cut.Segment)
	selection := image.Rect(x, inset, x+inner/count, size.Y-inset)
	FillCut(gtx.Ops, selection, segmentCut, disabledInk(gtx, th.R.Sel))
	PlaceBlade(gtx, th, selection, segmentCut, layout.Horizontal)

	for i, label := range s.Labels[:count] {
		width := edge(i+1) - edge(i)
		face := image.Rect(0, inset, width, size.Y-inset)
		itemPreview := StateRest
		if i == previewIndex {
			itemPreview = s.Preview
		}
		offset := op.Offset(image.Pt(edge(i), 0)).Push(gtx.Ops)
		s.segments[i].Layout(exactContext(gtx, image.Pt(width, size.Y)), func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(label).Add(gtx.Ops)
			semantic.SelectedOp(i == s.Selected).Add(gtx.Ops)
			semantic.EnabledOp(gtx.Source.Enabled()).Add(gtx.Ops)
			hover := previewTransition(gtx, &s.segments[i].hover, itemPreview, StateHover, s.segments[i].Hovered() && i != s.Selected, th.Motion.HoverIn, th.Motion.HoverOut)
			ink := th.R.Fg2
			if i == s.Selected {
				ink = th.R.OnSel
			} else if hover > 0 {
				FillCut(gtx.Ops, face, segmentCut, disabledInk(gtx, palette.WithAlpha(th.R.Fg2, tokens.AlphaHoverFill*min(hover, 1))))
			}
			labelOffset := op.Offset(face.Min).Push(gtx.Ops)
			layout.Center.Layout(exactContext(gtx, face.Size()), rawText(th, th.T.Label, disabledInk(gtx, ink), label))
			labelOffset.Pop()
			return layout.Dimensions{Size: gtx.Constraints.Min}
		})
		focus := previewTransition(gtx, &s.segments[i].focus, itemPreview, StateFocus, gtx.Focused(&s.segments[i].Clickable), th.Motion.FocusIn, th.Motion.FocusOut)
		FocusRingAlpha(gtx, th, face, segmentCut, focus)
		offset.Pop()
	}
	return layout.Dimensions{Size: size}
}

func (s *Segmented) choose(index int) {
	if index == s.Selected {
		return
	}
	s.Selected = index
	s.changed, s.pending = index, true
}

func (s *Segmented) ensure(count int) {
	for len(s.segments) < count {
		s.segments = append(s.segments, segmentState{})
	}
	s.segments = s.segments[:count]
}
