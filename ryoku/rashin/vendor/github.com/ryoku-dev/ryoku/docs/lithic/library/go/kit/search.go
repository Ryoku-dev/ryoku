package kit

import (
	"image"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

var (
	iconSearchControl = Icon(symbols.Search)
	iconClearSearch   = Icon(symbols.Close)
)

// Search is a single-line search editor with hint and clear action.
type Search struct {
	widget.Editor
	Placeholder string
	Hint        string
	Preview     State
	clear       IconPlaque
	focus       anim.Tween
}

// Focus moves keyboard focus to the search editor.
func (s *Search) Focus(gtx layout.Context) {
	gtx.Execute(key.FocusCmd{Tag: &s.Editor})
}

// Layout draws and operates the search field.
func (s *Search) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	s.SingleLine = true
	for {
		event, ok := gtx.Event(key.Filter{Focus: &s.Editor, Name: key.NameEscape})
		if !ok {
			break
		}
		if event, ok := event.(key.Event); ok && event.State == key.Press {
			s.SetText("")
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	if s.clear.Clicked(gtx) {
		s.SetText("")
		s.Focus(gtx)
	}

	width := gtx.Constraints.Max.X
	if width <= 0 {
		width = gtx.Dp(unit.Dp(tokens.SizeSearchWidth))
	}
	height := gtx.Dp(unit.Dp(tokens.SizeSearchHeight))
	size := gtx.Constraints.Constrain(image.Pt(width, height))
	bounds := image.Rectangle{Max: size}
	cut := gtx.Dp(th.Cut.Mid)
	focus := previewTransition(gtx, &s.focus, s.Preview, StateFocus, gtx.Focused(&s.Editor), th.Motion.FocusIn, th.Motion.FocusOut)
	drawFieldSurface(gtx, th, bounds, cut, th.R.Field, focus, false)

	iconSize := gtx.Dp(unit.Dp(tokens.SizeSearchIcon))
	offset := op.Offset(image.Pt(gtx.Dp(unit.Dp(tokens.SizeSearchIconInset)), (size.Y-iconSize)/2)).Push(gtx.Ops)
	iconSearchControl.Layout(exactContext(gtx, image.Pt(iconSize, iconSize)), disabledInk(gtx, th.R.Fg3))
	offset.Pop()

	right := gtx.Dp(unit.Dp(tokens.SizeTextFieldInset))
	if s.Len() > 0 {
		buttonSize := gtx.Dp(unit.Dp(tokens.SizeSearchClear))
		right = buttonSize + gtx.Dp(unit.Dp(tokens.SizeSearchClearInset+tokens.SpaceS1))
		s.clear.Small = true
		s.clear.Kind = PlaqueQuiet
		s.clear.Icon = iconClearSearch
		s.clear.Description = "Clear search"
		offset = op.Offset(image.Pt(size.X-buttonSize-gtx.Dp(unit.Dp(tokens.SizeSearchClearInset)), (size.Y-buttonSize)/2)).Push(gtx.Ops)
		s.clear.Layout(exactContext(gtx, image.Pt(buttonSize, buttonSize)), th)
		offset.Pop()
	} else if s.Hint != "" && !gtx.Focused(&s.Editor) {
		measureGTX := gtx
		measureGTX.Constraints.Min = image.Point{}
		record := op.Record(gtx.Ops)
		hintDims := rawText(th, th.T.MonoSm, disabledInk(measureGTX, th.R.Fg3), s.Hint)(measureGTX)
		call := record.Stop()
		right = hintDims.Size.X + gtx.Dp(unit.Dp(tokens.SizePlaqueInset))
		offset = op.Offset(image.Pt(size.X-right+gtx.Dp(th.Sp.S1), (size.Y-hintDims.Size.Y)/2)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
	}

	innerWidth := max(0, size.X-gtx.Dp(unit.Dp(tokens.SizeSearchLeading))-right)
	editorHeight := max(0, size.Y-2*gtx.Dp(th.Sp.S2))
	editorGTX := exactContext(gtx, image.Pt(innerWidth, editorHeight))
	offset = op.Offset(image.Pt(gtx.Dp(unit.Dp(tokens.SizeSearchLeading)), (size.Y-editorHeight)/2)).Push(gtx.Ops)
	semantic.Editor.Add(gtx.Ops)
	semantic.LabelOp("Search").Add(gtx.Ops)
	layoutTextEditor(editorGTX, th, &s.Editor, th.T.Body, s.Placeholder)
	offset.Pop()
	return layout.Dimensions{Size: size}
}
