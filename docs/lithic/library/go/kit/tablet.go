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

// Tablet is an interactive resource row in the theme's design.
//
// Lithic draws the documented tablet: a square stone2 plane with the tone's
// notch at its north-west corner and a blade along its foot for progress.
// Material, Adwaita and Fluent draw a row that lies on whatever it stands on,
// in the design's row corner (a pill in Material), with the tone as a ToneMark
// dot and a round progress line inside its insets. In every design the pointer
// and a press are fg washes and the selection is the Hub's selection wash with
// its ink, never a sel fill.
type Tablet struct {
	Click
	// Pane is the plane the tablet lies on. Lithic cuts an idle tablet's notch
	// in it; the zero value is the design's page pane, which hides the idle
	// notch on a page. A list and detail window sets its list pane's stone1.
	// Round designs do not read it.
	Pane color.NRGBA
	// Listed says a RowList draws the selection under this tablet: the tablet
	// lays no plane of its own and wears only the selected inks, so the list's
	// gliding wash shows through and is not painted twice.
	Listed  bool
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
// selected marks the chosen row; listFocused lights it as the pointer would
// while its list holds keyboard focus.
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
		round := th.Design.Profile().Round
		bounds := image.Rect(0, 0, width, height)
		cut := 0
		if round {
			cut = gtx.Dp(th.Cut.Small)
		} else if !tablet.Listed {
			FillCut(gtx.Ops, bounds, 0, th.R.Stone2)
		}
		selectWash, selectInk, selectQuiet := selectionInks(th)
		if selected && !tablet.Listed {
			Fill(gtx, th, bounds, cut, selectWash)
		}
		// the Hub's row shows the press wash in place of the hover wash
		if lit := hover * (1 - press); lit > 0 {
			Fill(gtx, th, bounds, cut, palette.WithAlpha(th.R.Hover(), lit))
		}
		if press > 0 {
			Fill(gtx, th, bounds, cut, palette.WithAlpha(th.R.Press(), press))
		}

		markInk := th.R.Stone(content.Tone)
		if content.Tone == palette.Idle {
			markInk = color.NRGBA{}
			if !round && !tablet.Listed {
				// an idle Lithic tablet's corner is cut in the plane beneath it
				markInk = tablet.Pane
				if markInk == (color.NRGBA{}) {
					markInk = th.Planes().Pane
				}
			}
		}
		toneMark(gtx, th, bounds, gtx.Dp(th.Cut.Notch), markInk, tablet.notch.Value(gtx))

		titleInk, quietInk := th.R.Fg, th.R.Fg3
		claimInk := th.R.Stone(content.Tone)
		if content.Tone == palette.Idle {
			claimInk = th.R.Fg2
		}
		if selected {
			titleInk, quietInk = selectInk, selectQuiet
			if content.Tone == palette.Idle {
				claimInk = selectInk
			}
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
			progressHeight := max(1, gtx.Dp(tokens.SizeTabletProgress))
			if round {
				// a round row keeps the line inside its insets, clear of its corners
				left := gtx.Dp(tokens.SizeTabletInsetLeft)
				span := max(0, width-left-gtx.Dp(tokens.SizeTabletInsetRight))
				foot := height - gtx.Dp(th.Sp.S1)
				if barWidth := int(math.Round(float64(span) * float64(fraction))); barWidth > 0 {
					FillBlade(gtx, th, image.Rect(left, foot-progressHeight, left+barWidth, foot), th.R.Lapis)
				}
			} else if bladeWidth := int(math.Round(float64(width) * float64(fraction))); bladeWidth > 0 {
				Blade(gtx.Ops, image.Rect(0, height-progressHeight, bladeWidth, height), th.R.Lapis)
			}
		}
		return layout.Dimensions{Size: image.Pt(width, height)}
	})
	// drawn outside the click's clip: the ring stands a gap outside the row
	focusAlpha := float32(1)
	if tablet.Preview != StateFocus {
		focusAlpha = focusValue(gtx, th, &tablet.focus, gtx.Focused(&tablet.Click.Clickable))
	}
	ringCut := 0
	if th.Design.Profile().Round {
		ringCut = gtx.Dp(th.Cut.Small)
	}
	FocusRingAlpha(gtx, th, image.Rectangle{Max: dimensions.Size}, ringCut, focusAlpha)
	return dimensions
}

// selectionInks returns the Hub's selection mark in th: the wash under a
// chosen row (Tone.select), the ink on it (Tone.selectInk) and a quiet ink
// for its second line. On Lithic's ladder the wash is faint, so the quiet ink
// stays fg3; an opaque container from the desktop's palette takes a quiet ink
// of its own, as onSel2 is made for sel.
func selectionInks(th *theme.Theme) (wash, ink, quiet color.NRGBA) {
	wash, ink = th.R.Select(), th.R.OnContainer
	if ink.A == 0 {
		ink = th.R.Fg
	}
	quiet = th.R.Fg3
	if wash.A == 255 {
		quiet = palette.Mix(ink, wash, tokens.AlphaOnSel2)
	}
	return wash, ink, quiet
}
