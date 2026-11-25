package daemon

import (
	"bytes"
	"context"
	"errors"
	"image"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/oni/capture"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/ocr"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/protocol"
)

// Select-and-ask: the app or the CLI arms a pick, the presenter maps its
// pick overlay while the pick frame says armed, and its one click names a
// spot. Oni perches on the window under it, reads the words there on this
// computer, refuses a private field before any model call, then asks about
// the spot as oni.ask asks: the route and the picture first, once a
// session or every time. The connection that armed it may wait on the
// click and gets what the click came to, a question it answers as oni.ask's
// caller does included. Nothing is armed, mapped or timed at rest.

// The pick's spot, in logical pixels around the click.
const (
	oniPickSpotW = 96
	oniPickSpotH = 40
)

// oniPick is the armed pick.
type oniPick struct {
	mu    sync.Mutex
	armed bool
	// conn armed it: the one, with the app, that answers its consent, and
	// whose standing a hosted route is checked against
	conn  *connection
	text  string
	seq   uint64
	timer presence.Timer
	// last is when the last pick was taken, for the repeat guard
	last time.Time
	// waiter is the last arm's: kept past its click, cancel or lapse until
	// its outcome is taken or another arm replaces it
	waiter *pickWaiter
}

// pickWaiter is what the connection that armed a pick waits on: the
// outcome of its click, or ended, given once.
type pickWaiter struct {
	conn *connection
	out  chan pickOutcome
}

type pickOutcome struct {
	res protocol.OniAskResult
	err error
}

// give hands the outcome over unless one already was.
func (w *pickWaiter) give(res protocol.OniAskResult, err error) {
	if w == nil {
		return
	}
	select {
	case w.out <- pickOutcome{res: res, err: err}:
	default:
	}
}

var pickEnded = protocol.OniAskResult{Status: protocol.OniAskEnded}

func oniPickGuard() time.Duration {
	if s, err := oniGuideSpec(); err == nil {
		return time.Duration(s.Guide.PickRepeatGuardMs * float64(time.Millisecond))
	}
	return 1200 * time.Millisecond
}

// oniPickCall answers oni.pick.
func (d *Daemon) oniPickCall(c *connection, p protocol.OniPickParams) (protocol.OniAskResult, error) {
	if err := d.oniCaller(c, true); err != nil {
		return protocol.OniAskResult{}, err
	}
	switch p.Action {
	case protocol.OniPickArm:
		return d.pickArm(c, p.Text)
	case protocol.OniPickPoint:
		return d.pickPoint(c, p)
	case protocol.OniPickWait:
		return d.pickWait(c)
	}
	d.pickDisarm(0)
	return pickEnded, nil
}

