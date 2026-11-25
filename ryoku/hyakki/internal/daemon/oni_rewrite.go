package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/dictate/target"
	handsapp "ryoku-rashin/internal/hands/app"
	"ryoku-rashin/internal/hands/input"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rewrite/apply"
	"ryoku-rashin/internal/rewrite/sel"
	"ryoku-rashin/internal/writing"
)

// A rewrite in place reads the text selected in the focused window (the
// PRIMARY selection, read only), shows it and the route on a card, and only
// on a yes sends it to the model. The checked result is typed back over the
// selection through dictation's verified typing, a chunk at a time while
// that window keeps focus; a result of more than one paragraph is never
// typed, it waits to be copied. The original stays in memory for the
// spec's undo time and is then zeroed. Undo presses Ctrl+Z and reads the
// window back; when it cannot see the original return, the original is
// offered to copy instead. No clipboard is read or written anywhere, and
// neither text rides on a frame, an event or a log line.

// OniRewriteDeps are a rewrite's selection reader and undo key; each nil
// field is the real one.
type OniRewriteDeps struct {
	// Select reads the PRIMARY selection; nil is wl-paste through its one
	// allowlisted argv.
	Select sel.Runner
	// Keys presses the undo chord; nil is wtype through hands/input.
	Keys input.Runner
	// Wait is the pause before an undo reads the window back; 0 is
	// apply.DefaultWait, negative none.
	Wait time.Duration
}

// What a rewrite record is waiting for.
const (
	rewriteWaiting = iota // the card is up
	rewriteRunning        // the model or the typing is under way
	rewriteTyped          // typed back; undo may take it
	rewriteCopy           // the result waits to be copied
)

// oniRewriteState is the one rewrite card or record, and the kinds of
// window where undo was seen to work. It holds nothing at rest but that
// list of app ids.
type oniRewriteState struct {
	mu       sync.Mutex
	rec      *rewriteRecord
	verified map[string]bool
}

// rewriteRecord is one rewrite from its card to the end of its undo time.
// The texts are byte slices so they can be zeroed when it ends.
type rewriteRecord struct {
	id          string
	conn        *connection
	phase       int
	window      window.Window
	target      target.Target
	windows     window.Provider
	original    []byte
	result      []byte
	style       string
	instruction string
	voice       bool
	plan        llm.Plan
	chunks      int
	timer       presence.Timer
	reviewed    bool // the app's button may hand focus back, without focusing it
}

// wipe zeroes the record's texts and stops its timer.
func (r *rewriteRecord) wipe() {
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
	clear(r.original)
	clear(r.result)
	r.original, r.result = nil, nil
}

func (r *rewriteRecord) destination() string {
	if r.plan.Local {
		return "local"
	}
	return "hosted"
}

func rewriteLimits() spec.Rewrite {
	sp, err := spec.Load()
	if err != nil || sp.Rewrite.MaxRunes <= 0 {
		return spec.Rewrite{MaxRunes: writing.InPlaceMaxRunes, RequestS: 60, UndoS: 60}
	}
	return sp.Rewrite
}

func rewriteRefused(code string) protocol.OniRewriteResult {
	return protocol.OniRewriteResult{Status: protocol.OniRewriteRefused, Refused: code}
}

// oniRewriteCall serves oni.rewrite for the app and the CLI.
func (d *Daemon) oniRewriteCall(c *connection, p protocol.OniRewriteParams) (protocol.OniRewriteResult, error) {
	switch p.Action {
	case protocol.OniRewriteStart:
		return d.rewriteStart(c, p)
	case protocol.OniRewriteApply:
		return d.rewriteApply(c, p.ID)
	case protocol.OniRewriteUndo:
		return d.rewriteUndo(c, p.ID)
	case protocol.OniRewriteCancel:
		return d.rewriteCancel(c, p.ID)
	}
	return protocol.OniRewriteResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Unknown action %s.", p.Action))
}

