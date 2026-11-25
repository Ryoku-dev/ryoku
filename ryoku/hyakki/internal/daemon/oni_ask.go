package daemon

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/oni/capture"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/protocol"
)

// A question about the perched window is read-only from end to end: the
// model gets one picture of that window and no tools, and its reply only
// moves the strand and fills the bubble. The picture lives in memory, never
// on disk or in a session, and is cleared on every way the question ends:
// sent, declined, expired, replaced, detached or Oni stopping.

// Limits on one question.
const (
	oniPreviewPx  = 320
	oniAskTimeout = 3 * time.Minute
	oniReplyMax   = 64 << 10
	oniBubbleMax  = 600
	// oniSelectTokens bounds the answer about a picked spot
	oniSelectTokens = 400
)

// oniAsk is the question waiting on its consent answer and the one being
// sent. asked holds the routes the user said yes to while presence runs.
type oniAsk struct {
	mu sync.Mutex
	// clock arms the consent expiry; nil is the wall clock
	clock   presence.Clock
	pending *oniQuestion
	asked   map[string]bool
	gen     uint64
	// cancel ends the request being sent
	cancel context.CancelFunc
}

// oniQuestion is one capture and where it would go. A guide step's carries
// guide, the read that wants it; a pick's carries the spot picked and the
// words read on this computer.
type oniQuestion struct {
	id     string
	conn   *connection
	text   string
	window window.Window
	shot   *capture.Shot
	pic    []byte
	size   screenask.Size
	disc   capture.Disclosure
	plan   llm.Plan
	timer  presence.Timer
	guide  *guideAsk
	// hands is the Hands step the picture is for; its question carries
	// the page's text and control names too
	hands *handsQuestion
	spot  *feature.Box
	words []screenask.Word
	// answered is set for a question the MCP bridge asked: it always waits
	// for the person's answer, and gets what that answer came to, since
	// the bridge cannot answer it
	answered chan oniAskReply
}

func (q *oniQuestion) clear() {
	if q.timer != nil {
		q.timer.Stop()
		q.timer = nil
	}
	clearShot(q.shot)
	q.pic = nil
	q.words = nil
}

// oniQuestionDropped clears a question another took the place of, or one
// nobody can answer any more; a guide step's ends its guide.
func (d *Daemon) oniQuestionDropped(q *oniQuestion) {
	q.clear()
	q.reply(protocol.OniAskResult{Status: protocol.OniAskDeclined}, nil)
	if q.guide != nil {
		d.guideUnwindRun(q.guide.run, "consent")
	}
	if q.hands != nil {
		d.handsQuestionLost(q)
	}
}

func (a *oniAsk) now() presence.Clock {
	if a.clock != nil {
		return a.clock
	}
	return presence.SystemClock()
}

// oniAskCall captures the perched window for text, or for a description
// when text is empty, and either asks before it is sent or, when the user
// already said yes to this route for the session, sends it.
func (d *Daemon) oniAskCall(c *connection, text string) (protocol.OniAskResult, error) {
	if err := d.oniCaller(c, false); err != nil {
		return protocol.OniAskResult{}, err
	}
	settings := protocol.EffectiveOni(d.Settings())
	if decision := feature.Decide(feature.Request{Feature: feature.Inspect}, settings); !decision.Allow {
		message := i18n.T("Looking at windows is off. Turn it on in Oni's settings.")
		if decision.Reason == feature.ReasonOniOff {
			message = i18n.T("Oni is off.")
		}
		return protocol.OniAskResult{}, &protocol.Error{Code: protocol.CodeDisabled, Message: message, Reason: decision.Reason}
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
	defer cancel()
	plan, err := d.deps.Gateway.Plan(ctx, oniPlanRequest())
	if err != nil {
		return protocol.OniAskResult{}, noRouteError(err)
	}
	if err := hostedNeedsApp(c, plan.Route()); err != nil {
		return protocol.OniAskResult{}, err
	}
	shot, disc, err := d.oniCapture(provider).Capture(ctx, w, d.oniMasks(ctx, w.Output))
	if err != nil {
		return protocol.OniAskResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", err.Error())
	}
	q := &oniQuestion{conn: c, text: text, window: w, shot: shot, disc: disc, plan: plan}
	c.mu.RLock()
	bridge := c.kind == protocol.ClientMCP
	c.mu.RUnlock()
	if bridge {
		q.answered = make(chan oniAskReply, 1)
	}
	if q.pic, err = shot.JPEG(capture.AskLongSide, capture.AskQuality); err != nil {
		shot.Clear()
		return protocol.OniAskResult{}, protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("The picture could not be prepared: %v.", err))
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(q.pic)); err == nil {
		q.size = screenask.Size{W: cfg.Width, H: cfg.Height}
	}
	res, err := d.oniQuestionOut(q, svc, settings)
	if bridge && err == nil && res.Status == protocol.OniAskConsent {
		return d.oniAskAwait(q)
	}
	return res, err
}

