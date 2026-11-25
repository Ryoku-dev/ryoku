package settings

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/keep"
	"ryoku-rashin/internal/protocol"
)

// watchRetry is how soon a watch that failed may open again, on a frame
// that draws the panel; nothing wakes to retry it
const watchRetry = 2 * time.Second

// oniGuide is Settings' part of Oni's guides: the three switches, a guide
// started from here, the live step panel, the keep offer and the kept
// walkthroughs. The step panel hears the guide's frames on a connection
// of its own, opened as an observer only while the panel is drawn and
// closed the frame it is not.
type oniGuide struct {
	enabled, keep, hints kit.Toggle

	field                     kit.TextField
	start, next, circle, stop kit.Plaque
	busy                      bool

	answer  *protocol.OniAnswer
	note    string
	problem string

	// the live step panel: drawn this frame, the watch while it runs and
	// which one it is, when the last one failed, whether it has heard a
	// frame, the guide's frame and the line the last guide closed with
	drawn    bool
	watch    context.CancelFunc
	watchID  int
	failedAt time.Time
	heard    bool
	frame    *feature.GuideFrame
	closing  string

	// the offer to keep a finished guide, nil once answered; shownOffer
	// stays for the dialog's way out, and prompting while it is drawn
	offer         *protocol.OniKeepOffer
	shownOffer    protocol.OniKeepOffer
	offerDeadline time.Time
	prompt        kit.Dialog
	prompting     bool
	keepYes       kit.Plaque
	keepNo        kit.Plaque

	// the kept walkthroughs as last read, and why they could not be
	keeps        []keep.Entry
	keepsRead    bool
	reading      bool
	keepsProblem string
	damaged      bool
	// why the record of hints offered cannot be read, "" when it can
	hintsProblem string
	forgets      map[string]*kit.Plaque
	forgetAll    kit.Plaque
	startOver    kit.Plaque

	// why Oni cannot read text in a window, read once per visit
	ocr     string
	ocrRead bool
}

// forget lets go of what was read, so coming back reads it again, and
// closes the watch
func (g *oniGuide) forget() {
	g.keepsRead, g.ocrRead = false, false
	g.stopWatch()
}

func (g *oniGuide) noted(note string)     { g.note, g.busy = note, false }
func (g *oniGuide) failed(problem string) { g.problem, g.busy = problem, false }

// receive takes what a guide call came to: a picture to ask about, a step,
// the end and its keep offer, or a no
func (g *oniGuide) receive(result *protocol.OniAskResult) {
	g.busy = false
	switch result.Status {
	case protocol.OniAskAnswered:
		g.answer = nil
		if result.Answer != nil && result.Answer.Mode == protocol.OniModeGuide {
			g.answer = result.Answer
		}
	case protocol.OniAskEnded:
		g.answer = nil
		if result.Answer != nil {
			g.closing = result.Answer.Text
		}
		if result.Keep != nil {
			g.offerKeep(*result.Keep)
		}
	case protocol.OniAskDeclined:
		g.answer = nil
		g.note = i18n.T("Cancelled. Nothing was sent.")
	}
}

// call sends one guide action and hears what came of it; a picture to ask
// about goes to the screen ask's dialog, which answers for the guide
func (p *Page) guideCall(e *env.Env, params protocol.OniGuideParams) {
	g := &p.oni.guide
	if g.busy {
		return
	}
	g.busy, g.note, g.problem = true, "", ""
	result := new(protocol.OniAskResult)
	e.Call(protocol.MOniGuide, params, result, func(err error) {
		if err != nil {
			g.failed(err.Error())
			return
		}
		if result.Status == protocol.OniAskConsent && result.Consent != nil {
			g.busy = false
			p.oni.ask.present(result.Consent, g)
			return
		}
		g.receive(result)
	})
}

// guidesOn says whether a guide may run: Oni on, looking at windows and
// the guide switch
func guidesOn(settings protocol.Settings) bool {
	return feature.Decide(feature.Request{Feature: feature.Guide}, protocol.EffectiveOni(settings)).Allow
}

