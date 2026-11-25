package settings

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/jpeg"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// oniAsk is a question about the window Oni is perched on. The daemon takes
// the picture but sends nothing until the user has seen it, and where it
// would go, and allowed it once; a no, a closed dialog or the question running
// out of time drops it. The same dialog asks about a guide's pictures, for
// the guide.
type oniAsk struct {
	field         kit.TextField
	send, letGo   kit.Plaque
	busy          bool
	allow, cancel kit.Plaque
	dialog        kit.Dialog

	// the question waiting for the user, nil once answered; shown stays for
	// the dialog's way out
	consent *protocol.OniConsent
	shown   protocol.OniConsent
	// whoever asked: this question, or a guide; it hears what came of it
	from asker
	// when the daemon stops waiting, from the first frame that showed it
	deadline time.Time
	thumb    paint.ImageOp
	pixels   *image.RGBA

	answer  *protocol.OniAnswer
	note    string
	problem string
}

type asker interface {
	receive(result *protocol.OniAskResult)
	noted(note string)
	failed(problem string)
}

func (a *oniAsk) noted(note string)     { a.note = note }
func (a *oniAsk) failed(problem string) { a.problem = problem }

// ask sends the question; the daemon answers it, or first asks the user
func (a *oniAsk) ask(e *env.Env) {
	text := strings.TrimSpace(a.field.Text())
	if text == "" || a.busy || a.consent != nil {
		return
	}
	a.busy, a.answer, a.note, a.problem = true, nil, "", ""
	result := new(protocol.OniAskResult)
	e.Call(protocol.MOniAsk, protocol.OniAskParams{Text: text}, result, func(err error) {
		a.busy = false
		if err != nil {
			a.problem = err.Error()
			return
		}
		a.receive(result)
	})
}

func (a *oniAsk) receive(result *protocol.OniAskResult) {
	switch result.Status {
	case protocol.OniAskConsent:
		if result.Consent != nil {
			a.present(result.Consent, a)
		}
	case protocol.OniAskAnswered:
		a.answer = result.Answer
	case protocol.OniAskDeclined:
		a.note = i18n.T("Cancelled. Nothing was sent.")
	}
}

// present opens the question with the exact picture the daemon would send,
// for from to hear what came of it. The JPEG is decoded once and its bytes
// zeroed; the pixels live until the dialog has gone.
func (a *oniAsk) present(consent *protocol.OniConsent, from asker) {
	a.clearPicture()
	if decoded, err := jpeg.Decode(bytes.NewReader(consent.Preview)); err == nil {
		a.pixels = image.NewRGBA(decoded.Bounds())
		draw.Draw(a.pixels, a.pixels.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
		a.thumb = paint.NewImageOp(a.pixels)
		if ycc, ok := decoded.(*image.YCbCr); ok {
			clear(ycc.Y)
			clear(ycc.Cb)
			clear(ycc.Cr)
		}
	}
	clear(consent.Preview)
	consent.Preview = nil
	a.consent, a.shown, a.deadline, a.from = consent, *consent, time.Time{}, from
	a.dialog.Open, a.dialog.Width = true, 560
}

// reply answers the waiting question once; the dialog's way out is left to
// whoever closed it
func (a *oniAsk) reply(e *env.Env, yes bool) {
	if a.consent == nil {
		return
	}
	id, from := a.consent.ID, a.from
	a.consent = nil
	answer := "no"
	if yes {
		answer = "yes"
	}
	result := new(protocol.OniAskResult)
	a.busy = yes && from == asker(a)
	e.Call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: id, Answer: answer}, result, func(err error) {
		if from == asker(a) {
			a.busy = false
		}
		if err != nil {
			if !yes {
				from.noted(i18n.T("Cancelled. Nothing was sent."))
				return
			}
			if wire := new(protocol.Error); errors.As(err, &wire) && wire.Code == protocol.CodeNotFound {
				from.noted(i18n.T("The picture had already been dropped, so nothing was sent. Ask again."))
				return
			}
			from.failed(err.Error())
			return
		}
		from.receive(result)
	})
	if !yes {
		from.noted(i18n.T("Cancelled. Nothing was sent."))
	}
}

// the question ran out of time: the daemon has dropped the picture by now
func (a *oniAsk) expire() {
	a.consent = nil
	a.from.noted(i18n.T("The picture waited too long and was dropped. Nothing was sent."))
}