// rewriteStart reads the selection in the focused window and puts up the
// card that shows where it would go. Nothing is sent.
func (d *Daemon) rewriteStart(c *connection, p protocol.OniRewriteParams) (protocol.OniRewriteResult, error) {
	settings := protocol.EffectiveOni(d.Settings())
	if !feature.Decide(feature.Request{Feature: feature.Rewrite}, settings).Allow {
		return rewriteRefused(protocol.OniRewriteRefusedOff), nil
	}
	// A spoken invocation has already captured PRIMARY in the target app.
	// The app retrieves that exact disclosure instead of recapturing itself.
	s := &d.oni.rewrite
	s.mu.Lock()
	if rec := s.rec; c != nil && c.app && rec != nil && rec.conn == nil && rec.phase == rewriteWaiting {
		res := protocol.OniRewriteResult{Status: protocol.OniRewriteConsent, ID: rec.id, Style: rec.style,
			Chars: utf8.RuneCount(rec.original), Destination: rec.destination(), Route: rec.plan.Route(),
			Text: string(rec.original), ExpiresMs: feature.ConsentTTL.Milliseconds()}
		s.mu.Unlock()
		return res, nil
	}
	s.mu.Unlock()
	style := p.Style
	if style == "" {
		style = settings.Rewrite.Style
	}
	if style == "" {
		style = protocol.OniRewriteGrammar
	}
	instruction, err := writing.InPlaceInstruction(style, p.Instruction, false)
	if err != nil {
		return protocol.OniRewriteResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a rewrite style; only custom takes an instruction."))
	}
	limits := rewriteLimits()
	provider, err := d.oniWindows()
	if err != nil {
		return rewriteRefused(protocol.OniRewriteRefusedWindow), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), oniTargetTimeout)
	defer cancel()
	t, err := target.Capture(ctx, provider)
	if err != nil {
		return rewriteRefused(protocol.OniRewriteRefusedWindow), nil
	}
	w, ok := rewriteWindow(ctx, provider, t.Window)
	if !ok {
		return rewriteRefused(protocol.OniRewriteRefusedWindow), nil
	}
	switch apply.Barred(w) {
	case apply.BarredSelf:
		return rewriteRefused(protocol.OniRewriteRefusedSelf), nil
	case apply.BarredPrivate:
		return rewriteRefused(protocol.OniRewriteRefusedPrivate), nil
	case apply.BarredTerminal:
		return rewriteRefused(protocol.OniRewriteRefusedTerminal), nil
	}
	text, err := sel.Read(ctx, d.rewriteSelector(), limits.MaxRunes)
	switch {
	case errors.Is(err, sel.ErrTooLong):
		return rewriteRefused(protocol.OniRewriteRefusedTooLong), nil
	case err != nil:
		return rewriteRefused(protocol.OniRewriteRefusedNoText), nil
	}
	if code := rewritePrepared(writing.PrepareInPlace(text, limits.MaxRunes)); code != "" {
		return rewriteRefused(code), nil
	}
	if d.deps.Gateway == nil {
		return rewriteRefused(protocol.OniRewriteRefusedModel), nil
	}
	plan, err := d.deps.Gateway.Plan(ctx, llm.Request{Light: true, Messages: []llm.Message{{Role: llm.RoleSystem, Content: instruction}, {Role: llm.RoleUser, Content: ""}}})
	if err != nil {
		return protocol.OniRewriteResult{}, noRouteError(err)
	}
	caller := c
	if caller == nil {
		caller = &connection{}
	}
	if err := hostedNeedsApp(caller, plan.Route()); err != nil {
		return protocol.OniRewriteResult{}, err
	}
	id, err := consentID()
	if err != nil {
		return protocol.OniRewriteResult{}, protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("The question could not be prepared: %v.", err))
	}
	id = feature.RewriteCardPrefix + strings.TrimPrefix(id, "c-")
	rec := &rewriteRecord{
		id: id, conn: c, phase: rewriteWaiting, window: w, target: t, windows: provider,
		original: []byte(text), style: style, instruction: p.Instruction, voice: settings.Rewrite.Voice, plan: plan,
	}
	chars := utf8.RuneCountInString(text)
	question := i18n.Tf("Send the %d characters selected in %s to %s to rewrite?", chars, rewriteWindowName(w), plan.Route())
	if plan.Local {
		question = i18n.Tf("Rewrite the %d characters selected in %s with %s, on this computer?", chars, rewriteWindowName(w), plan.Route())
	}
	card := feature.RewriteCard(id, rec.destination(), plan.Route(), oniText(question, 400))
	d.rewriteHold(rec, feature.ConsentTTL)
	if svc := d.oni.service.Load(); svc != nil {
		_ = svc.Consent(*card)
	}
	return protocol.OniRewriteResult{
		Status: protocol.OniRewriteConsent, ID: id, Style: style, Chars: chars,
		Destination: rec.destination(), Route: plan.Route(), Text: text, ExpiresMs: feature.ConsentTTL.Milliseconds(),
	}, nil
}

