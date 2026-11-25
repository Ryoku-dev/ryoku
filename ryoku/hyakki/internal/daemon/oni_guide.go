package daemon

import (
	"bytes"
	"context"
	"errors"
	"image"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/oni/capture"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/guide"
	"ryoku-rashin/internal/oni/keep"
	"ryoku-rashin/internal/oni/ocr"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/screenask"
	fingerprint "ryoku-rashin/internal/oni/settle"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/protocol"
)

// A guide walks the user toward a goal on the perched window one step at a
// time. guide.Machine decides every move; this file carries out what it
// asks and hands back what came of it. Nothing runs while no guide does:
// no timer, no watch and no poll. A step's picture is taken only when the
// machine asks for a read, asked about before it is sent exactly as an
// oni.ask picture is, and cleared once its reply is read; the settle watch
// keeps only small gray fingerprints, and only until the step moves on.
// The model's reply only moves the strand and fills the guide card: no
// reply runs anything, and a guide never clicks or types.
//
// Every guide event runs on one serial queue (guideDo), so the machine
// sees one event at a time. Presence calls are made from that queue with
// no daemon lock held: presence calls back (arrival, cancel, rest, detach)
// only post more work, so nothing waits on itself.

// OniGuideDeps are what guides, Keep and select-and-ask read and keep
// with; each nil field is the real one.
type OniGuideDeps struct {
	// OCR reads the words in a captured picture on this computer; nil is
	// tesseract, through its one allowlisted argv.
	OCR ocr.Reader
	// Keeps is where kept guides live; nil is the store in the state
	// directory.
	Keeps *keep.Store
	// Settle takes one fresh picture of w to fingerprint after a step was
	// advanced; nil captures the window again. A picture with a Clear
	// method is cleared once fingerprinted.
	Settle func(ctx context.Context, w window.Window) (fingerprint.Gray, error)
	// Clock times the reads, the settle frames, the keep offer, the pick
	// and the closing line; nil is the wall clock.
	Clock presence.Clock
}

// What the keep and walk offers ask, and the reason a pick or a replay
// needs the text reader.
var (
	oniKeepQuestion = i18n.T("Keep these steps for another time?")
	oniWalkLine     = i18n.T("Want me to walk you through it? Press Next.")
)

// oniOCRMissing is the reason a pick is refused when no text reader is
// installed: the private-field check before any model call needs it.
const oniOCRMissing = "ocr-unavailable"

// oniRefusedAppOnly is the refusal code of a guide whose model read the
// planned route keeps for the Hyakki app, started by another caller.
const oniRefusedAppOnly = "app-only"

// oniGuide is the guide that runs, its keep offer and the walk offer a
// select answer left. Only the queue's drainer reads or writes the fields
// below the queue.
type oniGuide struct {
	mu       sync.Mutex
	queue    []func()
	draining bool

	keepsMu sync.Mutex
	keeps   *keep.Store
	hints   *keep.Hints

	// gen counts guides; seq is the last guide frame sent, across guides
	gen  uint64
	seq  uint64
	run  *guideRun
	walk *guideWalk
}

// guideRun is one guide from its start until its idle frame.
type guideRun struct {
	gen  uint64
	m    *guide.Machine
	conn *connection
	// asker is the connection that answers the consent question of the
	// next read: the starter, then whoever last waited on Next or Circle
	asker atomic.Pointer[connection]
	plan  llm.Plan
	// window is the perched window the guide started on
	window window.Window
	goal   string
	path   string
	// voice is the talk source a spoken question started the guide from,
	// "" otherwise; said is the last card line spoken for it
	voice string
	said  string

	// work are the reads, looks and settle watches in flight, by token;
	// parked is the read waiting on its consent answer, consent the
	// question shown for it
	work    map[uint64]*guideWork
	parked  uint64
	consent *protocol.OniConsent

	// held is the picture just read while its reply is handled; base is
	// the fingerprint of the picture the current step was read from
	held *capture.Shot
	base *fingerprint.Print

	out     uint64
	box     feature.Box
	frame   feature.GuideFrame
	finish  string
	refused string
	offer   *guideOffer
	end     *guide.End
	idle    presence.Timer
	waiters []chan guideAnswer
}

type guideWork struct {
	cancel   context.CancelFunc
	deadline presence.Timer
}

// guideOffer is the offer to keep a finished guide.
type guideOffer struct {
	id    string
	entry keep.Entry
	timer presence.Timer
}

// guideWalk is the offer, after a select answer, to walk through it, or a
// hint offering a kept walkthrough.
type guideWalk struct {
	goal  string
	conn  *connection
	timer presence.Timer
	// hint is the hint this offer is, nil for a walk offer
	hint *guideHint
}

// guideAsk ties a step's picture to the read that wants it.
type guideAsk struct {
	run   *guideRun
	token uint64
	done  []string
	words []screenask.Word
	ctx   context.Context
}

type guideAnswer struct {
	res protocol.OniAskResult
	err error
}

