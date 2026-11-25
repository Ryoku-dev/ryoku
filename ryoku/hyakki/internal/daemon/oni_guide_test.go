package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/keep"
	"ryoku-rashin/internal/oni/ocr"
	"ryoku-rashin/internal/oni/screenask"
	fingerprint "ryoku-rashin/internal/oni/settle"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/protocol"
)

// These tests drive guides, Keep and select-and-ask over the daemon's own
// socket. The desktop, grim, model, source-relative OCR words and settle pictures
// are fakes, and every clock the guide reads is the test's.

// keep guide fixtures in source coordinates, rasterization belongs to the reader
type fakeOCR struct {
	mu      sync.Mutex
	words   []screenask.Word
	missing bool
	calls   int
}

func (f *fakeOCR) set(words ...screenask.Word) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.words = words
}

func (f *fakeOCR) gone() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.missing = true
}

func (f *fakeOCR) Words(_ context.Context, _ []byte) ([]screenask.Word, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.missing {
		return nil, ocr.ErrUnavailable
	}
	return slices.Clone(f.words), nil
}

func (f *fakeOCR) state() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// fakeScreen is the settle watch's fresh pictures: one flat gray level the
// test sets. The guide's read is of fakeGrim's level-40 picture, so 40 is
// an unchanged window and anything far from it a changed one.
type fakeScreen struct {
	level  atomic.Int32
	frames atomic.Int32
}

func newFakeScreen() *fakeScreen {
	s := &fakeScreen{}
	s.level.Store(40)
	return s
}

func (s *fakeScreen) take(context.Context, window.Window) (fingerprint.Gray, error) {
	s.frames.Add(1)
	return flatGray(s.level.Load()), nil
}

type flatGray uint8

func (g flatGray) Gray(w, h int, _ *feature.Box) []uint8 {
	out := make([]uint8, w*h)
	for i := range out {
		out[i] = uint8(g)
	}
	return out
}

// The fixture window's controls, in fractions of the editor window, and
// the replies that point at them.
var (
	fileBox  = feature.Box{X: 0.1, Y: 0.05, W: 0.1, H: 0.05}
	saveBox  = feature.Box{X: 0.3, Y: 0.05, W: 0.1, H: 0.05}
	fileWord = screenask.Word{Text: "File", Box: fileBox}
	saveWord = screenask.Word{Text: "Save", Box: saveBox}
)

const (
	stepFile = "Open the File menu.\n[POINT:0.1,0.05,0.1,0.05:File][CONF:0.9]"
	stepSave = "Press Save.\n[POINT:0.3,0.05,0.1,0.05:Save][CONF:0.9][DONE]"
)

func (r *wireRig) guideOn(t *testing.T, keepOn bool) {
	t.Helper()
	patch := fmt.Sprintf(`{"oni":{"guide":{"enabled":true,"keep":%t}}}`, keepOn)
	if _, err := r.d.applySettings(context.Background(), json.RawMessage(patch)); err != nil {
		t.Fatal(err)
	}
}

// allowRoute is the user having said yes to the fixture route this session.
func (r *wireRig) allowRoute() {
	a := &r.d.oni.ask
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.asked == nil {
		a.asked = map[string]bool{}
	}
	a.asked[localEyes.Route()] = true
}

// guideRunning reads, on the guide's own queue, whether a guide runs.
func (r *wireRig) guideRunning() bool {
	ch := make(chan bool, 1)
	r.d.guideDo(func() { ch <- r.d.guideLive(r.d.oni.guide.run) })
	return <-ch
}