// oniQuestionOut asks before q's picture is sent, or, when the user already
// said yes to its route for the session, sends it. A question still waiting
// is dropped for it.
func (d *Daemon) oniQuestionOut(q *oniQuestion, svc *presence.Service, settings protocol.OniSettings) (protocol.OniAskResult, error) {
	a := &d.oni.ask
	a.mu.Lock()
	if a.pending != nil {
		d.oniQuestionDropped(a.pending)
		a.pending = nil
	}
	must := q.answered != nil || feature.MustAsk(feature.Inspect, settings, a.asked[q.plan.Route()])
	if !must {
		a.mu.Unlock()
		return d.oniSend(q)
	}
	consent, err := q.consent()
	if err != nil {
		a.mu.Unlock()
		q.clear()
		return protocol.OniAskResult{}, err
	}
	a.gen++
	gen := a.gen
	q.timer = a.now().AfterFunc(feature.ConsentTTL, func() { d.oniConsentExpired(gen) })
	a.pending = q
	a.mu.Unlock()
	if err := svc.Consent(feature.ConsentFrame{
		ID: q.id, Feature: feature.Inspect, Class: feature.ConsentDisclosure,
		Destination: consent.Destination, Route: consent.Route, Question: consent.Question,
		ExpiresMs: consent.ExpiresMs,
	}); err != nil {
		d.oniAskEnd()
		return protocol.OniAskResult{}, presenceRefusal(err)
	}
	return protocol.OniAskResult{Status: protocol.OniAskConsent, Consent: consent}, nil
}

// consent is the question to show: where the picture goes, the path that
// took it, the exact picture as a thumbnail and its size.
func (q *oniQuestion) consent() (*protocol.OniConsent, error) {
	id, err := consentID()
	if err != nil {
		return nil, protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("The question could not be prepared: %v.", err))
	}
	if q.hands != nil {
		id = feature.HandsStepCardPrefix + strings.TrimPrefix(id, "c-")
	}
	q.id = id
	var preview bytes.Buffer
	if thumb := q.shot.Thumb(oniPreviewPx); thumb != nil {
		if err := jpeg.Encode(&preview, thumb, &jpeg.Options{Quality: 80}); err != nil {
			return nil, protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("The preview could not be prepared: %v.", err))
		}
	}
	route := q.plan.Route()
	destination, question := "hosted", i18n.Tf("Send this picture of %s to %s?", q.windowName(), route)
	if q.plan.Local {
		destination, question = "local", i18n.Tf("Show this picture of %s to %s, on this computer?", q.windowName(), route)
	}
	if q.hands != nil {
		question = i18n.Tf("Send this picture of the tab, with its text and the names of its controls, to %s?", route)
		if q.plan.Local {
			question = i18n.Tf("Show this picture of the tab, with its text and the names of its controls, to %s, on this computer?", route)
		}
		switch q.hands.run.scope {
		case protocol.OniHandsScopeWindow:
			question = i18n.Tf("Send this picture of the window you picked, with the words read in it, to %s?", route)
			if q.plan.Local {
				question = i18n.Tf("Show this picture of the window you picked, with the words read in it, to %s, on this computer?", route)
			}
		case protocol.OniHandsScopeNested, protocol.OniHandsScopeDesktop:
			question = i18n.Tf("Send this picture of Hyakki's separate desktop, with the words read in it, to %s?", route)
			if q.plan.Local {
				question = i18n.Tf("Show this picture of Hyakki's separate desktop, with the words read in it, to %s, on this computer?", route)
			}
		}
	}
	return &protocol.OniConsent{
		ID: q.id, Destination: destination, Route: route,
		Path: string(q.disc.Path), Disclosure: q.disc.Text, ScopeExact: q.disc.ScopeExact,
		Question: oniText(question, 400), Preview: preview.Bytes(),
		Width: q.size.W, Height: q.size.H, ExpiresMs: feature.ConsentTTL.Milliseconds(),
	}, nil
}

