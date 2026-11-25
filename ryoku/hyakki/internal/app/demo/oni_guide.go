package demo

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/keep"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/protocol"
)

// demoGuideSteps is how many steps the demo's one walkthrough takes: open
// the notes window's File menu, then press Save.
const demoGuideSteps = 2

// demoGuideStep is step n of the walkthrough and the box it points at, in
// fractions of the notes window
func demoGuideStep(n int) (string, protocol.OniRect) {
	if n == 1 {
		return i18n.T("Open the File menu."), protocol.OniRect{X: .02, Y: .04, W: .06, H: .04}
	}
	return i18n.T("Click Save."), protocol.OniRect{X: .02, Y: .11, W: .14, H: .045}
}

// guideFixtures is the demo's guide, keep and pick state: the guide that
// runs, the keep offer waiting, the goals kept, the armed pick and the
// strand targets sent, and the frames a watcher hears: the guide's frame
// and keep offer while they last, and those shown since the last take.
type guideFixtures struct {
	running *demoGuide
	offers  map[string]demoKeepOffer
	offered int
	kept    map[string]bool
	// store keeps walkthroughs on disk, in the demo's own directory; nil
	// keeps them for this run only
	store *keep.Store
	pick  *demoPick
	// waiter is the last arm's outcome, kept past its click for a late
	// oni.pick wait; waitTaken once a wait holds it
	waiter    demoPickWait
	waitTaken bool
	strand    uint64

	frameSeq uint64
	frame    *feature.GuideFrame
	offer    *feature.ConsentFrame
	told     []feature.Payload
}

type demoGuide struct {
	goal   string
	step   int
	replay bool
}

type demoKeepOffer struct {
	goal string
	at   time.Time
}

type demoPick struct {
	question string
}

// demoPickWait carries what an armed pick came to, given once; the server
// holds an oni.pick wait on it outside its lock.
type demoPickWait chan demoPickOutcome

type demoPickOutcome struct {
	result any
	err    error
}

func (w demoPickWait) give(result any, err error) {
	select {
	case w <- demoPickOutcome{result: result, err: err}:
	default:
	}
}

func newGuideFixtures() guideFixtures {
	return guideFixtures{offers: map[string]demoKeepOffer{}, kept: map[string]bool{}}
}

// drop ends the guide, its keep offer and an armed pick, and the wait on
// it; what was kept stays
func (g *guideFixtures) drop() {
	if g.pick != nil {
		g.waiter.give(protocol.OniAskResult{Status: protocol.OniAskEnded}, nil)
	}
	g.running, g.pick = nil, nil
	clear(g.offers)
	g.idle()
}

// demoGoalKey is a goal as kept: lowercase words
func demoGoalKey(goal string) string {
	return strings.Join(strings.Fields(strings.ToLower(goal)), " ")
}

// the demo's guide, keep and pick calls answer as the daemon will, over the
// notes window
func (s *server) callOniGuide(method string, raw json.RawMessage) (any, error) {
	o := &s.data.oni
	settings := protocol.EffectiveOni(s.data.snapshot.Settings)
	if !settings.Master {
		o.perch = nil
		o.dropAll()
	}
	params, err := feature.DecodeCall(method, raw)
	if err != nil {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The Oni call is invalid: %v.", err))
	}
	switch p := params.(type) {
	case *protocol.OniGuideParams:
		return o.guide(settings, *p)
	case *protocol.OniPickParams:
		return o.pickCall(settings, *p)
	case *protocol.OniKeepParams:
		if method == protocol.MOniKeep {
			return o.keep(settings, p.ID)
		}
		return protocol.Empty{}, o.skip(p.ID)
	}
	return nil, notFound(method)
}