func (r *wireRig) awaitNoGuide(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for r.guideRunning() {
		if time.Now().After(deadline) {
			t.Fatal("the guide is still running")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// pump moves the guide's clock a settle frame at a time until done.
func (r *wireRig) pump(t *testing.T, done func() bool) {
	t.Helper()
	for range 400 {
		if done() {
			return
		}
		r.clock.Advance(150 * time.Millisecond)
		time.Sleep(3 * time.Millisecond)
	}
	t.Fatal("the guide never came to rest")
}

type callResult struct {
	res protocol.OniAskResult
	err error
}

// async makes a call that may wait on the guide's clock.
func (p *socketPeer) async(method string, params any) chan callResult {
	ch := make(chan callResult, 1)
	go func() {
		var res protocol.OniAskResult
		err := p.call(method, params, &res)
		ch <- callResult{res, err}
	}()
	return ch
}

func (r *wireRig) guide(t *testing.T, p *socketPeer, action string) protocol.OniAskResult {
	t.Helper()
	ch := p.async(protocol.MOniGuide, protocol.OniGuideParams{Action: action})
	settling := make(chan bool, 1)
	checkSettling := func() {
		waiting := false
		if run := r.d.oni.guide.run; run != nil && run.end == nil {
			for token, work := range run.work {
				if token != run.parked && work.deadline == nil {
					waiting = true
					break
				}
			}
		}
		settling <- waiting
	}
	tick := time.NewTicker(3 * time.Millisecond)
	defer tick.Stop()
	frames := 0
	for {
		select {
		case got := <-ch:
			if got.err != nil {
				t.Fatalf("oni.guide %s: %v", action, got.err)
			}
			return got.res
		case <-tick.C:
			r.d.guideDo(checkSettling)
			if !<-settling {
				continue
			}
			if frames == 400 {
				t.Fatal("the guide never came to rest")
			}
			// CPU-bound PNG/JPEG work does not consume fake settle frames.
			r.clock.Advance(150 * time.Millisecond)
			frames++
		}
	}
}

func startGuide(t *testing.T, p *socketPeer, goal string) protocol.OniAskResult {
	t.Helper()
	var res protocol.OniAskResult
	if err := p.call(protocol.MOniGuide, protocol.OniGuideParams{Action: protocol.OniGuideStart, Goal: goal}, &res); err != nil {
		t.Fatalf("oni.guide start: %v", err)
	}
	return res
}

// await reads oni frames until one matches, skipping the rest.
func (p *socketPeer) await(what string, match func(feature.Payload) bool) feature.Payload {
	p.t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case e, ok := <-p.conn.Events():
			if !ok {
				p.t.Fatalf("the connection closed before %s", what)
			}
			if e.Event != protocol.EvtOni {
				continue
			}
			f, err := feature.DecodeFrame(e.Data)
			if err != nil {
				p.t.Fatalf("the daemon sent a frame the contract refuses: %v: %s", err, e.Data)
			}
			if match(f) {
				return f
			}
		case <-timeout:
			p.t.Fatalf("no %s", what)
		}
	}
}

// noStrand reads the frames for a moment and fails on any strand target.
func (p *socketPeer) noStrand() {
	p.t.Helper()
	timeout := time.After(150 * time.Millisecond)
	for {
		select {
		case e := <-p.conn.Events():
			if f, err := feature.DecodeFrame(e.Data); err == nil {
				if _, ok := f.(*feature.StrandTargetFrame); ok {
					p.t.Fatalf("a strand went out: %s", e.Data)
				}
			}
		case <-timeout:
			return
		}
	}
}

// drain drops the frames already sent.
func (p *socketPeer) drain() {
	for {
		select {
		case <-p.conn.Events():
		case <-time.After(50 * time.Millisecond):
			return
		}
	}
}

func guidePhase(phase string, step int) func(feature.Payload) bool {
	return func(p feature.Payload) bool {
		g, ok := p.(*feature.GuideFrame)
		return ok && g.Phase == phase && (step == 0 || g.Step == step)
	}
}

func strandAt(seq uint64) func(feature.Payload) bool {
	return func(p feature.Payload) bool {
		s, ok := p.(*feature.StrandTargetFrame)
		return ok && s.Seq == seq
	}
}

// guideRig is a serving daemon with the guide on, Oni perched on the editor
// and a presenter and an observer drawing it.
func guideRig(t *testing.T, keepOn bool) (r *wireRig, surface, watcher, cli *socketPeer) {
	t.Helper()
	r = serveWire(t, true, localEyes)
	r.guideOn(t, keepOn)
	r.tess.set(fileWord, saveWord)
	surface, watcher = r.presenting(t)
	cli = dialSocket(t, protocol.ClientCLI)
	perchOn(t, cli, "0x1")
	surface.frames(2)
	watcher.frames(2)
	r.desk.await(t, 1, "0x1")
	return r, surface, watcher, cli
}

// A guide asks before its first picture leaves, exactly as oni.ask does;
// each step is one capture, its words read here, one model call, a point
// and a guide frame; the next step waits for the strand to land, and a
// stale landing advances nothing; a trusted finish is offered for keeping,
// and only oni.keep writes it.
func TestOniGuideAsksFirstWalksEachStepAfterItsLandingAndKeepsOnlyOnKeep(t *testing.T) {
	r, surface, watcher, cli := guideRig(t, true)
	r.model.reply(stepFile)
	res := startGuide(t, cli, "save this file")
	if res.Status != protocol.OniAskConsent || res.Consent == nil || res.Consent.Route != localEyes.Route() || res.Consent.Destination != "local" || len(res.Consent.Preview) == 0 {
		t.Fatalf("start = %+v", res)
	}
	if c, ok := surface.await("the consent frame", func(p feature.Payload) bool { _, ok := p.(*feature.ConsentFrame); return ok }).(*feature.ConsentFrame); !ok || c.ID != res.Consent.ID || c.Feature != feature.Guide || c.Class != feature.ConsentDisclosure {
		t.Fatalf("consent frame = %+v", c)
	}
	if calls := r.tess.state(); r.grim.count() != 1 || calls != 1 || len(r.model.sent()) != 0 {
		t.Fatalf("before the yes: %d grabs, %d text reads, %d requests", r.grim.count(), calls, len(r.model.sent()))
	}
	if err := cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: res.Consent.ID, Answer: "yes"}, &res); err != nil {
		t.Fatal(err)
	}
	a := res.Answer
	if res.Status != protocol.OniAskAnswered || a == nil || a.Mode != protocol.OniModeGuide || a.Step != 1 || a.Seq == 0 || a.Point == nil || *a.Point != (protocol.OniRect{X: 0.1, Y: 0.05, W: 0.1, H: 0.05}) || a.Text != "Open the File menu." || a.Route != localEyes.Route() || a.Destination != "local" {
		t.Fatalf("step 1 = %+v %+v", res, a)
	}
	sent := r.model.sent()
	if len(sent) != 1 || !sent[0].Strict || !sent[0].LocalOnly || len(sent[0].Tools) != 0 || !strings.Contains(sent[0].Messages[1].Content, "save this file") {
		t.Fatalf("requests = %+v", sent)
	}
	if !r.shots.cleared() {
		t.Fatal("step 1's picture outlived its read")
	}
	first := a.Seq
	if s := surface.await("step 1's strand", strandAt(first)).(*feature.StrandTargetFrame); s.Mode != feature.StrandModeLasso || s.Label != "File" {
		t.Fatalf("strand = %+v", s)
	}
	if g := surface.await("step 1's card", guidePhase(feature.GuidePointing, 1)).(*feature.GuideFrame); g.Target != first || g.Text != "Open the File menu." || g.Kicker != "Step 1" {
		t.Fatalf("card = %+v", g)
	}

	// Next before the strand lands is held, and a stale landing moves nothing
	if err := cli.call(protocol.MOniGuide, protocol.OniGuideParams{Action: protocol.OniGuideNext}, &res); err != nil || res.Answer == nil || res.Answer.Seq != first {
		t.Fatalf("an early next = %+v, %v", res, err)
	}
	if err := surface.arrived(first+7, ""); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if r.screen.frames.Load() != 0 || len(r.model.sent()) != 1 {
		t.Fatal("a stale landing advanced the guide")
	}
	// the landing lets the held next go on: the window changed, step 2 reads
	r.screen.level.Store(120)
	r.model.reply(stepSave)
	if err := surface.arrived(first, ""); err != nil {
		t.Fatal(err)
	}
	r.pump(t, func() bool { return len(r.model.sent()) == 2 })
	card := surface.await("step 2's card", guidePhase(feature.GuidePointing, 2)).(*feature.GuideFrame)
	if card.Plan == nil || !slices.Equal(card.Plan.Done, []string{"Open the File menu."}) {
		t.Fatalf("step 2's card = %+v", card)
	}
	second := card.Target
	if user := r.model.sent()[1].Messages[1].Content; !strings.Contains(user, "Open the File menu.") {
		t.Fatalf("step 2 was asked without the step done: %q", user)
	}
	if frames := r.screen.frames.Load(); frames < 2 || int(frames) > oniSettleConfig().MaxFrames {
		t.Fatalf("the settle watch took %d frames", frames)
	}
	if err := surface.arrived(second, ""); err != nil {
		t.Fatal(err)
	}
	surface.await("step 2 waiting", guidePhase(feature.GuideWaiting, 2))

	// the last step done: the trusted guide is offered for keeping
	res = r.guide(t, cli, protocol.OniGuideNext)
	if res.Status != protocol.OniAskEnded || res.Answer == nil || !res.Answer.Done || res.Keep == nil || res.Keep.ExpiresMs != feature.KeepOfferTTL().Milliseconds() {
		t.Fatalf("the finish = %+v %+v", res, res.Answer)
	}
	offer := surface.await("the keep offer", func(p feature.Payload) bool {
		c, ok := p.(*feature.ConsentFrame)
		return ok && c.Feature == feature.GuideKeep
	}).(*feature.ConsentFrame)
	if offer.ID != res.Keep.ID || offer.Destination != feature.EgressLocal || offer.Route != "" {
		t.Fatalf("offer = %+v", offer)
	}
	if _, err := os.Stat(keep.DefaultStorePath()); !os.IsNotExist(err) {
		t.Fatalf("the offer wrote the store: %v", err)
	}
	// the presenter never keeps; the CLI does
	if err := surface.call(protocol.MOniKeep, protocol.OniKeepParams{ID: offer.ID}, nil); !forbidden(err) {
		t.Fatalf("the presenter kept: %v", err)
	}
	var kept protocol.OniKeepResult
	if err := cli.call(protocol.MOniKeep, protocol.OniKeepParams{ID: offer.ID}, &kept); err != nil || !kept.Kept || kept.Entries != 1 {
		t.Fatalf("keep = %+v, %v", kept, err)
	}
	if ans := surface.await("the offer's answer", func(p feature.Payload) bool { _, ok := p.(*feature.ConsentAnswerFrame); return ok }).(*feature.ConsentAnswerFrame); ans.ID != offer.ID || ans.Answer != "yes" {
		t.Fatalf("answer frame = %+v", ans)
	}
	info, err := os.Stat(keep.DefaultStorePath())
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("store = %v, %v", info, err)
	}
	raw, err := os.ReadFile(keep.DefaultStorePath())
	if err != nil || bytes.Contains(raw, []byte("notes.txt")) || !bytes.Contains(raw, []byte(`"Save"`)) || bytes.Contains(raw, []byte("Open the File menu.\n")) {
		t.Fatalf("store holds %s, %v", raw, err)
	}
	if code, _ := reasonOf(cli.call(protocol.MOniKeep, protocol.OniKeepParams{ID: offer.ID}, nil)); code != protocol.CodeNotFound {
		t.Fatalf("a second keep: %s", code)
	}
	// the closing line stays, then the card goes and nothing is left armed
	surface.await("the closing card", guidePhase(feature.GuideDone, 0))
	r.clock.Advance(time.Minute)
	watcher.await("the idle card", guidePhase(feature.GuideIdle, 0))
	if r.clock.Armed() != 0 || !r.shots.cleared() {
		t.Fatalf("%d timers left armed, pictures cleared %v", r.clock.Armed(), r.shots.cleared())
	}
}