// settle runs once a frame, after the page is drawn: the watch runs while
// the panel was drawn and a guide may run, and stops the frame it was not
func (g *oniGuide) settle(gtx layout.Context, e *env.Env) {
	want := g.drawn && guidesOn(e.Snapshot().Settings)
	g.drawn = false
	switch {
	case !want:
		g.stopWatch()
	case g.watch == nil && e.Post != nil && e.Client != nil && e.Client.Connected() &&
		(g.failedAt.IsZero() || gtx.Now.Sub(g.failedAt) >= watchRetry):
		g.startWatch(gtx, e)
	}
}

// startWatch opens the observer: each frame it hears is decoded off the
// frame goroutine and applied on it, while this watch is the current one
func (g *oniGuide) startWatch(gtx layout.Context, e *env.Env) {
	ctx, cancel := context.WithCancel(context.Background())
	g.watchID++
	id, client, post, started := g.watchID, e.Client, e.Post, gtx.Now
	g.watch = cancel
	go func() {
		err := client.WatchOni(ctx, func(raw json.RawMessage) {
			payload, err := feature.DecodeFrame(raw)
			if err != nil {
				return
			}
			post(func() {
				if g.watchID == id && g.watch != nil {
					g.hear(payload)
				}
			})
		})
		cancel()
		post(func() {
			if g.watchID != id {
				return
			}
			// the daemon went or refused: the panel shows nothing it cannot hear
			g.watch, g.frame, g.heard = nil, nil, false
			if err != nil || ctx.Err() == nil {
				g.failedAt = started
			}
		})
	}()
}

func (g *oniGuide) stopWatch() {
	if g.watch == nil {
		return
	}
	g.watch()
	g.watch, g.frame, g.heard = nil, nil, false
	g.watchID++
}

// hear applies one frame: the guide's state, an offer to keep the guide
// that finished, or that offer answered elsewhere
func (g *oniGuide) hear(payload feature.Payload) {
	g.heard = true
	switch f := payload.(type) {
	case *feature.GuideFrame:
		if f.Phase == feature.GuideDone && f.Text != "" {
			g.closing = f.Text
		}
		if f.Phase == feature.GuideIdle {
			g.frame = nil
			// a guide that went idle took its offer with it
			g.closeOffer()
			return
		}
		frame := *f
		g.frame = &frame
	case *feature.ConsentFrame:
		if f.Feature == feature.GuideKeep {
			g.offerKeep(protocol.OniKeepOffer{ID: f.ID, Question: f.Question, ExpiresMs: f.ExpiresMs})
		}
	case *feature.ConsentAnswerFrame:
		if g.offer != nil && g.offer.ID == f.ID {
			g.closeOffer()
		}
	}
}

// offerKeep opens the keep question, once per offer while it is in hand
func (g *oniGuide) offerKeep(offer protocol.OniKeepOffer) {
	if offer.ID == g.shownOffer.ID && (g.offer != nil || g.prompting) {
		return
	}
	g.offer, g.shownOffer, g.offerDeadline = &offer, offer, time.Time{}
	g.prompt.Open, g.prompt.Width, g.prompting = true, 560, true
}

// closeOffer drops the question without answering it: it was answered
// elsewhere, or it went with its guide
func (g *oniGuide) closeOffer() {
	g.offer = nil
}

// answerOffer keeps the finished guide or lets it go, once
func (g *oniGuide) answerOffer(e *env.Env, yes bool) {
	if g.offer == nil {
		return
	}
	id := g.offer.ID
	g.offer = nil
	if !yes {
		e.Call(protocol.MOniSkip, protocol.OniKeepParams{ID: id}, nil, func(err error) {
			// an offer that already went keeps nothing either way
			if wire := new(protocol.Error); err != nil && !(errors.As(err, &wire) && wire.Code == protocol.CodeNotFound) {
				g.problem = err.Error()
				return
			}
			g.note = i18n.T("Not kept. Oni will ask a model again next time.")
		})
		return
	}
	var result protocol.OniKeepResult
	e.Call(protocol.MOniKeep, protocol.OniKeepParams{ID: id}, &result, func(err error) {
		if err != nil {
			if wire := new(protocol.Error); errors.As(err, &wire) && wire.Code == protocol.CodeNotFound {
				g.note = i18n.T("That offer was no longer waiting, so nothing was kept.")
				return
			}
			g.problem = err.Error()
			return
		}
		g.note = i18n.T("Kept. Next time Oni shows it again on this computer, with no picture sent.")
		g.keepsRead = false
	})
}