func (a *oniAsk) clearPicture() {
	if a.pixels != nil {
		clear(a.pixels.Pix)
	}
	a.pixels, a.thumb = nil, paint.ImageOp{}
}

// overlay draws the question while it waits and while it leaves
func (a *oniAsk) overlay(gtx layout.Context, e *env.Env) bool {
	if a.consent != nil {
		if a.deadline.IsZero() {
			a.deadline = gtx.Now.Add(time.Duration(a.consent.ExpiresMs) * time.Millisecond)
		}
		if !gtx.Now.Before(a.deadline) {
			a.expire()
			a.dialog.Dismiss(gtx)
		} else {
			// one wake when it runs out, no ticking before
			gtx.Execute(op.InvalidateCmd{At: a.deadline})
		}
	}
	// Escape or a press outside closed it: that is a no
	if a.consent != nil && !a.dialog.Open {
		a.reply(e, false)
	}
	if a.pixels == nil && a.consent == nil && !a.dialog.Open {
		return false
	}
	th := e.Theme
	a.cancel.Kind, a.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	a.allow.Kind, a.allow.Label = kit.PlaqueGold, i18n.T("Allow once")
	if a.cancel.Clicked(gtx) {
		a.reply(e, false)
		a.dialog.Dismiss(gtx)
	}
	if a.allow.Clicked(gtx) && a.consent != nil {
		a.reply(e, true)
		a.dialog.Dismiss(gtx)
	}
	consent := a.shown
	local := consent.Destination == feature.EgressLocal
	title := i18n.T("Send this picture?")
	if local {
		title = i18n.T("Show Oni this picture?")
	}
	settings := protocol.EffectiveOni(e.Snapshot().Settings)
	a.dialog.Layout(gtx, th.Theme, title, func(gtx layout.Context) layout.Dimensions {
		return a.layoutBody(gtx, e, consent, local, settings.Disclosure)
	},
		func(gtx layout.Context) layout.Dimensions { return a.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return a.allow.Layout(gtx, th.Theme) },
	)
	if a.dialog.Closed() {
		a.clearPicture()
		a.shown = protocol.OniConsent{}
	}
	return a.dialog.Open
}

func (a *oniAsk) layoutBody(gtx layout.Context, e *env.Env, consent protocol.OniConsent, local bool, disclosure string) layout.Dimensions {
	th := e.Theme
	where, whereDetail := i18n.Tf("Goes to %s", consent.Route), i18n.T("It leaves this computer for that provider. Hyakki keeps no copy.")
	if local {
		where, whereDetail = i18n.T("Stays on this computer"), i18n.Tf("%s reads it here; it does not leave this computer.", consent.Route)
	}
	coverage := i18n.T("Oni asks before every picture.")
	if disclosure != protocol.OniDisclosureAlways {
		coverage = i18n.T("Oni will not ask again for this destination until it leaves the desktop. To be asked every time, change Ask before sending under Oni in Settings.")
	}
	facts := []look.FactRow{
		{Label: i18n.T("Captured with"), Value: capturePathLabel(consent.Path)},
		{Label: i18n.T("Picture"), Value: i18n.Tf("%d × %d pixels", consent.Width, consent.Height), Mono: true},
	}
	gap := layout.Spacer{Height: th.Sp.S3}.Layout
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			name := i18n.Tf("The picture that would be sent, %d by %d pixels", consent.Width, consent.Height)
			return look.Named(gtx, name, func(gtx layout.Context) layout.Dimensions { return a.layoutThumb(gtx, e) })
		}),
		layout.Rigid(gap),
		layout.Rigid(look.Paragraph(th, th.T.Row, th.R.Fg, where)),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, whereDetail)),
		layout.Rigid(gap),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.Facts(gtx, th, facts) }),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, consent.Disclosure)),
		layout.Rigid(gap),
		layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, coverage+" "+expiryLine(consent.ExpiresMs))),
	)
}