// demoRefusal is why a feature may not run, or nil
func demoRefusal(settings protocol.OniSettings, id string) error {
	d := feature.Decide(feature.Request{Feature: id}, settings)
	switch {
	case d.Allow:
		return nil
	case d.Reason == feature.ReasonOniOff:
		return &protocol.Error{Code: protocol.CodeDisabled, Reason: d.Reason, Message: i18n.T("Oni is off. Turn it on in Hyakki's Settings first.")}
	}
	message := i18n.T("Looking at windows is off. Turn it on in Settings, under Oni.")
	switch id {
	case feature.Guide:
		message = i18n.T("Guides are off. Turn them and looking at windows on in Settings, under Oni.")
	case feature.GuideKeep:
		message = i18n.T("Keeping guides is off. Turn it on in Settings, under Oni.")
	case feature.Reflex:
		message = i18n.T("Quick commands are off. Turn them on in Settings, under Oni.")
	case feature.Reminders:
		message = i18n.T("Reminders are off. Turn them and quick commands on in Settings, under Oni.")
	case feature.Routines:
		message = i18n.T("Routines are off. Turn them and quick commands on in Settings, under Oni.")
	case feature.HandsBrowser:
		message = i18n.T("Hands is off. Turn on Hands in a browser and Looking at a window in the Hyakki app's Settings, under Oni › Hands.")
	case feature.HandsApp:
		message = i18n.T("Hands is off. Turn on Hands in one app window and Looking at a window in the Hyakki app's Settings, under Oni › Hands.")
	case feature.HandsDesktop:
		message = i18n.T("Hands is off. Turn on Hands on a separate desktop and Looking at a window in the Hyakki app's Settings, under Oni › Hands.")
	}
	return &protocol.Error{Code: protocol.CodeDisabled, Reason: d.Reason, Message: message}
}

func (o *oniFixtures) guide(settings protocol.OniSettings, p protocol.OniGuideParams) (any, error) {
	g := o.guides.running
	if p.Action != protocol.OniGuideStart && p.Action != protocol.OniGuideStop && g == nil {
		return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("No guide is running. Start one first."))
	}
	switch p.Action {
	case protocol.OniGuideStart:
		if err := demoRefusal(settings, feature.Guide); err != nil {
			return nil, err
		}
		if o.perch == nil {
			return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Oni is not perched on a window. Drag it onto one first."))
		}
		// a new guide ends the one that ran, its pictures and its offer
		o.dropGuide()
		goal := strings.TrimSpace(p.Goal)
		if feature.Decide(feature.Request{Feature: feature.GuideKeep}, settings).Allow && o.guides.isKept(goal) {
			return o.guideStep(goal, 1, true, "", feature.EgressLocal), nil
		}
		return o.guideRead(settings, goal, 1), nil
	case protocol.OniGuideNext:
		if g.step >= demoGuideSteps {
			return o.guideEnd(settings, *g), nil
		}
		if g.replay {
			return o.guideStep(g.goal, g.step+1, true, "", feature.EgressLocal), nil
		}
		return o.guideRead(settings, g.goal, g.step+1), nil
	case protocol.OniGuideCircle:
		route, destination := o.guideRoute(*g)
		return o.guideStep(g.goal, g.step, g.replay, route, destination), nil
	}
	o.dropGuide()
	if g == nil {
		return protocol.OniAskResult{Status: protocol.OniAskEnded}, nil
	}
	route, destination := o.guideRoute(*g)
	answer := protocol.OniAnswer{Text: i18n.T("Stopped. Ask again whenever you like."), Mode: protocol.OniModeGuide, Step: g.step, Replay: g.replay, Route: route, Destination: destination, Path: "toplevel"}
	return protocol.OniAskResult{Status: protocol.OniAskEnded, Answer: &answer}, nil
}

// a replayed guide ran on this computer with no model
func (o *oniFixtures) guideRoute(g demoGuide) (route, destination string) {
	if g.replay {
		return "", feature.EgressLocal
	}
	return o.route()
}

// guideRead reads the window for step: at once when the route was allowed
// for the session, else after the question about the picture
func (o *oniFixtures) guideRead(settings protocol.OniSettings, goal string, step int) protocol.OniAskResult {
	route, destination := o.route()
	if settings.Disclosure != protocol.OniDisclosureAlways && o.allowed[route] {
		return o.guideStep(goal, step, false, route, destination)
	}
	o.guides.show(readingFrame(step))
	return o.ask(o.perch.Window, demoConsent{goal: goal, step: step})
}