// rewritePrepared is the refusal code for a selection PrepareInPlace refused.
func rewritePrepared(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, writing.ErrInPlaceLong):
		return protocol.OniRewriteRefusedTooLong
	case errors.Is(err, writing.ErrInPlaceSecret):
		return protocol.OniRewriteRefusedSecret
	default:
		return protocol.OniRewriteRefusedNoText
	}
}

// rewriteWindow is the window id names in provider's snapshot.
func rewriteWindow(ctx context.Context, provider window.Provider, id window.ID) (window.Window, bool) {
	s, err := provider.Snapshot(ctx)
	if err != nil {
		return window.Window{}, false
	}
	for _, w := range s.Windows {
		if w.ID == id {
			return w, true
		}
	}
	return window.Window{}, false
}

func rewriteWindowName(w window.Window) string {
	if a := oniText(w.App, 60); a != "" {
		return a
	}
	return i18n.T("this window")
}

// rewriteHold makes rec the one record, dropping whatever was there, and
// arms its lapse after ttl.
func (d *Daemon) rewriteHold(rec *rewriteRecord, ttl time.Duration) {
	s := &d.oni.rewrite
	s.mu.Lock()
	old := s.rec
	s.rec = rec
	rec.timer = d.oni.ask.now().AfterFunc(ttl, func() { d.rewriteLapse(rec) })
	s.mu.Unlock()
	if old != nil && old != rec {
		d.rewriteDropped(old)
	}
}

// rewriteLapse ends rec when its card or its undo time runs out.
func (d *Daemon) rewriteLapse(rec *rewriteRecord) {
	s := &d.oni.rewrite
	s.mu.Lock()
	if s.rec != rec || rec.phase == rewriteRunning {
		s.mu.Unlock()
		return
	}
	s.rec = nil
	rec.timer = nil
	s.mu.Unlock()
	d.rewriteDropped(rec)
}

// rewriteDropped zeroes rec and, if its card was up, takes the card down.
func (d *Daemon) rewriteDropped(rec *rewriteRecord) {
	waiting := rec.phase == rewriteWaiting
	rec.wipe()
	if waiting && d.oni.service.Load() != nil {
		d.oniEmit(&feature.ConsentAnswerFrame{ID: rec.id, Answer: "no"})
	}
}