func (d *Daemon) oniGuideClock() presence.Clock {
	if d.deps.OniGuide.Clock != nil {
		return d.deps.OniGuide.Clock
	}
	return presence.SystemClock()
}

func (d *Daemon) oniOCR() ocr.Reader {
	if d.deps.OniGuide.OCR != nil {
		return d.deps.OniGuide.OCR
	}
	return ocr.New(ocr.Exec(os.Environ()), "eng")
}

// oniKeeps is the kept-guides store, opened on first use.
func (d *Daemon) oniKeeps() *keep.Store {
	g := &d.oni.guide
	g.keepsMu.Lock()
	defer g.keepsMu.Unlock()
	if g.keeps == nil {
		g.keeps = d.deps.OniGuide.Keeps
		if g.keeps == nil {
			g.keeps = keep.NewStore("")
		}
	}
	return g.keeps
}

var oniGuideSpec = sync.OnceValues(spec.Load)

// oniGuideConfig is the machine's numbers from the spec, with the read
// timeout the Oni lab may shorten.
func oniGuideConfig(getenv func(string) string) guide.Config {
	var c guide.Config
	if s, err := oniGuideSpec(); err == nil {
		c = guide.FromSpec(s)
	}
	if t := oniGuideReadTimeout(getenv); t > 0 {
		c.ReadTimeout = t
	}
	return c
}

// oniGuideReadTimeout is, in the Oni lab only (HYAKKI_ONI_LAB=1), the read
// timeout HYAKKI_ONI_GUIDE_READ_TIMEOUT_MS names, 1000 to 75000 ms; zero
// otherwise.
func oniGuideReadTimeout(getenv func(string) string) time.Duration {
	if getenv("HYAKKI_ONI_LAB") != "1" {
		return 0
	}
	ms, err := strconv.Atoi(getenv("HYAKKI_ONI_GUIDE_READ_TIMEOUT_MS"))
	if err != nil || ms < 1000 || ms > 75000 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}

func oniSettleConfig() fingerprint.Config {
	if s, err := oniGuideSpec(); err == nil {
		return fingerprint.FromSpec(s)
	}
	return fingerprint.Config{}
}

// guideDo runs f on the guide's queue: at once when nothing drains it,
// else after what is queued, on the goroutine draining it.
func (d *Daemon) guideDo(f func()) {
	g := &d.oni.guide
	g.mu.Lock()
	g.queue = append(g.queue, f)
	if g.draining {
		g.mu.Unlock()
		return
	}
	g.draining = true
	for len(g.queue) > 0 {
		next := g.queue[0]
		g.queue[0] = nil
		g.queue = g.queue[1:]
		g.mu.Unlock()
		next()
		d.guideRest()
		g.mu.Lock()
	}
	g.queue = nil
	g.draining = false
	g.mu.Unlock()
}

// guideWait waits for a guide call's outcome.
func (d *Daemon) guideWait(ch chan guideAnswer) (protocol.OniAskResult, error) {
	select {
	case a := <-ch:
		return a.res, a.err
	case <-time.After(oniAskTimeout):
		return protocol.OniAskResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("The guide did not answer in time."))
	}
}

// guideRest hands the running guide's outcome to the calls waiting on it
// once nothing but the user, the presenter or a consent answer is awaited.
// It runs on the queue.
func (d *Daemon) guideRest() {
	run := d.oni.guide.run
	if run == nil || len(run.waiters) == 0 {
		return
	}
	if run.end == nil {
		for token := range run.work {
			if token != run.parked {
				return
			}
		}
	}
	run.flush()
}

func (r *guideRun) flush() {
	res := r.result()
	for _, ch := range r.waiters {
		ch <- guideAnswer{res: res}
	}
	r.waiters = nil
}

// live says run is the guide that runs and has not ended. On the queue.
func (d *Daemon) guideLive(run *guideRun) bool {
	return run != nil && d.oni.guide.run == run && run.end == nil
}

// answer is the guide's state as the wire answers it.
func (r *guideRun) answer() *protocol.OniAnswer {
	a := &protocol.OniAnswer{Mode: protocol.OniModeGuide, Step: r.frame.Step, Replay: r.frame.Replay, Path: r.path}
	switch {
	case r.frame.Replay:
		a.Destination = feature.EgressLocal
	case r.plan.Local:
		a.Route, a.Destination = r.plan.Route(), "local"
	default:
		a.Route, a.Destination = r.plan.Route(), "hosted"
	}
	return a
}