// Next draws step 1's strand back and reads step 2 at once. The presenter's
// reports from before the retract held still only because it had not drawn
// the retract yet: step 2's picture waits for the reports that follow it and
// masks where the parts came to rest, never where step 1 left them (the
// card swept over the next control while the strand drew back).
func TestOniGuideStepPictureWaitsForTheRetractToBeDrawn(t *testing.T) {
	r, surface, _, cli := guideRig(t, false)
	r.allowRoute()
	r.model.reply(stepFile)
	res := startGuide(t, cli, "save this file")
	if res.Status != protocol.OniAskAnswered || res.Answer == nil || res.Answer.Seq == 0 {
		t.Fatalf("start = %+v", res)
	}
	first := res.Answer.Seq
	surface.await("step 1's strand", strandAt(first))
	at := func(x float64) protocol.OniOverlayParams {
		return protocol.OniOverlayParams{Output: "DP-1", Rects: []protocol.OniRect{{X: x, Y: 80, W: 200, H: 120}}}
	}
	// step 1's strand and card as they sat once it landed, then still
	if err := surface.call(protocol.MOniOverlay, at(100), nil); err != nil {
		t.Fatal(err)
	}
	if err := surface.arrived(first, ""); err != nil {
		t.Fatal(err)
	}
	surface.await("step 1 waiting", guidePhase(feature.GuideWaiting, 1))
	time.Sleep(2 * oniOverlayQuiet)

	// the presenter draws the retract a frame after it reads it: three
	// frames, the card coming to rest at x 500
	drawn := make(chan error, 1)
	go func() {
		for e := range surface.conn.Events() {
			if e.Event != protocol.EvtOni {
				continue
			}
			f, err := feature.DecodeFrame(e.Data)
			if err != nil {
				drawn <- err
				return
			}
			if _, ok := f.(*feature.StrandRetractFrame); !ok {
				continue
			}
			for i := range 3 {
				time.Sleep(20 * time.Millisecond)
				if err := surface.call(protocol.MOniOverlay, at(float64(400+50*i)), nil); err != nil {
					drawn <- err
					return
				}
			}
			drawn <- nil
			return
		}
		drawn <- fmt.Errorf("the presenter's connection closed")
	}()
	r.screen.level.Store(120)
	r.model.reply(stepSave)
	next := cli.async(protocol.MOniGuide, protocol.OniGuideParams{Action: protocol.OniGuideNext})
	r.pump(t, func() bool { return len(r.model.sent()) == 2 })
	if err := <-drawn; err != nil {
		t.Fatal(err)
	}
	if got := <-next; got.err != nil || got.res.Answer == nil || got.res.Answer.Step != 2 {
		t.Fatalf("next = %+v, %v", got.res, got.err)
	}
	if got, want := r.shots.lastMasks(), []window.Box{{X: 500, Y: 80, W: 200, H: 120}}; !slices.Equal(got, want) {
		t.Fatalf("step 2's picture masked %+v, want where the presenter came to rest after the retract %+v", got, want)
	}
}

// Next on an unchanged window re-points a trusted step once ("again"),
// then explains; a stale landing of the first strand moves nothing, and
// Next past the explanation goes on to the next read.
func TestOniGuideUnchangedScreenRepointsOnceThenExplains(t *testing.T) {
	r, surface, _, cli := guideRig(t, false)
	r.allowRoute()
	r.model.reply(stepFile)
	res := startGuide(t, cli, "save this file")
	first := res.Answer.Seq
	if res.Status != protocol.OniAskAnswered || first == 0 {
		t.Fatalf("start = %+v", res)
	}
	if err := surface.arrived(first, ""); err != nil {
		t.Fatal(err)
	}
	surface.await("step 1 waiting", guidePhase(feature.GuideWaiting, 1))
	res = r.guide(t, cli, protocol.OniGuideNext)
	again := res.Answer.Seq
	if res.Status != protocol.OniAskAnswered || again == first || res.Answer.Step != 1 {
		t.Fatalf("an unchanged next = %+v", res.Answer)
	}
	if g := surface.await("the again card", guidePhase(feature.GuidePointing, 1)).(*feature.GuideFrame); g.Kicker != "Step 1 · again" || g.Target != again {
		t.Fatalf("again = %+v", g)
	}
	if n := int(r.screen.frames.Load()); n != oniSettleConfig().MaxFrames {
		t.Fatalf("an unchanged window took %d settle frames", n)
	}
	if len(r.model.sent()) != 1 {
		t.Fatal("re-pointing a trusted step asked the model")
	}
	if err := surface.arrived(first, ""); err != nil {
		t.Fatal(err)
	}
	if err := surface.arrived(again, ""); err != nil {
		t.Fatal(err)
	}
	surface.await("the again step waiting", guidePhase(feature.GuideWaiting, 1))
	res = r.guide(t, cli, protocol.OniGuideNext)
	if res.Status != protocol.OniAskAnswered || res.Answer.Text == "" || res.Answer.Seq != again {
		t.Fatalf("a second unchanged next = %+v", res.Answer)
	}
	surface.await("the explaining card", guidePhase(feature.GuideExplaining, 1))
	// Next past it is the user's call: the next step is read
	r.model.reply("You are done.\n[POINT:none][DONE]")
	res = r.guide(t, cli, protocol.OniGuideNext)
	if res.Status != protocol.OniAskEnded || res.Answer == nil || !res.Answer.Done || res.Answer.Text != "You are done." || res.Keep != nil || len(r.model.sent()) != 2 {
		t.Fatalf("forced on = %+v %+v", res, res.Answer)
	}
}

