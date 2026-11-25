package daemon

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"image"
	"image/jpeg"
	"path/filepath"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/oni/assign"
	"ryoku-rashin/internal/oni/capture"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Assignment mode solves the problem the perched window shows, the way a
// question about it is asked: one picture, read-only, no tools, shown on a
// card with its route before it is sent. With no pin it goes to the
// smallest local model that reads pictures and never to a paid one; a
// hosted route is the user's own pin and still shows the card. Two lanes
// run at once: the fast one paints the answer first, the checker streams
// the steps, and when they disagree a judge on the same route says which
// stands. Spend is counted per local day: a hosted route stops at the
// spec's attempt or token cap, and once a hosted reply came back unmetered
// it waits for the next day; a local route is counted and never stopped.
// No picture, problem, answer or step is written anywhere; the ledger
// rows hold the lane, the route, the tokens and the outcome only.

// oniAssign is assignment mode: the solved views kept to go back to, the
// one shown and its step, the card waiting, and the day's ledger.
type oniAssign struct {
	mu      sync.Mutex
	running bool
	work    int
	style   string
	cache   *assign.Cache
	shown   int
	step    int
	// looked is the last view looked at, for the gate
	looked *assign.View
	card   *assignCard
	asked  map[string]bool

	ledgerOnce sync.Once
	ledger     *assign.Ledger
}

// assignCard is one picture waiting on its answer.
type assignCard struct {
	id    string
	conn  *connection
	view  assign.View
	pic   []byte
	plan  llm.Plan
	timer presence.Timer
}

func (c *assignCard) clear() {
	if c.timer != nil {
		c.timer.Stop()
	}
	clear(c.pic)
	c.pic = nil
}

// assignLimits is the spec's assignment section.
func assignLimits() spec.Assignment {
	if s, err := oniPhase16Spec(); err == nil {
		return s.Assignment
	}
	return spec.Assignment{}
}

// assignLedger is the day's ledger, opened on first use.
func (d *Daemon) assignLedger() *assign.Ledger {
	a := &d.oni.assign
	a.ledgerOnce.Do(func() {
		l := assignLimits()
		path := filepath.Join(paths.StateDir(), "oni", "assignment-day.json")
		a.ledger = assign.NewLedger(path, assign.Caps{Attempts: l.AttemptsPerDay, Tokens: l.TokensPerDay}, 3*l.AttemptsPerDay, time.Local)
	})
	return a.ledger
}

// Why assignment mode refused, or stopped at a cap.
const (
	assignRefusedSensitive = "sensitive"
	assignRefusedNotFound  = "not-found"
	assignRefusedReply     = "reply"
	assignCappedAttempts   = "attempts"
	assignCappedTokens     = "tokens"
	assignCappedUnmetered  = "unmetered"
)

// assignRefusal says why assignment mode may not run, or nil.
func assignRefusal(settings protocol.OniSettings) error {
	decision := feature.Decide(feature.Request{Feature: feature.Assignment}, settings)
	if decision.Allow {
		return nil
	}
	message := i18n.T("Assignment mode is off. Turn it on, with looking at windows, in Oni's settings.")
	if decision.Reason == feature.ReasonOniOff {
		message = i18n.T("Oni is off.")
	}
	return &protocol.Error{Code: protocol.CodeDisabled, Message: message, Reason: decision.Reason}
}

// oniAssignmentCall serves oni.assignment.
func (d *Daemon) oniAssignmentCall(c *connection, p protocol.OniAssignmentParams) (protocol.OniAssignmentResult, error) {
	now := d.reflexClock().Now()
	if p.Action == protocol.OniAssignmentStatus {
		return d.assignStatus(now), nil
	}
	settings := protocol.EffectiveOni(d.Settings())
	if err := assignRefusal(settings); err != nil {
		return protocol.OniAssignmentResult{}, err
	}
	switch p.Action {
	case protocol.OniAssignmentStart:
		style := p.Style
		if style == "" {
			style = settings.Assignment.Style
		}
		a := &d.oni.assign
		a.mu.Lock()
		a.running, a.style = true, style
		if a.cache == nil {
			a.cache, a.shown = assign.NewCache(assignLimits().CacheViews), -1
		}
		a.mu.Unlock()
		return d.assignLook(c, settings, now)
	case protocol.OniAssignmentNext, protocol.OniAssignmentBack:
		return d.assignMove(c, settings, now, p.Action == protocol.OniAssignmentNext)
	}
	d.assignStop()
	res := d.assignStatus(now)
	res.Status = protocol.OniAssignmentStopped
	return res, nil
}