// overlay draws the keep question while it waits and while it leaves
func (g *oniGuide) overlay(gtx layout.Context, e *env.Env) bool {
	if g.offer != nil {
		if g.offerDeadline.IsZero() {
			g.offerDeadline = gtx.Now.Add(time.Duration(g.offer.ExpiresMs) * time.Millisecond)
		}
		if !gtx.Now.Before(g.offerDeadline) {
			// the offer lapsed to a skip; the daemon has let it go
			g.offer = nil
			g.note = i18n.T("Not kept: the offer waited too long.")
			g.prompt.Dismiss(gtx)
		} else {
			gtx.Execute(op.InvalidateCmd{At: g.offerDeadline})
		}
	}
	if g.offer == nil && g.prompt.Open {
		g.prompt.Dismiss(gtx)
	}
	// Escape or a press outside closed it: that is a no
	if g.offer != nil && !g.prompt.Open {
		g.answerOffer(e, false)
	}
	if !g.prompting {
		return false
	}
	th := e.Theme
	g.keepNo.Kind, g.keepNo.Label = kit.PlaqueQuiet, i18n.T("Not now")
	g.keepYes.Kind, g.keepYes.Label = kit.PlaqueGold, i18n.T("Keep it")
	if g.keepNo.Clicked(gtx) {
		g.answerOffer(e, false)
		g.prompt.Dismiss(gtx)
	}
	if g.keepYes.Clicked(gtx) && g.offer != nil {
		g.answerOffer(e, true)
		g.prompt.Dismiss(gtx)
	}
	question := g.shownOffer.Question
	if question == "" {
		question = i18n.T("Keep these steps for another time?")
	}
	g.prompt.Layout(gtx, th.Theme, question, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(look.Paragraph(th, th.T.Row, th.R.Fg, i18n.T("Oni keeps the goal, the app and each step's sentence and control, on this computer. Next time it shows the same steps without sending a picture."))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("It never keeps a picture, a window title or anything you typed. Not now keeps nothing.")+" "+keepLapseLine(g.shownOffer.ExpiresMs))),
		)
	},
		func(gtx layout.Context) layout.Dimensions { return g.keepNo.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return g.keepYes.Layout(gtx, th.Theme) },
	)
	if g.prompt.Closed() {
		g.prompting = false
	}
	return g.prompt.Open
}

func keepLapseLine(ms int64) string {
	return i18n.Tf("Without an answer within %d seconds, nothing is kept.", (ms+999)/1000)
}

// readKeeps reads the kept walkthroughs off the frame goroutine
func (g *oniGuide) readKeeps(e *env.Env) {
	store := e.Guide.Keeps
	if store == nil || g.reading || e.Post == nil {
		return
	}
	g.reading, g.keepsRead = true, true
	post := e.Post
	go func() {
		entries, err := store.List()
		hintsErr := store.Hints().Check()
		post(func() {
			g.reading = false
			g.keeps, g.keepsProblem, g.damaged, g.hintsProblem = entries, "", false, ""
			if err != nil {
				g.keeps, g.keepsProblem = nil, err.Error()
				var wire *protocol.Error
				g.damaged = errors.As(err, &wire) && wire.Reason == keep.ReasonDamaged
			}
			if wire := (*protocol.Error)(nil); errors.As(hintsErr, &wire) && wire.Reason == keep.ReasonHintsDamaged {
				g.hintsProblem = hintsErr.Error()
			}
		})
	}()
}

// changeKeeps runs one change to the store off the frame goroutine, then
// reads it again
func (g *oniGuide) changeKeeps(e *env.Env, change func(*keep.Store) error, done string) {
	store := e.Guide.Keeps
	if store == nil || g.reading || e.Post == nil {
		return
	}
	g.reading = true
	post, notify := e.Post, e.Notify
	go func() {
		err := change(store)
		post(func() {
			g.reading = false
			g.keepsRead = false
			if err != nil {
				g.keepsProblem = err.Error()
				return
			}
			if notify != nil {
				notify(done)
			}
		})
	}()
}

