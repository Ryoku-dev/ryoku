package look

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

type Class uint8

const (
	Compact Class = iota
	Medium
	Expanded
)

const (
	BreakpointMedium   float32 = 760
	BreakpointExpanded float32 = 1100
)

const (
	TopBar              unit.Dp = 56
	Margin              unit.Dp = 8
	Gap                 unit.Dp = 8
	SidebarExpanded     unit.Dp = 264
	SidebarRail         unit.Dp = 72
	Panel               unit.Dp = 440
	ContentMinWithPanel unit.Dp = 480
	PaneHeaderHeight    unit.Dp = 52
	ReadingColumnWidth  unit.Dp = 760
	BoardWidth          unit.Dp = 1240
	BottomNav           unit.Dp = 56
)

func ClassFor(widthDP float32) Class {
	switch {
	case widthDP >= BreakpointExpanded:
		return Expanded
	case widthDP >= BreakpointMedium:
		return Medium
	default:
		return Compact
	}
}

func ContentInset(class Class) unit.Dp {
	switch class {
	case Expanded:
		return 34
	case Medium:
		return 21
	default:
		return 13
	}
}

func ReadingColumn(gtx layout.Context, class Class, child layout.Widget) layout.Dimensions {
	inset := gtx.Dp(ContentInset(class))
	available := max(0, gtx.Constraints.Max.X-2*inset)
	width := min(available, gtx.Dp(ReadingColumnWidth))
	if width <= 0 {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	left := max(inset, (gtx.Constraints.Max.X-width)/2)
	right := max(inset, gtx.Constraints.Max.X-left-width)
	return layout.Inset{Left: unit.Dp(float32(left) / gtx.Metric.PxPerDp), Right: unit.Dp(float32(right) / gtx.Metric.PxPerDp)}.
		Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = min(gtx.Constraints.Min.X, width)
			gtx.Constraints.Max.X = width
			return child(gtx)
		})
}

// Centred stands child in the middle of the pane, at most width wide and
// inset from the sides by the size class.
func Centred(gtx layout.Context, class Class, width unit.Dp, child layout.Widget) layout.Dimensions {
	inset := gtx.Dp(ContentInset(class))
	inner := min(gtx.Constraints.Max.X-2*inset, gtx.Dp(width))
	if inner <= 0 {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	column := gtx
	column.Constraints = layout.Constraints{Min: image.Pt(inner, 0), Max: image.Pt(inner, gtx.Constraints.Max.Y)}
	offset := op.Offset(image.Pt((gtx.Constraints.Max.X-inner)/2, 0)).Push(gtx.Ops)
	dimensions := child(column)
	offset.Pop()
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, dimensions.Size.Y)}
}