func (q *oniQuestion) windowName() string {
	if t := oniText(q.window.Title, 80); t != "" {
		return t
	}
	if a := oniText(q.window.App, 80); a != "" {
		return a
	}
	return i18n.T("this window")
}

// oniConsentAnswer takes the answer to the waiting question: yes sends the
// picture shown, no clears it. Only the connection that asked, or the app,
// answers; a guide step's picture is also answered by whoever waits on the
// guide's Next or Circle now, and the MCP bridge's by the surface drawing
// Oni, never by the bridge. The reflex decision card is answered here too,
// by the app only.
func (d *Daemon) oniConsentAnswer(c *connection, p protocol.OniConsentAnswerParams) (protocol.OniAskResult, error) {
	// a Yokai approval card is the presenter's or the app's to answer
	if res, ok, err := d.approvalCardAnswer(c, p); ok {
		return res, err
	}
	// a question the MCP bridge asked is the presenter's or the app's
	if res, ok, err := d.bridgeConsentAnswer(c, p); ok {
		return res, err
	}
	if err := d.oniCaller(c, false); err != nil {
		return protocol.OniAskResult{}, err
	}
	if res, ok, err := d.reflexCardAnswer(c, p); ok {
		return res, err
	}
	if strings.HasPrefix(p.ID, feature.GuardCardPrefix) && !c.app {
		return protocol.OniAskResult{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Use the focus guard's card in Hyakki to close this feed."))
	}
	if res, ok := d.guardCardAnswer(p); ok {
		return res, nil
	}
	if res, ok, err := d.rewriteCardAnswer(c, p); ok {
		return res, err
	}
	if res, ok, err := d.memoryCardAnswer(c, p); ok {
		return res, err
	}
	if res, ok, err := d.recallCardAnswer(c, p); ok {
		return res, err
	}
	if res, ok, err := d.assignCardAnswer(c, p); ok {
		return res, err
	}
	allow := p.Answer == "yes"
	q, err := d.oniConsentTake(p.ID, allow, func(q *oniQuestion) error {
		if q.answered != nil && !c.app || q.answered == nil && q.conn != c && !c.app && (q.guide == nil || q.guide.run.asker.Load() != c) {
			return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only whoever asked, or the Hyakki app, answers this question."))
		}
		return nil
	})
	if err != nil {
		return protocol.OniAskResult{}, err
	}
	res, err := d.oniConsentGive(q, allow)
	q.reply(res, err)
	return res, err
}

// oniConsentTake takes the waiting question off its slot for an answer:
// the one id names, or whichever waits when id is "". may refuses a
// question the caller cannot answer. A yes keeps its route as said yes to.
func (d *Daemon) oniConsentTake(id string, allow bool, may func(*oniQuestion) error) (*oniQuestion, error) {
	a := &d.oni.ask
	a.mu.Lock()
	defer a.mu.Unlock()
	q := a.pending
	if q == nil || id != "" && q.id != id {
		return nil, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That question has expired or was answered."))
	}
	if may != nil {
		if err := may(q); err != nil {
			return nil, err
		}
	}
	a.pending = nil
	if q.timer != nil {
		q.timer.Stop()
		q.timer = nil
	}
	if allow {
		if a.asked == nil {
			a.asked = map[string]bool{}
		}
		a.asked[q.plan.Route()] = true
	}
	return q, nil
}

// oniConsentGive carries out the answer to a question oniConsentTake took:
// yes sends its picture, or hands it to the guide step it is for; no clears
// it, and ends the guide it was for.
func (d *Daemon) oniConsentGive(q *oniQuestion, allow bool) (protocol.OniAskResult, error) {
	switch {
	case !allow && q.hands != nil:
		d.handsDeclined(q)
	case q.hands != nil:
		return d.handsConsented(q)
	case !allow && q.guide != nil:
		d.guideDeclined(q)
	case !allow:
		q.clear()
	case q.guide != nil:
		return d.guideConsented(q)
	default:
		return d.oniSend(q)
	}
	return protocol.OniAskResult{Status: protocol.OniAskDeclined}, nil
}

func (d *Daemon) oniConsentExpired(gen uint64) {
	a := &d.oni.ask
	a.mu.Lock()
	q := a.pending
	if q == nil || gen != a.gen {
		a.mu.Unlock()
		return
	}
	a.pending = nil
	q.timer = nil
	a.mu.Unlock()
	d.oniQuestionDropped(q)
}

// oniAskEnd clears the waiting question and stops the one being sent: the
// perch ended, Oni stopped or looking was turned off.
func (d *Daemon) oniAskEnd() {
	a := &d.oni.ask
	a.mu.Lock()
	q := a.pending
	a.pending = nil
	a.gen++
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()
	if q != nil {
		d.oniQuestionDropped(q)
	}
}

// oniAskGone clears the question c asked when c goes away unanswered: no
// one else is waiting on it.
func (d *Daemon) oniAskGone(c *connection) {
	a := &d.oni.ask
	a.mu.Lock()
	q := a.pending
	if q == nil || q.conn != c {
		a.mu.Unlock()
		return
	}
	a.pending = nil
	a.gen++
	a.mu.Unlock()
	d.oniQuestionDropped(q)
}

// oniAskReset is oniAskEnd that also forgets the routes said yes to.
func (d *Daemon) oniAskReset() {
	d.oniAskEnd()
	a := &d.oni.ask
	a.mu.Lock()
	a.asked = nil
	a.mu.Unlock()
}

// oniSend sends q's picture to the planned route alone and shows the reply:
// a point moves the strand onto the window, words fill the bubble. The
// picture is cleared before it returns.
func (d *Daemon) oniSend(q *oniQuestion) (protocol.OniAskResult, error) {
	defer q.clear()
	a := &d.oni.ask
	ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
	defer cancel()
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	a.cancel = cancel
	a.mu.Unlock()
	answer := protocol.OniAnswer{Route: q.plan.Route(), Destination: "hosted", Path: string(q.disc.Path)}
	if q.plan.Local {
		answer.Destination = "local"
	}
	in := screenask.Input{Question: q.text, App: q.window.App, Title: q.window.Title}
	var err error
	switch {
	case q.spot != nil:
		err = d.oniSpotAsk(ctx, q, in, &answer)
	case q.text == "":
		err = d.oniAnswer(ctx, q, screenask.ModeDescribe, in, &answer)
	default:
		err = d.oniPoint(ctx, q, in, &answer)
	}
	if err != nil {
		return protocol.OniAskResult{}, err
	}
	if svc := d.oni.service.Load(); svc != nil {
		tone := "answer"
		if answer.Refused != "" {
			tone = "warn"
		}
		_ = svc.Bubble(answer.Seq, tone, oniText(answer.Text, oniBubbleMax))
	}
	return protocol.OniAskResult{Status: protocol.OniAskAnswered, Answer: &answer}, nil
}

// oniPoint asks where the answer to q is on the window. A point that passes
// the parser and the private-field check moves the strand there; a
// question better answered in words is asked again as one.
func (d *Daemon) oniPoint(ctx context.Context, q *oniQuestion, in screenask.Input, answer *protocol.OniAnswer) error {
	reply, err := d.oniModel(ctx, q, screenask.KindPoint, in, q.pic)
	if err != nil {
		return err
	}
	answer.Mode = screenask.KindPoint
	p, err := screenask.ParsePoint(reply, nil, nil, q.size)
	if refusal := (*screenask.Refusal)(nil); errors.As(err, &refusal) {
		answer.Refused, answer.Text = refusal.Code, refusal.Line
		return nil
	} else if err != nil {
		return protocol.Errorf(protocol.CodeInternal, "%s", err.Error())
	}
	switch p.Kind {
	case screenask.KindAnswer:
		pic, err := q.shot.JPEG(capture.AnswerLongSide, capture.AnswerQuality)
		if err != nil {
			return protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("The picture could not be prepared: %v.", err))
		}
		return d.oniAnswerWith(ctx, q, screenask.ModeAsk, in, pic, answer)
	case screenask.KindElement, screenask.KindBox:
		if p.Box == nil {
			answer.Text = p.Say
			return nil
		}
		if _, refused := screenask.Sensitive(p.Label, nil, *p.Box); refused {
			answer.Refused, answer.Text = screenask.CodeSensitive, screenask.RefusalLine
			return nil
		}
		box := *p.Box
		answer.Point = &protocol.OniRect{X: box.X, Y: box.Y, W: box.W, H: box.H}
		answer.Text = firstText(p.Say, p.Label, p.Note)
		if svc := d.oni.service.Load(); svc != nil {
			seq, err := svc.Strand(feature.StrandTargetFrame{X: box.X, Y: box.Y, W: box.W, H: box.H, Label: oniText(p.Label, 120), Mode: feature.StrandModePoint})
			if err == nil {
				answer.Seq = seq
			}
		}
		return nil
	}
	answer.Text = firstText(p.Say, p.Note, i18n.T("Nothing on this window matches that."))
	return nil
}