// oniGuideBlocks are the Guides group and the Kept walkthroughs group
func (p *Page) oniGuideBlocks(e *env.Env, oni, defaults protocol.OniSettings) (guides, kept []look.Setting) {
	th := e.Theme
	g := &p.oni.guide
	if !g.ocrRead {
		g.ocrRead, g.ocr = true, ""
		if e.Guide.OCR != nil {
			g.ocr = e.Guide.OCR()
		}
	}
	if !g.keepsRead {
		g.readKeeps(e)
	}
	settings := e.Snapshot().Settings
	master := protocol.EffectiveOni(settings).Master
	g.enabled.Value, g.keep.Value, g.hints.Value = oni.Guide.Enabled, oni.Guide.Keep, oni.Guide.Hints
	guideName, keepName, hintsName := i18n.T("Show me how, step by step"), i18n.T("Offer to keep a walkthrough"), i18n.T("Hints from kept walkthroughs")
	guideDetail := i18n.T("Ask Oni how to do something in the perched window: it points at one control at a time, and you click it yourself or press Next. Each step shows Oni one picture of the window, asked about as Ask before sending says.")
	if !oni.Inspect {
		guideDetail += "\n" + i18n.T("Needs Let Oni look at a window, which is off; until it is on, no guide starts.")
	}
	keepDetail := i18n.T("When a guide finishes and every step was checked on the screen, Oni asks whether to keep it. A kept walkthrough runs again on this computer with no picture sent.")
	if !oni.Guide.Enabled {
		keepDetail += "\n" + i18n.T("Needs Show me how, step by step; until it is on, nothing is offered or replayed.")
	}
	hintsDetail := i18n.T("While Oni is perched on an app you kept a walkthrough for, it may offer that walkthrough, at most once a day. It never watches the screen for this.")
	if !oni.Guide.Keep {
		hintsDetail += "\n" + i18n.T("Needs Offer to keep a walkthrough; with nothing kept there is nothing to offer.")
	}
	off := ""
	if !master {
		off = i18n.T("Oni is off, so it cannot guide.")
	}
	guide := func(values map[string]any) { set(e, "oni", map[string]any{"guide": values}) }
	guides = []look.Setting{
		{Title: guideName, Detail: guideDetail, Unavailable: off,
			Changed: oni.Guide.Enabled != defaults.Guide.Enabled, Reset: func() { guide(map[string]any{"enabled": defaults.Guide.Enabled}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, guideName, &g.enabled, func(on bool) { guide(map[string]any{"enabled": on}) })
			}},
		{Title: keepName, Detail: keepDetail, Unavailable: off,
			Changed: oni.Guide.Keep != defaults.Guide.Keep, Reset: func() { guide(map[string]any{"keep": defaults.Guide.Keep}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, keepName, &g.keep, func(on bool) { guide(map[string]any{"keep": on}) })
			}},
		{Title: hintsName, Detail: hintsDetail, Unavailable: off,
			Changed: oni.Guide.Hints != defaults.Guide.Hints, Reset: func() { guide(map[string]any{"hints": defaults.Guide.Hints}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, hintsName, &g.hints, func(on bool) { guide(map[string]any{"hints": on}) })
			}},
	}
	if g.ocr != "" {
		guides = append(guides, look.Setting{Title: i18n.T("Text reader needed"),
			Detail: g.ocr + " " + i18n.T("Guides still run, as best guesses from the picture, but no step can be checked on the screen, so keeping a walkthrough, replaying one and asking about a picked spot are unavailable. Install tesseract with its English data to use them.")})
	}
	unavailable := ""
	switch {
	case !master:
		unavailable = i18n.T("Oni is off, so it cannot guide.")
	case !oni.Inspect:
		unavailable = i18n.T("Turn on Let Oni look at a window first.")
	case !oni.Guide.Enabled:
		unavailable = i18n.Tf("Turn on %s first.", guideName)
	}
	guides = append(guides, look.Setting{Title: i18n.T("Walk me through it"),
		Detail:      i18n.T("Say what you want to do in the perched window. Oni shows each step here while this part of Settings is open, and beside itself on the desktop."),
		Unavailable: unavailable, Below: true,
		Control: func(gtx layout.Context) layout.Dimensions { return p.layoutGuidePanel(gtx, e) }})
	switch {
	case g.problem != "":
		guides = append(guides, look.Setting{Title: i18n.T("The guide could not go on"), Detail: g.problem})
	case g.note != "":
		guides = append(guides, look.Setting{Title: i18n.T("Guide"), Detail: g.note})
	}
	return guides, g.keptRows(e)
}