// Every way a guide is wound back retracts its strand, cancels its read,
// clears its pictures and lets a late reply change nothing.
func TestOniGuideUnwindsOnEveryPath(t *testing.T) {
	paths := map[string]func(t *testing.T, r *wireRig, surface, cli *socketPeer){
		"dismiss": func(t *testing.T, _ *wireRig, _, _ *socketPeer) {
			other := dialSocket(t, protocol.ClientCLI)
			if err := other.call(protocol.MOniGuide, protocol.OniGuideParams{Action: protocol.OniGuideStop}, nil); err != nil {
				t.Fatal(err)
			}
		},
		"detach": func(t *testing.T, _ *wireRig, _, cli *socketPeer) {
			if err := cli.call(protocol.MOniDetach, protocol.OniDetachParams{}, nil); err != nil {
				t.Fatal(err)
			}
		},
		"window close": func(_ *testing.T, r *wireRig, _, _ *socketPeer) {
			closed := desktopWindows()
			closed.Windows = closed.Windows[1:]
			r.desk.set(closed)
		},
		"oni off": func(t *testing.T, r *wireRig, _, _ *socketPeer) {
			if _, err := r.d.applySettings(context.Background(), json.RawMessage(`{"oni":{"master":false}}`)); err != nil {
				t.Fatal(err)
			}
		},
		"presenter lost": func(_ *testing.T, _ *wireRig, surface, _ *socketPeer) {
			surface.conn.Close()
		},
		"settings change": func(t *testing.T, r *wireRig, _, _ *socketPeer) {
			if _, err := r.d.applySettings(context.Background(), json.RawMessage(`{"oni":{"guide":{"enabled":false}}}`)); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, unwind := range paths {
		t.Run(name+" mid-read", func(t *testing.T) {
			r, surface, watcher, cli := guideRig(t, false)
			r.allowRoute()
			hold := make(chan struct{})
			r.model.mu.Lock()
			r.model.hold = hold
			r.model.mu.Unlock()
			r.model.reply(stepFile)
			ch := cli.async(protocol.MOniGuide, protocol.OniGuideParams{Action: protocol.OniGuideStart, Goal: "save this file"})
			r.pump(t, func() bool { return len(r.model.sent()) == 1 })
			unwind(t, r, surface, cli)
			got := <-ch
			if got.err != nil || got.res.Status != protocol.OniAskEnded {
				t.Fatalf("the start ended as %+v, %v", got.res, got.err)
			}
			r.awaitNoGuide(t)
			close(hold)
			watcher.noStrand()
			if !r.shots.cleared() {
				t.Fatal("a picture outlived the guide")
			}
		})
		t.Run(name+" while pointing", func(t *testing.T) {
			r, surface, watcher, cli := guideRig(t, false)
			r.allowRoute()
			r.model.reply(stepFile)
			res := startGuide(t, cli, "save this file")
			seq := res.Answer.Seq
			watcher.await("the step's strand", strandAt(seq))
			unwind(t, r, surface, cli)
			if f := watcher.await("the strand winding back", func(p feature.Payload) bool { _, ok := p.(*feature.StrandRetractFrame); return ok }); f.(*feature.StrandRetractFrame).Seq != seq {
				t.Fatalf("retracted %+v", f)
			}
			r.awaitNoGuide(t)
			if code, _ := reasonOf(cli.call(protocol.MOniGuide, protocol.OniGuideParams{Action: protocol.OniGuideNext}, nil)); code != protocol.CodeConflict {
				t.Fatalf("next after the unwind: %s", code)
			}
			if !r.shots.cleared() {
				t.Fatal("a picture outlived the guide")
			}
		})
	}
}

// A consent question that went stale is answered not_found and sends
// nothing: the guide stopped elsewhere, the question expired, or the asker
// went away.
func TestOniGuideStaleConsentSendsNothing(t *testing.T) {
	r, surface, _, cli := guideRig(t, false)
	r.model.reply(stepFile, stepFile, stepFile)
	answerStale := func(id string) {
		t.Helper()
		if code, _ := reasonOf(cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: id, Answer: "yes"}, nil)); code != protocol.CodeNotFound {
			t.Fatalf("a stale yes: %s", code)
		}
		r.awaitNoGuide(t)
		if len(r.model.sent()) != 0 || !r.shots.cleared() {
			t.Fatal("a stale consent sent or kept the picture")
		}
	}
	res := startGuide(t, cli, "save this file")
	other := dialSocket(t, protocol.ClientCLI)
	if err := other.call(protocol.MOniGuide, protocol.OniGuideParams{Action: protocol.OniGuideStop}, nil); err != nil {
		t.Fatal(err)
	}
	answerStale(res.Consent.ID)

	res = startGuide(t, cli, "save this file")
	r.clock.Advance(feature.ConsentTTL)
	answerStale(res.Consent.ID)

	asker := dialSocket(t, protocol.ClientCLI)
	res = startGuide(t, asker, "save this file")
	asker.conn.Close()
	r.awaitNoGuide(t)
	answerStale(res.Consent.ID)

	// no clears it and ends the guide
	res = startGuide(t, cli, "save this file")
	if err := cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: res.Consent.ID, Answer: "no"}, &res); err != nil || res.Status != protocol.OniAskDeclined {
		t.Fatalf("no = %+v, %v", res, err)
	}
	r.awaitNoGuide(t)
	if len(r.model.sent()) != 0 || !r.shots.cleared() {
		t.Fatal("a no sent or kept the picture")
	}
	surface.await("the winding card", guidePhase(feature.GuideWinding, 0))
}