func (r *guideRun) result() protocol.OniAskResult {
	switch {
	case r.end != nil:
		a := r.answer()
		a.Text = oniText(r.end.Line, oniBubbleMax)
		a.Done = r.end.Reason == guide.EndDone
		if r.end.Reason == guide.EndRefused {
			a.Refused = firstText(r.refused, screenask.CodeSensitive)
		}
		return protocol.OniAskResult{Status: protocol.OniAskEnded, Answer: a}
	case r.offer != nil:
		a := r.answer()
		a.Text = oniText(firstText(r.finish, i18n.T("That is it. You are done.")), oniBubbleMax)
		a.Done = true
		offer := &protocol.OniKeepOffer{ID: r.offer.id, Question: oniKeepQuestion, ExpiresMs: feature.KeepOfferTTL().Milliseconds()}
		return protocol.OniAskResult{Status: protocol.OniAskEnded, Answer: a, Keep: offer}
	case r.consent != nil:
		c := *r.consent
		return protocol.OniAskResult{Status: protocol.OniAskConsent, Consent: &c}
	}
	a := r.answer()
	a.Text = r.frame.Text
	if r.frame.Target != 0 {
		a.Seq = r.frame.Target
		a.Point = &protocol.OniRect{X: r.box.X, Y: r.box.Y, W: r.box.W, H: r.box.H}
	}
	return protocol.OniAskResult{Status: protocol.OniAskAnswered, Answer: a}
}

// guideRefusal says why feature id may not run, or nil.
func guideRefusal(settings protocol.OniSettings, id string) error {
	decision := feature.Decide(feature.Request{Feature: id}, settings)
	if decision.Allow {
		return nil
	}
	message := i18n.T("Looking at windows is off. Turn it on in Oni's settings.")
	switch {
	case decision.Reason == feature.ReasonOniOff:
		message = i18n.T("Oni is off.")
	case id == feature.Guide:
		message = i18n.T("Guides are off. Turn them and looking at windows on in Oni's settings.")
	case id == feature.GuideKeep:
		message = i18n.T("Keeping guides is off. Turn it on in Oni's settings.")
	}
	return &protocol.Error{Code: protocol.CodeDisabled, Message: message, Reason: decision.Reason}
}

// oniGuideCall answers oni.guide.
func (d *Daemon) oniGuideCall(c *connection, p protocol.OniGuideParams) (protocol.OniAskResult, error) {
	if err := d.oniCaller(c, true); err != nil {
		return protocol.OniAskResult{}, err
	}
	switch p.Action {
	case protocol.OniGuideStart:
		return d.guideStart(c, p.Goal)
	case protocol.OniGuideNext:
		if res, took, err := d.guideNextWalk(c, ""); took {
			return res, err
		}
	}
	ch := make(chan guideAnswer, 1)
	d.guideDo(func() {
		g := &d.oni.guide
		run := g.run
		if run == nil || run.end != nil {
			if p.Action == protocol.OniGuideStop {
				d.guideDropWalk(true)
				ch <- guideAnswer{res: protocol.OniAskResult{Status: protocol.OniAskEnded}}
				return
			}
			ch <- guideAnswer{err: protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("No guide is running. Start one first."))}
			return
		}
		run.waiters = append(run.waiters, ch)
		// whoever waits on Next or Circle answers the picture it asks about,
		// unless it is the presenter's card, which answers no question
		if p.Action != protocol.OniGuideStop && !d.oniIsPresenter(c) {
			run.asker.Store(c)
		}
		switch p.Action {
		case protocol.OniGuideNext:
			d.guideHandle(run, guide.Advance{By: guide.ByNext})
		case protocol.OniGuideCircle:
			d.guideHandle(run, guide.Circle{})
		default:
			d.guideHandle(run, guide.Stop{Why: "user"})
		}
	})
	return d.guideWait(ch)
}

// guideNextWalk takes the walk offer for Next when no guide runs and
// starts its guide as c, who waits on it and answers its pictures' consent
// questions as with Next in a running guide; the presenter's card and a
// talk press (c nil, spoken from voice) answer no question, so for them it
// starts as walk.conn when it has one. took is false when no offer waits.
func (d *Daemon) guideNextWalk(c *connection, voice string) (res protocol.OniAskResult, took bool, err error) {
	walk := d.guideTakeWalk()
	if walk == nil {
		return protocol.OniAskResult{}, false, nil
	}
	if walk.conn != nil && (c == nil || d.oniIsPresenter(c)) {
		c = walk.conn
	}
	res, err = d.guideBegin(c, walk.goal, voice)
	if err != nil {
		d.guideDo(func() {
			if d.oni.guide.run == nil && d.oni.guide.walk == nil {
				d.guideIdleFrame()
			}
		})
	}
	return res, true, err
}

// guideStart starts a guide toward goal on the perched window, replaying a
// kept one when Keep is on and one matches.
func (d *Daemon) guideStart(c *connection, goal string) (protocol.OniAskResult, error) {
	return d.guideBegin(c, goal, "")
}

