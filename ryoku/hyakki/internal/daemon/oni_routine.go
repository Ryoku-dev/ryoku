package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/oni/routine"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/protocol"
)

// Routines: while reflex.routines is on, each quick command that ran is
// watched, and a short run of them is offered once as a routine, for the
// chip's time. Only oni.keep on the offer's id, or a spoken yes from a valid
// press, writes the store; a skip, the offer lapsing, the feature or voice
// turning off write nothing. "Run my routine" and oni.routines run replay a
// kept routine through the same executors and caps, with no decision and no
// model asked.

// oniRoutine is the watcher of one run and its one offer. It holds nothing
// until a quick command runs with routines on.
type oniRoutine struct {
	mu   sync.Mutex
	work int
	// rec watches the run of voice generation gen; one offer per run
	rec   *routine.Recorder
	gen   uint64
	offer *routineOffer

	storeMu sync.Mutex
	store   *routine.Store
}

// routineOffer is the offer waiting on an answer.
type routineOffer struct {
	id    string
	steps []routine.Step
	timer presence.Timer
}

// routineOfferPrefix starts every routine offer's id.
const routineOfferPrefix = "r-"

// isRoutineOffer says id names a routine offer, not a guide's or a hint's.
func isRoutineOffer(id string) bool { return strings.HasPrefix(id, routineOfferPrefix) }

func routineOfferID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return routineOfferPrefix + hex.EncodeToString(b[:]), nil
}

// oniRoutines is the routines store, opened on first use.
func (d *Daemon) oniRoutines() *routine.Store {
	rt := &d.oni.routine
	rt.storeMu.Lock()
	defer rt.storeMu.Unlock()
	if rt.store == nil {
		rt.store = d.deps.OniReflex.Routines
		if rt.store == nil {
			rt.store = routine.NewStore("")
		}
	}
	return rt.store
}

// routineRefusal says why routines may not be offered, kept or run, or nil.
func routineRefusal(settings protocol.OniSettings) error {
	decision := feature.Decide(feature.Request{Feature: feature.Routines}, settings)
	if decision.Allow {
		return nil
	}
	message := i18n.T("Routines are off. Turn them on in Oni's settings.")
	if decision.Reason == feature.ReasonOniOff {
		message = i18n.T("Oni is off.")
	}
	return &protocol.Error{Code: protocol.CodeDisabled, Message: message, Reason: decision.Reason}
}

// voiceGen is the voice loop's generation: a run lasts while it holds.
func (d *Daemon) voiceGen() uint64 {
	v := &d.oni.voice
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.gen
}

// routineObserve watches a quick command that ran from speech. When it
// completes a run, the run is offered once as a routine. Replays are never
// watched.
func (d *Daemon) routineObserve(a reflex.Action, u route.Utterance) {
	if !routine.Replayable(a.Intent) || routineRefusal(protocol.EffectiveOni(d.Settings())) != nil {
		return
	}
	gen := d.voiceGen()
	rt := &d.oni.routine
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.rec == nil || rt.gen != gen {
		d.routineCloseLocked(false)
		rt.rec, rt.gen = routine.NewRecorder(routine.OfferMinSteps, routine.Gap), gen
	}
	steps, ok := rt.rec.Observe(a, u.At)
	if !ok {
		return
	}
	id, err := routineOfferID()
	if err != nil {
		return
	}
	offer := &routineOffer{id: id, steps: steps}
	offer.timer = d.reflexClock().AfterFunc(feature.RoutineOfferTTL(), func() {
		rt.mu.Lock()
		defer rt.mu.Unlock()
		if rt.offer == offer {
			d.routineCloseLocked(false)
		}
	})
	rt.offer = offer
	if svc := d.oni.service.Load(); svc != nil {
		_ = svc.Consent(*feature.RoutineOffer(id, i18n.Tf("Keep these %d steps as a routine?", len(steps))))
	}
}

// routineCloseLocked drops the offer and says so on the frames. rt.mu is
// held.
func (d *Daemon) routineCloseLocked(kept bool) {
	rt := &d.oni.routine
	offer := rt.offer
	if offer == nil {
		return
	}
	rt.offer = nil
	offer.timer.Stop()
	answer := "no"
	if kept {
		answer = "yes"
	}
	if d.oni.service.Load() != nil {
		d.oniEmit(&feature.ConsentAnswerFrame{ID: offer.id, Answer: answer})
	}
}

// routineOfferWaiting is the id of the routine offer waiting on an answer,
// or "".
func (d *Daemon) routineOfferWaiting() string {
	rt := &d.oni.routine
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.offer == nil {
		return ""
	}
	return rt.offer.id
}

// routineKeep keeps the offered run, the one write an offer makes.
func (d *Daemon) routineKeep(id string) (protocol.OniKeepResult, error) {
	if err := routineRefusal(protocol.EffectiveOni(d.Settings())); err != nil {
		return protocol.OniKeepResult{}, err
	}
	rt := &d.oni.routine
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.offer == nil || rt.offer.id != id {
		return protocol.OniKeepResult{}, oniOfferGone()
	}
	store := d.oniRoutines()
	if err := store.Put(routine.Routine{Steps: rt.offer.steps}); err != nil {
		d.routineCloseLocked(false)
		return protocol.OniKeepResult{}, err
	}
	n := 0
	if list, err := store.List(); err == nil {
		n = len(list)
	}
	d.routineCloseLocked(true)
	return protocol.OniKeepResult{Kept: true, Entries: n}, nil
}

// routineSkip lets the offer id go, writing nothing.
func (d *Daemon) routineSkip(id string) error {
	rt := &d.oni.routine
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.offer == nil || rt.offer.id != id {
		return oniOfferGone()
	}
	d.routineCloseLocked(false)
	return nil
}