// assignStatus is where assignment mode stands, with the day's spend.
func (d *Daemon) assignStatus(now time.Time) protocol.OniAssignmentResult {
	a := &d.oni.assign
	a.mu.Lock()
	defer a.mu.Unlock()
	res := protocol.OniAssignmentResult{Status: protocol.OniAssignmentIdle, Day: d.assignLedger().Day(now)}
	if !a.running {
		return res
	}
	res.Status = protocol.OniAssignmentWatching
	if s, ok := a.cache.At(a.shown); ok {
		res.Status = protocol.OniAssignmentAnswered
		res.Answer, res.Steps, res.Step = s.Solve.Answer, s.Solve.Steps, a.step
	}
	if a.card != nil {
		res.Status = protocol.OniAssignmentConsent
	}
	return res
}

// assignStop ends assignment mode: the solved views, the view looked at
// and the card are dropped. The day's spend stays.
func (d *Daemon) assignStop() {
	a := &d.oni.assign
	a.mu.Lock()
	defer a.mu.Unlock()
	d.assignCardCloseLocked(false)
	if a.cache != nil {
		a.cache.Clear()
	}
	a.running, a.looked, a.shown, a.step, a.asked = false, nil, -1, 0, nil
}

// assignMove moves through the solved view's steps, back to the views
// before it, or, past the last step, looks at the window again.
func (d *Daemon) assignMove(c *connection, settings protocol.OniSettings, now time.Time, next bool) (protocol.OniAssignmentResult, error) {
	a := &d.oni.assign
	a.mu.Lock()
	if !a.running {
		a.mu.Unlock()
		return protocol.OniAssignmentResult{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Start assignment mode first."))
	}
	s, ok := a.cache.At(a.shown)
	switch {
	case ok && next && a.step < len(s.Solve.Steps):
		a.step++
	case ok && !next && a.step > 1:
		a.step--
	case !next && a.shown > 0:
		a.shown--
		prev, _ := a.cache.At(a.shown)
		a.step = max(len(prev.Solve.Steps), 1)
	case next:
		a.mu.Unlock()
		return d.assignLook(c, settings, now)
	}
	a.mu.Unlock()
	res := d.assignStatus(now)
	res.Cached = true
	d.assignShowStep(res)
	return res, nil
}

// assignShowStep fills the bubble with the step shown.
func (d *Daemon) assignShowStep(res protocol.OniAssignmentResult) {
	svc := d.oni.service.Load()
	if svc == nil || res.Step < 1 || res.Step > len(res.Steps) {
		return
	}
	_ = svc.Bubble(0, "answer", oniText(i18n.Tf("Step %d: %s", res.Step, res.Steps[res.Step-1]), oniBubbleMax))
}

// assignLook looks at the perched window: the same problem as the last
// view stays as it is; one solved before comes back with no request; a new
// one is planned, capped and shown on its card, or sent when its route was
// already said yes to.
func (d *Daemon) assignLook(c *connection, settings protocol.OniSettings, now time.Time) (protocol.OniAssignmentResult, error) {
	svc := d.oni.service.Load()
	o := &d.oni.perch
	o.mu.Lock()
	w, provider := o.window, o.provider
	o.mu.Unlock()
	if svc == nil || w.ID == "" || provider == nil {
		return protocol.OniAssignmentResult{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Perch Oni on a window first."))
	}
	if d.deps.Gateway == nil {
		return protocol.OniAssignmentResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("No model is set up to look at pictures."))
	}
	ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
	defer cancel()
	shot, _, err := d.oniCapture(provider).Capture(ctx, w, d.oniMasks(ctx, w.Output))
	if err != nil {
		return protocol.OniAssignmentResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", err.Error())
	}
	defer clearShot(shot)
	words, err := d.oniWords(ctx, shot)
	if err != nil {
		words = nil
	}
	day := d.assignLedger().Day(now)
	if _, refused := screenask.Sensitive("", words, feature.Box{X: 0, Y: 0, W: 1, H: 1}); refused {
		return protocol.OniAssignmentResult{Status: protocol.OniAssignmentRefused, Refused: assignRefusedSensitive, Day: day}, nil
	}
	l := assignLimits()
	view := assign.View{Gray: shot.Gray(l.Gate.WidthPx, l.Gate.HeightPx, nil), Words: words}
	gate := assign.Gate(l.Gate)
	a := &d.oni.assign
	a.mu.Lock()
	if a.looked != nil && !gate.NewProblem(*a.looked, view).New {
		a.mu.Unlock()
		res := d.assignStatus(now)
		if res.Status == protocol.OniAssignmentAnswered {
			res.Status, res.Cached = protocol.OniAssignmentWatching, true
		}
		return res, nil
	}
	if _, i, ok := a.cache.Find(gate, view); ok {
		// The remembered fingerprint outlives shot's borrowed buffers.
		view.Gray = bytes.Clone(view.Gray)
		a.looked, a.shown, a.step = &view, i, 1
		a.mu.Unlock()
		res := d.assignStatus(now)
		res.Cached = true
		d.assignPaint(res)
		return res, nil
	}
	a.mu.Unlock()
	plan, err := d.assignPlan(ctx, settings)
	if err != nil {
		return protocol.OniAssignmentResult{}, err
	}
	if c != nil {
		if err := hostedNeedsApp(c, plan.Route()); err != nil {
			return protocol.OniAssignmentResult{}, err
		}
	}
	if res, capped := d.assignCapped(now, plan); capped {
		return res, nil
	}
	pic, err := shot.JPEG(capture.AskLongSide, capture.AskQuality)
	if err != nil {
		return protocol.OniAssignmentResult{}, protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("The picture could not be prepared: %v.", err))
	}
	// Both the solved cache and a pending card retain this fingerprint
	// after the capture is cleared; they share the immutable owned copy.
	view.Gray = bytes.Clone(view.Gray)
	card := &assignCard{conn: c, view: view, pic: pic, plan: plan}
	a.mu.Lock()
	must := feature.MustAsk(feature.Assignment, settings, a.asked[plan.Route()])
	a.mu.Unlock()
	if !must {
		return d.assignSolve(card)
	}
	// Only consent defers sending until after shot has been cleared.
	card.pic = bytes.Clone(pic)
	return d.assignCardOut(svc, card, shot, now)
}