// guideBegin is guideStart for c, or, with c nil, for the user's own talk
// press: its source is voice, and the guide's card lines are spoken. The
// pictures are asked about the same way whoever started it.
func (d *Daemon) guideBegin(c *connection, goal, voice string) (protocol.OniAskResult, error) {
	settings := protocol.EffectiveOni(d.Settings())
	if err := guideRefusal(settings, feature.Guide); err != nil {
		return protocol.OniAskResult{}, err
	}
	svc := d.oni.service.Load()
	o := &d.oni.perch
	o.mu.Lock()
	w, provider := o.window, o.provider
	o.mu.Unlock()
	if svc == nil || w.ID == "" || provider == nil {
		return protocol.OniAskResult{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Perch Oni on a window first."))
	}
	if _, ok := svc.Perched(); !ok {
		return protocol.OniAskResult{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Perch Oni on a window first."))
	}
	if d.deps.Gateway == nil {
		return protocol.OniAskResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("No model is set up to look at pictures."))
	}
	ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
	plan, err := d.deps.Gateway.Plan(ctx, oniPlanRequest())
	cancel()
	if err != nil {
		return protocol.OniAskResult{}, noRouteError(err)
	}
	cfg := oniGuideConfig(os.Getenv)
	cfg.Keep = feature.Decide(feature.Request{Feature: feature.GuideKeep}, settings).Allow
	var entry *keep.Entry
	if cfg.Keep {
		if e, _, ok := d.oniKeeps().Match(w.App, goal); ok {
			entry = &e
		}
	}
	// a kept replay sends nothing, so the route's rules wait for a model
	// read, should a step miss and fall through to one
	if c != nil && entry == nil {
		if err := hostedNeedsApp(c, plan.Route()); err != nil {
			return protocol.OniAskResult{}, err
		}
	}
	ch := make(chan guideAnswer, 1)
	d.guideDo(func() {
		g := &d.oni.guide
		d.guideDropWalk(false)
		if old := g.run; old != nil {
			d.guideRetire(old, "replaced")
		}
		g.gen++
		cfg.SeqBase = g.seq
		run := &guideRun{gen: g.gen, conn: c, plan: plan, window: w, goal: strings.TrimSpace(goal), voice: voice, work: map[uint64]*guideWork{}, waiters: []chan guideAnswer{ch}}
		run.asker.Store(c)
		run.m = guide.New(cfg, d.oniGuideClock().Now)
		g.run = run
		d.guideHandle(run, guide.Start{Goal: run.goal, App: w.App, Entry: entry})
	})
	return d.guideWait(ch)
}

// guideRetire ends run for a guide that takes its place: no closing frame
// is left to wait on. On the queue.
func (d *Daemon) guideRetire(run *guideRun, why string) {
	if run.end == nil {
		d.guideHandle(run, guide.Stop{Why: why})
	}
	if run.idle != nil {
		run.idle.Stop()
		run.idle = nil
	}
	run.flush()
	if d.oni.guide.run == run {
		d.oni.guide.run = nil
	}
}

// guideAdvance tells the running guide the user acted on its step: Next,
// a spoken next from a valid press, or a change on the perched window.
func (d *Daemon) guideAdvance(by string) {
	d.guideDo(func() {
		if run := d.oni.guide.run; d.guideLive(run) {
			d.guideHandle(run, guide.Advance{By: by})
		}
	})
}

// guideUnwind stops the running guide and drops a walk offer: dismissed,
// detached, the window closed, Oni off, the presenter gone or the settings
// changed. It never waits, so a caller may hold a lock.
func (d *Daemon) guideUnwind(why string) {
	go d.guideDo(func() {
		if run := d.oni.guide.run; d.guideLive(run) {
			d.guideHandle(run, guide.Stop{Why: why})
		}
		d.guideDropWalk(true)
	})
}

// guideUnwindRun stops run alone: its consent question went stale or
// unanswered.
func (d *Daemon) guideUnwindRun(run *guideRun, why string) {
	go d.guideDo(func() {
		if d.guideLive(run) {
			d.guideHandle(run, guide.Stop{Why: why})
		}
	})
}

// guideUnwindOff stops the running guide when it is not on window id: Oni
// perched elsewhere.
func (d *Daemon) guideUnwindOff(id window.ID) {
	go d.guideDo(func() {
		if run := d.oni.guide.run; d.guideLive(run) && run.window.ID != id {
			d.guideHandle(run, guide.Stop{Why: "perch"})
		}
	})
}

// guideArrived and guideCancelled pass the strand's landing and its end on
// to the running guide.
func (d *Daemon) guideArrived(seq uint64, skipped string) {
	d.guideDo(func() {
		run := d.oni.guide.run
		if !d.guideLive(run) {
			return
		}
		if skipped != "" && run.out == seq {
			run.out = 0
		}
		d.guideHandle(run, guide.Arrived{Seq: seq, Skipped: skipped})
	})
}

func (d *Daemon) guideCancelled(seq uint64, why string) {
	d.guideDo(func() {
		run := d.oni.guide.run
		if !d.guideLive(run) {
			return
		}
		if run.out == seq {
			run.out = 0
		}
		d.guideHandle(run, guide.Cancelled{Seq: seq, Why: why})
	})
}

func (d *Daemon) guideHandle(run *guideRun, ev guide.Event) {
	d.guideApply(run, run.m.Handle(ev))
}

// guideApply carries out the machine's effects in order. A Point is
// answered with its Pointed before anything else. On the queue.
func (d *Daemon) guideApply(run *guideRun, effects []guide.Effect) {
	for len(effects) > 0 {
		e := effects[0]
		effects = effects[1:]
		switch e := e.(type) {
		case guide.Show:
			d.guideShow(run, e.Frame)
		case guide.ReadScreen:
			ctx := d.guideWork(run, e.Token, e.Until)
			go d.guideRead(run, e, ctx)
		case guide.Look:
			ctx := d.guideWork(run, e.Token, e.Until)
			go d.guideLook(run, e.Token, ctx)
		case guide.Point:
			seq, err := d.guidePoint(run, e)
			effects = append(run.m.Handle(guide.Pointed{Token: e.Token, Seq: seq, Err: err}), effects...)
		case guide.Retract:
			if svc := d.oni.service.Load(); svc != nil && run.out == e.Seq {
				// the next picture waits for the presenter to draw this
				at := time.Now()
				if svc.Retract() {
					d.oniMoved(at)
				}
			}
			if run.out == e.Seq {
				run.out = 0
			}
		case guide.CancelRead:
			d.guideCancel(run, e.Token)
		case guide.SetDeadline:
			d.guideRearm(run, e.Token, e.Until)
		case guide.StartSettle:
			d.guideSettle(run, e)
		case guide.OfferKeep:
			d.guideOffer(run, e.Entry)
		case guide.Hit:
			if err := d.oniKeeps().Hit(e.Key); err != nil {
				d.warn("oni keep hit: %v", err)
			}
		case guide.Miss:
			if _, err := d.oniKeeps().Miss(e.Key); err != nil {
				d.warn("oni keep miss: %v", err)
			}
		case guide.Forget:
			run.refused = screenask.CodeSensitive
			if err := d.oniKeeps().Forget(e.Key); err != nil {
				d.warn("oni keep forget: %v", err)
			}
		case guide.End:
			d.guideEnded(run, e)
		}
	}
}

func (d *Daemon) guideShow(run *guideRun, f feature.GuideFrame) {
	run.frame = f
	d.oni.guide.seq = max(d.oni.guide.seq, f.Seq)
	if svc := d.oni.service.Load(); svc != nil {
		_ = svc.Guide(f)
	}
	d.voiceGuideFrame(run, f)
}

// guideIdleFrame takes the guide card down. On the queue.
func (d *Daemon) guideIdleFrame() {
	g := &d.oni.guide
	g.seq++
	if svc := d.oni.service.Load(); svc != nil {
		_ = svc.Guide(feature.GuideFrame{Seq: g.seq, Phase: feature.GuideIdle})
	}
}

// guideWork records a read or look with token, armed with its deadline.
func (d *Daemon) guideWork(run *guideRun, token uint64, until time.Time) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	run.work[token] = &guideWork{cancel: cancel, deadline: d.guideDeadline(run, token, until)}
	return ctx
}

