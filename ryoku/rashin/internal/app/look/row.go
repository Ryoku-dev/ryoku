package look

import (
	"image"
	"time"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// SummaryRow is a dense resource row with a prioritised title, state, and
// mono metadata. Excerpt adds one optional third line for current activity.
type SummaryRow struct {
	kit.Click
	Title   string
	State   string
	Meta    string
	Excerpt string
	Tone    palette.Tone

	eventAt               time.Time
	eventReady            bool
	clicked, double       bool
	clickedOut, doubleOut bool
}

func (row *SummaryRow) poll(gtx layout.Context) {
	if row.eventReady && row.eventAt == gtx.Now {
		return
	}
	row.eventAt, row.eventReady = gtx.Now, true
	row.clicked, row.double = false, false
	row.clickedOut, row.doubleOut = false, false
	for {
		click, ok := row.Click.Update(gtx)
		if !ok {
			break
		}
		row.clicked = true
		if click.NumClicks >= 2 {
			row.double = true
		}
	}
}

// Clicked reports a primary selection without consuming double activation.
func (row *SummaryRow) Clicked(gtx layout.Context) bool {
	row.poll(gtx)
	if row.clicked && !row.clickedOut {
		row.clickedOut = true
		return true
	}
	return false
}

// DoubleClicked reports a primary double activation.
func (row *SummaryRow) DoubleClicked(gtx layout.Context) bool {
	row.poll(gtx)
	if row.double && !row.doubleOut {
		row.doubleOut = true
		return true
	}
	return false
}

// Layout draws the summary row. The title owns remaining first-line width;
// state and metadata are bounded and elide instead of collapsing the title.
func (row *SummaryRow) Layout(gtx layout.Context, th *Theme, selected bool) layout.Dimensions {
	row.poll(gtx)
	width := gtx.Constraints.Max.X
	height := gtx.Dp(56)
	if row.Excerpt != "" {
		height = gtx.Dp(76)
	}
	rowGTX := gtx
	rowGTX.Constraints = layout.Exact(image.Pt(width, height))
	return row.Click.Layout(rowGTX, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(row.Title).Add(gtx.Ops)
		fill := th.R.Stone2
		if row.Hovered() {
			fill = palette.Mix(fill, th.R.Fg, tokens.AlphaHoverPlane)
		}
		titleInk, quietInk := th.R.Fg, th.R.Fg3
		stateInk := th.R.Stone(row.Tone)
		if row.Tone == palette.Idle {
			stateInk = th.R.Fg2
		}
		if selected {
			fill, titleInk, quietInk, stateInk = th.R.Sel, th.R.OnSel, th.R.OnSel2, th.R.OnSel
		}
		kit.FillCut(gtx.Ops, image.Rect(0, 0, width, height), 0, fill)
		if row.Tone != palette.Idle {
			kit.Notch(gtx.Ops, image.Point{}, gtx.Dp(th.Cut.Notch), stateInk)
		}

		body := func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return kit.Label{Style: th.T.Row, Color: titleInk, MaxLines: 1}.Layout(gtx, th.Theme, row.Title)
						}),
						layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(104))
							return kit.Label{Style: th.T.Label, Color: stateInk, MaxLines: 1, Align: text.End}.Layout(gtx, th.Theme, row.State)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return kit.Label{Style: th.T.MonoSm, Color: quietInk, MaxLines: 1}.Layout(gtx, th.Theme, row.Meta)
				}),
			}
			if row.Excerpt != "" {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return kit.Label{Style: th.T.Caption, Color: quietInk, MaxLines: 1}.Layout(gtx, th.Theme, row.Excerpt)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}
		layout.Inset{Top: 7, Right: 13, Bottom: 7, Left: 13}.Layout(gtx, body)
		return layout.Dimensions{Size: image.Pt(width, height)}
	})
}
