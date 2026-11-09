package kit

import (
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// EngravedChart is a role-only polyline over a clipped hatch field.
type EngravedChart struct {
	Values   []float64
	Min, Max float64
	Height   unit.Dp
	Ticks    []string
	Empty    string
}

func chartRange(values []float64, low, high float64) (float64, float64) {
	autoLow, autoHigh := math.IsNaN(low), math.IsNaN(high)
	if autoLow {
		low = math.Inf(1)
	}
	if autoHigh {
		high = math.Inf(-1)
	}
	for _, value := range values {
		if autoLow {
			low = min(low, value)
		}
		if autoHigh {
			high = max(high, value)
		}
	}
	if math.IsInf(low, 0) {
		low = 0
	}
	if math.IsInf(high, 0) {
		high = 1
	}
	if high <= low {
		padding := math.Max(math.Abs(low)*0.05, 1)
		low -= padding
		high += padding
	}
	return low, high
}

func chartPoints(values []float64, low, high float64, size image.Point) []f32.Point {
	low, high = chartRange(values, low, high)
	points := make([]f32.Point, 0, len(values))
	last := max(1, len(values)-1)
	for index, value := range values {
		x := float32(index) * float32(max(0, size.X-1)) / float32(last)
		normalized := (value - low) / (high - low)
		normalized = max(0, min(1, normalized))
		y := float32(max(0, size.Y-1)) * (1 - float32(normalized))
		points = append(points, f32.Pt(x, y))
	}
	return points
}

// Layout draws the chart without gradients, area fills, frames, or idle work.
func (chart EngravedChart) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	labelHeight := gtx.Sp(th.T.MonoSm.Line)
	tickHeight := max(1, gtx.Dp(tokens.SizeChartTick))
	requestedPlotHeight := max(1, gtx.Dp(chart.Height))
	extra := tickHeight + labelHeight
	size := gtx.Constraints.Constrain(image.Pt(gtx.Constraints.Max.X, requestedPlotHeight+extra))
	plotHeight := max(1, min(requestedPlotHeight, size.Y-extra))
	plot := image.Rect(0, 0, size.X, plotHeight)
	baselineY := plot.Max.Y - max(1, gtx.Dp(tokens.SizeChartBaseline))
	paint.FillShape(gtx.Ops, th.R.Vein, clip.Rect(image.Rect(0, baselineY, size.X, plot.Max.Y)).Op())

	points := chartPoints(chart.Values, chart.Min, chart.Max, plot.Size())
	if len(points) < 2 {
		message := chart.Empty
		if message == "" {
			message = "Not enough data yet."
		}
		child := gtx
		child.Constraints = layout.Exact(image.Pt(size.X, plotHeight))
		layout.Center.Layout(child, func(gtx layout.Context) layout.Dimensions {
			return Label{Style: th.T.Sentence, Color: th.R.Fg3, MaxLines: 2, Align: text.Middle}.Layout(gtx, th, message)
		})
		return layout.Dimensions{Size: size}
	}

	var area clip.Path
	area.Begin(gtx.Ops)
	area.MoveTo(points[0])
	for _, point := range points[1:] {
		area.LineTo(point)
	}
	area.LineTo(f32.Pt(points[len(points)-1].X, float32(baselineY)))
	area.LineTo(f32.Pt(points[0].X, float32(baselineY)))
	area.Close()
	Hatch(gtx.Ops, area.End(), plot, max(1, gtx.Dp(tokens.SizeChartHatchGap)), max(1, gtx.Dp(tokens.SizeChartHatchLine)), palette.WithAlpha(th.R.Fg, tokens.AlphaHatch))

	var line clip.Path
	line.Begin(gtx.Ops)
	line.MoveTo(points[0])
	for _, point := range points[1:] {
		line.LineTo(point)
	}
	paint.FillShape(gtx.Ops, th.R.Fg, clip.Stroke{Path: line.End(), Width: float32(max(1, gtx.Dp(tokens.SizeChartLine)))}.Op())

	for index, label := range chart.Ticks {
		if label == "" {
			continue
		}
		x := 0
		if len(chart.Ticks) > 1 {
			x = index * max(0, size.X-1) / (len(chart.Ticks) - 1)
		}
		tickWidth := max(1, gtx.Dp(tokens.SizeChartBaseline))
		paint.FillShape(gtx.Ops, th.R.Vein, clip.Rect(image.Rect(x, plotHeight, x+tickWidth, plotHeight+tickHeight)).Op())
		labelContext := gtx
		labelContext.Constraints.Min = image.Point{}
		labelContext.Constraints.Max = image.Pt(size.X, labelHeight)
		record := op.Record(gtx.Ops)
		labelDimensions := Label{Style: th.T.MonoSm, Color: th.R.Fg3, MaxLines: 1}.Layout(labelContext, th, label)
		call := record.Stop()
		labelX := max(0, min(size.X-labelDimensions.Size.X, x-labelDimensions.Size.X/2))
		offset := op.Offset(image.Pt(labelX, plotHeight+tickHeight)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
	}
	return layout.Dimensions{Size: size}
}
