package daemon

import (
	"context"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/oni/routine"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/reflex/act"
)

// These tests drive routines over the daemon's own socket: quick commands
// said with the voice fakes, an executor that only records, a catalog of
// names, and a decision route that counts its calls.

// routineExec records every action it is asked to carry out, and does it.
type routineExec struct {
	mu   sync.Mutex
	done []reflex.Action
}

func (e *routineExec) Do(_ context.Context, a reflex.Action) (act.Outcome, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.done = append(e.done, a)
	return act.Outcome{Done: true, Line: "Done."}, nil
}

func (e *routineExec) Caps() map[reflex.Intent]bool {
	caps := map[reflex.Intent]bool{}
	for _, info := range reflex.Registry() {
		caps[info.ID] = true
	}
	return caps
}

func (e *routineExec) calls() []reflex.Action {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.done)
}

// routineCatalog finds the apps it names, ignoring case.
type routineCatalog []string

func (c routineCatalog) Resolve(spoken string) (name, exec string, score float64, ok bool) {
	for _, n := range c {
		if strings.EqualFold(strings.TrimSpace(spoken), n) {
			return n, strings.ToLower(strings.ReplaceAll(n, " ", "-")), 1, true
		}
	}
	return "", "", 0, false
}

// routineDecider is a decision route that counts its calls and never
// resolves.
type routineDecider struct{ n atomic.Int32 }

func (d *routineDecider) Decide(context.Context, protocol.FastDecisionParams) (protocol.FastDecisionResult, error) {
	d.n.Add(1)
	return protocol.FastDecisionResult{}, nil
}

const routineApp = "Lindqvist notes"

type routineRig struct {
	*voiceRig
	exec     *routineExec
	store    *routine.Store
	decider  *routineDecider
	notified atomic.Int32
	surface  *socketPeer
	log      *frameLog
	cli      *socketPeer
	// listen is the open-mic feed when open mic is on
	listen chan []int16
}

// serveRoutines serves voice and quick commands with routines on, the
// typed decision route on and local, Oni on screen.
func serveRoutines(t *testing.T, openMic bool) *routineRig {
	t.Helper()
	rr := &routineRig{exec: &routineExec{}, decider: &routineDecider{}}
	rr.voiceRig = serveVoice(t, func(s *protocol.Settings) {
		s.Oni.VoiceSettings.OpenMic = openMic
		s.Oni.Reflex = true
		s.Oni.ReflexSettings.Routines = true
		s.Oni.ReflexSettings.Decisions = true
		s.Providers.Decisions.Route = protocol.DecisionLocal
	}, nil, func(d *Deps) {
		rr.store = routine.NewStore(t.TempDir() + "/oni-reflex-routines.json")
		d.FastDecisions = rr.decider
		d.OniReflex = OniReflexDeps{
			Exec:     rr.exec,
			Catalog:  routineCatalog{routineApp, "Firefox"},
			Routines: rr.store,
			Clock:    d.OniVoice.Clock,
			Notify: func(string, string) error {
				rr.notified.Add(1)
				return nil
			},
		}
	})
	rr.surface, _ = rr.presenting(t)
	rr.log = &frameLog{p: rr.surface}
	rr.cli = dialSocket(t, protocol.ClientCLI)
	if openMic {
		rr.listen = rr.mic.next(t)
	}
	return rr
}