// keptRows lists the kept walkthroughs by goal and app, each with Forget:
// the store holds no picture or title, so none is shown
func (g *oniGuide) keptRows(e *env.Env) []look.Setting {
	th := e.Theme
	if e.Guide.Keeps == nil {
		return []look.Setting{{Title: i18n.T("Kept walkthroughs"), Detail: i18n.T("Kept walkthroughs cannot be read here.")}}
	}
	var rows []look.Setting
	if g.damaged || g.hintsProblem != "" {
		// Start over moves aside whichever file could not be read
		keepsBad, hintsBad := g.damaged, g.hintsProblem != ""
		title, detail := i18n.T("Kept walkthroughs"), g.keepsProblem
		if !keepsBad {
			title, detail = i18n.T("Hints from kept walkthroughs"), g.hintsProblem
		} else if hintsBad {
			detail += "\n" + g.hintsProblem
		}
		g.startOver.Kind, g.startOver.Small, g.startOver.Label = kit.PlaqueTonal, true, i18n.T("Start over")
		rows = append(rows, look.Setting{Title: title, Detail: detail,
			Control: func(gtx layout.Context) layout.Dimensions {
				if g.startOver.Clicked(gtx) {
					g.changeKeeps(e, func(s *keep.Store) error {
						if keepsBad {
							if err := s.Reset(); err != nil {
								return err
							}
						}
						if hintsBad {
							return s.Hints().Reset()
						}
						return nil
					}, i18n.T("Started over; the old file is kept beside the new one."))
				}
				return g.startOver.Layout(gtx, th.Theme)
			}})
		if keepsBad {
			return rows
		}
	}
	if g.keepsProblem != "" {
		rows = append(rows, look.Setting{Title: i18n.T("Kept walkthroughs"), Detail: g.keepsProblem})
	}
	if len(g.keeps) == 0 {
		return append(rows, look.Setting{Title: i18n.T("Nothing kept yet"), Detail: i18n.T("Finish a guide with Offer to keep a walkthrough on and say Keep it.")})
	}
	if g.forgets == nil {
		g.forgets = map[string]*kit.Plaque{}
	}
	listed := make(map[string]bool, len(g.keeps))
	for _, entry := range g.keeps {
		listed[entry.Key] = true
		plaque := g.forgets[entry.Key]
		if plaque == nil {
			plaque = new(kit.Plaque)
			g.forgets[entry.Key] = plaque
		}
		plaque.Kind, plaque.Small, plaque.Label = kit.PlaqueQuiet, true, i18n.T("Forget")
		key, goal := entry.Key, entry.Goal
		rows = append(rows, look.Setting{Title: goal, Detail: keptDetail(entry),
			Control: func(gtx layout.Context) layout.Dimensions {
				if plaque.Clicked(gtx) {
					g.changeKeeps(e, func(s *keep.Store) error { return s.Forget(key) }, i18n.Tf("Forgot “%s”.", goal))
				}
				if g.reading {
					gtx = gtx.Disabled()
				}
				return look.Named(gtx, i18n.Tf("Forget “%s”", goal), func(gtx layout.Context) layout.Dimensions { return plaque.Layout(gtx, th.Theme) })
			}})
	}
	for key := range g.forgets {
		if !listed[key] {
			delete(g.forgets, key)
		}
	}
	g.forgetAll.Kind, g.forgetAll.Small, g.forgetAll.Label = kit.PlaqueTonal, true, i18n.T("Forget all")
	return append(rows, look.Setting{Title: i18n.T("Forget every walkthrough"), Detail: i18n.T("Oni stops replaying them and asks a model again. They cannot be brought back."),
		Control: func(gtx layout.Context) layout.Dimensions {
			if g.forgetAll.Clicked(gtx) {
				g.changeKeeps(e, func(s *keep.Store) error {
					if err := s.ForgetAll(); err != nil {
						return err
					}
					return s.Hints().ForgetAll()
				}, i18n.T("Forgot every kept walkthrough."))
			}
			if g.reading {
				gtx = gtx.Disabled()
			}
			return g.forgetAll.Layout(gtx, th.Theme)
		}})
}