// assignPlan names the route: the user's pin, or the smallest local model
// that reads pictures, never a hosted or paid one without a pin.
func (d *Daemon) assignPlan(ctx context.Context, settings protocol.OniSettings) (llm.Plan, error) {
	req := oniPlanRequest()
	if pin := settings.Assignment.Model; pin != "" {
		req.Model, req.Strict, req.Light = pin, true, false
	} else {
		req.LocalOnly = true
	}
	plan, err := d.deps.Gateway.Plan(ctx, req)
	if err != nil {
		return llm.Plan{}, noRouteError(err)
	}
	if settings.Assignment.Model == "" && (!plan.Local || plan.Paid) {
		return llm.Plan{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("No model on this computer reads pictures. Pick one for assignments in Oni's settings."))
	}
	return plan, nil
}

// assignCapped reports whether today's cap stops a solve on plan's route,
// with the result that says so.
func (d *Daemon) assignCapped(now time.Time, plan llm.Plan) (protocol.OniAssignmentResult, bool) {
	ledger := d.assignLedger()
	err := ledger.Allow(now, plan.Local)
	if err == nil {
		return protocol.OniAssignmentResult{}, false
	}
	res := protocol.OniAssignmentResult{Status: protocol.OniAssignmentCapped, Day: ledger.Day(now)}
	switch {
	case errors.Is(err, assign.ErrAttempts):
		res.Refused = assignCappedAttempts
	case errors.Is(err, assign.ErrTokens):
		res.Refused = assignCappedTokens
	default:
		res.Refused = assignCappedUnmetered
	}
	if svc := d.oni.service.Load(); svc != nil {
		_ = svc.Bubble(0, "warn", i18n.T("That's today's limit for solving on a hosted model."))
	}
	return res, true
}