// Disclosure "always" asks before every step's picture, "session" once.
func TestOniGuideDisclosureAlwaysAsksEveryStep(t *testing.T) {
	r, surface, _, cli := guideRig(t, false)
	if _, err := r.d.applySettings(context.Background(), json.RawMessage(`{"oni":{"disclosure":"always"}}`)); err != nil {
		t.Fatal(err)
	}
	r.screen.level.Store(120)
	r.model.reply(stepFile, stepSave)
	res := startGuide(t, cli, "save this file")
	if err := cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: res.Consent.ID, Answer: "yes"}, &res); err != nil || res.Status != protocol.OniAskAnswered {
		t.Fatalf("step 1 = %+v, %v", res, err)
	}
	if err := surface.arrived(res.Answer.Seq, ""); err != nil {
		t.Fatal(err)
	}
	surface.await("step 1 waiting", guidePhase(feature.GuideWaiting, 1))
	res = r.guide(t, cli, protocol.OniGuideNext)
	if res.Status != protocol.OniAskConsent || res.Consent == nil || len(r.model.sent()) != 1 {
		t.Fatalf("step 2 under always = %+v", res)
	}
	if err := cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: res.Consent.ID, Answer: "yes"}, &res); err != nil || res.Status != protocol.OniAskAnswered || res.Answer.Step != 2 {
		t.Fatalf("step 2 = %+v, %v", res, err)
	}
}

// A later step's picture is answered by whoever pressed Next for it, as
// `hyakki oni guide next` is a connection of its own once the one that
// started the guide has gone; a bystander still may not answer.
func TestOniGuideNextCallerAnswersTheStepItAskedFor(t *testing.T) {
	r, surface, _, cli := guideRig(t, false)
	if _, err := r.d.applySettings(context.Background(), json.RawMessage(`{"oni":{"disclosure":"always"}}`)); err != nil {
		t.Fatal(err)
	}
	r.screen.level.Store(120)
	r.model.reply(stepFile, stepSave)
	starter := dialSocket(t, protocol.ClientCLI)
	res := startGuide(t, starter, "save this file")
	if err := starter.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: res.Consent.ID, Answer: "yes"}, &res); err != nil || res.Status != protocol.OniAskAnswered {
		t.Fatalf("step 1 = %+v, %v", res, err)
	}
	if err := surface.arrived(res.Answer.Seq, ""); err != nil {
		t.Fatal(err)
	}
	surface.await("step 1 waiting", guidePhase(feature.GuideWaiting, 1))
	starter.conn.Close()
	res = r.guide(t, cli, protocol.OniGuideNext)
	if res.Status != protocol.OniAskConsent || res.Consent == nil || len(r.model.sent()) != 1 {
		t.Fatalf("step 2 under always = %+v", res)
	}
	bystander := dialSocket(t, protocol.ClientCLI)
	if code, _ := reasonOf(bystander.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: res.Consent.ID, Answer: "yes"}, nil)); code != protocol.CodeForbidden {
		t.Fatalf("a bystander's yes: %s", code)
	}
	if err := cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: res.Consent.ID, Answer: "yes"}, &res); err != nil || res.Status != protocol.OniAskAnswered || res.Answer.Step != 2 {
		t.Fatalf("the Next caller's yes = %+v, %v", res, err)
	}
	if len(r.model.sent()) != 2 {
		t.Fatalf("%d pictures sent, want 2", len(r.model.sent()))
	}
}

// A guide ends at the step ceiling, and a read that outlives its deadline
// ends it with the timeout line; the late reply changes nothing.
func TestOniGuideEndsAtTheCeilingAndTheReadTimeout(t *testing.T) {
	t.Run("ceiling", func(t *testing.T) {
		r, surface, _, cli := guideRig(t, false)
		r.allowRoute()
		r.screen.level.Store(120)
		ceiling := feature.MaxGuideSteps()
		for range ceiling {
			r.model.reply(stepFile)
		}
		res := startGuide(t, cli, "save this file")
		for step := 1; ; step++ {
			if res.Status != protocol.OniAskAnswered || res.Answer.Step != step {
				t.Fatalf("step %d = %+v %+v", step, res, res.Answer)
			}
			if err := surface.arrived(res.Answer.Seq, ""); err != nil {
				t.Fatal(err)
			}
			surface.await("the step waiting", guidePhase(feature.GuideWaiting, step))
			res = r.guide(t, cli, protocol.OniGuideNext)
			if step == ceiling {
				break
			}
		}
		if res.Status != protocol.OniAskEnded || res.Answer.Done || res.Answer.Text == "" || len(r.model.sent()) != ceiling {
			t.Fatalf("the ceiling = %+v %+v, %d requests", res, res.Answer, len(r.model.sent()))
		}
	})
	t.Run("timeout", func(t *testing.T) {
		r, _, watcher, cli := guideRig(t, false)
		r.allowRoute()
		hold := make(chan struct{})
		r.model.mu.Lock()
		r.model.hold = hold
		r.model.mu.Unlock()
		r.model.reply(stepFile)
		ch := cli.async(protocol.MOniGuide, protocol.OniGuideParams{Action: protocol.OniGuideStart, Goal: "save this file"})
		for len(r.model.sent()) == 0 {
			time.Sleep(2 * time.Millisecond)
		}
		r.clock.Advance(74 * time.Second)
		time.Sleep(20 * time.Millisecond)
		if len(ch) != 0 {
			t.Fatal("the read ended before its deadline")
		}
		r.clock.Advance(time.Second)
		got := <-ch
		if got.err != nil || got.res.Status != protocol.OniAskEnded || got.res.Answer.Done || got.res.Answer.Text == "" {
			t.Fatalf("the timeout = %+v, %v", got.res, got.err)
		}
		close(hold)
		watcher.noStrand()
		r.awaitNoGuide(t)
	})
}