// rewriteTake takes the record id names in phase want for c, marking a
// waiting one as running so a second yes cannot run it again.
func (d *Daemon) rewriteTake(c *connection, id string, want int) (*rewriteRecord, string) {
	s := &d.oni.rewrite
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.rec
	if rec == nil || id == "" || rec.id != id {
		return nil, protocol.OniRewriteRefusedUnknownID
	}
	if c != nil && rec.conn != c && !c.app {
		return nil, protocol.OniRewriteRefusedUnknownID
	}
	if rec.phase != want {
		if want == rewriteTyped {
			return nil, protocol.OniRewriteRefusedNoUndo
		}
		return nil, protocol.OniRewriteRefusedUnknownID
	}
	if rec.timer != nil {
		rec.timer.Stop()
		rec.timer = nil
	}
	rec.phase = rewriteRunning
	return rec, ""
}

// rewriteApply is the app's or the CLI's yes to the card: send, check and
// type. A result waiting to be copied is given again.
func (d *Daemon) rewriteApply(c *connection, id string) (protocol.OniRewriteResult, error) {
	if res, ok := d.rewriteCopyAgain(c, id); ok {
		return res, nil
	}
	rec, code := d.rewriteTake(c, id, rewriteWaiting)
	if rec == nil {
		return rewriteRefused(code), nil
	}
	rec.reviewed = c != nil && c.app
	if d.oni.service.Load() != nil {
		d.oniEmit(&feature.ConsentAnswerFrame{ID: rec.id, Answer: "yes"})
	}
	return d.rewriteRun(rec), nil
}

// rewriteCopyAgain returns the result waiting to be copied under id.
func (d *Daemon) rewriteCopyAgain(c *connection, id string) (protocol.OniRewriteResult, bool) {
	s := &d.oni.rewrite
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.rec
	if rec == nil || rec.id != id || rec.phase != rewriteCopy || rec.conn != c && !c.app {
		return protocol.OniRewriteResult{}, false
	}
	return protocol.OniRewriteResult{Status: protocol.OniRewriteCopy, ID: rec.id, Style: rec.style, Destination: rec.destination(),
		Route: rec.plan.Route(), Text: string(rec.result)}, true
}

// rewriteCancel takes the card down; nothing was sent.
func (d *Daemon) rewriteCancel(c *connection, id string) (protocol.OniRewriteResult, error) {
	rec, code := d.rewriteTake(c, id, rewriteWaiting)
	if rec == nil {
		return rewriteRefused(code), nil
	}
	d.rewriteEnd(rec)
	rec.phase = rewriteWaiting
	d.rewriteDropped(rec)
	return protocol.OniRewriteResult{Status: protocol.OniRewriteDeclined, ID: id}, nil
}

// rewriteEnd clears the slot if it still holds rec.
func (d *Daemon) rewriteEnd(rec *rewriteRecord) {
	s := &d.oni.rewrite
	s.mu.Lock()
	if s.rec == rec {
		s.rec = nil
	}
	s.mu.Unlock()
}