// keptDetail is where a walkthrough was kept, how long it is and when it
// was kept
func keptDetail(entry keep.Entry) string {
	steps := i18n.T("1 step")
	if len(entry.Steps) != 1 {
		steps = i18n.Tf("%d steps", len(entry.Steps))
	}
	line := i18n.Tf("In %s · %s · kept %s", entry.App, steps, entry.Kept.Local().Format("2 Jan 2006"))
	if entry.Misses > 0 {
		line += "\n" + i18n.T("Its last replay could not find a step; one more miss retires it.")
	}
	return line
}

// layoutGuidePanel is the live step panel while a guide runs, else the
// goal field and its start. The frames heard decide; until the watch has
// heard one, the step this page's own call answered stands in.
func (p *Page) layoutGuidePanel(gtx layout.Context, e *env.Env) layout.Dimensions {
	g := &p.oni.guide
	g.drawn = true
	if f := g.frame; f != nil {
		return g.layoutStep(gtx, p, e, *f)
	}
	if a := g.answer; a != nil && !g.heard && a.Step > 0 {
		kicker := i18n.Tf("Step %d", a.Step)
		if a.Replay {
			kicker = i18n.T("Kept from before")
		}
		return g.layoutStep(gtx, p, e, feature.GuideFrame{Phase: feature.GuideWaiting, Step: a.Step, Kicker: kicker, Text: a.Text, Replay: a.Replay, Target: a.Seq,
			Actions: []string{feature.GuideActionNext, feature.GuideActionStop, feature.GuideActionCircle}})
	}
	return g.layoutStart(gtx, p, e)
}

// layoutStart is the goal field with Show me how, and the line the last
// guide closed with
func (g *oniGuide) layoutStart(gtx layout.Context, p *Page, e *env.Env) layout.Dimensions {
	th := e.Theme
	g.field.Label, g.field.Placeholder, g.field.Editor.Submit = i18n.T("Goal"), i18n.T("Save this file"), true
	submitted := false
	for {
		event, ok := g.field.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			submitted = true
		}
	}
	g.start.Kind, g.start.Small, g.start.Label, g.start.Busy = kit.PlaqueGold, true, i18n.T("Show me how"), g.busy
	goal := strings.TrimSpace(g.field.Text())
	if (g.start.Clicked(gtx) || submitted) && goal != "" && !g.busy {
		p.guideCall(e, protocol.OniGuideParams{Action: protocol.OniGuideStart, Goal: goal})
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return g.field.Layout(gtx, th.Theme) }),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if goal == "" {
				gtx = gtx.Disabled()
			}
			return g.start.Layout(gtx, th.Theme)
		}),
	}
	if g.closing != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.Tf("Last guide: %s", g.closing))))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// layoutStep is the guide's card as the frame says: the kicker, the step,