// The keep offer writes nothing when it is skipped, when it lapses, or when
// a newer guide takes its place.
func TestOniGuideKeepOfferWritesNothingUnlessKept(t *testing.T) {
	finish := func(t *testing.T, r *wireRig, surface, cli *socketPeer) protocol.OniKeepOffer {
		t.Helper()
		r.allowRoute()
		r.screen.level.Store(120)
		r.model.reply(stepSave)
		res := startGuide(t, cli, "save this file")
		if err := surface.arrived(res.Answer.Seq, ""); err != nil {
			t.Fatal(err)
		}
		surface.await("the step waiting", guidePhase(feature.GuideWaiting, 1))
		res = r.guide(t, cli, protocol.OniGuideNext)
		if res.Keep == nil {
			t.Fatalf("no keep offer: %+v %+v", res, res.Answer)
		}
		return *res.Keep
	}
	nothing := func(t *testing.T, surface *socketPeer, id string) {
		t.Helper()
		if ans := surface.await("the offer's answer", func(p feature.Payload) bool { _, ok := p.(*feature.ConsentAnswerFrame); return ok }).(*feature.ConsentAnswerFrame); ans.ID != id || ans.Answer != "no" {
			t.Fatalf("answer frame = %+v", ans)
		}
		if _, err := os.Stat(keep.DefaultStorePath()); !os.IsNotExist(err) {
			t.Fatalf("the store was written: %v", err)
		}
	}
	t.Run("skip", func(t *testing.T) {
		r, surface, _, cli := guideRig(t, true)
		offer := finish(t, r, surface, cli)
		if err := surface.call(protocol.MOniSkip, protocol.OniKeepParams{ID: offer.ID}, nil); err != nil {
			t.Fatal(err)
		}
		nothing(t, surface, offer.ID)
		if code, _ := reasonOf(cli.call(protocol.MOniKeep, protocol.OniKeepParams{ID: offer.ID}, nil)); code != protocol.CodeNotFound {
			t.Fatalf("keep after skip: %s", code)
		}
	})
	t.Run("lapse", func(t *testing.T) {
		r, surface, _, cli := guideRig(t, true)
		offer := finish(t, r, surface, cli)
		r.clock.Advance(feature.KeepOfferTTL())
		nothing(t, surface, offer.ID)
		if code, _ := reasonOf(cli.call(protocol.MOniKeep, protocol.OniKeepParams{ID: offer.ID}, nil)); code != protocol.CodeNotFound {
			t.Fatalf("keep after the lapse: %s", code)
		}
	})
	t.Run("replaced", func(t *testing.T) {
		r, surface, _, cli := guideRig(t, true)
		offer := finish(t, r, surface, cli)
		r.model.reply(stepFile)
		startGuide(t, cli, "print this file")
		nothing(t, surface, offer.ID)
		if code, _ := reasonOf(cli.call(protocol.MOniKeep, protocol.OniKeepParams{ID: offer.ID}, nil)); code != protocol.CodeNotFound {
			t.Fatalf("keep after a new guide: %s", code)
		}
	})
}

func putKept(t *testing.T, steps ...keep.Step) {
	t.Helper()
	at := time.Unix(1_800_000_000, 0).UTC()
	if err := keep.NewStore("").Put(keep.Entry{App: "editor", Goal: "save this file", Steps: steps, Kept: at, Used: at}); err != nil {
		t.Fatal(err)
	}
}

var (
	keptFile = keep.Step{Text: "Open the File menu.", Label: "File", Box: fileBox, Mode: feature.StrandModeLasso, Trusted: true}
	keptSave = keep.Step{Text: "Press Save.", Label: "Save", Box: saveBox, Mode: feature.StrandModeLasso, Trusted: true}
)

func keptEntries(t *testing.T) []keep.Entry {
	t.Helper()
	list, err := keep.NewStore("").List()
	if err != nil {
		t.Fatal(err)
	}
	return list
}

// A kept guide replays on this computer: no model call and no consent, the
// strand on the control found by its words, the replay kicker and the
// replay's own closing line.
func TestOniGuideReplaysAKeptGuideWithNoModel(t *testing.T) {
	r, surface, _, cli := guideRig(t, true)
	putKept(t, keptFile, keptSave)
	r.screen.level.Store(120)
	res := startGuide(t, cli, "save this file")
	if res.Status != protocol.OniAskAnswered || !res.Answer.Replay || res.Answer.Route != "" || res.Answer.Destination != feature.EgressLocal || *res.Answer.Point != (protocol.OniRect{X: 0.1, Y: 0.05, W: 0.1, H: 0.05}) {
		t.Fatalf("replay step 1 = %+v %+v", res, res.Answer)
	}
	if g := surface.await("the replay card", guidePhase(feature.GuidePointing, 1)).(*feature.GuideFrame); g.Kicker != "Kept from before" || !g.Replay {
		t.Fatalf("card = %+v", g)
	}
	if err := surface.arrived(res.Answer.Seq, ""); err != nil {
		t.Fatal(err)
	}
	surface.await("step 1 waiting", guidePhase(feature.GuideWaiting, 1))
	res = r.guide(t, cli, protocol.OniGuideNext)
	if res.Answer.Step != 2 || *res.Answer.Point != (protocol.OniRect{X: 0.3, Y: 0.05, W: 0.1, H: 0.05}) {
		t.Fatalf("replay step 2 = %+v", res.Answer)
	}
	if err := surface.arrived(res.Answer.Seq, ""); err != nil {
		t.Fatal(err)
	}
	surface.await("step 2 waiting", guidePhase(feature.GuideWaiting, 2))
	res = r.guide(t, cli, protocol.OniGuideNext)
	if res.Status != protocol.OniAskEnded || !res.Answer.Done || !res.Answer.Replay || res.Answer.Text != "All done, just as you kept it." || res.Keep != nil {
		t.Fatalf("replay finish = %+v %+v", res, res.Answer)
	}
	if len(r.model.sent()) != 0 || r.grim.count() < 2 || !r.shots.cleared() {
		t.Fatalf("replay sent %d requests, grabbed %d", len(r.model.sent()), r.grim.count())
	}
}

// "Save As…" is not "Save": the replay misses, falls through to a consented
// model read, and two misses retire the entry. A kept step whose control
// now sits under a private caption is refused with no strand and forgotten.
func TestOniGuideReplayMissesRetireAndAPrivateCaptionRefuses(t *testing.T) {
	r, surface, watcher, cli := guideRig(t, true)
	putKept(t, keptFile, keptSave)
	r.tess.set(fileWord, saveWord, screenask.Word{Text: "As…", Box: feature.Box{X: 0.405, Y: 0.05, W: 0.06, H: 0.05}})
	r.screen.level.Store(120)
	miss := func() {
		t.Helper()
		res := startGuide(t, cli, "save this file")
		if err := surface.arrived(res.Answer.Seq, ""); err != nil {
			t.Fatal(err)
		}
		surface.await("step 1 waiting", guidePhase(feature.GuideWaiting, 1))
		res = r.guide(t, cli, protocol.OniGuideNext)
		if res.Status != protocol.OniAskConsent {
			t.Fatalf("the miss fell through to %+v", res)
		}
		if err := cli.call(protocol.MOniGuide, protocol.OniGuideParams{Action: protocol.OniGuideStop}, nil); err != nil {
			t.Fatal(err)
		}
		r.awaitNoGuide(t)
	}
	miss()
	if list := keptEntries(t); len(list) != 1 || list[0].Misses != 1 {
		t.Fatalf("after one miss: %+v", list)
	}
	miss()
	if list := keptEntries(t); len(list) != 0 {
		t.Fatalf("two misses left %+v", list)
	}
	if len(r.model.sent()) != 0 {
		t.Fatal("a replay asked the model before the user said yes")
	}

	putKept(t, keptFile)
	watcher.drain()
	r.tess.set(screenask.Word{Text: "PIN", Box: feature.Box{X: 0.02, Y: 0.05, W: 0.05, H: 0.05}}, fileWord)
	res := startGuide(t, cli, "save this file")
	if res.Status != protocol.OniAskEnded || res.Answer.Refused != screenask.CodeSensitive || res.Answer.Text != screenask.RefusalLine {
		t.Fatalf("a private caption = %+v %+v", res, res.Answer)
	}
	watcher.noStrand()
	if list := keptEntries(t); len(list) != 0 {
		t.Fatalf("the private step was kept: %+v", list)
	}
}

