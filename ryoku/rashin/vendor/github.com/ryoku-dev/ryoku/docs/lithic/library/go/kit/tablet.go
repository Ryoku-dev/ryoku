package kit

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// TabletContent is the content and state shown by one resource tablet.
type TabletContent struct {
	Mark        layout.Widget
	Title       string
	Facts       string
	FactsWidget layout.Widget
	FactsMono   bool
	Claim       string
	Proof       string
	Tone        palette.Tone
	Progress    float64
	Action      *Plaque
}

// Tablet is an interactive resource row.
type Tablet struct {
	Click
	Pane    color.NRGBA
	Preview State

	hover, press, focus, notch anim.Tween
	progress                   anim.Spring
	tone                       palette.Tone
	toneReady                  bool
	eventAt                    time.Time
	eventReady                 bool
	clicked, double            bool
	clickedOut, doubleOut      bool
}

func (tablet *Tablet) poll(gtx layout.Context) {
	if tablet.eventReady && tablet.eventAt == gtx.Now {
		return
	}
	tablet.eventAt, tablet.eventReady = gtx.Now, true
	tablet.clicked, tablet.double = false, false
	tablet.clickedOut, tablet.doubleOut = false, false
	// Return selects and runs the primary action, like a double click; Space
	// only selects, through the click below
	for {
		value, ok := gtx.Event(
			key.Filter{Focus: &tablet.Clickable, Name: key.NameReturn},
			key.Filter{Focus: &tablet.Clickable, Name: key.NameEnter},
		)
		if !ok {
			break
		}
		if pressed, ok := value.(key.Event); ok && pressed.State == key.Press {
			tablet.clicked, tablet.double = true, true
			gtx.Execute(op.InvalidateCmd{})
		}
	}
	for {
		click, ok := tablet.Click.Update(gtx)
		if !ok {
			break
		}
		tablet.clicked = true
		if click.NumClicks >= 2 {
			tablet.double = true
		}
	}
}

// Clicked reports selection without consuming a double activation.
func (tablet *Tablet) Clicked(gtx layout.Context) bool {
	tablet.poll(gtx)
	if tablet.clicked && !tablet.clickedOut {
		tablet.clickedOut = true
		return true
	}
	return false
}

// DoubleClicked reports a primary double activation.
func (tablet *Tablet) DoubleClicked(gtx layout.Context) bool {
	tablet.poll(gtx)
	if tablet.double && !tablet.doubleOut {
		tablet.doubleOut = true
		return true
	}
	return false
}

