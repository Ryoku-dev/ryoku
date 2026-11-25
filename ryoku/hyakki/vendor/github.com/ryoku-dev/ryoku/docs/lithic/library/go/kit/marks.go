package kit

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

// Mark is one ordered resource trait.
type Mark struct {
	Text      string
	Attention bool
}

// ResourceMarks returns the canonical resource traits in their survival order.
func ResourceMarks(disposable, instant, sealed bool) []Mark {
	marks := make([]Mark, 0, 3)
	if disposable {
		marks = append(marks, Mark{Text: "Disposable", Attention: true})
	}
	if instant {
		marks = append(marks, Mark{Text: "Instant"})
	}
	if sealed {
		marks = append(marks, Mark{Text: "Sealed"})
	}
	return marks
}

// Marks composes resource traits in the detail type, attention first and
// amber, set apart by space rather than a separator glyph.
func Marks(th *theme.Theme, marks []Mark) layout.Widget {
	spans := make([]Span, 0, len(marks)*2)
	for _, mark := range marks {
		if mark.Text == "" {
			continue
		}
		if len(spans) > 0 {
			spans = append(spans, Span{Text: markGap})
		}
		span := Span{Text: mark.Text}
		if mark.Attention {
			span.Color = th.R.Amber
		}
		spans = append(spans, span)
	}
	return func(gtx layout.Context) layout.Dimensions {
		return Sentence(gtx, th, th.T.Detail, th.R.Fg2, spans)
	}
}

// markGap is the space between traits: about one 13 dp step at detail size.
const markGap = "    "

type markedFactRun struct {
	text  string
	style theme.TextStyle
	ink   color.NRGBA
}

// MarkedFacts composes facts and marks, dropping whole trailing marks before truncating facts.
func MarkedFacts(th *theme.Theme, facts string, factsMono bool, marks []Mark, ink color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		style := th.T.Caption
		if factsMono {
			style = th.T.Mono
		}
		runs := make([]markedFactRun, 0, len(marks)+1)
		if facts != "" {
			runs = append(runs, markedFactRun{text: facts, style: style, ink: ink})
		}
		for _, mark := range marks {
			if mark.Text == "" {
				continue
			}
			prefix := ""
			if len(runs) > 0 {
				prefix = markGap
			}
			markInk := th.R.Fg3
			if mark.Attention {
				markInk = th.R.Amber
			}
			runs = append(runs, markedFactRun{text: prefix + mark.Text, style: th.T.Caption, ink: markInk})
		}
		return layoutMarkedFactRuns(gtx, th, runs)
	}
}

func layoutMarkedFactRuns(gtx layout.Context, th *theme.Theme, runs []markedFactRun) layout.Dimensions {
	if len(runs) == 0 {
		return layout.Dimensions{}
	}
	widths := make([]int, len(runs))
	total := 0
	for index := range runs {
		widths[index] = measureMarkedFactRun(gtx, th, runs[index])
		total += widths[index]
	}
	maximum := gtx.Constraints.Max.X
	if total > maximum {
		ellipsis := markedFactRun{text: "…", style: runs[len(runs)-1].style, ink: th.R.Fg3}
		ellipsisWidth := measureMarkedFactRun(gtx, th, ellipsis)
		for len(runs) > 1 && total+ellipsisWidth > maximum {
			last := len(runs) - 1
			total -= widths[last]
			runs, widths = runs[:last], widths[:last]
		}
		if total+ellipsisWidth > maximum {
			last := len(runs) - 1
			available := max(0, maximum-(total-widths[last])-ellipsisWidth)
			runs[last].text = fitMarkedFactRun(gtx, th, runs[last], available)
		}
		runs = append(runs, ellipsis)
	}
	children := make([]layout.FlexChild, 0, len(runs))
	for _, run := range runs {
		run := run
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return Label{Style: run.style, Color: run.ink, MaxLines: 1}.Layout(gtx, th, run.text)
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx, children...)
}

func fitMarkedFactRun(gtx layout.Context, th *theme.Theme, run markedFactRun, width int) string {
	runes := []rune(run.text)
	low, high := 0, len(runes)
	for low < high {
		middle := (low + high + 1) / 2
		candidate := run
		candidate.text = string(runes[:middle])
		if measureMarkedFactRun(gtx, th, candidate) <= width {
			low = middle
		} else {
			high = middle - 1
		}
	}
	return string(runes[:low])
}

func measureMarkedFactRun(gtx layout.Context, th *theme.Theme, run markedFactRun) int {
	measure := gtx
	measure.Constraints.Min = image.Point{}
	measure.Constraints.Max.X = 1 << 20
	record := op.Record(gtx.Ops)
	dimensions := Label{Style: run.style, Color: run.ink, MaxLines: 1}.Layout(measure, th, run.text)
	record.Stop()
	return dimensions.Size.X
}