// rewriteRun sends rec's selection, checks the reply and types it back.
func (d *Daemon) rewriteRun(rec *rewriteRecord) protocol.OniRewriteResult {
	limits := rewriteLimits()
	base := protocol.OniRewriteResult{ID: rec.id, Style: rec.style, Chars: utf8.RuneCount(rec.original), Destination: rec.destination(), Route: rec.plan.Route()}
	fail := func(code, line string) protocol.OniRewriteResult {
		d.rewriteEnd(rec)
		rec.wipe()
		d.rewriteSay("warn", line)
		base.Status, base.Refused = protocol.OniRewriteRefused, code
		return base
	}
	ctx, cancel := context.WithTimeout(context.Background(), limits.Request()+oniDictateTimeout)
	defer cancel()
	if err := d.rewriteReady(ctx, rec, rec.reviewed); err != nil {
		return fail(protocol.OniRewriteRefusedWindow, i18n.T("Return to the unchanged original window before applying a rewrite. Nothing was sent or typed."))
	}
	selected, selectErr := sel.Read(ctx, d.rewriteSelector(), limits.MaxRunes)
	if selectErr != nil || selected != string(rec.original) {
		return fail(protocol.OniRewriteRefusedChanged, i18n.T("The selection changed, so nothing was sent or typed."))
	}
	mctx, mcancel := context.WithTimeout(ctx, limits.Request())
	original := string(rec.original)
	out, err := writing.InPlace(mctx, d.deps.Gateway, writing.InPlaceParams{
		Text: original, Style: rec.style, Instruction: rec.instruction,
		Model: rec.plan.Route(), Local: rec.plan.Local, Voice: rec.voice, MaxRunes: limits.MaxRunes,
	})
	mcancel()
	switch {
	case errors.Is(err, writing.ErrInPlaceAdded):
		return fail(protocol.OniRewriteRefusedFacts, i18n.T("The rewrite added details your text did not have, so nothing was typed."))
	case errors.Is(err, writing.ErrInPlaceSecret):
		return fail(protocol.OniRewriteRefusedSecret, i18n.T("That text looks private, so it was not rewritten."))
	case err != nil:
		return fail(protocol.OniRewriteRefusedModel, i18n.T("The rewrite did not come back in a usable form, so nothing was typed."))
	case out.Same:
		d.rewriteEnd(rec)
		rec.wipe()
		d.rewriteSay("answer", i18n.T("It already reads well, so nothing was typed."))
		base.Status = protocol.OniRewriteSame
		return base
	}
	rec.result = []byte(out.Text)
	if strings.ContainsAny(out.Text, "\n\r\u2028\u2029") {
		return d.rewriteToCopy(rec, base, limits, i18n.T("The rewrite runs over more than one paragraph, so it was not typed. It waits in Hyakki to copy."))
	}
	applier := d.rewriteApplier(rec)
	typed, err := applier.Apply(ctx, rec.target, original, out.Text)
	base.Landed, base.Rest = typed.Landed, typed.Rest
	rec.chunks = typed.Chunks
	switch {
	case errors.Is(err, apply.ErrSelectionChanged):
		return fail(protocol.OniRewriteRefusedChanged, i18n.T("The selection changed, so nothing was typed."))
	case errors.Is(err, apply.ErrMultiline):
		return d.rewriteToCopy(rec, base, limits, i18n.T("The rewrite runs over more than one paragraph, so it was not typed. It waits in Hyakki to copy."))
	case err != nil && typed.Landed == 0 && typed.Unsure == 0:
		return fail(protocol.OniRewriteRefusedWindow, i18n.T("The window changed before anything was typed, so nothing was."))
	case err != nil:
		// part of it went in: the original goes on the card to copy, and
		// undo may still try for the chunks that ran
		base.Status, base.Refused, base.Text = protocol.OniRewriteKept, protocol.OniRewriteRefusedWindow, original
		base.ExpiresMs = limits.Undo().Milliseconds()
		d.rewriteKeep(rec, limits)
		d.rewriteSay("warn", i18n.T("Typing stopped part way. Your original waits in Hyakki to copy."))
		return base
	}
	base.Status = protocol.OniRewriteApplied
	base.UndoVerified = d.rewriteVerified(rec.target.App)
	base.ExpiresMs = limits.Undo().Milliseconds()
	d.rewriteKeep(rec, limits)
	if base.UndoVerified {
		d.rewriteSay("answer", i18n.T("Rewritten. Ctrl+Z puts it back."))
	} else {
		d.rewriteSay("answer", i18n.T("Rewritten. Your original is kept for a short while; Undo in Hyakki tries to bring it back."))
	}
	return base
}

// rewriteToCopy keeps rec's result to be copied for the undo time.
func (d *Daemon) rewriteToCopy(rec *rewriteRecord, base protocol.OniRewriteResult, limits spec.Rewrite, line string) protocol.OniRewriteResult {
	clear(rec.original)
	rec.original = nil
	base.Status, base.Text = protocol.OniRewriteCopy, string(rec.result)
	base.Landed, base.Rest = 0, ""
	base.ExpiresMs = limits.Undo().Milliseconds()
	d.rewriteSettle(rec, rewriteCopy, limits.Undo())
	d.rewriteSay("answer", line)
	return base
}