// the thumbnail at its proportions, as wide as the dialog lets it and no
// taller than a third of a small window
func (a *oniAsk) layoutThumb(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	size := a.thumb.Size()
	if size.X <= 0 || size.Y <= 0 {
		size = image.Pt(max(a.shown.Width, 16), max(a.shown.Height, 10))
	}
	width := gtx.Constraints.Max.X
	if tall := gtx.Dp(unit.Dp(240)); width*size.Y/size.X > tall {
		width = tall * size.X / size.Y
	}
	bounds := image.Rectangle{Max: image.Pt(width, width*size.Y/size.X)}
	cut := gtx.Dp(th.Cut.Small)
	kit.Fill(gtx, th.Theme, bounds, cut, th.Planes().Card)
	if a.pixels != nil {
		inner := bounds.Inset(gtx.Dp(2))
		clipped := clip.Rect(inner).Push(gtx.Ops)
		scale := f32.Pt(float32(inner.Dx())/float32(size.X), float32(inner.Dy())/float32(size.Y))
		transform := op.Affine(f32.Affine2D{}.Scale(f32.Point{}, scale).Offset(f32.Pt(float32(inner.Min.X), float32(inner.Min.Y)))).Push(gtx.Ops)
		a.thumb.Filter = paint.FilterLinear
		a.thumb.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		transform.Pop()
		clipped.Pop()
	}
	kit.Stroke(gtx, th.Theme, bounds, cut, max(1, gtx.Dp(unit.Dp(1))), th.R.Vein)
	return layout.Dimensions{Size: bounds.Size()}
}

// how the picture was taken, in words; the daemon's disclosure says the rest
func capturePathLabel(path string) string {
	switch path {
	case "toplevel":
		return i18n.T("The window's own picture")
	case "niri-window":
		return i18n.T("niri's window screenshot")
	case "output-crop":
		return i18n.T("Cut from a picture of the screen")
	case "portal":
		return i18n.T("The desktop portal")
	case "x11":
		return i18n.T("X11 screen capture")
	}
	return path
}

func expiryLine(ms int64) string {
	switch minutes := ms / 60000; {
	case ms%60000 == 0 && minutes == 1:
		return i18n.T("Without an answer within a minute, the picture is dropped.")
	case ms%60000 == 0 && minutes > 1:
		return i18n.Tf("Without an answer within %d minutes, the picture is dropped.", minutes)
	}
	return i18n.Tf("Without an answer within %d seconds, the picture is dropped.", (ms+999)/1000)
}

// layoutEntry is the question field with its Ask and Let go
func (a *oniAsk) layoutEntry(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	a.field.Label, a.field.Placeholder, a.field.Editor.Submit = i18n.T("Question"), i18n.T("What does this window do?"), true
	submitted := false
	for {
		event, ok := a.field.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			submitted = true
		}
	}
	a.send.Kind, a.send.Small, a.send.Label, a.send.Busy = kit.PlaqueGold, true, i18n.T("Ask Oni"), a.busy
	a.letGo.Kind, a.letGo.Small, a.letGo.Label = kit.PlaqueQuiet, true, i18n.T("Let go of the window")
	if a.send.Clicked(gtx) || submitted {
		a.ask(e)
	}
	if a.letGo.Clicked(gtx) {
		e.Call(protocol.MOniDetach, protocol.OniDetachParams{}, nil, func(err error) {
			if err == nil && e.Notify != nil {
				e.Notify(i18n.T("Oni let go of the window."))
			}
			e.Report(err)
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.field.Layout(gtx, th.Theme) }),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if strings.TrimSpace(a.field.Text()) == "" {
						gtx = gtx.Disabled()
					}
					return a.send.Layout(gtx, th.Theme)
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.letGo.Layout(gtx, th.Theme) }),
			)
		}),
	)
}

// result is the row that says what the last question came to, if anything
func (a *oniAsk) result() (look.Setting, bool) {
	switch {
	case a.problem != "":
		return look.Setting{Title: i18n.T("Oni could not ask"), Detail: a.problem}, true
	case a.note != "":
		return look.Setting{Title: i18n.T("Nothing was sent"), Detail: a.note}, true
	case a.answer != nil:
		answer := a.answer
		title := i18n.T("Oni's answer")
		if answer.Refused != "" {
			title = i18n.T("Oni would not point there")
		}
		ran := i18n.Tf("Answered by %s, which received the picture over the network.", answer.Route)
		if answer.Destination == feature.EgressLocal {
			ran = i18n.Tf("Answered by %s on this computer.", answer.Route)
		}
		captured := i18n.Tf("Captured with: %s", capturePathLabel(answer.Path))
		text := answer.Text
		if answer.Point != nil {
			text += " " + i18n.T("Oni points at it on the window.")
		}
		return look.Setting{Title: title, Detail: text + "\n" + ran + "\n" + captured}, true
	}
	return look.Setting{}, false
}
