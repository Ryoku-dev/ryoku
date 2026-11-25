package look

import (
	"math"
	"strconv"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
	"ryoku-i18n"
)

// Trend is a chart section that sits open on its pane: a title, a proof line, and an engraved chart.
type Trend struct {
	Title  string    // section title; empty draws no title (a tile's sparkline)
	Proof  string    // one sentence in words saying what the line shows
	Values []float64 // one value per day, oldest first
	Ticks  []string  // tick labels, from DayTicks
	Empty  string    // the sentence shown when there are fewer than two values
	Height unit.Dp   // 0 means 120; tiles use 48
}

func (t Trend) Layout(gtx layout.Context, th *Theme) layout.Dimensions {
	height := t.Height
	if height == 0 {
		height = 120
	}
	empty := t.Empty
	if empty == "" {
		empty = i18n.T("A line shows from the second day.")
	}
	chart := kit.EngravedChart{Values: t.Values, Min: 0, Max: math.NaN(), Height: height, Ticks: t.Ticks, Empty: empty}
	// with nothing above zero the chart would centre the flat line; it belongs on the baseline
	top := 0.0
	for _, value := range t.Values {
		top = max(top, value)
	}
	if top == 0 {
		chart.Max = 1
	}
	children := make([]layout.FlexChild, 0, 6)
	if t.Title != "" {
		children = append(children,
			layout.Rigid(kit.Text(th.Theme, th.T.Section, th.R.Fg, t.Title)),
			layout.Rigid(layout.Spacer{Height: tokens.SizeSlabTitleGap}.Layout),
		)
	}
	if t.Proof != "" {
		children = append(children,
			layout.Rigid(Paragraph(th, th.T.Detail, th.R.Fg2, t.Proof)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		)
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return chart.Layout(gtx, th.Theme) }))
	// a titled trend is a section and keeps a section's gap under it
	if t.Title != "" {
		children = append(children, layout.Rigid(layout.Spacer{Height: tokens.SizeSlabBelow}.Layout))
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// HoldUntil lays widget out, drawing it only once ready, so a line not read yet keeps the place it will take and nothing under it moves when it arrives.
func HoldUntil(gtx layout.Context, ready bool, widget layout.Widget) layout.Dimensions {
	if ready {
		return widget(gtx)
	}
	record := op.Record(gtx.Ops)
	dimensions := widget(gtx)
	record.Stop()
	return dimensions
}

// MonthDays is the days of month ("2006-01", empty for now's) in "2006-01-02" form, from its first to today, or to its end for a month gone by.
func MonthDays(now time.Time, month string) []string {
	now = now.Local()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	if parsed, err := time.ParseInLocation("2006-01", month, time.Local); err == nil {
		first = parsed
	}
	count := first.AddDate(0, 1, -1).Day()
	if first.Year() == now.Year() && first.Month() == now.Month() {
		count = now.Day()
	}
	days := make([]string, count)
	for index := range days {
		days[index] = first.AddDate(0, 0, index).Format("2006-01-02")
	}
	return days
}

// LastDays is the count days ending on end's day, in end's location, oldest first, in "2006-01-02" form.
func LastDays(end time.Time, count int) []string {
	days := make([]string, max(0, count))
	for index := range days {
		days[index] = end.AddDate(0, 0, index-(count-1)).Format("2006-01-02")
	}
	return days
}

// DayValues is one value per day of days, read from byDay; a day it lacks is zero.
func DayValues(days []string, byDay map[string]float64) []float64 {
	values := make([]float64, len(days))
	for index, day := range days {
		values[index] = byDay[day]
	}
	return values
}

// Peak is the day of days with the highest value and that value; the earliest wins a tie, and "" means no day is above zero.
func Peak(days []string, values []float64) (string, float64) {
	day, peak := "", 0.0
	for index, value := range values {
		if index < len(days) && value > peak {
			day, peak = days[index], value
		}
	}
	return day, peak
}

// DayName says a "2006-01-02" day as a proof does: "12 Nov".
func DayName(day string) string {
	parsed, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	return parsed.Format("2 Jan")
}

// DayTicks labels a run of days ("2026-11-01" form) with the first, middle and last day of the month numbers.
func DayTicks(days []string) []string {
	ticks := make([]string, len(days))
	if len(days) == 0 {
		return ticks
	}
	for _, index := range []int{0, (len(days) - 1) / 2, len(days) - 1} {
		if day, err := time.Parse("2006-01-02", days[index]); err == nil {
			ticks[index] = strconv.Itoa(day.Day())
		}
	}
	return ticks
}