// pickWait holds until the pick c armed last was clicked, cancelled or
// lapsed, or c closed, and answers with what it came to.
func (d *Daemon) pickWait(c *connection) (protocol.OniAskResult, error) {
	k := &d.oni.pick
	k.mu.Lock()
	w := k.waiter
	k.mu.Unlock()
	if w == nil || w.conn != c {
		return protocol.OniAskResult{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Nothing this connection armed waits for a click."))
	}
	select {
	case o := <-w.out:
		k.mu.Lock()
		if k.waiter == w {
			k.waiter = nil
		}
		k.mu.Unlock()
		return o.res, o.err
	case <-c.done:
		return pickEnded, nil
	}
}

// pickArm maps the presenter's pick overlay until its click, a cancel or
// the consent time.
func (d *Daemon) pickArm(c *connection, text string) (protocol.OniAskResult, error) {
	if err := guideRefusal(protocol.EffectiveOni(d.Settings()), feature.Inspect); err != nil {
		return protocol.OniAskResult{}, err
	}
	d.oni.clientsMu.Lock()
	drawn := d.oni.presenter != nil
	d.oni.clientsMu.Unlock()
	if d.oni.service.Load() == nil || !drawn {
		return protocol.OniAskResult{}, perchRefusal(protocol.CodeUnavailable, feature.PerchPresenterLost, i18n.T("Oni is not being drawn right now, so a pick has nowhere to land."))
	}
	k := &d.oni.pick
	k.mu.Lock()
	if k.timer != nil {
		k.timer.Stop()
	}
	// an arm still waiting for its click is replaced, and its wait ends; a
	// clicked one's outcome is on its way to its own waiter, so a new arm
	// right after the click (while the click is read) leaves it be
	var old *pickWaiter
	if k.armed {
		old = k.waiter
	}
	k.armed, k.conn, k.text = true, c, strings.TrimSpace(text)
	k.waiter = &pickWaiter{conn: c, out: make(chan pickOutcome, 1)}
	k.seq++
	seq := k.seq
	k.timer = d.oniGuideClock().AfterFunc(feature.ConsentTTL, func() { d.pickDisarm(seq) })
	k.mu.Unlock()
	old.give(pickEnded, nil)
	d.oniEmit(&feature.PickFrame{Seq: seq, Armed: true})
	return protocol.OniAskResult{Status: protocol.OniAskArmed}, nil
}

// pickDisarm unmaps the overlay for a cancel or a lapse, ending the wait on
// it: seq names the arm a lapse belongs to, 0 any arm.
func (d *Daemon) pickDisarm(seq uint64) {
	if w := d.pickOff(seq); w != nil {
		w.give(pickEnded, nil)
	}
}

// pickOff unmaps the overlay and returns the waiter of the arm it ended,
// nil when none was armed: seq names the arm, 0 any arm.
func (d *Daemon) pickOff(seq uint64) *pickWaiter {
	k := &d.oni.pick
	k.mu.Lock()
	if !k.armed || (seq != 0 && seq != k.seq) {
		k.mu.Unlock()
		return nil
	}
	w := k.waiter
	k.armed, k.conn, k.text = false, nil, ""
	if k.timer != nil {
		k.timer.Stop()
		k.timer = nil
	}
	k.seq++
	off := k.seq
	k.mu.Unlock()
	if d.oni.service.Load() != nil {
		d.oniEmit(&feature.PickFrame{Seq: off, Armed: false})
	}
	return w
}

// pickReset forgets the pick when presence stops; nothing is left to draw
// a disarm. A wait on it ends.
func (d *Daemon) pickReset() {
	k := &d.oni.pick
	k.mu.Lock()
	if k.timer != nil {
		k.timer.Stop()
		k.timer = nil
	}
	w := k.waiter
	k.armed, k.conn, k.text, k.waiter = false, nil, "", nil
	k.mu.Unlock()
	w.give(pickEnded, nil)
}

// pickPoint takes the armed pick's one click at p. What it comes to also
// goes to the arm's waiter.
func (d *Daemon) pickPoint(c *connection, p protocol.OniPickParams) (res protocol.OniAskResult, err error) {
	k := &d.oni.pick
	k.mu.Lock()
	if !k.armed {
		k.mu.Unlock()
		return protocol.OniAskResult{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Nothing is waiting for a click. Arm a pick first."))
	}
	asker, text := k.conn, k.text
	now := d.oniGuideClock().Now()
	repeat := !k.last.IsZero() && now.Sub(k.last) < oniPickGuard()
	if !repeat {
		k.last = now
	}
	k.mu.Unlock()
	waiter := d.pickOff(0)
	defer func() { waiter.give(res, err) }()
	if repeat {
		// a click right after the last pick is a double click, not a pick
		return pickEnded, nil
	}
	settings := protocol.EffectiveOni(d.Settings())
	if err := guideRefusal(settings, feature.Inspect); err != nil {
		return protocol.OniAskResult{}, err
	}
	provider, err := d.oniProvider()
	if err != nil {
		return protocol.OniAskResult{}, perchRefusal(protocol.CodeUnavailable, feature.PerchNoGeometry, i18n.Tf("Oni cannot read the windows on this desktop: %v.", err))
	}
	ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
	defer cancel()
	read, stop := context.WithTimeout(ctx, oniWindowRead)
	snap, err := provider.Snapshot(read)
	stop()
	if err != nil {
		return protocol.OniAskResult{}, perchRefusal(protocol.CodeUnavailable, feature.PerchNoGeometry, i18n.Tf("Oni cannot read the windows on this desktop: %v.", err))
	}
	w, found := oniFind(snap, protocol.OniPerchParams{Output: p.Output, X: p.X, Y: p.Y})
	if reason, message := perchable(w, found); reason != "" {
		code := protocol.CodeUnavailable
		if reason == feature.PerchClosed {
			code = protocol.CodeNotFound
			message = i18n.T("There is no window there to ask about.")
		}
		return protocol.OniAskResult{}, perchRefusal(code, reason, message)
	}
	o := &d.oni.perch
	o.mu.Lock()
	perched := o.window.ID
	o.mu.Unlock()
	if perched != w.ID {
		if _, err := d.oniPerchCall(c, protocol.OniPerchParams{Window: string(w.ID)}); err != nil {
			return protocol.OniAskResult{}, err
		}
	}
	spot := pickSpot(snap, w, p)
	svc := d.oni.service.Load()
	if svc == nil {
		return protocol.OniAskResult{}, perchRefusal(protocol.CodeUnavailable, feature.PerchPresenterLost, i18n.T("Nothing is drawing Oni, so it cannot perch."))
	}
	if d.deps.Gateway == nil {
		return protocol.OniAskResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("No model is set up to look at pictures."))
	}
	plan, err := d.deps.Gateway.Plan(ctx, oniPlanRequest())
	if err != nil {
		return protocol.OniAskResult{}, noRouteError(err)
	}
	if err := hostedNeedsApp(asker, plan.Route()); err != nil {
		return protocol.OniAskResult{}, err
	}
	o.mu.Lock()
	current := o.window
	o.mu.Unlock()
	if current.ID != w.ID {
		return protocol.OniAskResult{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Perch Oni on a window first."))
	}
	shot, disc, err := d.oniCapture(provider).Capture(ctx, current, d.oniMasks(ctx, current.Output))
	if err != nil {
		return protocol.OniAskResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", err.Error())
	}
	// the private-field check reads the words beside the spot here, before
	// any model call; without a text reader there is no pick
	words, err := d.oniWords(ctx, shot)
	if err != nil {
		shot.Clear()
		if errors.Is(err, ocr.ErrUnavailable) {
			return protocol.OniAskResult{}, &protocol.Error{Code: protocol.CodeUnavailable, Reason: oniOCRMissing, Message: i18n.T("Picking a spot needs a text reader on this computer (tesseract), so Oni will not look there.")}
		}
		return protocol.OniAskResult{}, &protocol.Error{Code: protocol.CodeUnavailable, Reason: oniOCRMissing, Message: i18n.Tf("The words on the window could not be read, so Oni will not look there: %v.", err)}
	}
	if _, refused := screenask.Sensitive(text, words, spot); refused {
		shot.Clear()
		// the line goes beside Oni on the perch's own strand; no strand goes
		// to the field
		_ = svc.Bubble(0, "warn", screenask.RefusalLine)
		answer := protocol.OniAnswer{Text: screenask.RefusalLine, Mode: protocol.OniModeSelect, Refused: screenask.CodeSensitive, Route: plan.Route(), Destination: "hosted", Path: string(disc.Path)}
		if plan.Local {
			answer.Destination = "local"
		}
		return protocol.OniAskResult{Status: protocol.OniAskAnswered, Answer: &answer}, nil
	}
	q := &oniQuestion{conn: asker, text: text, window: current, shot: shot, disc: disc, plan: plan, spot: &spot, words: words}
	if q.pic, err = shot.JPEG(capture.AnswerLongSide, capture.AnswerQuality); err != nil {
		shot.Clear()
		return protocol.OniAskResult{}, protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("The picture could not be prepared: %v.", err))
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(q.pic)); err == nil {
		q.size = screenask.Size{W: cfg.Width, H: cfg.Height}
	}
	return d.oniQuestionOut(q, svc, settings)
}

// pickSpot is a small box round the click at p, in fractions of w. The click
// is local to p's output and w's box to w's own output, so both go through
// the global layout: on a second output neither origin is zero.
func pickSpot(snap window.Snapshot, w window.Window, p protocol.OniPickParams) feature.Box {
	var ox, oy float64
	for _, o := range snap.Outputs {
		if o.Name == p.Output {
			ox, oy = float64(o.X), float64(o.Y)
		}
	}
	b, ok := window.Global(w, snap.Outputs)
	if !ok {
		b = *w.Box
	}
	sw, sh := min(oniPickSpotW/b.W, 1), min(oniPickSpotH/b.H, 1)
	x := min(max((ox+p.X-b.X)/b.W-sw/2, 0), 1-sw)
	y := min(max((oy+p.Y-b.Y)/b.H-sh/2, 0), 1-sh)
	return feature.Box{X: x, Y: y, W: sw, H: sh}
}

// oniSpotAsk asks what the picked spot is, in words, and points at it. A
// reply that offers a walk through it leaves the walk offer beside Oni.
func (d *Daemon) oniSpotAsk(ctx context.Context, q *oniQuestion, in screenask.Input, answer *protocol.OniAnswer) error {
	in.Spot, in.Words = q.spot, q.words
	reply, err := d.oniModel(ctx, q, screenask.KindSelect, in, q.pic)
	if err != nil {
		return err
	}
	answer.Mode = protocol.OniModeSelect
	sel, err := screenask.ParseSelection(reply)
	if refusal := (*screenask.Refusal)(nil); errors.As(err, &refusal) {
		answer.Refused, answer.Text = refusal.Code, refusal.Line
		return nil
	} else if err != nil {
		return protocol.Errorf(protocol.CodeInternal, "%s", err.Error())
	}
	answer.Text, answer.Walk = sel.Answer, sel.Walk
	spot := *q.spot
	answer.Point = &protocol.OniRect{X: spot.X, Y: spot.Y, W: spot.W, H: spot.H}
	if svc := d.oni.service.Load(); svc != nil {
		if seq, err := svc.Strand(feature.StrandTargetFrame{X: spot.X, Y: spot.Y, W: spot.W, H: spot.H, Mode: feature.StrandModePoint}); err == nil {
			answer.Seq = seq
		}
	}
	if sel.Walk && feature.Decide(feature.Request{Feature: feature.Guide}, protocol.EffectiveOni(d.Settings())).Allow {
		d.guideOfferWalk(q.conn, firstText(q.text, sel.Answer))
	}
	return nil
}