// the checklist, and the actions the frame offers
func (g *oniGuide) layoutStep(gtx layout.Context, p *Page, e *env.Env, f feature.GuideFrame) layout.Dimensions {
	th := e.Theme
	for _, b := range []struct {
		plaque *kit.Plaque
		action string
	}{{&g.next, protocol.OniGuideNext}, {&g.circle, protocol.OniGuideCircle}, {&g.stop, protocol.OniGuideStop}} {
		if b.plaque.Clicked(gtx) {
			p.guideCall(e, protocol.OniGuideParams{Action: b.action})
		}
	}
	g.next.Kind, g.next.Small, g.next.Label = kit.PlaqueGold, true, i18n.T("Next")
	g.circle.Kind, g.circle.Small, g.circle.Label = kit.PlaqueTonal, true, i18n.T("Circle it again")
	g.stop.Kind, g.stop.Small, g.stop.Label = kit.PlaqueQuiet, true, i18n.T("Stop")

	kicker, text := f.Kicker, f.Text
	switch f.Phase {
	case feature.GuideReading:
		text = i18n.T("Reading the window…")
	case feature.GuideKeeping:
		if kicker == "" {
			kicker = i18n.T("Done")
		}
		text = g.closing
	case feature.GuideWinding:
		text = i18n.T("Stopping…")
	}
	if f.Replay && kicker == "" {
		kicker = i18n.T("Kept from before")
	}
	inner := []layout.FlexChild{}
	if kicker != "" {
		inner = append(inner, layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Gold, kicker)), layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout))
	}
	if text != "" {
		inner = append(inner, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.Named(gtx, i18n.Tf("Guide step: %s", text), look.Paragraph(th, th.T.Row, th.R.Fg, text))
		}))
	}
	if f.Plan != nil {
		inner = append(inner, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutChecklist(gtx, th, *f.Plan)
		}))
	}
	if f.Phase == feature.GuidePointing || f.Phase == feature.GuideWaiting {
		inner = append(inner, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("Click it in the window, or press Next."))))
	}
	var buttons []layout.FlexChild
	for _, action := range f.Actions {
		var plaque *kit.Plaque
		switch action {
		case feature.GuideActionNext:
			plaque = &g.next
		case feature.GuideActionCircle:
			plaque = &g.circle
		case feature.GuideActionStop:
			plaque = &g.stop
		default:
			// keep and skip are the keep question's
			continue
		}
		if len(buttons) > 0 {
			buttons = append(buttons, layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout))
		}
		buttons = append(buttons, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if g.busy {
				gtx = gtx.Disabled()
			}
			return plaque.Layout(gtx, th.Theme)
		}))
	}
	if len(buttons) > 0 {
		inner = append(inner, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, buttons...)
		}))
	}
	return layoutCard(gtx, th, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, inner...)
	})
}

// layoutCard draws body on a card with the design's corners and hairline
func layoutCard(gtx layout.Context, th *look.Theme, body layout.Widget) layout.Dimensions {
	pad := th.Sp.S3
	macro := op.Record(gtx.Ops)
	inner := gtx
	inner.Constraints.Min = image.Pt(gtx.Constraints.Max.X, 0)
	dims := layout.UniformInset(pad).Layout(inner, body)
	call := macro.Stop()
	bounds := image.Rectangle{Max: image.Pt(gtx.Constraints.Max.X, dims.Size.Y)}
	cut := gtx.Dp(th.Cut.Small)
	kit.Fill(gtx, th.Theme, bounds, cut, th.Planes().Card)
	kit.Stroke(gtx, th.Theme, bounds, cut, max(1, gtx.Dp(unit.Dp(1))), th.R.Vein)
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: bounds.Size()}
}

// layoutChecklist lists the steps done, the one now and those ahead, each
// with a mark in the design's corners: filled for done, the accent for
// now, an outline for ahead
func layoutChecklist(gtx layout.Context, th *look.Theme, plan feature.GuidePlan) layout.Dimensions {
	type item struct {
		text, state string
	}
	var items []item
	for _, line := range plan.Done {
		items = append(items, item{line, "done"})
	}
	if plan.Current != "" {
		items = append(items, item{plan.Current, "now"})
	}
	for _, line := range plan.Next {
		items = append(items, item{line, "next"})
	}
	children := make([]layout.FlexChild, 0, 2*len(items))
	for index, it := range items {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout))
		}
		name := i18n.Tf("Next: %s", it.text)
		switch it.state {
		case "done":
			name = i18n.Tf("Done: %s", it.text)
		case "now":
			name = i18n.Tf("Now: %s", it.text)
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.Named(gtx, name, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						side := gtx.Dp(unit.Dp(10))
						mark := image.Rectangle{Max: image.Pt(side, side)}
						cut := gtx.Dp(unit.Dp(3))
						switch it.state {
						case "done":
							kit.Fill(gtx, th.Theme, mark, cut, th.R.Fg2)
						case "now":
							kit.Fill(gtx, th.Theme, mark, cut, th.R.Gold)
						default:
							kit.Stroke(gtx, th.Theme, mark, cut, max(1, gtx.Dp(unit.Dp(1))), th.R.Fg2)
						}
						return layout.Dimensions{Size: mark.Size()}
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						colour := th.R.Fg
						if it.state != "now" {
							colour = th.R.Fg2
						}
						return look.Paragraph(th, th.T.Detail, colour, it.text)(gtx)
					}),
				)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