// oniAnswer asks for an answer in words in mode.
func (d *Daemon) oniAnswer(ctx context.Context, q *oniQuestion, mode string, in screenask.Input, answer *protocol.OniAnswer) error {
	pic, err := q.shot.JPEG(capture.AnswerLongSide, capture.AnswerQuality)
	if err != nil {
		return protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("The picture could not be prepared: %v.", err))
	}
	return d.oniAnswerWith(ctx, q, mode, in, pic, answer)
}

func (d *Daemon) oniAnswerWith(ctx context.Context, q *oniQuestion, mode string, in screenask.Input, pic []byte, answer *protocol.OniAnswer) error {
	reply, err := d.oniModel(ctx, q, mode, in, pic)
	if err != nil {
		return err
	}
	answer.Mode = mode
	got, err := screenask.ParseAnswer(mode, reply)
	if refusal := (*screenask.Refusal)(nil); errors.As(err, &refusal) {
		answer.Refused, answer.Text = refusal.Code, refusal.Line
		return nil
	} else if err != nil {
		return protocol.Errorf(protocol.CodeInternal, "%s", err.Error())
	}
	answer.Text = got.Text
	return nil
}

// oniModel sends one picture to the planned route, pinned, with no tools,
// and returns the reply's text.
func (d *Daemon) oniModel(ctx context.Context, q *oniQuestion, kind string, in screenask.Input, pic []byte) (string, error) {
	system, user := screenask.Prompt(kind, in)
	req := oniRequest(system, user, pic)
	req.Model, req.Strict, req.LocalOnly = q.plan.Route(), true, q.plan.Local
	if kind == screenask.KindSelect {
		req.MaxTokens = oniSelectTokens
	}
	stream, err := d.deps.Gateway.Stream(ctx, req)
	if err != nil {
		return "", noRouteError(err)
	}
	defer stream.Close()
	var reply strings.Builder
	for {
		delta, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", noRouteError(err)
		}
		if reply.Len()+len(delta.Text) > oniReplyMax {
			break
		}
		reply.WriteString(delta.Text)
	}
	return reply.String(), nil
}