// Layout draws the tablet and owns its pointer and keyboard interaction.
func (tablet *Tablet) Layout(gtx layout.Context, th *theme.Theme, content TabletContent, selected, listFocused bool) layout.Dimensions {
	tablet.poll(gtx)
	tablet.notch.Duration = th.Motion.Notch
	tablet.notch.Ease = anim.EaseChisel
	if !tablet.toneReady {
		tablet.tone, tablet.toneReady = content.Tone, true
		tablet.notch.Set(gtx, 1)
	} else if tablet.tone != content.Tone {
		tablet.tone = content.Tone
		tablet.notch = anim.Tween{Duration: th.Motion.Notch, Ease: anim.EaseChisel}
		tablet.notch.Set(gtx, 0)
		tablet.notch.Set(gtx, 1)
	}

	height := gtx.Dp(th.RowHeight())
	width := max(gtx.Constraints.Min.X, gtx.Constraints.Max.X)
	rowContext := gtx
	rowContext.Constraints = layout.Exact(image.Pt(width, height))
	dimensions := tablet.Click.Layout(rowContext, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(content.Title).Add(gtx.Ops)
		realHovered := tablet.Hovered() || listFocused
		realPressed := tablet.Pressed()
		realFocused := gtx.Focused(&tablet.Click.Clickable)
		hover := float32(1)
		if tablet.Preview != StateHover {
			hover = hoverValue(gtx, th, &tablet.hover, realHovered)
		}
		press := float32(1)
		if tablet.Preview != StatePressed {
			press = transitionValue(gtx, &tablet.press, realPressed, th.Motion.Press, th.Motion.Press)
		}
		focused := realFocused || tablet.Preview == StateFocus
		if selected {
			FillCut(gtx.Ops, image.Rect(0, 0, width, height), 0, th.R.Sel)
			if press > 0 {
				FillCut(gtx.Ops, image.Rect(0, 0, width, height), 0, palette.WithAlpha(th.R.OnSel, tokens.AlphaPressRow*press))
			}

		} else {
			fill := palette.Mix(th.R.Stone2, th.R.Stone3, hover)
			fill = palette.Mix(fill, th.R.Fg, tokens.AlphaPressRow*press)
			FillCut(gtx.Ops, image.Rect(0, 0, width, height), 0, fill)
		}

		notchInk := th.R.Stone(content.Tone)
		if content.Tone == palette.Idle {
			notchInk = tablet.Pane
			if notchInk == (color.NRGBA{}) {
				notchInk = th.R.Stone1
			}
		}
		notchSize := int(math.Round(float64(gtx.Dp(th.Cut.Notch)) * float64(tablet.notch.Value(gtx))))
		if notchSize > 0 {
			Notch(gtx.Ops, image.Point{}, notchSize, notchInk)
		}

		titleInk, quietInk := th.R.Fg, th.R.Fg3
		claimInk := th.R.Stone(content.Tone)
		if content.Tone == palette.Idle {
			claimInk = th.R.Fg2
		}
		if selected {
			titleInk, quietInk, claimInk = th.R.OnSel, th.R.OnSel2, th.R.OnSel
		}
		body := func(gtx layout.Context) layout.Dimensions {
			mark := func(gtx layout.Context) layout.Dimensions {
				size := gtx.Dp(tokens.SizeTabletMark)
				gtx.Constraints = layout.Exact(image.Pt(size, size))
				if content.Mark == nil {
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}
				return layout.Center.Layout(gtx, content.Mark)
			}
			copyBlock := func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{layout.Rigid(Text(th, th.T.Row, titleInk, content.Title))}
				style := th.T.Caption
				if content.FactsMono {
					style = th.T.Mono
				}
				if content.FactsWidget != nil {
					children = append(children, layout.Rigid(content.FactsWidget))
				} else if content.Facts != "" {
					children = append(children, layout.Rigid(Text(th, style, quietInk, content.Facts)))
				}
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx, children...)
			}
			trailing := func(gtx layout.Context) layout.Dimensions {
				if (realHovered || focused || tablet.Preview == StateHover) && content.Action != nil {
					return content.Action.Layout(gtx, th)
				}
				gtx.Constraints.Min.X = min(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeTabletTrailingMin))
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return Label{Style: th.T.Label, Color: claimInk, MaxLines: 1, Align: text.End}.Layout(gtx, th, content.Claim)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return Label{Style: th.T.Caption, Color: quietInk, MaxLines: 1, Align: text.End}.Layout(gtx, th, content.Proof)
					}),
				)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(mark),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Flexed(1, copyBlock),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Rigid(trailing),
			)
		}
		layout.Inset{Left: tokens.SizeTabletInsetLeft, Right: tokens.SizeTabletInsetRight}.Layout(gtx, body)
		if content.Progress >= 0 {
			tablet.progress.Omega = 4 / float32(th.Motion.Select.Seconds())
			tablet.progress.Rest = .001
			tablet.progress.Set(float32(min(max(content.Progress, 0), 1)))
			fraction := tablet.progress.Value(gtx)
			bladeWidth := int(math.Round(float64(width) * float64(fraction)))
			if bladeWidth > 0 {
				Blade(gtx.Ops, image.Rect(0, height-max(1, gtx.Dp(tokens.SizeTabletProgress)), bladeWidth, height), th.R.Lapis)
			}
		}
		focusAlpha := float32(1)
		if tablet.Preview != StateFocus {
			focusAlpha = focusValue(gtx, th, &tablet.focus, realFocused)
		}
		FocusRingAlpha(gtx, th, image.Rect(0, 0, width, height), 0, focusAlpha)
		return layout.Dimensions{Size: image.Pt(width, height)}
	})
	return dimensions
}
