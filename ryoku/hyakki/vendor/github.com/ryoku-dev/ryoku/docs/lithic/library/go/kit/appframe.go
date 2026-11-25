package kit

import (
	"image"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// FrameClass is the Lithic window size class.
type FrameClass uint8

const (
	FrameCompact FrameClass = iota
	FrameMedium
	FrameExpanded
)

// AppFrameContent supplies the compositor-agnostic regions of an application window.
type AppFrameContent struct {
	Rail, List, Work         layout.Widget
	Overlays, WindowControls layout.Widget
	DismissOverlay           func() bool
}

// AppFrame owns ground, size class, pane placement, deferred overlays, and Escape routing.
// HideCompactBar leaves the compact class without a bottom rail; the host
// then navigates on its own.
type AppFrame struct {
	Class          FrameClass
	Preview        State
	HideCompactBar bool
}

// ClassForWidth resolves the canonical breakpoint for a width in dp.
func ClassForWidth(width unit.Dp) FrameClass {
	switch {
	case width >= unit.Dp(tokens.BreakpointExpanded):
		return FrameExpanded
	case width >= unit.Dp(tokens.BreakpointMedium):
		return FrameMedium
	default:
		return FrameCompact
	}
}

// Layout composes the frame and returns the selected size class on AppFrame.Class.
func (frame *AppFrame) Layout(gtx layout.Context, th *theme.Theme, content AppFrameContent) layout.Dimensions {
	window := gtx.Constraints.Max
	frame.Class = ClassForWidth(gtx.Metric.PxToDp(window.X))
	paint.FillShape(gtx.Ops, th.R.Ground, clip.Rect{Max: window}.Op())
	if frame.Class == FrameCompact {
		frame.layoutCompact(gtx, th, content)
	} else {
		frame.layoutWide(gtx, th, content)
	}
	if content.WindowControls != nil || content.Overlays != nil {
		record := op.Record(gtx.Ops)
		if content.WindowControls != nil {
			child := overlayContentContext(gtx)
			child.Constraints.Min = image.Point{}
			content.WindowControls(child)
		}
		if content.Overlays != nil {
			child := overlayContentContext(gtx)
			child.Constraints = layout.Exact(window)
			content.Overlays(child)
		}
		presentOverlay(gtx, record.Stop(), frame.Preview != StateRest, window)
	}
	// Escape is read last, so the focused content (a menu, a Search with text,
	// a dialog) takes it first and DismissOverlay hears only what is left
	if content.DismissOverlay != nil {
		for {
			value, ok := gtx.Event(key.Filter{Name: key.NameEscape})
			if !ok {
				break
			}
			if keyEvent, ok := value.(key.Event); ok && keyEvent.State == key.Press {
				if content.DismissOverlay() {
					gtx.Execute(op.InvalidateCmd{})
				}
			}
		}
	}
	return layout.Dimensions{Size: window}
}

func (frame *AppFrame) layoutWide(gtx layout.Context, th *theme.Theme, content AppFrameContent) {
	window := gtx.Constraints.Max
	railWidth := gtx.Dp(tokens.SizeRailMedium)
	listWidth := gtx.Dp(tokens.SizeListMedium)
	if frame.Class == FrameExpanded {
		railWidth = gtx.Dp(tokens.SizeRailExpanded)
		listWidth = gtx.Dp(tokens.SizeListExpanded)
	}
	gap := gtx.Dp(tokens.SizePaneGap)
	top := gtx.Dp(tokens.SizePaneTop)
	paneHeight := max(0, window.Y-top)
	if content.Rail != nil {
		layoutAt(gtx, image.Point{}, image.Pt(railWidth, window.Y), content.Rail)
	}
	contentX := railWidth + gap
	if content.List == nil {
		if content.Work != nil {
			layoutAt(gtx, image.Pt(contentX, top), image.Pt(max(0, window.X-contentX), paneHeight), frame.pane(th, true, content.Work))
		}
		return
	}
	layoutAt(gtx, image.Pt(contentX, top), image.Pt(min(listWidth, max(0, window.X-contentX)), paneHeight), frame.pane(th, false, content.List))
	workX := contentX + listWidth + gap
	if content.Work != nil {
		layoutAt(gtx, image.Pt(workX, top), image.Pt(max(0, window.X-workX), paneHeight), frame.pane(th, true, content.Work))
	}
}

func (frame *AppFrame) layoutCompact(gtx layout.Context, th *theme.Theme, content AppFrameContent) {
	window := gtx.Constraints.Max
	bottomHeight := gtx.Dp(tokens.SizeRailBottomHeight)
	gap := gtx.Dp(tokens.SizePaneGap)
	top := gtx.Dp(tokens.SizePaneTop)
	bar := !frame.HideCompactBar
	paneHeight := max(0, window.Y-top)
	if bar {
		paneHeight = max(0, window.Y-bottomHeight-gap-top)
	}
	body, work := content.List, false
	if content.Work != nil {
		body, work = content.Work, true
	}
	if body != nil {
		layoutAt(gtx, image.Pt(0, top), image.Pt(window.X, paneHeight), frame.pane(th, work, body))
	}
	if bar && content.Rail != nil {
		layoutAt(gtx, image.Pt(0, max(0, window.Y-bottomHeight)), image.Pt(window.X, bottomHeight), content.Rail)
	}
}

func (frame *AppFrame) pane(th *theme.Theme, work bool, body layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		kind := PaneList
		if work {
			kind = PaneWork
		}
		return (Pane{Kind: kind}).Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
			child := gtx
			child.Constraints = layout.Exact(size)
			body(child)
			return layout.Dimensions{Size: size}
		})
	}
}