// rewriteKeep keeps rec for undo for the spec's undo time.
func (d *Daemon) rewriteKeep(rec *rewriteRecord, limits spec.Rewrite) {
	d.rewriteSettle(rec, rewriteTyped, limits.Undo())
}

// rewriteSettle moves rec to phase and arms its lapse, unless another
// record took the slot meanwhile.
func (d *Daemon) rewriteSettle(rec *rewriteRecord, phase int, ttl time.Duration) {
	s := &d.oni.rewrite
	s.mu.Lock()
	if s.rec != rec {
		s.mu.Unlock()
		rec.wipe()
		return
	}
	rec.phase = phase
	rec.timer = d.oni.ask.now().AfterFunc(ttl, func() { d.rewriteLapse(rec) })
	s.mu.Unlock()
}

func (d *Daemon) rewriteVerified(app string) bool {
	s := &d.oni.rewrite
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.verified[strings.ToLower(app)]
}

// rewriteUndo presses Ctrl+Z in the window the rewrite went into until it
// reads the original back, or offers the original to copy.
func (d *Daemon) rewriteUndo(c *connection, id string) (protocol.OniRewriteResult, error) {
	rec, code := d.rewriteTake(c, id, rewriteTyped)
	if rec == nil {
		return rewriteRefused(code), nil
	}
	defer func() {
		d.rewriteEnd(rec)
		rec.wipe()
	}()
	original := string(rec.original)
	ctx, cancel := context.WithTimeout(context.Background(), oniDictateTimeout)
	defer cancel()
	if err := d.rewriteReady(ctx, rec, c != nil && c.app); err != nil {
		return protocol.OniRewriteResult{Status: protocol.OniRewriteKept, ID: rec.id, Text: original}, nil
	}
	undone, err := d.rewriteApplier(rec).Undo(ctx, apply.Record{Target: rec.target, Original: original, Rewritten: string(rec.result), Chunks: rec.chunks})
	if err == nil && undone.Verified {
		s := &d.oni.rewrite
		s.mu.Lock()
		if s.verified == nil {
			s.verified = map[string]bool{}
		}
		s.verified[strings.ToLower(rec.target.App)] = true
		s.mu.Unlock()
		d.rewriteSay("answer", i18n.T("Your original is back."))
		return protocol.OniRewriteResult{Status: protocol.OniRewriteUndone, ID: rec.id, Style: rec.style, UndoVerified: true}, nil
	}
	d.rewriteSay("warn", i18n.T("Hyakki could not confirm your original came back. It waits in Hyakki to copy."))
	return protocol.OniRewriteResult{Status: protocol.OniRewriteKept, ID: rec.id, Style: rec.style, Text: original}, nil
}

// rewriteApplier types and undoes in rec's window.
func (d *Daemon) rewriteApplier(rec *rewriteRecord) apply.Applier {
	limits := rewriteLimits()
	return apply.Applier{
		Windows: rec.windows,
		Type:    d.voiceTyper(),
		Keys:    d.rewriteKeys(),
		Select: func(ctx context.Context) (string, error) {
			return sel.Read(ctx, d.rewriteSelector(), limits.MaxRunes)
		},
		Look: d.rewriteLook(rec.windows, rec.target.Window),
		Wait: d.deps.OniRewrite.Wait,
	}
}

// rewriteLook reads the words in a masked picture of window id, here; the
// picture and the words stay in memory and are cleared.
func (d *Daemon) rewriteLook(provider window.Provider, id window.ID) apply.Look {
	return func(ctx context.Context) ([]string, error) {
		w, ok := rewriteWindow(ctx, provider, id)
		if !ok {
			return nil, errors.New("the window is gone")
		}
		shot, _, err := d.oniCapture(provider).Capture(ctx, w, d.oniMasks(ctx, w.Output))
		if err != nil {
			return nil, err
		}
		defer clearShot(shot)
		words, err := d.oniWords(ctx, shot)
		if err != nil {
			return nil, err
		}
		out := make([]string, len(words))
		for i, word := range words {
			out[i] = word.Text
		}
		return out, nil
	}
}