// command says text with the chord and waits until the executor has done
// want actions in all.
func (rr *routineRig) command(t *testing.T, text string, want int) {
	t.Helper()
	if listen := rr.chord(t, rr.surface, text, rr.listen != nil); listen != nil {
		rr.listen = listen
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(rr.exec.calls()) < want {
		if time.Now().After(deadline) {
			r := &rr.d.oni.reflex
			r.mu.Lock()
			rows := slices.Clone(r.rows)
			r.mu.Unlock()
			t.Fatalf("after %q the executor did %+v; ledger %+v", text, rr.exec.calls(), rows)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := rr.exec.calls(); len(got) != want {
		t.Fatalf("after %q the executor did %+v", text, got)
	}
}

// await polls cond in real time; the fake clock stays where it is.
func (rr *routineRig) await(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s never came", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// three runs three replayable quick commands and returns the offer.
func (rr *routineRig) three(t *testing.T) *feature.ConsentFrame {
	t.Helper()
	rr.command(t, "open "+routineApp, 1)
	rr.command(t, "turn it up", 2)
	rr.command(t, "maximize", 3)
	deadline := time.Now().Add(5 * time.Second)
	for rr.d.routineOfferWaiting() == "" {
		if time.Now().After(deadline) {
			r := &rr.d.oni.reflex
			r.mu.Lock()
			rows := slices.Clone(r.rows)
			r.mu.Unlock()
			rt := &rr.d.oni.routine
			rt.mu.Lock()
			rec := rt.rec
			rt.mu.Unlock()
			t.Fatalf("no routine offer; ledger %+v, recorder %+v", rows, rec)
		}
		time.Sleep(5 * time.Millisecond)
	}
	return rr.log.until("the routine offer", consentFor(feature.Routines)).(*feature.ConsentFrame)
}

func (rr *routineRig) kept(t *testing.T) []routine.Routine {
	t.Helper()
	list, err := rr.store.List()
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func (rr *routineRig) nothingKept(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(rr.store.Path()); !os.IsNotExist(err) {
		t.Fatalf("the routines file was written: %v", err)
	}
}

func answered(id string) func(feature.Payload) bool {
	return func(f feature.Payload) bool {
		a, ok := f.(*feature.ConsentAnswerFrame)
		return ok && a.ID == id
	}
}

var threeSteps = []routine.Step{{Intent: reflex.OpenApp, Arg: routineApp}, {Intent: reflex.VolumeUp}, {Intent: reflex.Maximize}}

// Three quick commands in a row are offered once as a routine, for the
// chip's time. Open-mic and wake yes never keep it; a yes from the chord
// does, with the steps and nothing said. The run is not offered again.
func TestOniRoutineOfferIsKeptOnlyByAPressedYes(t *testing.T) {
	rr := serveRoutines(t, true)
	offer := rr.three(t)
	if !strings.HasPrefix(offer.ID, "r-") || offer.Class != feature.ConsentPerAction || offer.Destination != feature.EgressLocal ||
		offer.ExpiresMs != feature.RoutineOfferTTL().Milliseconds() || offer.Route != "" {
		t.Fatalf("offer = %+v", offer)
	}
	if rr.d.routineOfferWaiting() != offer.ID {
		t.Fatal("the offer is not waiting")
	}

	// open mic is told to press, and keeps nothing
	rr.overheard(rr.listen, "yes")
	rr.log.until("the press line", spoken(route.NeedsPressLine))
	// nor does a yes that came with no press, as from the wake phrase
	if err := (voiceConsent{rr.d}).Answer(route.Press{}, true); !forbidden(err) {
		t.Fatalf("a yes with no press: %v", err)
	}
	rr.nothingKept(t)
	if rr.d.routineOfferWaiting() != offer.ID {
		t.Fatal("unpressed speech answered the offer")
	}
	// the presenter may not keep it
	if err := rr.surface.call(protocol.MOniKeep, protocol.OniKeepParams{ID: offer.ID}, nil); !forbidden(err) {
		t.Fatalf("the presenter kept: %v", err)
	}

	// the chord's yes keeps it
	rr.listen = rr.chord(t, rr.surface, "yes", true)
	if a := rr.log.until("the offer's answer", answered(offer.ID)).(*feature.ConsentAnswerFrame); a.Answer != "yes" {
		t.Fatalf("answered %q", a.Answer)
	}
	list := rr.kept(t)
	if len(list) != 1 || !slices.Equal(list[0].Steps, threeSteps) || list[0].Name == "" {
		t.Fatalf("kept %+v", list)
	}
	if info, err := os.Stat(rr.store.Path()); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("routines file: %v, %v", info, err)
	}
	raw, err := os.ReadFile(rr.store.Path())
	if err != nil || strings.Contains(string(raw), "turn it up") {
		t.Fatalf("the file keeps words that were said: %s, %v", raw, err)
	}

	// three more in the same run are not offered again (another app: the
	// same one is held by the launch cooldown on the unmoved clock)
	rr.command(t, "open firefox", 4)
	rr.command(t, "a bit quieter please", 5)
	rr.command(t, "mute", 6)
	rr.log.quiet("a second offer", consentFor(feature.Routines))
	if rr.d.routineOfferWaiting() != "" || len(rr.kept(t)) != 1 {
		t.Fatal("the run was offered twice")
	}
	if rr.decider.n.Load() != 0 || len(rr.model.sent()) != 0 || len(rr.questions()) != 0 {
		t.Fatalf("quick commands asked: %d decisions, %d reads, %q", rr.decider.n.Load(), len(rr.model.sent()), rr.questions())
	}
}

// Skipping the offer, letting it lapse, or turning routines or voice off
// writes nothing; the offer then cannot be kept.
func TestOniRoutineSkipLapseOrUnwindKeepsNothing(t *testing.T) {
	cases := map[string]func(rr *routineRig, offer *feature.ConsentFrame){
		"skip": func(rr *routineRig, offer *feature.ConsentFrame) {
			if err := rr.surface.call(protocol.MOniSkip, protocol.OniKeepParams{ID: offer.ID}, nil); err != nil {
				t.Fatal(err)
			}
		},
		"spoken no": func(rr *routineRig, offer *feature.ConsentFrame) {
			rr.chord(t, rr.surface, "no", false)
		},
		"lapse": func(rr *routineRig, offer *feature.ConsentFrame) {
			rr.clock.Advance(feature.RoutineOfferTTL() - time.Millisecond)
			if rr.d.routineOfferWaiting() != offer.ID {
				t.Fatal("the offer lapsed early")
			}
			rr.clock.Advance(time.Millisecond)
		},
		"routines off": func(rr *routineRig, offer *feature.ConsentFrame) {
			rr.patch(t, `{"oni":{"reflexSettings":{"routines":false}}}`)
		},
		"voice off": func(rr *routineRig, offer *feature.ConsentFrame) {
			rr.patch(t, `{"oni":{"voice":false}}`)
		},
	}
	for name, unwind := range cases {
		t.Run(name, func(t *testing.T) {
			rr := serveRoutines(t, false)
			offer := rr.three(t)
			unwind(rr, offer)
			if a := rr.log.until("the offer's answer", answered(offer.ID)).(*feature.ConsentAnswerFrame); a.Answer != "no" {
				t.Fatalf("answered %q", a.Answer)
			}
			if rr.d.routineOfferWaiting() != "" {
				t.Fatal("the offer still waits")
			}
			if code, _ := reasonOf(rr.cli.call(protocol.MOniKeep, protocol.OniKeepParams{ID: offer.ID}, nil)); code != protocol.CodeNotFound && code != protocol.CodeDisabled {
				t.Fatalf("keep after %s: %s", name, code)
			}
			rr.nothingKept(t)
		})
	}
	// the CLI keeps, the same as a pressed yes
	t.Run("keep", func(t *testing.T) {
		rr := serveRoutines(t, false)
		offer := rr.three(t)
		var kept protocol.OniKeepResult
		if err := rr.cli.call(protocol.MOniKeep, protocol.OniKeepParams{ID: offer.ID}, &kept); err != nil || !kept.Kept || kept.Entries != 1 {
			t.Fatalf("keep = %+v, %v", kept, err)
		}
		if code, _ := reasonOf(rr.cli.call(protocol.MOniKeep, protocol.OniKeepParams{ID: offer.ID}, nil)); code != protocol.CodeNotFound {
			t.Fatalf("a second keep: %s", code)
		}
		if len(rr.kept(t)) != 1 {
			t.Fatal("not kept")
		}
	})
}

// "Run my routine" replays the latest routine through the executors with no
// decision and no model asked, skips a step whose app is gone with its line
// and a ledger row, and never offers the replay as a new routine. Open mic
// cannot run it.
func TestOniRoutineRunReplaysWithNoDecisionOrModel(t *testing.T) {
	rr := serveRoutines(t, true)
	if err := rr.store.Put(routine.Routine{Name: "Morning", Steps: []routine.Step{
		{Intent: reflex.OpenApp, Arg: routineApp},
		{Intent: reflex.OpenApp, Arg: "Gone app"},
		{Intent: reflex.VolumeUp},
		{Intent: reflex.Maximize},
	}}); err != nil {
		t.Fatal(err)
	}
	before, _, _ := rr.store.Find("Morning")

	// open mic, even calling Oni by name, cannot run it
	name := oniActiveProfile(protocol.EffectiveOni(rr.d.Settings()).Character).Name
	rr.overheard(rr.listen, name+", run my routine")
	rr.log.until("the press line", spoken(route.ReflexNeedsPressLine))
	time.Sleep(50 * time.Millisecond)
	if len(rr.exec.calls()) != 0 {
		t.Fatalf("open mic ran the routine: %+v", rr.exec.calls())
	}
	asked, read, decided := len(rr.questions()), len(rr.model.sent()), rr.decider.n.Load()

	rr.command(t, "run my routine", 3)
	want := []reflex.Action{{Intent: reflex.OpenApp, Arg: routineApp, Confidence: 1}, {Intent: reflex.VolumeUp, Confidence: 1}, {Intent: reflex.Maximize, Confidence: 1}}
	if got := rr.exec.calls(); !slices.Equal(got, want) {
		t.Fatalf("replayed %+v", got)
	}
	rr.log.until("the skipped step's line", func(f feature.Payload) bool {
		v, ok := f.(*feature.VoiceFrame)
		return ok && v.Phase == feature.VoiceSpeaking && strings.Contains(v.Text, "Gone app")
	})
	if n := rr.decider.n.Load(); n != decided || len(rr.model.sent()) != read || len(rr.questions()) != asked {
		t.Fatalf("the replay asked: %d decisions, %d reads, %q", n-decided, len(rr.model.sent())-read, rr.questions()[asked:])
	}
	if rr.d.routineOfferWaiting() != "" {
		t.Fatal("the replay was offered as a routine")
	}
	rr.await(t, "the routine marked used", func() bool {
		after, _, _ := rr.store.Find("Morning")
		return after.Used.After(before.Used)
	})
	var log protocol.OniReflexLogResult
	if err := rr.cli.call(protocol.MOniReflexLog, protocol.Empty{}, &log); err != nil {
		t.Fatal(err)
	}
	outcomes := map[string]int{}
	for _, row := range log.Rows {
		if row.Route != protocol.DecisionRules {
			t.Errorf("row %+v is not on the rules route", row)
		}
		outcomes[string(row.Intent)+"/"+row.Outcome]++
	}
	if outcomes["open_app/skipped"] != 1 || outcomes["open_app/done"] != 1 || outcomes["volume_up/done"] != 1 || outcomes["maximize/done"] != 1 {
		t.Fatalf("ledger %v", outcomes)
	}

	// a routine name the store does not keep runs nothing
	rr.listen = rr.chord(t, rr.surface, "run my evening routine", true)
	rr.log.until("the unknown routine's line", spoken("I don't know that routine."))
	if len(rr.exec.calls()) != 3 {
		t.Fatal("an unknown routine ran")
	}
}

// A replay goes through the same caps as speech: one utterance's worth of
// actions, and the per-minute bound shared with voice.
func TestOniRoutineReplayKeepsTheCaps(t *testing.T) {
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	most, perMinute := sp.Reflex.MaxActions, sp.Reflex.MaxActionsPerMinute
	rr := serveRoutines(t, false)
	long := make([]routine.Step, 0, sp.Reflex.Routines.MaxSteps)
	for i := range sp.Reflex.Routines.MaxSteps {
		long = append(long, routine.Step{Intent: []reflex.Intent{reflex.VolumeUp, reflex.VolumeDown}[i%2]})
	}
	if len(long) <= most {
		t.Skipf("a routine of %d steps fits one utterance of %d", len(long), most)
	}
	if err := rr.store.Put(routine.Routine{Name: "Long", Steps: long}); err != nil {
		t.Fatal(err)
	}
	var res protocol.OniRoutinesResult
	if err := rr.cli.call(protocol.MOniRoutines, protocol.OniRoutinesParams{Action: protocol.OniRoutinesRun, Name: "long"}, &res); err != nil || !res.Ran {
		t.Fatalf("run = %+v, %v", res, err)
	}
	if n := len(rr.exec.calls()); n != most {
		t.Fatalf("one replay ran %d actions, want %d", n, most)
	}
	// replays one after another stop at the per-minute bound
	for range perMinute {
		if err := rr.cli.call(protocol.MOniRoutines, protocol.OniRoutinesParams{Action: protocol.OniRoutinesRun}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(rr.exec.calls()); n != perMinute {
		t.Fatalf("replays ran %d actions in a minute, want %d", n, perMinute)
	}
	if rr.decider.n.Load() != 0 || len(rr.model.sent()) != 0 {
		t.Fatal("a replay asked a decision or a model")
	}
}

// oni.routines lists, runs, forgets and forgets all, for the app and the
// CLI, and needs reflex.routines.
func TestOniRoutinesCallListsRunsAndForgets(t *testing.T) {
	rr := serveRoutines(t, false)
	call := func(p protocol.OniRoutinesParams) (protocol.OniRoutinesResult, error) {
		var res protocol.OniRoutinesResult
		err := rr.cli.call(protocol.MOniRoutines, p, &res)
		return res, err
	}
	list, err := call(protocol.OniRoutinesParams{Action: protocol.OniRoutinesList})
	if err != nil || list.Routines == nil || len(list.Routines) != 0 || list.Ran {
		t.Fatalf("empty list = %#v, %v", list, err)
	}
	rr.nothingKept(t)
	if code, _ := reasonOf(rr.cli.call(protocol.MOniRoutines, protocol.OniRoutinesParams{Action: protocol.OniRoutinesRun}, nil)); code != protocol.CodeNotFound {
		t.Fatalf("running with nothing kept: %s", code)
	}
	for _, name := range []string{"Morning", "Wind down"} {
		if err := rr.store.Put(routine.Routine{Name: name, Steps: threeSteps}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	list, err = call(protocol.OniRoutinesParams{Action: protocol.OniRoutinesList})
	if err != nil || len(list.Routines) != 2 || list.Routines[0].Name != "Wind down" || list.Routines[1].Steps != 3 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	ran, err := call(protocol.OniRoutinesParams{Action: protocol.OniRoutinesRun, Name: "morning"})
	if err != nil || !ran.Ran || ran.Routines[0].Name != "Morning" || len(rr.exec.calls()) != 3 {
		t.Fatalf("run = %+v, %v, did %+v", ran, err, rr.exec.calls())
	}
	if code, _ := reasonOf(rr.cli.call(protocol.MOniRoutines, protocol.OniRoutinesParams{Action: protocol.OniRoutinesRun, Name: "Evening"}, nil)); code != protocol.CodeNotFound {
		t.Fatalf("running an unknown routine: %s", code)
	}
	if code, _ := reasonOf(rr.cli.call(protocol.MOniRoutines, protocol.OniRoutinesParams{Action: protocol.OniRoutinesForget, Name: "Evening"}, nil)); code != protocol.CodeNotFound {
		t.Fatalf("forgetting an unknown routine: %s", code)
	}
	forgot, err := call(protocol.OniRoutinesParams{Action: protocol.OniRoutinesForget, Name: "MORNING"})
	if err != nil || len(forgot.Routines) != 1 || forgot.Routines[0].Name != "Wind down" || forgot.Ran {
		t.Fatalf("forget = %+v, %v", forgot, err)
	}
	all, err := call(protocol.OniRoutinesParams{Action: protocol.OniRoutinesForgetAll})
	if err != nil || all.Routines == nil || len(all.Routines) != 0 {
		t.Fatalf("forget all = %#v, %v", all, err)
	}
	rr.nothingKept(t)
	if rr.decider.n.Load() != 0 || len(rr.model.sent()) != 0 {
		t.Fatal("oni.routines asked a decision or a model")
	}

	// the presenter may not call it, and with routines off it is refused
	if err := rr.surface.call(protocol.MOniRoutines, protocol.OniRoutinesParams{Action: protocol.OniRoutinesList}, nil); !forbidden(err) {
		t.Fatalf("the presenter: %v", err)
	}
	rr.patch(t, `{"oni":{"reflexSettings":{"routines":false}}}`)
	if code, reason := reasonOf(rr.cli.call(protocol.MOniRoutines, protocol.OniRoutinesParams{Action: protocol.OniRoutinesList}, nil)); code != protocol.CodeDisabled || reason == "" {
		t.Fatalf("with routines off: %s %q", code, reason)
	}
}

// With routines off, quick commands are watched by nothing: no recorder is
// built and no offer is made.
func TestOniRoutineOffRecordsNothing(t *testing.T) {
	rr := serveRoutines(t, false)
	rr.patch(t, `{"oni":{"reflexSettings":{"routines":false}}}`)
	rr.command(t, "open "+routineApp, 1)
	rr.command(t, "turn it up", 2)
	rr.command(t, "maximize", 3)
	rr.log.quiet("an offer with routines off", consentFor(feature.Routines))
	rr.d.oni.routine.mu.Lock()
	built := rr.d.oni.routine.rec != nil
	rr.d.oni.routine.mu.Unlock()
	if built {
		t.Fatal("a recorder was built with routines off")
	}
	rr.nothingKept(t)
}