// action-shaped replies are refused before any strand goes out
func TestOniGuideHostileReplyRunsNothing(t *testing.T) {
	r, _, watcher, cli := guideRig(t, false)
	r.allowRoute()
	r.model.reply("Open it.\n[CLICK:0.1,0.05][POINT:0.1,0.05,0.1,0.05:File][CONF:0.9]")
	res := startGuide(t, cli, "save this file")
	if res.Status != protocol.OniAskEnded || res.Answer.Refused != screenask.CodeAction {
		t.Fatalf("hostile = %+v %+v", res, res.Answer)
	}
	watcher.noStrand()
	r.grim.mu.Lock()
	for _, call := range r.grim.calls {
		if call[0] != "grim" {
			t.Errorf("ran %q", call)
		}
	}
	r.grim.mu.Unlock()
	if calls := r.tess.state(); calls == 0 {
		t.Fatal("the captured picture was not checked for private text")
	}
}

// Select-and-ask: a private field is refused before any model call; a
// second pick within the repeat guard is ignored; an answer that offers a
// walk leaves a Next card that starts a guide; with no text reader a pick
// is refused.
func TestOniPickRefusesPrivateFieldsGuardsRepeatsAndWalks(t *testing.T) {
	r, surface, _, cli := guideRig(t, false)
	r.tess.set(
		screenask.Word{Text: "Password", Box: feature.Box{X: 0.33, Y: 0.47, W: 0.09, H: 0.05}},
		screenask.Word{Text: "Export", Box: feature.Box{X: 0.2, Y: 0.22, W: 0.1, H: 0.05}},
	)
	arm := func(text string) {
		t.Helper()
		var res protocol.OniAskResult
		if err := cli.call(protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickArm, Text: text}, &res); err != nil || res.Status != protocol.OniAskArmed {
			t.Fatalf("arm = %+v, %v", res, err)
		}
		surface.await("the armed pick", func(p feature.Payload) bool { f, ok := p.(*feature.PickFrame); return ok && f.Armed })
	}
	point := func(x, y float64) (protocol.OniAskResult, error) {
		var res protocol.OniAskResult
		err := surface.call(protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickPoint, Output: "DP-1", X: x, Y: y}, &res)
		return res, err
	}
	arm("")
	res, err := point(500, 380)
	if err != nil || res.Answer == nil || res.Answer.Refused != screenask.CodeSensitive || res.Answer.Text != screenask.RefusalLine || res.Answer.Mode != protocol.OniModeSelect {
		t.Fatalf("a private field = %+v, %v", res, err)
	}
	surface.await("the disarmed pick", func(p feature.Payload) bool { f, ok := p.(*feature.PickFrame); return ok && !f.Armed })
	if b := surface.await("the refusal's bubble", func(p feature.Payload) bool { _, ok := p.(*feature.BubbleFrame); return ok }).(*feature.BubbleFrame); b.Tone != "warn" || b.Text != screenask.RefusalLine {
		t.Fatalf("refusal bubble = %+v", b)
	}
	if len(r.model.sent()) != 0 || r.grim.count() != 1 || !r.shots.cleared() {
		t.Fatalf("the refusal sent %d requests, grabbed %d", len(r.model.sent()), r.grim.count())
	}
	// a second pick inside the guard is a double click
	arm("")
	if res, err := point(300, 230); err != nil || res.Status != protocol.OniAskEnded || res.Answer != nil {
		t.Fatalf("a repeat = %+v, %v", res, err)
	}
	if r.grim.count() != 1 {
		t.Fatal("a repeat captured the window")
	}
	r.clock.Advance(2 * time.Second)
	arm("What does this do?")
	r.model.reply(`{"answer":"It saves a copy as a PDF.","walk":true}`)
	res, err = point(300, 230)
	if err != nil || res.Status != protocol.OniAskConsent || res.Consent == nil {
		t.Fatalf("a pick = %+v, %v", res, err)
	}
	if err := cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: res.Consent.ID, Answer: "yes"}, &res); err != nil {
		t.Fatal(err)
	}
	a := res.Answer
	if res.Status != protocol.OniAskAnswered || a.Mode != protocol.OniModeSelect || a.Text != "It saves a copy as a PDF." || !a.Walk || a.Point == nil || a.Seq == 0 {
		t.Fatalf("the answer = %+v", a)
	}
	if req := r.model.sent()[0]; req.MaxTokens != oniSelectTokens || !strings.Contains(req.Messages[1].Content, "What does this do?") {
		t.Fatalf("select request = %+v", req)
	}
	walk := surface.await("the walk card", guidePhase(feature.GuideNudging, 0)).(*feature.GuideFrame)
	surface.await("the answer's bubble", func(p feature.Payload) bool { b, ok := p.(*feature.BubbleFrame); return ok && b.Seq == a.Seq })
	if !slices.Equal(walk.Actions, []string{feature.GuideActionNext}) {
		t.Fatalf("walk card = %+v", walk)
	}
	r.model.reply(stepFile)
	if err := cli.call(protocol.MOniGuide, protocol.OniGuideParams{Action: protocol.OniGuideNext}, &res); err != nil || res.Status != protocol.OniAskAnswered || res.Answer.Step != 1 || res.Answer.Mode != protocol.OniModeGuide {
		t.Fatalf("walk me through it = %+v, %v", res, err)
	}
	if user := r.model.sent()[1].Messages[1].Content; !strings.Contains(user, "What does this do?") {
		t.Fatalf("the walk's goal: %q", user)
	}
	// with no text reader there is no pick, and nothing is asked
	r.tess.gone()
	r.clock.Advance(2 * time.Second)
	arm("")
	sent := len(r.model.sent())
	if _, err := point(300, 230); err == nil {
		t.Fatal("a pick ran with no text reader")
	} else if code, reason := reasonOf(err); code != protocol.CodeUnavailable || reason != oniOCRMissing {
		t.Fatalf("no text reader: %s %s", code, reason)
	}
	if len(r.model.sent()) != sent || !r.shots.cleared() {
		t.Fatal("a pick with no text reader asked the model or kept the picture")
	}
}