// assignCardOut shows the picture, its route and size on the card.
func (d *Daemon) assignCardOut(svc *presence.Service, card *assignCard, shot *capture.Shot, now time.Time) (protocol.OniAssignmentResult, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		card.clear()
		return protocol.OniAssignmentResult{}, err
	}
	card.id = feature.AssignmentCardPrefix + hex.EncodeToString(b[:])
	var preview bytes.Buffer
	if thumb := shot.Thumb(oniPreviewPx); thumb != nil {
		if err := jpeg.Encode(&preview, thumb, &jpeg.Options{Quality: 80}); err != nil {
			card.clear()
			return protocol.OniAssignmentResult{}, protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("The preview could not be prepared: %v.", err))
		}
	}
	size := screenask.Size{}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(card.pic)); err == nil {
		size = screenask.Size{W: cfg.Width, H: cfg.Height}
	}
	route := card.plan.Route()
	destination, question := "hosted", i18n.Tf("Send this picture of the problem to %s to solve it?", route)
	if card.plan.Local {
		destination, question = "local", i18n.Tf("Show this picture of the problem to %s, on this computer, to solve it?", route)
	}
	question = oniText(question, 400)
	a := &d.oni.assign
	a.mu.Lock()
	d.assignCardCloseLocked(false)
	card.timer = d.reflexClock().AfterFunc(feature.ConsentTTL, func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.card == card {
			d.assignCardCloseLocked(false)
		}
	})
	a.card = card
	a.mu.Unlock()
	if err := svc.Consent(*feature.AssignmentCard(card.id, destination, route, question)); err != nil {
		a.mu.Lock()
		if a.card == card {
			d.assignCardCloseLocked(false)
		}
		a.mu.Unlock()
		return protocol.OniAssignmentResult{}, presenceRefusal(err)
	}
	return protocol.OniAssignmentResult{Status: protocol.OniAssignmentConsent, Day: d.assignLedger().Day(now), Consent: &protocol.OniConsent{
		ID: card.id, Destination: destination, Route: route, Question: question, Preview: preview.Bytes(),
		Width: size.W, Height: size.H, ExpiresMs: feature.ConsentTTL.Milliseconds(),
	}}, nil
}

// assignCardCloseLocked drops the card and its picture. a.mu is held.
func (d *Daemon) assignCardCloseLocked(yes bool) {
	a := &d.oni.assign
	card := a.card
	if card == nil {
		return
	}
	a.card = nil
	if card.timer != nil {
		card.timer.Stop()
		card.timer = nil
	}
	if !yes {
		card.clear()
	}
	answer := "no"
	if yes {
		answer = "yes"
	}
	if d.oni.service.Load() != nil {
		d.oniEmit(&feature.ConsentAnswerFrame{ID: card.id, Answer: answer})
	}
}

// assignCardAnswer serves oni.consent.answer for the assignment card: the
// connection that started it, or the app. A yes keeps the route as said
// yes to while assignment mode runs. It reports false when id is not the
// card.
func (d *Daemon) assignCardAnswer(c *connection, p protocol.OniConsentAnswerParams) (protocol.OniAskResult, bool, error) {
	if !strings.HasPrefix(p.ID, feature.AssignmentCardPrefix) {
		return protocol.OniAskResult{}, false, nil
	}
	a := &d.oni.assign
	a.mu.Lock()
	card := a.card
	if card == nil || card.id != p.ID {
		a.mu.Unlock()
		return protocol.OniAskResult{}, true, oniOfferGone()
	}
	if card.conn != c && !c.app {
		a.mu.Unlock()
		return protocol.OniAskResult{}, true, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only whoever asked, or the Hyakki app, answers this question."))
	}
	allow := p.Answer == "yes"
	d.assignCardCloseLocked(allow)
	if allow {
		if a.asked == nil {
			a.asked = map[string]bool{}
		}
		a.asked[card.plan.Route()] = true
	}
	a.mu.Unlock()
	if !allow {
		return protocol.OniAskResult{Status: protocol.OniAskDeclined}, true, nil
	}
	if err := assignRefusal(protocol.EffectiveOni(d.Settings())); err != nil {
		card.clear()
		return protocol.OniAskResult{}, true, err
	}
	if res, capped := d.assignCapped(d.reflexClock().Now(), card.plan); capped {
		card.clear()
		return protocol.OniAskResult{Status: protocol.OniAskDeclined, Answer: &protocol.OniAnswer{Refused: res.Refused, Text: i18n.T("That's today's limit for solving on a hosted model."), Route: card.plan.Route()}}, true, nil
	}
	res, err := d.assignSolve(card)
	if err != nil {
		return protocol.OniAskResult{}, true, err
	}
	answer := &protocol.OniAnswer{Text: res.Answer, Refused: res.Refused, Step: res.Step, Route: card.plan.Route(), Destination: "hosted"}
	if card.plan.Local {
		answer.Destination = "local"
	}
	return protocol.OniAskResult{Status: protocol.OniAskAnswered, Answer: answer}, true, nil
}