// guideDeadline arms the deadline of the work with token at until.
func (d *Daemon) guideDeadline(run *guideRun, token uint64, until time.Time) presence.Timer {
	clock := d.oniGuideClock()
	return clock.AfterFunc(until.Sub(clock.Now()), func() {
		d.guideDo(func() {
			if d.guideLive(run) {
				d.guideHandle(run, guide.Deadline{Token: token})
			}
		})
	})
}

// guideRearm moves the deadline of the work with token: held, with a zero
// until, while its picture waits on a yes, and armed at until once the
// picture is allowed.
func (d *Daemon) guideRearm(run *guideRun, token uint64, until time.Time) {
	w := run.work[token]
	if w == nil {
		return
	}
	if w.deadline != nil {
		w.deadline.Stop()
		w.deadline = nil
	}
	if !until.IsZero() {
		w.deadline = d.guideDeadline(run, token, until)
	}
}

// guideFinish forgets the work with token once its answer is in.
func (run *guideRun) finishWork(token uint64) bool {
	w := run.work[token]
	if w == nil {
		return false
	}
	if w.deadline != nil {
		w.deadline.Stop()
	}
	w.cancel()
	delete(run.work, token)
	return true
}

// guideCancel abandons the work with token: its late answer is dropped,
// and a picture waiting on its consent answer is cleared.
func (d *Daemon) guideCancel(run *guideRun, token uint64) {
	run.finishWork(token)
	if run.parked == token {
		run.parked, run.consent = 0, nil
	}
	d.guideDropQuestion(run)
}

// guideDropQuestion clears the consent question run's read left waiting.
func (d *Daemon) guideDropQuestion(run *guideRun) {
	a := &d.oni.ask
	a.mu.Lock()
	q := a.pending
	if q == nil || q.guide == nil || q.guide.run != run {
		a.mu.Unlock()
		return
	}
	a.pending = nil
	a.gen++
	a.mu.Unlock()
	q.clear()
}