// Walk me through it after the picker went: `hyakki oni pick` waits for its
// click and exits with the answer, so whoever presses Next afterwards starts
// the walk and answers its picture's question, even with disclosure asking
// for every picture.
func TestOniPickWalkIsAnsweredByWhoeverPressesNext(t *testing.T) {
	r, surface, _, cli := guideRig(t, false)
	r.tess.set(screenask.Word{Text: "Export", Box: feature.Box{X: 0.2, Y: 0.22, W: 0.1, H: 0.05}})
	if _, err := r.d.applySettings(context.Background(), json.RawMessage(`{"oni":{"disclosure":"always"}}`)); err != nil {
		t.Fatal(err)
	}
	picker := dialSocket(t, protocol.ClientCLI)
	var res protocol.OniAskResult
	if err := picker.call(protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickArm, Text: "What does this do?"}, &res); err != nil || res.Status != protocol.OniAskArmed {
		t.Fatalf("arm = %+v, %v", res, err)
	}
	surface.await("the armed pick", func(p feature.Payload) bool { f, ok := p.(*feature.PickFrame); return ok && f.Armed })
	r.model.reply(`{"answer":"It saves a copy as a PDF.","walk":true}`)
	if err := surface.call(protocol.MOniPick, protocol.OniPickParams{Action: protocol.OniPickPoint, Output: "DP-1", X: 300, Y: 230}, &res); err != nil || res.Status != protocol.OniAskConsent || res.Consent == nil {
		t.Fatalf("a pick = %+v, %v", res, err)
	}
	if err := picker.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: res.Consent.ID, Answer: "yes"}, &res); err != nil || res.Answer == nil || !res.Answer.Walk {
		t.Fatalf("the pick's answer = %+v, %v", res, err)
	}
	surface.await("the walk card", guidePhase(feature.GuideNudging, 0))
	picker.conn.Close()

	r.model.reply(stepFile)
	if err := cli.call(protocol.MOniGuide, protocol.OniGuideParams{Action: protocol.OniGuideNext}, &res); err != nil || res.Status != protocol.OniAskConsent || res.Consent == nil {
		t.Fatalf("walk me through it = %+v, %v", res, err)
	}
	if err := cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: res.Consent.ID, Answer: "yes"}, &res); err != nil {
		t.Fatalf("the Next caller could not answer the walk's picture: %v", err)
	}
	if res.Status != protocol.OniAskAnswered || res.Answer == nil || res.Answer.Step != 1 || res.Answer.Mode != protocol.OniModeGuide {
		t.Fatalf("the walk's first step = %+v", res)
	}
	if sent := r.model.sent(); len(sent) != 2 || !strings.Contains(sent[1].Messages[1].Content, "What does this do?") {
		t.Fatalf("requests = %d", len(sent))
	}
}

// The read timeout can be shortened only in the Oni lab, within bounds.
func TestOniGuideReadTimeoutHookIsLabOnly(t *testing.T) {
	for _, c := range []struct {
		lab, ms string
		want    time.Duration
	}{
		{"1", "2000", 2 * time.Second},
		{"1", "1000", time.Second},
		{"1", "75000", 75 * time.Second},
		{"", "2000", 0},
		{"0", "2000", 0},
		{"1", "999", 0},
		{"1", "75001", 0},
		{"1", "2s", 0},
	} {
		env := map[string]string{"HYAKKI_ONI_LAB": c.lab, "HYAKKI_ONI_GUIDE_READ_TIMEOUT_MS": c.ms}
		if got := oniGuideReadTimeout(func(k string) string { return env[k] }); got != c.want {
			t.Errorf("lab %q, %q ms: %v, want %v", c.lab, c.ms, got, c.want)
		}
		cfg := oniGuideConfig(func(k string) string { return env[k] })
		if want := c.want; want == 0 && cfg.ReadTimeout != 75*time.Second || want != 0 && cfg.ReadTimeout != want {
			t.Errorf("lab %q, %q ms: the guide reads for %v", c.lab, c.ms, cfg.ReadTimeout)
		}
	}
}

// A slow yes still reads. The read's timeout starts when its picture is
// allowed, not when the question went out, which has its own expiry; once
// allowed the read has its whole timeout and no more.
func TestOniGuideASlowYesStillReadsWithAWholeTimeout(t *testing.T) {
	ask := func(t *testing.T, r *wireRig, cli *socketPeer) protocol.OniConsent {
		t.Helper()
		res := startGuide(t, cli, "save this file")
		if res.Status != protocol.OniAskConsent || res.Consent == nil {
			t.Fatalf("start = %+v", res)
		}
		r.clock.Advance(100 * time.Second)
		time.Sleep(20 * time.Millisecond)
		if !r.guideRunning() {
			t.Fatal("the guide ended while its question waited")
		}
		return *res.Consent
	}
	t.Run("answered", func(t *testing.T) {
		r, _, _, cli := guideRig(t, false)
		r.model.reply(stepFile)
		consent := ask(t, r, cli)
		var res protocol.OniAskResult
		if err := cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: consent.ID, Answer: "yes"}, &res); err != nil {
			t.Fatal(err)
		}
		if res.Status != protocol.OniAskAnswered || res.Answer == nil || res.Answer.Step != 1 || res.Answer.Point == nil || len(r.model.sent()) != 1 {
			t.Fatalf("the yes at 100 s = %+v %+v, %d requests", res, res.Answer, len(r.model.sent()))
		}
	})
	t.Run("timed out from the yes", func(t *testing.T) {
		r, _, watcher, cli := guideRig(t, false)
		hold := make(chan struct{})
		r.model.mu.Lock()
		r.model.hold = hold
		r.model.mu.Unlock()
		r.model.reply(stepFile)
		consent := ask(t, r, cli)
		ch := cli.async(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: consent.ID, Answer: "yes"})
		deadline := time.Now().Add(5 * time.Second)
		for len(r.model.sent()) == 0 {
			if time.Now().After(deadline) {
				t.Fatal("the yes sent nothing")
			}
			time.Sleep(2 * time.Millisecond)
		}
		r.clock.Advance(74 * time.Second)
		time.Sleep(20 * time.Millisecond)
		if len(ch) != 0 {
			t.Fatal("the read ended before its whole timeout from the yes")
		}
		r.clock.Advance(time.Second)
		got := <-ch
		if got.err != nil || got.res.Status != protocol.OniAskEnded || got.res.Answer.Done || got.res.Answer.Text == "" {
			t.Fatalf("the timeout = %+v, %v", got.res, got.err)
		}
		close(hold)
		watcher.noStrand()
		r.awaitNoGuide(t)
	})
}