// laneReply is one lane's parsed reply and its ledger row.
type laneReply struct {
	solve assign.Solve
	err   error
}

// assignSolve runs the two lanes on card's picture: the fast answer is
// painted as soon as it comes, the checker's steps after it, and a judge on
// the same route settles a disagreement. The picture is cleared before it
// returns.
func (d *Daemon) assignSolve(card *assignCard) (protocol.OniAssignmentResult, error) {
	defer card.clear()
	if !d.activityAssignBegin() {
		return protocol.OniAssignmentResult{}, context.Canceled
	}
	defer d.activityAssignEnd()
	l := assignLimits()
	ledger := d.assignLedger()
	clock := d.reflexClock()
	if err := ledger.Attempt(clock.Now(), card.plan.Local); err != nil {
		d.warn("oni assignment: %v", err)
	}
	a := &d.oni.assign
	a.mu.Lock()
	walk := a.style != protocol.OniAssignmentAnswers
	a.mu.Unlock()
	steps := assign.StepLimits{Min: l.Steps.Min, Max: l.Steps.Max}
	fast := make(chan laneReply, 1)
	checker := make(chan laneReply, 1)
	pic := bytes.Clone(card.pic)
	defer clear(pic)
	go func() {
		text, err := d.assignLane(card.plan, protocol.OniAssignmentLaneFast, assign.FastSystem, pic, l.Fast(), l.Hedge())
		if err != nil {
			fast <- laneReply{err: err}
			return
		}
		s, err := assign.ParseFast(text)
		fast <- laneReply{solve: s, err: err}
	}()
	if walk {
		go func() {
			text, err := d.assignLane(card.plan, protocol.OniAssignmentLaneChecker, assign.Checker(steps), pic, l.Checker(), 0)
			if err != nil {
				checker <- laneReply{err: err}
				return
			}
			s, err := assign.ParseChecker(text, steps)
			checker <- laneReply{solve: s, err: err}
		}()
	}
	f := <-fast
	if f.err == nil && f.solve.Found {
		if svc := d.oni.service.Load(); svc != nil {
			_ = svc.Bubble(0, "answer", oniText(i18n.Tf("Answer: %s", f.solve.Answer), oniBubbleMax))
		}
	}
	solved := f.solve
	if walk {
		ch := <-checker
		switch {
		case ch.err != nil && f.err != nil:
			return d.assignFailed(f.err), nil
		case ch.err != nil:
		case f.err != nil || !f.solve.Found:
			solved = ch.solve
		default:
			solved.Steps, solved.Problem, solved.Check, solved.Confidence = ch.solve.Steps, ch.solve.Problem, ch.solve.Check, ch.solve.Confidence
			if !assign.Same(f.solve.Answer, ch.solve.Answer) && !d.assignJudge(card.plan, f.solve.Answer, ch.solve.Answer) {
				solved.Answer = ch.solve.Answer
			}
		}
	} else if f.err != nil {
		return d.assignFailed(f.err), nil
	}
	now := clock.Now()
	if !solved.Found {
		return protocol.OniAssignmentResult{Status: protocol.OniAssignmentRefused, Refused: assignRefusedNotFound, Day: ledger.Day(now)}, nil
	}
	a.mu.Lock()
	if !a.running {
		a.mu.Unlock()
		return protocol.OniAssignmentResult{Status: protocol.OniAssignmentStopped, Day: ledger.Day(now)}, nil
	}
	a.cache.Put(assign.Solved{View: card.view, Solve: solved})
	view := card.view
	a.looked, a.shown, a.step = &view, a.cache.Len()-1, 1
	a.mu.Unlock()
	res := d.assignStatus(now)
	d.assignShowStep(res)
	return res, nil
}

// assignFailed is the result of a solve no lane answered.
func (d *Daemon) assignFailed(err error) protocol.OniAssignmentResult {
	refused := assignRefusedReply
	if !errors.Is(err, assign.ErrShape) && !errors.Is(err, assign.ErrSteps) && !errors.Is(err, assign.ErrPrivate) {
		d.warn("oni assignment: %v", err)
	}
	if svc := d.oni.service.Load(); svc != nil {
		_ = svc.Bubble(0, "warn", i18n.T("I couldn't solve that one."))
	}
	return protocol.OniAssignmentResult{Status: protocol.OniAssignmentRefused, Refused: refused, Day: d.assignLedger().Day(d.reflexClock().Now())}
}