// guideShot captures the perched window, the guide's own, with the
// presenter's pixels masked out once they held still.
func (d *Daemon) guideShot(ctx context.Context, w window.Window) (*capture.Shot, capture.Disclosure, error) {
	o := &d.oni.perch
	o.mu.Lock()
	current, provider := o.window, o.provider
	o.mu.Unlock()
	if provider == nil || current.ID != w.ID {
		return nil, capture.Disclosure{}, errors.New(i18n.T("Oni is no longer perched on that window."))
	}
	return d.oniCapture(provider).Capture(ctx, current, d.oniMasks(ctx, current.Output))
}

// oniWords reads the words in shot on this computer.
func (d *Daemon) oniWords(ctx context.Context, shot *capture.Shot) ([]screenask.Word, error) {
	pic, err := shot.PNG()
	if err != nil {
		return nil, err
	}
	defer clear(pic)
	return d.oniOCR().Words(ctx, pic)
}

func clearShot(s *capture.Shot) {
	if s != nil {
		s.Clear()
	}
}

// guideRead takes the step's picture and reads its words here, then asks
// before it is sent as oni.ask does, or sends it.
func (d *Daemon) guideRead(run *guideRun, e guide.ReadScreen, ctx context.Context) {
	fail := func(err error) {
		d.guideDo(func() { d.guideReadDone(run, e.Token, nil, "", screenask.Pointing{}, err) })
	}
	// a hosted route is the app's alone, as at the start; a replay that
	// fell through to this read meets the rule here, before any picture
	if run.conn != nil {
		if err := hostedNeedsApp(run.conn, run.plan.Route()); err != nil {
			fail(&guide.Barred{Line: err.Error()})
			return
		}
	}
	shot, disc, err := d.guideShot(ctx, run.window)
	if err != nil {
		fail(err)
		return
	}
	q := &oniQuestion{conn: run.asker.Load(), text: run.goal, window: run.window, shot: shot, disc: disc, plan: run.plan}
	if q.pic, err = shot.JPEG(capture.AskLongSide, capture.AskQuality); err != nil {
		q.clear()
		fail(err)
		return
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(q.pic)); err == nil {
		q.size = screenask.Size{W: cfg.Width, H: cfg.Height}
	}
	// the words are read here, before anything is sent; without a text
	// reader the step is a best guess
	words, _ := d.oniWords(ctx, shot)
	q.guide = &guideAsk{run: run, token: e.Token, done: e.Done, words: words, ctx: ctx}
	if ctx.Err() != nil {
		q.clear()
		return
	}
	settings := protocol.EffectiveOni(d.Settings())
	a := &d.oni.ask
	a.mu.Lock()
	if !feature.MustAsk(feature.Guide, settings, a.asked[q.plan.Route()]) {
		a.mu.Unlock()
		d.guideSend(q)
		return
	}
	consent, err := q.consent()
	if err != nil {
		a.mu.Unlock()
		q.clear()
		fail(err)
		return
	}
	old := a.pending
	a.gen++
	gen := a.gen
	q.timer = a.now().AfterFunc(feature.ConsentTTL, func() { d.oniConsentExpired(gen) })
	a.pending = q
	a.mu.Unlock()
	if old != nil {
		d.oniQuestionDropped(old)
	}
	d.guideDo(func() {
		if !d.guideLive(run) || run.work[e.Token] == nil {
			d.guideDropQuestion(run)
			return
		}
		// the read's deadline waits with the picture; the question's own
		// expiry bounds the wait
		d.guideHandle(run, guide.Asking{Token: e.Token})
		if !d.guideLive(run) {
			return
		}
		run.parked, run.consent, run.path = e.Token, consent, string(disc.Path)
		if svc := d.oni.service.Load(); svc != nil {
			_ = svc.Consent(feature.ConsentFrame{
				ID: consent.ID, Feature: feature.Guide, Class: feature.ConsentDisclosure,
				Destination: consent.Destination, Route: consent.Route, Question: consent.Question,
				ExpiresMs: consent.ExpiresMs,
			})
		}
	})
}

// guideSend sends the step's picture to the planned route alone, reads the
// reply as a guide step and hands it to the guide.
func (d *Daemon) guideSend(q *oniQuestion) {
	ga := q.guide
	in := screenask.Input{Question: ga.run.goal, Goal: ga.run.goal, Done: ga.done, App: q.window.App, Title: q.window.Title, Words: ga.words}
	reply, err := d.oniModel(ga.ctx, q, screenask.KindGuide, in, q.pic)
	q.pic = nil
	var p screenask.Pointing
	if err == nil {
		p, err = screenask.ParseStep(reply, nil, ga.words, q.size)
	}
	d.guideDo(func() {
		defer q.clear()
		d.guideReadDone(ga.run, ga.token, q.shot, string(q.disc.Path), p, err)
	})
}