// routineSettings follows the settings: with routines or voice off, the
// offer goes unanswered, nothing is written and the run is forgotten.
func (d *Daemon) routineSettings(next protocol.Settings) {
	eff := protocol.EffectiveOni(next)
	if routineRefusal(eff) == nil && feature.Decide(feature.Request{Feature: feature.Voice}, eff).Allow {
		return
	}
	rt := &d.oni.routine
	rt.mu.Lock()
	defer rt.mu.Unlock()
	d.routineCloseLocked(false)
	rt.rec = nil
}

// routineRunSpoken runs the routine arg names, or the latest, for "run my
// routine" said with a valid press. It returns the line to say, or "" when
// each step was announced as it ran.
func (d *Daemon) routineRunSpoken(u route.Utterance, source, arg string) string {
	if routineRefusal(protocol.EffectiveOni(d.Settings())) != nil {
		return i18n.T("Routines are off.")
	}
	r, err := d.routineFind(arg)
	var refused *protocol.Error
	switch {
	case err == nil:
	case errors.As(err, &refused) && refused.Code == protocol.CodeNotFound && arg == "":
		return i18n.T("No routine kept yet.")
	case errors.As(err, &refused) && refused.Code == protocol.CodeNotFound:
		return i18n.T("I don't know that routine.")
	default:
		return i18n.T("I can't read your routines.")
	}
	d.routineReplay(r, reflexRunOpts{Turn: u.Turn, Source: source, Route: protocol.DecisionRules, Replay: true})
	return ""
}

// routineFind is the routine name names, or the latest when name is empty.
func (d *Daemon) routineFind(name string) (routine.Routine, error) {
	store := d.oniRoutines()
	if name == "" {
		list, err := store.List()
		if err != nil {
			return routine.Routine{}, err
		}
		if len(list) == 0 {
			return routine.Routine{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("No routine is kept yet."))
		}
		return list[0], nil
	}
	r, ok, err := store.Find(name)
	if err != nil {
		return routine.Routine{}, err
	}
	if !ok {
		return routine.Routine{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("There is no routine by that name."))
	}
	return r, nil
}

// oniRoutineReplayTimeout bounds one replay's executors.
const oniRoutineReplayTimeout = time.Minute

// routineReplay runs r's steps through the quick commands' executors and
// caps, at most an utterance's worth. A step whose app is gone is skipped
// with a line and a ledger row. Nothing is decided and no model is asked.
func (d *Daemon) routineReplay(r routine.Routine, o reflexRunOpts) {
	rt := &d.oni.routine
	rt.mu.Lock()
	if routineRefusal(protocol.EffectiveOni(d.Settings())) != nil {
		rt.mu.Unlock()
		return
	}
	rt.work++
	rt.mu.Unlock()
	defer func() {
		rt.mu.Lock()
		rt.work--
		rt.mu.Unlock()
	}()
	cat := d.reflexCatalog()
	most := routineMaxActions()
	var actions []reflex.Action
	for _, st := range r.Steps {
		one := routine.Plan(routine.Routine{Steps: []routine.Step{st}}, cat)
		for _, line := range one.Lines {
			d.reflexAnnounce(o, line)
			d.reflexLedger(o, reflex.Action{Intent: st.Intent, Arg: st.Arg, Confidence: 1}, protocol.OniReflexSkipped)
		}
		for _, a := range one.Actions {
			if len(actions) == most {
				d.reflexLedger(o, a, protocol.OniReflexRefused)
				continue
			}
			actions = append(actions, a)
		}
	}
	if len(actions) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), oniRoutineReplayTimeout)
		defer cancel()
		d.reflexRun(ctx, actions, o)
	}
	if err := d.oniRoutines().Ran(r.Name); err != nil {
		d.warn("oni routine: %v", err)
	}
}

// routineMaxActions is the most actions one replay runs: one utterance's
// worth, the spec's reflex.maxActions.
func routineMaxActions() int {
	s, err := oniGuideSpec()
	if err != nil || s.Reflex.MaxActions < 1 {
		return 1
	}
	return s.Reflex.MaxActions
}

// oniRoutinesCall answers oni.routines: list, run, forget or forget_all.
// The gate lets only the app and the CLI through.
func (d *Daemon) oniRoutinesCall(p protocol.OniRoutinesParams) (protocol.OniRoutinesResult, error) {
	if err := routineRefusal(protocol.EffectiveOni(d.Settings())); err != nil {
		return protocol.OniRoutinesResult{}, err
	}
	store := d.oniRoutines()
	ran := false
	switch p.Action {
	case protocol.OniRoutinesRun:
		r, err := d.routineFind(p.Name)
		if err != nil {
			return protocol.OniRoutinesResult{}, err
		}
		d.routineReplay(r, reflexRunOpts{Route: protocol.DecisionRules, Replay: true})
		ran = true
	case protocol.OniRoutinesForget:
		if err := store.Forget(p.Name); err != nil {
			return protocol.OniRoutinesResult{}, err
		}
	case protocol.OniRoutinesForgetAll:
		if err := store.ForgetAll(); err != nil {
			return protocol.OniRoutinesResult{}, err
		}
	}
	list, err := store.List()
	if err != nil {
		return protocol.OniRoutinesResult{}, err
	}
	infos := make([]protocol.OniRoutineInfo, 0, len(list))
	for _, r := range list {
		infos = append(infos, protocol.OniRoutineInfo{Name: r.Name, Steps: len(r.Steps), Saved: r.Saved, Used: r.Used})
	}
	return protocol.OniRoutinesResult{Routines: infos, Ran: ran}, nil
}