// guideStep is the guide at step, pointing at its control
func (o *oniFixtures) guideStep(goal string, step int, replay bool, route, destination string) protocol.OniAskResult {
	o.guides.running = &demoGuide{goal: goal, step: step, replay: replay}
	o.guides.strand++
	o.guides.show(stepFrame(step, replay, o.guides.strand))
	text, box := demoGuideStep(step)
	answer := protocol.OniAnswer{Text: text, Mode: protocol.OniModeGuide, Point: &box, Seq: o.guides.strand, Step: step, Replay: replay, Route: route, Destination: destination, Path: "toplevel"}
	return protocol.OniAskResult{Status: protocol.OniAskAnswered, Answer: &answer}
}

// guideEnd finishes the guide; one the model walked, with Keep on, is
// offered to keep
func (o *oniFixtures) guideEnd(settings protocol.OniSettings, g demoGuide) protocol.OniAskResult {
	o.guides.running = nil
	route, destination := o.guideRoute(g)
	text := i18n.T("Done. The packing list is saved.")
	if g.replay {
		text = i18n.T("All done, just as you kept it.")
	}
	answer := protocol.OniAnswer{Text: text, Mode: protocol.OniModeGuide, Step: g.step, Done: true, Replay: g.replay, Route: route, Destination: destination, Path: "toplevel"}
	result := protocol.OniAskResult{Status: protocol.OniAskEnded, Answer: &answer}
	o.guides.show(feature.GuideFrame{Phase: feature.GuideDone, Step: g.step, Text: text, Replay: g.replay})
	if g.replay || !feature.Decide(feature.Request{Feature: feature.GuideKeep}, settings).Allow {
		o.guides.idle()
		return result
	}
	o.guides.offered++
	id := fmt.Sprintf("keep-%d", o.guides.offered)
	o.guides.offers[id] = demoKeepOffer{goal: g.goal, at: o.now()}
	question := i18n.T("Keep these steps for another time?")
	result.Keep = &protocol.OniKeepOffer{ID: id, Question: question, ExpiresMs: feature.KeepOfferTTL().Milliseconds()}
	o.guides.offerKeep(g.step, id, question)
	return result
}

// dropGuide ends the guide, the pictures it waits on and its keep offer
func (o *oniFixtures) dropGuide() {
	for id, p := range o.pending {
		if p.step > 0 {
			delete(o.pending, id)
			o.fates[id] = demoFateDropped
		}
	}
	o.guides.running = nil
	clear(o.guides.offers)
	o.guides.idle()
}