// guideReadDone hands a read's reply to the guide that asked, unless the
// read was abandoned. On the queue.
func (d *Daemon) guideReadDone(run *guideRun, token uint64, shot *capture.Shot, path string, p screenask.Pointing, err error) {
	if !d.guideLive(run) || !run.finishWork(token) {
		return
	}
	if path != "" {
		run.path = path
	}
	if refusal := (*screenask.Refusal)(nil); errors.As(err, &refusal) {
		run.refused = refusal.Code
	} else if barred := (*guide.Barred)(nil); errors.As(err, &barred) {
		run.refused = oniRefusedAppOnly
	} else if err == nil && p.Kind == screenask.KindNone && p.Done {
		run.finish = p.Say
	}
	run.held = shot
	d.guideHandle(run, guide.Read{Token: token, Pointing: p, Err: err})
	run.held = nil
}

// guideConsented sends the step's picture the user said yes to and waits
// for the guide's next rest.
func (d *Daemon) guideConsented(q *oniQuestion) (protocol.OniAskResult, error) {
	ch := make(chan guideAnswer, 1)
	ga := q.guide
	d.guideDo(func() {
		run := ga.run
		if !d.guideLive(run) || run.parked != ga.token || run.work[ga.token] == nil {
			q.clear()
			ch <- guideAnswer{err: protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That question has expired or was answered."))}
			return
		}
		run.parked, run.consent = 0, nil
		run.waiters = append(run.waiters, ch)
		// the read starts now, with its whole timeout
		d.guideHandle(run, guide.Allowed{Token: ga.token})
		go d.guideSend(q)
	})
	return d.guideWait(ch)
}

// guideDeclined ends the guide whose step's picture the user said no to.
func (d *Daemon) guideDeclined(q *oniQuestion) {
	q.clear()
	ga := q.guide
	done := make(chan struct{})
	d.guideDo(func() {
		defer close(done)
		if run := ga.run; d.guideLive(run) && run.parked == ga.token {
			run.parked, run.consent = 0, nil
			d.guideHandle(run, guide.Stop{Why: "declined"})
		}
	})
	<-done
}

// guideLook reads a kept step's window on this computer: a capture and
// its words, and nothing sent anywhere.
func (d *Daemon) guideLook(run *guideRun, token uint64, ctx context.Context) {
	shot, disc, err := d.guideShot(ctx, run.window)
	var words []screenask.Word
	if err == nil {
		words, err = d.oniWords(ctx, shot)
	}
	d.guideDo(func() {
		defer clearShot(shot)
		if !d.guideLive(run) || !run.finishWork(token) {
			return
		}
		run.path = string(disc.Path)
		run.held = shot
		d.guideHandle(run, guide.Looked{Token: token, Words: words, Err: err})
		run.held = nil
	})
}

// guidePoint sends the strand to the step's box. The fingerprint the
// settle watch compares against is taken here, from the picture the step
// was just read from.
func (d *Daemon) guidePoint(run *guideRun, e guide.Point) (uint64, error) {
	if run.held != nil {
		if p, err := oniSettleConfig().Take(run.held, e.Box); err == nil {
			run.clearBase()
			run.base = &p
		}
	}
	svc := d.oni.service.Load()
	if svc == nil {
		return 0, presence.ErrNoPresenter
	}
	t := feature.StrandTargetFrame{X: e.Box.X, Y: e.Box.Y, W: e.Box.W, H: e.Box.H, Label: oniText(e.Label, 120), Mode: e.Mode}
	if e.Mode == feature.StrandModeRegion && e.Region != nil {
		r := *e.Region
		t.Region = &r
	}
	seq, err := svc.Strand(t)
	if err == nil {
		run.out, run.box = seq, e.Box
	}
	return seq, err
}

func (r *guideRun) clearBase() {
	if r.base != nil {
		r.base.Clear()
		r.base = nil
	}
}

// guideSettle watches whether the window changed after the step was
// advanced, for at most the spec's frames.
func (d *Daemon) guideSettle(run *guideRun, e guide.StartSettle) {
	ctx, cancel := context.WithCancel(context.Background())
	run.work[e.Token] = &guideWork{cancel: cancel}
	var base *fingerprint.Print
	if run.base != nil {
		p := run.base.Clone()
		base = &p
	}
	go func() {
		changed := d.guideWatch(ctx, run.window, e.Box, base)
		if base != nil {
			base.Clear()
		}
		if ctx.Err() != nil {
			return
		}
		d.guideDo(func() {
			if d.guideLive(run) && run.finishWork(e.Token) {
				d.guideHandle(run, guide.Settled{Token: e.Token, Changed: changed})
			}
		})
	}()
}

// guideWatch fingerprints fresh pictures of w one frame apart: the window
// changed once one moved from base, and the watch ends early once two in a
// row are still. With no base it naps and reports no change.
func (d *Daemon) guideWatch(ctx context.Context, w window.Window, box feature.Box, base *fingerprint.Print) bool {
	cfg := oniSettleConfig()
	if base == nil || cfg.MaxFrames <= 0 {
		d.guideSleep(ctx, cfg.Nap)
		return false
	}
	changed := false
	var prev *fingerprint.Print
	defer func() {
		if prev != nil {
			prev.Clear()
		}
	}()
	for i := range cfg.MaxFrames {
		if i > 0 && !d.guideSleep(ctx, cfg.Frame) {
			return false
		}
		p, err := d.guideFrame(ctx, w, box, cfg)
		if err != nil {
			continue
		}
		switch {
		case !changed:
			changed = cfg.Moved(fingerprint.Delta(*base, p))
		case prev != nil && cfg.Still(fingerprint.Delta(*prev, p)):
			p.Clear()
			return true
		}
		if prev != nil {
			prev.Clear()
		}
		prev = &p
	}
	return changed
}

// guideFrame fingerprints one fresh picture of w; the picture is cleared
// at once.
func (d *Daemon) guideFrame(ctx context.Context, w window.Window, box feature.Box, cfg fingerprint.Config) (fingerprint.Print, error) {
	var g fingerprint.Gray
	if src := d.deps.OniGuide.Settle; src != nil {
		got, err := src(ctx, w)
		if err != nil {
			return fingerprint.Print{}, err
		}
		g = got
	} else {
		shot, _, err := d.guideShot(ctx, w)
		if err != nil {
			return fingerprint.Print{}, err
		}
		g = shot
	}
	p, err := cfg.Take(g, box)
	if c, ok := g.(interface{ Clear() }); ok {
		c.Clear()
	}
	return p, err
}

// guideSleep waits d on the guide's clock, or until ctx ends.
func (d *Daemon) guideSleep(ctx context.Context, wait time.Duration) bool {
	done := make(chan struct{})
	t := d.oniGuideClock().AfterFunc(wait, func() { close(done) })
	select {
	case <-done:
		return true
	case <-ctx.Done():
		t.Stop()
		return false
	}
}

// guideEnded drops everything the guide still holds: work in flight, a
// picture waiting on consent, the fingerprints, the keep offer. The closing
// frame stays for as long as its line takes to read, then the card goes.
// On the queue.
func (d *Daemon) guideEnded(run *guideRun, e guide.End) {
	run.end = &e
	for token := range run.work {
		run.finishWork(token)
	}
	run.parked, run.consent = 0, nil
	d.guideDropQuestion(run)
	run.clearBase()
	d.guideCloseOffer(run, false)
	if d.oni.service.Load() == nil {
		run.flush()
		if d.oni.guide.run == run {
			d.oni.guide.run = nil
		}
		return
	}
	dwell := screenask.Dwell(len(strings.Fields(e.Line)))
	run.idle = d.oniGuideClock().AfterFunc(dwell, func() {
		d.guideDo(func() {
			if d.oni.guide.run != run {
				return
			}
			run.idle = nil
			d.oni.guide.run = nil
			d.guideIdleFrame()
		})
	})
}

// guideOfferWalk offers, beside a select answer, to walk through it: a
// hint-like card with Next. It lapses with the hint time. A running guide
// is never interrupted for it.
func (d *Daemon) guideOfferWalk(conn *connection, goal string) {
	goal = strings.TrimSpace(goal)
	if utf8.RuneCountInString(goal) > protocol.OniGuideGoalMax {
		goal = strings.TrimSpace(string([]rune(goal)[:protocol.OniGuideGoalMax]))
	}
	if goal == "" {
		return
	}
	d.guideDo(func() {
		g := &d.oni.guide
		if d.guideLive(g.run) {
			return
		}
		if g.run != nil {
			d.guideRetire(g.run, "walk")
		}
		d.guideDropWalk(false)
		svc := d.oni.service.Load()
		if svc == nil {
			return
		}
		g.seq++
		if err := svc.Guide(feature.GuideFrame{Seq: g.seq, Phase: feature.GuideNudging, Text: oniWalkLine, Actions: []string{feature.GuideActionNext}}); err != nil {
			return
		}
		w := &guideWalk{goal: goal, conn: conn}
		w.timer = d.oniGuideClock().AfterFunc(feature.HintOfferTTL(), func() {
			d.guideDo(func() {
				if d.oni.guide.walk == w {
					d.guideDropWalk(true)
				}
			})
		})
		g.walk = w
	})
}

// guideDropWalk drops the walk offer, taking its card down when idle says
// so. On the queue.
func (d *Daemon) guideDropWalk(idle bool) {
	g := &d.oni.guide
	w := g.walk
	if w == nil {
		return
	}
	g.walk = nil
	w.timer.Stop()
	d.hintAnswer(w, false)
	if idle && g.run == nil {
		d.guideIdleFrame()
	}
}

// guideTakeWalk takes the walk offer for Next when no guide runs.
func (d *Daemon) guideTakeWalk() *guideWalk {
	ch := make(chan *guideWalk, 1)
	d.guideDo(func() {
		g := &d.oni.guide
		w := g.walk
		if w == nil || d.guideLive(g.run) {
			ch <- nil
			return
		}
		g.walk = nil
		w.timer.Stop()
		d.hintAnswer(w, true)
		ch <- w
	})
	return <-ch
}