// oniPlanRequest is the shape of every question, a system line and one
// picture, with no model picked: light work, so the smallest model that
// reads pictures answers it, a local one first and never a paid one.
func oniPlanRequest() llm.Request {
	req := oniRequest("", "", []byte{0})
	req.Light = true
	return req
}

func oniRequest(system, user string, pic []byte) llm.Request {
	return llm.Request{Messages: []llm.Message{
		{Role: llm.RoleSystem, Content: system},
		{Role: llm.RoleUser, Content: user, Images: []llm.Image{{Mime: "image/jpeg", Data: pic}}},
	}}
}

// noRouteError says why nothing can answer, or passes a model's failure on.
func noRouteError(err error) error {
	var noRoute *llm.NoRoute
	if errors.As(err, &noRoute) {
		code := protocol.CodeUnavailable
		if noRoute.NoVision {
			code = protocol.CodeNoVision
		}
		return protocol.Errorf(code, "%s", noRoute.Reason)
	}
	var perr *protocol.Error
	if errors.As(err, &perr) {
		return perr
	}
	return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The model did not answer: %v.", err))
}

func consentID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "c-" + hex.EncodeToString(b[:]), nil
}

func firstText(texts ...string) string {
	for _, t := range texts {
		if t = strings.TrimSpace(t); t != "" {
			return t
		}
	}
	return ""
}