// offer is the keep offer id, while it waits
func (o *oniFixtures) offer(id string) (demoKeepOffer, error) {
	offer, ok := o.guides.offers[id]
	if ok && o.now().Sub(offer.at) >= feature.KeepOfferTTL() {
		delete(o.guides.offers, id)
		ok = false
		// the offer lapsed to a skip
		if o.guides.offer != nil && o.guides.offer.ID == id {
			o.guides.idle()
		}
	}
	if !ok {
		return demoKeepOffer{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That offer is not waiting any more; nothing was kept."))
	}
	return offer, nil
}

func (o *oniFixtures) keep(settings protocol.OniSettings, id string) (any, error) {
	if err := demoRefusal(settings, feature.GuideKeep); err != nil {
		return nil, err
	}
	offer, err := o.offer(id)
	if err != nil {
		return nil, err
	}
	delete(o.guides.offers, id)
	entries, err := o.guides.remember(offer.goal)
	if err != nil {
		return nil, err
	}
	o.guides.answerOffer(id, true)
	o.guides.idle()
	return protocol.OniKeepResult{Kept: true, Entries: entries}, nil
}

func (o *oniFixtures) skip(id string) error {
	if _, err := o.offer(id); err != nil {
		return err
	}
	delete(o.guides.offers, id)
	o.guides.answerOffer(id, false)
	o.guides.idle()
	return nil
}

func (o *oniFixtures) pickCall(settings protocol.OniSettings, p protocol.OniPickParams) (any, error) {
	switch p.Action {
	case protocol.OniPickCancel:
		if o.guides.pick != nil {
			o.guides.waiter.give(protocol.OniAskResult{Status: protocol.OniAskEnded}, nil)
		}
		o.guides.pick = nil
		return protocol.OniAskResult{Status: protocol.OniAskEnded}, nil
	case protocol.OniPickArm:
		if err := demoRefusal(settings, feature.Inspect); err != nil {
			return nil, err
		}
		o.guides.waiter.give(protocol.OniAskResult{Status: protocol.OniAskEnded}, nil)
		o.guides.pick = &demoPick{question: strings.TrimSpace(p.Text)}
		o.guides.waiter, o.guides.waitTaken = make(demoPickWait, 1), false
		return protocol.OniAskResult{Status: protocol.OniAskArmed}, nil
	case protocol.OniPickWait:
		w := o.guides.waiter
		if w == nil || o.guides.waitTaken {
			return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Nothing this connection armed waits for a click."))
		}
		o.guides.waitTaken = true
		return w, nil
	}
	pick := o.guides.pick
	if pick == nil {
		return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Nothing is waiting for a click. Arm a pick first."))
	}
	o.guides.pick = nil
	result, err := o.pickPoint(settings, pick, p)
	o.guides.waiter.give(result, err)
	return result, err
}

// pickPoint answers the armed pick's click at p.
func (o *oniFixtures) pickPoint(settings protocol.OniSettings, pick *demoPick, p protocol.OniPickParams) (any, error) {
	if err := demoRefusal(settings, feature.Inspect); err != nil {
		return nil, err
	}
	windowID, spot, ok := demoHit(p.Output, p.X, p.Y)
	if !ok {
		return nil, &protocol.Error{Code: protocol.CodeNotFound, Reason: feature.PerchClosed, Message: i18n.T("There is no window there to ask about.")}
	}
	if strings.Contains(strings.ToLower(pick.question), "password") {
		// refused before any picture is taken
		answer := protocol.OniAnswer{Text: screenask.RefusalLine, Mode: protocol.OniModeSelect, Refused: "sensitive", Path: "toplevel"}
		return protocol.OniAskResult{Status: protocol.OniAskAnswered, Answer: &answer}, nil
	}
	route, destination := o.route()
	if settings.Disclosure != protocol.OniDisclosureAlways && o.allowed[route] {
		return o.selected(spot, pick.question, route, destination), nil
	}
	return o.ask(windowID, demoConsent{question: pick.question, spot: &spot}), nil
}

// demoHit is the placed window under a click on the demo's output and a
// small box around the click, in fractions of that window
func demoHit(output string, x, y float64) (string, protocol.OniRect, bool) {
	if output != demoOniOutput {
		return "", protocol.OniRect{}, false
	}
	ids := make([]string, 0, len(demoWindows))
	for id := range demoWindows {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		b := demoWindows[id].box
		if b == nil || x < b.X || y < b.Y || x >= b.X+b.W || y >= b.Y+b.H {
			continue
		}
		const w, h = .1, .06
		fx := min(max((x-b.X)/b.W-w/2, 0), 1-w)
		fy := min(max((y-b.Y)/b.H-h/2, 0), 1-h)
		return id, protocol.OniRect{X: fx, Y: fy, W: w, H: h}, true
	}
	return "", protocol.OniRect{}, false
}

// selected is what the demo's model says about a picked spot, offering a
// walk through it
func (o *oniFixtures) selected(spot protocol.OniRect, question, route, destination string) protocol.OniAskResult {
	o.guides.strand++
	text := i18n.T("That line is the rain jacket, already ticked off.")
	if question != "" {
		text = i18n.T("That is the rain jacket line; tick it once it is packed.")
	}
	answer := protocol.OniAnswer{Text: text, Mode: protocol.OniModeSelect, Point: &spot, Seq: o.guides.strand, Walk: true, Route: route, Destination: destination, Path: "toplevel"}
	return protocol.OniAskResult{Status: protocol.OniAskAnswered, Answer: &answer}
}