// Only the tools use the socket's directory; the daemon keeps its runtime.
func rewriteToolDisplay() (handsapp.Display, error) {
	display := os.Getenv("WAYLAND_DISPLAY")
	if display == "" || (filepath.IsAbs(display) && filepath.Clean(display) != display) {
		return handsapp.Display{}, input.ErrEnv
	}
	return handsapp.LiveDisplay(os.Getenv)
}

func (d *Daemon) rewriteSelector() sel.Runner {
	if d.deps.OniRewrite.Select != nil {
		return d.deps.OniRewrite.Select
	}
	display, err := rewriteToolDisplay()
	var env []string
	if err == nil {
		env, err = sel.Env(display.RuntimeDir, display.Name)
	}
	if err != nil {
		return func(context.Context, int, string, ...string) ([]byte, error) { return nil, err }
	}
	return sel.Exec(env)
}

func (d *Daemon) rewriteKeys() input.Runner {
	if d.deps.OniRewrite.Keys != nil {
		return d.deps.OniRewrite.Keys
	}
	display, err := rewriteToolDisplay()
	var env []string
	if err == nil {
		env, err = input.Env(display.RuntimeDir, display.Name)
	}
	if err != nil {
		return func(context.Context, string, ...string) error { return err }
	}
	return input.Exec(env)
}

// rewriteSay puts line in Oni's bubble when Oni is drawn. It never holds
// the selection or the result.
func (d *Daemon) rewriteSay(tone, line string) {
	if svc := d.oni.service.Load(); svc != nil && !protocol.EffectiveOni(d.Settings()).Hidden {
		_ = svc.Bubble(0, tone, oniText(line, oniBubbleMax))
	}
}

// rewriteCardWaiting is the id of the rewrite card up, or "".
func (d *Daemon) rewriteCardWaiting() string {
	s := &d.oni.rewrite
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rec == nil || s.rec.phase != rewriteWaiting {
		return ""
	}
	return s.rec.id
}

// rewriteCardAnswer serves oni.consent.answer for the rewrite card: the
// app or whoever started it answers. It reports false when id is not a
// rewrite card.
func (d *Daemon) rewriteCardAnswer(c *connection, p protocol.OniConsentAnswerParams) (protocol.OniAskResult, bool, error) {
	if !strings.HasPrefix(p.ID, feature.RewriteCardPrefix) {
		return protocol.OniAskResult{}, false, nil
	}
	rec, _ := d.rewriteTake(c, p.ID, rewriteWaiting)
	if rec == nil {
		return protocol.OniAskResult{}, true, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That question has expired or was answered."))
	}
	rec.reviewed = c != nil && c.app
	return d.rewriteAnswered(rec, p.Answer == "yes"), true, nil
}

// rewriteAnswered carries out a card answer: no drops it, yes runs it. The
// outcome reaches the bubble; the texts reach only oni.rewrite callers.
func (d *Daemon) rewriteAnswered(rec *rewriteRecord, allow bool) protocol.OniAskResult {
	if d.oni.service.Load() != nil {
		answer := "no"
		if allow {
			answer = "yes"
		}
		d.oniEmit(&feature.ConsentAnswerFrame{ID: rec.id, Answer: answer})
	}
	if !allow {
		d.rewriteEnd(rec)
		rec.wipe()
		return protocol.OniAskResult{Status: protocol.OniAskDeclined}
	}
	d.rewriteRun(rec)
	return protocol.OniAskResult{Status: protocol.OniAskAnswered}
}