// assignLane sends one lane's request to the planned route alone, pinned,
// with no tools, within timeout, and writes its ledger row. A lane with a
// hedge is sent once more, hedge after it began, only when the first send
// failed for a passing reason: never for a rate limit or a missing route.
func (d *Daemon) assignLane(plan llm.Plan, lane, system string, pic []byte, timeout, hedge time.Duration) (string, error) {
	clock := d.reflexClock()
	began := clock.Now()
	text, err := d.assignSend(plan, lane, system, pic, timeout)
	if err == nil || hedge <= 0 || !passing(err) {
		return text, err
	}
	if wait := hedge - clock.Now().Sub(began); wait > 0 {
		time.Sleep(wait)
	}
	return d.assignSend(plan, lane, system, pic, timeout)
}

func (d *Daemon) assignSend(plan llm.Plan, lane, system string, pic []byte, timeout time.Duration) (string, error) {
	if !d.activityAssignBegin() {
		return "", context.Canceled
	}
	defer d.activityAssignEnd()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req := oniRequest(system, i18n.T("Solve the problem in this picture."), pic)
	req.Model, req.Strict, req.LocalOnly = plan.Route(), true, plan.Local
	text, final, err := d.streamText(ctx, req)
	d.assignRecord(plan, lane, final, err)
	return text, err
}

// assignJudge asks the same route whether two answers mean the same; a
// failed judge keeps the checker's.
func (d *Daemon) assignJudge(plan llm.Plan, fast, checker string) bool {
	if !d.activityAssignBegin() {
		return false
	}
	defer d.activityAssignEnd()
	ctx, cancel := context.WithTimeout(context.Background(), assignLimits().Fast())
	defer cancel()
	req := llm.Request{Model: plan.Route(), Strict: true, LocalOnly: plan.Local, Messages: []llm.Message{
		{Role: llm.RoleSystem, Content: assign.JudgeSystem},
		{Role: llm.RoleUser, Content: assign.Judge(fast, checker)},
	}}
	text, final, err := d.streamText(ctx, req)
	d.assignRecord(plan, protocol.OniAssignmentLaneJudge, final, err)
	return err == nil && assign.JudgeSame(text)
}

// assignRecord writes one lane's row: route, measured tokens and their
// flags, and outcome, never what was sent or said.
func (d *Daemon) assignRecord(plan llm.Plan, lane string, final llm.Final, err error) {
	outcome := "ok"
	if err != nil {
		outcome = "failed"
	}
	row := protocol.OniAssignmentRow{At: d.reflexClock().Now(), Lane: lane, Route: plan.Route(), Input: final.Usage.Input, Output: final.Usage.Output,
		Estimated: final.Usage.Estimated, Unmetered: final.Usage.Unmetered, Outcome: outcome}
	if err := d.assignLedger().Record(row, plan.Local); err != nil {
		d.warn("oni assignment: %v", err)
	}
}

// passing reports an error a second send may get past: not a rate limit,
// not a missing route, not a cancel.
func passing(err error) bool {
	var noRoute *llm.NoRoute
	if errors.As(err, &noRoute) || errors.Is(err, context.Canceled) {
		return false
	}
	lower := strings.ToLower(err.Error())
	for _, s := range []string{"429", "rate limit", "too many requests", "quota"} {
		if strings.Contains(lower, s) {
			return false
		}
	}
	return true
}

// oniAssignmentLogCall serves oni.assignment.log: today's spend and rows,
// newest first.
func (d *Daemon) oniAssignmentLogCall() (protocol.OniAssignmentLogResult, error) {
	now := d.reflexClock().Now()
	ledger := d.assignLedger()
	rows := ledger.Rows(now)
	if rows == nil {
		rows = []protocol.OniAssignmentRow{}
	}
	return protocol.OniAssignmentLogResult{Day: ledger.Day(now), Rows: rows}, nil
}

// assignPaint fills the bubble with a solved view's answer.
func (d *Daemon) assignPaint(res protocol.OniAssignmentResult) {
	if svc := d.oni.service.Load(); svc != nil && res.Answer != "" {
		_ = svc.Bubble(0, "answer", oniText(i18n.Tf("Answer: %s", res.Answer), oniBubbleMax))
	}
}

// assignSettings follows the settings: with assignment mode off, it stops
// and the card goes unanswered.
func (d *Daemon) assignSettings(next protocol.Settings) {
	if assignRefusal(protocol.EffectiveOni(next)) == nil {
		return
	}
	d.assignStop()
}
