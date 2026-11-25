package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/hands/chrome"
	"ryoku-rashin/internal/hands/page"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/hands/consent"
	"ryoku-rashin/internal/oni/hands/loop"
	"ryoku-rashin/internal/oni/hands/proto"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/oni/voice/turn"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

const (
	handsTestOrigin = "http://127.0.0.1:8080"
	handsTestPID    = 4242
	handsTyped      = "Ada Lovelace"
)

// handsPage is a fake tab: three controls, a picture, and a print that
// moves with every step and every press the person makes.
type handsPage struct {
	mu         sync.Mutex
	acts       []page.Act
	awaits     []string
	prints     int
	extra      []page.Control
	readErrors []error
	events     chan page.Event
	once       sync.Once
}

func newHandsPage() *handsPage { return &handsPage{events: make(chan page.Event, 16)} }

func (p *handsPage) Tabs(context.Context) ([]page.Tab, error) {
	return []page.Tab{{ID: "T1", Origin: handsTestOrigin, Title: "Contact\tform"}}, nil
}

func (p *handsPage) Authorize(_ context.Context, id string) (page.Scope, error) {
	if id != "T1" {
		return page.Scope{}, page.ErrUnknownTab
	}
	return page.Scope{TabID: id, Origin: handsTestOrigin}, nil
}

func (p *handsPage) Page(context.Context) (page.Snapshot, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.readErrors) != 0 {
		err := p.readErrors[0]
		p.readErrors = p.readErrors[1:]
		return page.Snapshot{}, err
	}
	snapshot := page.Snapshot{
		Origin: handsTestOrigin,
		Controls: []page.Control{
			{ID: "c1", Role: "textbox", Name: "Name", Class: consent.Safe, Reason: consent.ReasonSafe},
			{ID: "c2", Role: "textbox", Name: "Message", Class: consent.Safe, Reason: consent.ReasonSafe},
			{ID: "c3", Role: "button", Name: "Send", Class: consent.Handoff, Reason: consent.ReasonRiskyLabel},
		},
		Text: "Contact us", Picture: handsPNG(), Print: "print-" + string(rune('a'+p.prints)),
	}
	snapshot.Controls = append(snapshot.Controls, p.extra...)
	return snapshot, nil
}

func (p *handsPage) Act(_ context.Context, a page.Act) (page.Result, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.acts = append(p.acts, a)
	p.prints++
	return page.Result{Class: consent.Safe, Reason: consent.ReasonSafe, Done: true, ReadBack: true}, nil
}

func (p *handsPage) Shot(context.Context) ([]byte, error) { return handsPNG(), nil }
func (p *handsPage) Events() <-chan page.Event            { return p.events }

func (p *handsPage) Await(_ context.Context, control string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.awaits = append(p.awaits, control)
	return nil
}

func (p *handsPage) Close() error {
	p.once.Do(func() { close(p.events) })
	return nil
}

func (p *handsPage) acted() []page.Act {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.acts)
}

func (p *handsPage) awaited() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.awaits)
}

// press is the person pressing control on the page themselves.
func (p *handsPage) press(control string) {
	p.mu.Lock()
	p.prints++
	p.mu.Unlock()
	p.events <- page.Event{Kind: page.Pressed, Control: control}
}

func handsPNG() []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 6))); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// handsFakeBrowser is a Hands browser with no process behind it.
type handsFakeBrowser struct {
	pg     *handsPage
	done   chan struct{}
	once   sync.Once
	closes atomic.Int32
}

func (b *handsFakeBrowser) Driver() page.Driver   { return b.pg }
func (b *handsFakeBrowser) PID() int              { return handsTestPID }
func (b *handsFakeBrowser) Done() <-chan struct{} { return b.done }

func (b *handsFakeBrowser) Close() error {
	b.closes.Add(1)
	b.exit()
	return nil
}

// exit is the browser going away, as when the person closes its window.
func (b *handsFakeBrowser) exit() {
	b.once.Do(func() {
		close(b.done)
		_ = b.pg.Close()
	})
}

// handsModel answers each request with the next reply the test gives, and
// counts the requests whose stream was cut before a reply came.
type handsModel struct {
	core.Gateway
	mu       sync.Mutex
	requests []llm.Request
	replies  chan string
	aborted  atomic.Int32
}

func (m *handsModel) Plan(context.Context, llm.Request) (llm.Plan, error) { return localEyes, nil }

func (m *handsModel) Stream(ctx context.Context, req llm.Request) (llm.Stream, error) {
	m.mu.Lock()
	m.requests = append(m.requests, req)
	m.mu.Unlock()
	select {
	case r := <-m.replies:
		return &fakeReply{text: r}, nil
	case <-ctx.Done():
		m.aborted.Add(1)
		return nil, ctx.Err()
	}
}

func (m *handsModel) Close() error { return nil }

func (m *handsModel) sent() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.requests)
}

func (m *handsModel) last() llm.Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.requests[len(m.requests)-1]
}

func handsReply(tool string, args map[string]any) string {
	b, err := json.Marshal(map[string]any{"type": "tool_call", "tool": tool, "args": args, "confirm": false, "permission": "normal"})
	if err != nil {
		panic(err)
	}
	return string(b)
}

// handsRig is a serving daemon whose Hands browser, stop key, windows,
// model and clock are all fakes.
type handsRig struct {
	d        *Daemon
	desk     *fakeDesktop
	model    *handsModel
	clock    *askClock
	app      *atomic.Bool
	browser  *handsFakeBrowser
	launches []chrome.Options
	finds    atomic.Int32
	mu       sync.Mutex
	onStop   func()
	keyStops atomic.Int32
}

func handsDesktop(focus window.ID) window.Snapshot {
	s := window.Snapshot{Outputs: []window.Output{dp1}, Windows: []window.Window{
		{ID: "0x1", App: "editor", Title: "notes.txt", Output: "DP-1", Visible: true},
		{ID: "0x7", App: "hyakki", Title: "Hyakki", Output: "DP-1", Visible: true},
		{ID: "0x9", App: "hyakki-hands-x", Title: "Contact", Output: "DP-1", Visible: true, PID: handsTestPID},
	}}
	for i := range s.Windows {
		s.Windows[i].Focused = s.Windows[i].ID == focus
	}
	return s
}

func serveHands(t *testing.T, hands string) *handsRig {
	t.Helper()
	isolateDaemon(t)
	shortRuntime(t)
	t.Setenv("HYAKKI_SOCKET", "")
	settings, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	settings.Enabled = true
	settings.Oni.Master = true
	settings.Oni.Inspect = true
	settings.Oni.Hands = hands
	if err := config.Save(settings); err != nil {
		t.Fatal(err)
	}
	r := &handsRig{desk: newDesktop(handsDesktop("0x7")), model: &handsModel{replies: make(chan string, 16)}, clock: &askClock{now: time.Unix(1_800_000_000, 0)}, app: &atomic.Bool{}}
	deps := Deps{
		Version:    "oni-hands-test",
		Gateway:    r.model,
		OniWindows: func() (window.Provider, error) { return r.desk, nil },
		OniHands: OniHandsDeps{
			Find: func() (OniHandsProgram, error) {
				r.finds.Add(1)
				return OniHandsProgram{Name: "chromium", Exec: []string{"/usr/bin/chromium"}}, nil
			},
			Launch: func(_ context.Context, o chrome.Options) (OniHandsBrowser, error) {
				r.mu.Lock()
				defer r.mu.Unlock()
				r.launches = append(r.launches, o)
				r.browser = &handsFakeBrowser{pg: newHandsPage(), done: make(chan struct{})}
				return r.browser, nil
			},
			Stop: func(onStop func()) (func(), error) {
				r.mu.Lock()
				r.onStop = onStop
				r.mu.Unlock()
				return func() { r.keyStops.Add(1) }, nil
			},
			Clock: r.clock,
		},
	}
	d := New(refuseRealDesktop(t, deps))
	d.appPeer = func(int32) bool { return r.app.Load() }
	d.oni.ask.clock = r.clock
	r.d = d
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	t.Cleanup(func() {
		stop()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("daemon did not stop")
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for !client.Reachable(context.Background(), paths.Socket()) {
		if time.Now().After(deadline) {
			t.Fatal("the daemon never listened")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return r
}

func (r *handsRig) dialApp(t *testing.T) *socketPeer {
	t.Helper()
	r.app.Store(true)
	defer r.app.Store(false)
	return dialSocket(t, protocol.ClientApp)
}

func (r *handsRig) page() *handsPage {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.browser.pg
}

func (r *handsRig) esc() {
	r.mu.Lock()
	f := r.onStop
	r.mu.Unlock()
	f()
}

func handsCall(t *testing.T, p *socketPeer, action string, extra ...string) protocol.OniHandsResult {
	t.Helper()
	params := protocol.OniHandsParams{Action: action}
	switch action {
	case protocol.OniHandsAuthorize:
		params.Tab = extra[0]
	case protocol.OniHandsStart:
		params.Goal = extra[0]
	}
	var res protocol.OniHandsResult
	if err := p.call(protocol.MOniHands, params, &res); err != nil {
		t.Fatalf("oni.hands %s: %v", action, err)
	}
	return res
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("never: %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func handsFrame(match func(*feature.HandsFrame) bool) func(feature.Payload) bool {
	return func(p feature.Payload) bool {
		f, ok := p.(*feature.HandsFrame)
		return ok && match(f)
	}
}

func handsConsentFor(id string) func(feature.Payload) bool {
	return func(p feature.Payload) bool {
		c, ok := p.(*feature.ConsentFrame)
		return ok && c.Feature == id
	}
}

// voiceRouter is the router the daemon builds, wired to its Hands slot,
// consent and card check.
func (r *handsRig) voiceRouter(t *testing.T) (*route.Router, *turn.Turn) {
	t.Helper()
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	tn := turn.New(sp, nil)
	router, err := route.New(route.Config{Turn: tn, Name: "Oni", Hands: r.d.voiceHands, Consent: voiceConsent{r.d}, Guide: voiceGuide{r.d},
		HandsStop: func() { r.d.handsStop(handsByVoice) }, HandsCard: r.d.oni.hands.card.Load})
	if err != nil {
		t.Fatal(err)
	}
	return router, tn
}

// readyTask opens the Hands browser, authorizes its tab and starts goal,
// answering the first step's picture question from the app. It returns
// the presenter.
func (r *handsRig) readyTask(t *testing.T, app *socketPeer, goal string) *socketPeer {
	t.Helper()
	surface := dialSocket(t, protocol.ClientSurface)
	if !surface.open(protocol.OniRendererNative).Presenter {
		t.Fatal("the surface is no presenter")
	}
	if res := handsCall(t, app, protocol.OniHandsOpen); res.State != protocol.OniHandsStateOpen || res.Browser != "chromium" {
		t.Fatalf("open = %+v", res)
	}
	tabs := handsCall(t, app, protocol.OniHandsTabs)
	if len(tabs.Tabs) != 1 || tabs.Tabs[0] != (protocol.OniHandsTab{ID: "T1", Origin: handsTestOrigin, Title: "Contact form"}) {
		t.Fatalf("tabs = %+v", tabs.Tabs)
	}
	if res := handsCall(t, app, protocol.OniHandsAuthorize, "T1"); res.State != protocol.OniHandsStateAuthorized || res.Origin != handsTestOrigin {
		t.Fatalf("authorize = %+v", res)
	}
	if res := handsCall(t, app, protocol.OniHandsStart, goal); res.State != protocol.OniHandsStateWorking {
		t.Fatalf("start = %+v", res)
	}
	q := surface.await("the step's picture question", handsConsentFor(feature.Inspect)).(*feature.ConsentFrame)
	if !strings.HasPrefix(q.ID, feature.HandsStepCardPrefix) || q.Class != feature.ConsentDisclosure || !strings.Contains(q.Question, "text and the names of its controls") {
		t.Fatalf("the step's question = %+v", q)
	}
	cli := dialSocket(t, protocol.ClientCLI)
	if code, _ := reasonOf(cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: q.ID, Answer: "yes"}, nil)); code != protocol.CodeForbidden {
		t.Fatalf("the CLI answered the step's question: %s", code)
	}
	if err := app.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: q.ID, Answer: "yes"}, nil); err != nil {
		t.Fatalf("the app's yes: %v", err)
	}
	return surface
}

func TestHandsInspectionApprovalFocusIsBoundToTheCallingApp(t *testing.T) {
	for _, test := range []struct {
		name      string
		pid       int
		own       bool
		presenter bool
		closeApp  bool
	}{
		{"calling app", os.Getpid(), true, false, false},
		{"presenter started, verified app approves", os.Getpid(), true, true, false},
		{"presenter started, app disconnected", os.Getpid(), false, true, true},
		{"same app label, another process", os.Getpid() + 1, false, false, false},
		{"same app label, no process identity", 0, false, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := serveHands(t, protocol.OniHandsBrowser)
			app := r.dialApp(t)
			desktop := func(focus window.ID) window.Snapshot {
				snapshot := handsDesktop(focus)
				for i := range snapshot.Windows {
					if snapshot.Windows[i].ID == "0x7" {
						snapshot.Windows[i].PID = test.pid
					}
				}
				return snapshot
			}
			r.desk.set(desktop("0x7"))
			surface := dialSocket(t, protocol.ClientSurface)
			if !surface.open(protocol.OniRendererNative).Presenter {
				t.Fatal("the surface is no presenter")
			}
			handsCall(t, app, protocol.OniHandsOpen)
			handsCall(t, app, protocol.OniHandsAuthorize, "T1")
			starter := app
			if test.presenter {
				starter = surface
			}
			handsCall(t, starter, protocol.OniHandsStart, "help with the contact form")
			card := surface.await("the inspection approval", handsConsentFor(feature.Inspect)).(*feature.ConsentFrame)
			if test.closeApp {
				app.conn.Close()
				eventually(t, "the app's authenticated connection closes", func() bool {
					r.d.mu.RLock()
					defer r.d.mu.RUnlock()
					for peer := range r.d.clients {
						if peer.app {
							return false
						}
					}
					return true
				})
			}
			r.desk.set(desktop("0x9"))
			eventually(t, "browser focus observed", func() bool {
				return handsWait(r.d, func() window.ID { return r.d.oni.hands.run.focus }) == "0x9"
			})
			r.desk.set(desktop("0x7"))
			eventually(t, "approval window focus observed", func() bool {
				return handsWait(r.d, func() window.ID { return r.d.oni.hands.run.focus }) == "0x7"
			})
			if r.model.sent() != 0 {
				t.Fatal("the picture left before approval")
			}
			if !test.own {
				paused := surface.await("foreign window pauses the task", handsFrame(func(frame *feature.HandsFrame) bool {
					return frame.Active && frame.Phase == protocol.OniHandsStatePaused
				})).(*feature.HandsFrame)
				if paused.Pause != protocol.OniHandsPauseFocus {
					t.Fatalf("pause = %+v", paused)
				}
				if err := app.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: "yes"}, nil); err == nil {
					t.Fatal("foreign focus left the picture approval usable")
				}
				return
			}
			if err := app.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: "yes"}, nil); err != nil {
				t.Fatalf("returning to the verified app destroyed its approval: %v", err)
			}
			eventually(t, "the approved picture reaches the model", func() bool { return r.model.sent() == 1 })
			r.desk.set(desktop("0x9"))
			eventually(t, "browser focus after approval", func() bool {
				return handsWait(r.d, func() window.ID { return r.d.oni.hands.run.focus }) == "0x9"
			})
			r.desk.set(desktop("0x7"))
			paused := surface.await("the app is no focus exception after approval", handsFrame(func(frame *feature.HandsFrame) bool {
				return frame.Active && frame.Phase == protocol.OniHandsStatePaused
			})).(*feature.HandsFrame)
			if paused.Pause != protocol.OniHandsPauseFocus {
				t.Fatalf("pause after approval = %+v", paused)
			}
			eventually(t, "the unapproved focus aborts the model", func() bool { return r.model.aborted.Load() == 1 })
		})
	}
}

// A task fills a field itself, hands Send to the person, and no spoken
// yes from any source, nor any answer at all, makes Hands press it: only
// the person's own press on the page goes on.
func TestHandsHandsOffSendAndNoSpokenYesActs(t *testing.T) {
	r := serveHands(t, protocol.OniHandsBrowser)
	app := r.dialApp(t)
	r.model.replies <- handsReply("fill", map[string]any{"id": "c1", "text": handsTyped})
	r.model.replies <- handsReply("click", map[string]any{"id": "c3"})
	surface := r.readyTask(t, app, "fill the contact form")
	if !r.d.handsResident() || r.d.Idle() {
		t.Fatal("an open Hands browser does not keep the daemon up")
	}
	o := r.launches[0]
	if o.ProfileDir != filepath.Join(paths.StateDir(), chrome.ProfileName) || !strings.HasPrefix(o.Class, "hyakki-hands-") || o.StartURL != "about:blank" || o.Headless || !slices.Equal(o.Exec, []string{"/usr/bin/chromium"}) {
		t.Fatalf("launch options = %+v", o)
	}
	card := surface.await("the handoff card", handsConsentFor(feature.HandsBrowser)).(*feature.ConsentFrame)
	if !strings.HasPrefix(card.ID, feature.HandsCardPrefix) || strings.HasPrefix(card.ID, feature.HandsStepCardPrefix) || card.Destination != feature.EgressLocal || card.Route != "" || !strings.Contains(card.Question, "Send") {
		t.Fatalf("the handoff card = %+v", card)
	}
	pg := r.page()
	if acts := pg.acted(); len(acts) != 1 || acts[0].Verb != "fill" || acts[0].Control != "c1" || acts[0].Text != handsTyped {
		t.Fatalf("acts = %+v", acts)
	}
	eventually(t, "the page watching Send", func() bool { return slices.Equal(pg.awaited(), []string{"c3"}) })
	if user := r.model.last().Messages[1].Content; !strings.Contains(user, "BEGIN PAGE TEXT") || !strings.Contains(user, "Send") {
		t.Fatalf("the request carried no fenced page: %q", user)
	}

	// a spoken yes, from every source, answers nothing and acts on nothing
	router, tn := r.voiceRouter(t)
	id := tn.Barge()
	for _, s := range []route.Source{route.Chord, route.Toggle, route.Wake, route.Open} {
		for _, text := range []string{"yes", "go ahead", "approve it"} {
			if d := router.Route(route.Utterance{Text: text, Source: s, Turn: id, At: time.Now()}); d.Kind != route.NeedsPress || d.Line != route.FirmNeedsButtonLine {
				t.Fatalf("%s %q = %+v", s, text, d)
			}
		}
	}
	// nor does any answer on the wire
	for _, p := range []*socketPeer{app, dialSocket(t, protocol.ClientCLI)} {
		if code, _ := reasonOf(p.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: "yes"}, nil)); code != protocol.CodeBadRequest {
			t.Fatalf("a yes to the handoff card: %s", code)
		}
	}
	if res := handsCall(t, app, protocol.OniHandsStatus); res.State != protocol.OniHandsStateWaiting {
		t.Fatalf("status = %+v", res)
	}
	if acts := pg.acted(); len(acts) != 1 {
		t.Fatalf("a spoken or answered yes acted: %+v", acts)
	}

	// the person presses Send themselves; the page agrees it is done
	r.model.replies <- handsReply("done", map[string]any{"text": "Sent."})
	pg.mu.Lock()
	pg.readErrors = []error{page.ErrStale}
	pg.mu.Unlock()
	pg.press("c3")
	end := surface.await("the task's end", handsFrame(func(f *feature.HandsFrame) bool { return !f.Active })).(*feature.HandsFrame)
	if end.Stage != protocol.OniHandsBrowser || end.Reason != feature.ReasonAllowed || end.End != "done" {
		t.Fatalf("end frame = %+v", end)
	}
	if acts := pg.acted(); len(acts) != 1 {
		t.Fatalf("Hands pressed something: %+v", acts)
	}
	var log protocol.OniHandsLogResult
	if err := app.call(protocol.MOniHandsLog, protocol.Empty{}, &log); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(log)
	if strings.Contains(string(raw), "Ada") || strings.Contains(string(raw), "Send") || strings.Contains(string(raw), "Name") {
		t.Fatalf("the ledger holds words: %s", raw)
	}
	var seen []string
	for _, row := range log.Rows {
		if !protocol.ValidOniHandsRow(row) {
			t.Fatalf("invalid row %+v", row)
		}
		seen = append(seen, row.Tool+"/"+row.Outcome+"/"+row.Reason)
	}
	slices.Reverse(seen)
	want := []string{"fill/done/", "click/handoff/risky-label", "user/done/", "end/done/done"}
	if !slices.Equal(seen, want) {
		t.Fatalf("ledger = %v, want %v", seen, want)
	}
}

func TestHandsSnapshotRefreshDoesNotRetryLostScope(t *testing.T) {
	r := serveHands(t, protocol.OniHandsBrowser)
	app := r.dialApp(t)
	r.model.replies <- handsReply("click", map[string]any{"id": "c3"})
	surface := r.readyTask(t, app, "help with the contact form")
	surface.await("the handoff card", handsConsentFor(feature.HandsBrowser))
	pg := r.page()
	eventually(t, "the page watching Send", func() bool { return slices.Equal(pg.awaited(), []string{"c3"}) })
	pg.mu.Lock()
	pg.readErrors = []error{page.ErrStale, page.ErrOrigin}
	pg.mu.Unlock()
	r.model.replies <- handsReply("done", map[string]any{"text": "Sent."})
	pg.press("c3")
	end := surface.await("the lost scope ends work", handsFrame(func(f *feature.HandsFrame) bool {
		return !f.Active
	})).(*feature.HandsFrame)
	if end.End != "failed" || r.model.sent() != 1 || len(pg.acted()) != 0 {
		t.Fatalf("lost scope was retried or acted on: end=%+v requests=%d acts=%+v",
			end, r.model.sent(), pg.acted())
	}
}

func TestHandsPrivateRefusalIsAuditedWithoutExposingTheRequest(t *testing.T) {
	r := serveHands(t, protocol.OniHandsBrowser)
	app := r.dialApp(t)
	handsCall(t, app, protocol.OniHandsOpen)
	pg := r.page()
	pg.mu.Lock()
	pg.extra = []page.Control{{ID: "pw", Role: "textbox", Name: "Account password", Class: consent.Refused, Reason: consent.ReasonPrivate}}
	pg.mu.Unlock()
	const goal = "help with the contact form"
	const secret = "never-record-this-private-value"
	r.model.replies <- handsReply("fill", map[string]any{"id": "pw", "text": secret})
	r.model.replies <- handsReply("ask_user", map[string]any{"text": "Use the browser for that field."})
	surface := r.readyTask(t, app, goal)
	surface.await("the refused task's end", handsFrame(func(f *feature.HandsFrame) bool { return !f.Active }))
	if len(pg.acted()) != 0 || len(pg.awaited()) != 0 {
		t.Fatal("a private refusal reached the executor or became a handoff")
	}
	var log protocol.OniHandsLogResult
	if err := app.call(protocol.MOniHandsLog, protocol.Empty{}, &log); err != nil {
		t.Fatal(err)
	}
	refusals := 0
	for _, row := range log.Rows {
		if row.Outcome == protocol.OniHandsOutcomeRefused {
			refusals++
			if row.Tool != "fill" || row.Reason != string(consent.ReasonPrivate) || row.Step != 1 || !protocol.ValidOniHandsRow(row) {
				t.Fatalf("refusal row %+v", row)
			}
		}
	}
	if refusals != 1 {
		t.Fatalf("private action has %d refusal rows: %+v", refusals, log.Rows)
	}
	raw, err := json.Marshal(log)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{goal, secret, "Account password", handsTestOrigin} {
		if strings.Contains(string(raw), private) {
			t.Fatalf("audit retained private content %q: %s", private, raw)
		}
	}
}

func TestHandsExplicitTaskStartsFreshAndSpokenFollowupKeepsHistory(t *testing.T) {
	for _, mode := range []string{"new-button-goal", "repeat-button-goal", "anchored-voice-goal", "spoken-followup"} {
		t.Run(mode, func(t *testing.T) {
			r := serveHands(t, protocol.OniHandsBrowser)
			app := r.dialApp(t)
			r.model.replies <- handsReply("fill", map[string]any{"id": "c1", "text": handsTyped})
			r.model.replies <- handsReply("ask_user", map[string]any{"text": "Which message?"})
			surface := r.readyTask(t, app, "fill the contact form")
			surface.await("the first task's end", handsFrame(func(f *feature.HandsFrame) bool { return !f.Active }))
			if !strings.Contains(r.model.last().Messages[1].Content, "- fill c1: done") {
				t.Fatal("the completed task had no step history")
			}
			switch mode {
			case "new-button-goal", "repeat-button-goal":
				goal := "fill the Message field"
				if mode == "repeat-button-goal" {
					goal = "fill the contact form"
				}
				handsCall(t, app, protocol.OniHandsStart, goal)
			default:
				router, tn := r.voiceRouter(t)
				text := "and fill the Message field"
				if mode == "anchored-voice-goal" {
					text = "fill the Message field on this page"
				}
				router.Route(route.Utterance{Text: text, Source: route.Chord, Turn: tn.Barge(), At: time.Now()})
			}
			eventually(t, "the next task's first request", func() bool { return r.model.sent() == 3 })
			carried := strings.Contains(r.model.last().Messages[1].Content, "- fill c1: done")
			if carried != (mode == "spoken-followup") {
				t.Fatalf("%s carried prior-task history: %v", mode, carried)
			}
		})
	}
}

func TestHandsPageTruncationReachesTheBoundedPrompt(t *testing.T) {
	d := &Daemon{}
	run := &handsRun{origin: handsTestOrigin, snap: page.Snapshot{Truncated: true}}
	for i := range 200 {
		run.snap.Controls = append(run.snap.Controls, page.Control{ID: fmt.Sprintf("c%d", i), Role: "button", Name: "Item"})
	}
	in := d.handsInput(run, loop.Ask{Goal: "read the page"})
	_, user := proto.Prompt(in)
	if !in.Truncated || strings.Count(user, "(more controls not listed)") != 1 || strings.Count(user, "] button ") != 200 {
		t.Fatalf("bounded snapshot lost truncation or controls: %+v\n%s", in.Bounds, user)
	}
}

func TestHandsInspectOptOutStopsWorkWithoutRetiringTheOwnedBrowser(t *testing.T) {
	r := serveHands(t, protocol.OniHandsBrowser)
	app := r.dialApp(t)
	r.model.replies <- handsReply("fill", map[string]any{"id": "c1", "text": handsTyped})
	surface := r.readyTask(t, app, "fill the contact form")
	eventually(t, "a blocked model request", func() bool { return r.model.sent() == 2 })
	b := r.browser
	if err := app.call(protocol.MSettingsSet, protocol.SettingsSetParams{Patch: json.RawMessage(`{"oni":{"inspect":false}}`)}, nil); err != nil {
		t.Fatal(err)
	}
	surface.await("Inspect stopped the task", handsFrame(func(f *feature.HandsFrame) bool { return !f.Active && f.End == "stopped" }))
	eventually(t, "the model aborted", func() bool { return r.model.aborted.Load() == 1 })
	if b.closes.Load() != 0 || !r.d.handsResident() {
		t.Fatal("Inspect off retired the owned browser")
	}
	if err := app.call(protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsStart, Goal: "again"}, nil); err == nil {
		t.Fatal("Inspect off allowed a new task")
	}
	if err := app.call(protocol.MSettingsSet, protocol.SettingsSetParams{Patch: json.RawMessage(`{"oni":{"inspect":true}}`)}, nil); err != nil {
		t.Fatal(err)
	}
	if r.d.oni.hands.active.Load() || r.model.sent() != 2 || len(r.page().acted()) != 1 {
		t.Fatal("re-enabling Inspect resumed work")
	}
}

func TestHandsBrowserRetiresWhenItsStageOrMasterIsTurnedOff(t *testing.T) {
	for _, change := range []string{"hands-off", "app-stage", "desktop-stage", "oni-off", "hyakki-off"} {
		t.Run(change, func(t *testing.T) {
			r := serveHands(t, protocol.OniHandsBrowser)
			app := r.dialApp(t)
			handsCall(t, app, protocol.OniHandsOpen)
			b := r.browser
			before := r.d.Settings()
			after := before
			switch change {
			case "hands-off":
				after.Oni.Hands = protocol.OniHandsOff
			case "app-stage":
				after.Oni.Hands = protocol.OniHandsApp
			case "desktop-stage":
				after.Oni.Hands = protocol.OniHandsDesktop
			case "oni-off":
				after.Oni.Master = false
			case "hyakki-off":
				after.Enabled = false
			}
			r.d.handsSettings(before, after)
			eventually(t, "the owned browser retired", func() bool { return b.closes.Load() == 1 && !r.d.handsResident() })
		})
	}
}

// Esc stops the task: the model request in flight is cut within the
// stop budget, every indication goes in one frame, the key reader and the
// window watch end. Before that, a focus change pauses it with no step
// after, and no Resume but the button's goes on.
func TestHandsAwayPausesAndEscStopsAbortingTheModel(t *testing.T) {
	r := serveHands(t, protocol.OniHandsBrowser)
	app := r.dialApp(t)
	r.model.replies <- handsReply("fill", map[string]any{"id": "c1", "text": handsTyped})
	surface := r.readyTask(t, app, "fill the contact form")
	pg := r.page()
	eventually(t, "the fill and the next request", func() bool { return len(pg.acted()) == 1 && r.model.sent() == 2 })
	eventually(t, "the window watch", func() bool { w, _ := r.desk.state(); return w == 1 })

	// the person focuses the browser: no pause; then another window: paused
	r.desk.set(handsDesktop("0x9"))
	r.desk.set(handsDesktop("0x1"))
	paused := surface.await("the pause", handsFrame(func(f *feature.HandsFrame) bool { return f.Active && f.Phase == protocol.OniHandsStatePaused })).(*feature.HandsFrame)
	if paused.Pause != protocol.OniHandsPauseFocus {
		t.Fatalf("paused frame = %+v", paused)
	}
	eventually(t, "the request cut by the pause", func() bool { return r.model.aborted.Load() == 1 })

	// no resume but the button's: the CLI is refused, a spoken "keep
	// going" is heard and resumes nothing
	cli := dialSocket(t, protocol.ClientCLI)
	if code, _ := reasonOf(cli.call(protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsResume}, nil)); code != protocol.CodeForbidden {
		t.Fatalf("the CLI resumed: %s", code)
	}
	router, tn := r.voiceRouter(t)
	id := tn.Barge()
	for _, s := range []route.Source{route.Chord, route.Toggle, route.Wake, route.Open} {
		router.Route(route.Utterance{Text: "keep going", Source: s, Turn: id, At: time.Now()})
	}
	time.Sleep(50 * time.Millisecond)
	if res := handsCall(t, app, protocol.OniHandsStatus); res.State != protocol.OniHandsStatePaused || len(pg.acted()) != 1 || r.model.sent() != 2 {
		t.Fatalf("after the refused resumes: %+v, %d acts, %d requests", res, len(pg.acted()), r.model.sent())
	}

	// the button resumes; the next request is cut by Esc
	if res := handsCall(t, app, protocol.OniHandsResume); res.State != protocol.OniHandsStateWorking {
		t.Fatalf("resume = %+v", res)
	}
	eventually(t, "the next request", func() bool { return r.model.sent() == 3 })
	r.esc()
	end := surface.await("the stop", handsFrame(func(f *feature.HandsFrame) bool { return !f.Active })).(*feature.HandsFrame)
	if end.Phase != "" || end.Pause != "" || end.Step != 0 || end.End != "stopped" {
		t.Fatalf("end frame = %+v", end)
	}
	eventually(t, "the request aborted", func() bool { return r.model.aborted.Load() == 2 })
	eventually(t, "the key reader and window watch gone", func() bool {
		w, g := r.desk.state()
		return r.keyStops.Load() == 1 && w == 0 && g == ""
	})
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	var log protocol.OniHandsLogResult
	if err := app.call(protocol.MOniHandsLog, protocol.Empty{}, &log); err != nil {
		t.Fatal(err)
	}
	stopRow := false
	for _, row := range log.Rows {
		if row.Tool == handsToolStop {
			stopRow = row.Reason == handsByEsc && row.Ms <= int64(sp.Hands.StopBudgetMs)
		}
	}
	if !stopRow {
		t.Fatalf("no Esc stop row within the budget: %+v", log.Rows)
	}
	if acts := pg.acted(); len(acts) != 1 {
		t.Fatalf("a step ran after the pause: %+v", acts)
	}
	if n := r.clock.Armed(); n != 0 {
		t.Fatalf("%d timers armed after the stop", n)
	}
	if res := handsCall(t, app, protocol.OniHandsStatus); res.State != protocol.OniHandsStateAuthorized {
		t.Fatalf("after the stop = %+v", res)
	}
}

// A model reply asking to click Send is a handoff card and no step; the
// presenter's pill stops it, and a lost presenter stops the next one.
func TestHandsClickOnSendIsAHandoffAndThePresenterStops(t *testing.T) {
	r := serveHands(t, protocol.OniHandsBrowser)
	app := r.dialApp(t)
	r.model.replies <- handsReply("click", map[string]any{"id": "c3"})
	surface := r.readyTask(t, app, "send it")
	surface.await("the handoff card", handsConsentFor(feature.HandsBrowser))
	if acts := r.page().acted(); len(acts) != 0 {
		t.Fatalf("Hands clicked Send: %+v", acts)
	}
	var res protocol.OniHandsResult
	if err := surface.call(protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsStop}, &res); err != nil {
		t.Fatalf("the pill's stop: %v", err)
	}
	surface.await("the stop", handsFrame(func(f *feature.HandsFrame) bool { return !f.Active }))

	r.model.replies <- handsReply("click", map[string]any{"id": "c3"})
	if res := handsCall(t, app, protocol.OniHandsStart, "send it"); res.State != protocol.OniHandsStateWorking {
		t.Fatalf("start = %+v", res)
	}
	surface.await("the second card", handsConsentFor(feature.HandsBrowser))
	surface.conn.Close()
	eventually(t, "the task stopped by the lost presenter", func() bool { return !r.d.oni.hands.active.Load() })
	if acts := r.page().acted(); len(acts) != 0 {
		t.Fatalf("Hands clicked: %+v", acts)
	}
	if code, _ := reasonOf(app.call(protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsStart, Goal: "again"}, nil)); code != protocol.CodeConflict {
		t.Fatalf("a start with no presenter: %s", code)
	}
}

// The Hands browser keeps the daemon up while it is open; closing it lets
// go, and only then may the app forget its profile, which deletes the
// profile directory and nothing else. Turning Hands off closes it.
func TestHandsResidencyForgetAndSettings(t *testing.T) {
	r := serveHands(t, protocol.OniHandsBrowser)
	app := r.dialApp(t)
	handsCall(t, app, protocol.OniHandsOpen)
	if !r.d.handsResident() {
		t.Fatal("no residency with the Hands browser open")
	}
	profile := filepath.Join(paths.StateDir(), chrome.ProfileName)
	sibling := filepath.Join(paths.StateDir(), "keep-me")
	for _, f := range []string{filepath.Join(profile, "Default", "Cookies"), sibling} {
		if err := os.MkdirAll(filepath.Dir(f), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if code, _ := reasonOf(app.call(protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsForget}, nil)); code != protocol.CodeConflict {
		t.Fatalf("forget while open: %s", code)
	}
	cli := dialSocket(t, protocol.ClientCLI)
	if code, _ := reasonOf(cli.call(protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsForget}, nil)); code != protocol.CodeForbidden {
		t.Fatalf("the CLI forgot: %s", code)
	}
	r.browser.exit()
	eventually(t, "residency released", func() bool { return !r.d.handsResident() })
	if res := handsCall(t, app, protocol.OniHandsStatus); res.State != protocol.OniHandsStateClosed || res.Browser != "chromium" {
		t.Fatalf("after the browser closed = %+v", res)
	}
	if res := handsCall(t, app, protocol.OniHandsForget); res.State != protocol.OniHandsStateClosed {
		t.Fatalf("forget = %+v", res)
	}
	if _, err := os.Stat(profile); !os.IsNotExist(err) {
		t.Fatalf("the profile is still there: %v", err)
	}
	if _, err := os.Stat(sibling); err != nil {
		t.Fatalf("forget took more than the profile: %v", err)
	}

	handsCall(t, app, protocol.OniHandsOpen)
	b := r.browser
	if err := app.call(protocol.MSettingsSet, protocol.SettingsSetParams{Patch: json.RawMessage(`{"oni":{"hands":"off"}}`)}, nil); err != nil {
		t.Fatalf("hands off: %v", err)
	}
	eventually(t, "the browser closed by the setting", func() bool { return b.closes.Load() == 1 && !r.d.handsResident() })
}

// The inactive frame that ends a task names how it ended: away when the
// pause window lapses, stopped when turning Hands off stops it (the frame
// then reports Hands off). done and stopped by Esc are asserted above.
func TestHandsEndFrameNamesHowTheTaskEnded(t *testing.T) {
	r := serveHands(t, protocol.OniHandsBrowser)
	app := r.dialApp(t)
	r.model.replies <- handsReply("fill", map[string]any{"id": "c1", "text": handsTyped})
	surface := r.readyTask(t, app, "fill the contact form")
	eventually(t, "the fill and the next request", func() bool { return len(r.page().acted()) == 1 && r.model.sent() == 2 })
	eventually(t, "the window watch", func() bool { w, _ := r.desk.state(); return w == 1 })
	r.desk.set(handsDesktop("0x1"))
	surface.await("the pause", handsFrame(func(f *feature.HandsFrame) bool { return f.Active && f.Phase == protocol.OniHandsStatePaused }))
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	r.clock.Advance(sp.Hands.Resume() + time.Second)
	away := surface.await("the away end", handsFrame(func(f *feature.HandsFrame) bool { return !f.Active })).(*feature.HandsFrame)
	if away.End != "away" || away.Stage != protocol.OniHandsBrowser || away.Reason != feature.ReasonAllowed {
		t.Fatalf("away end frame = %+v", away)
	}

	r.model.replies <- handsReply("click", map[string]any{"id": "c3"})
	if res := handsCall(t, app, protocol.OniHandsStart, "send it"); res.State != protocol.OniHandsStateWorking {
		t.Fatalf("start = %+v", res)
	}
	surface.await("the handoff card", handsConsentFor(feature.HandsBrowser))
	if err := app.call(protocol.MSettingsSet, protocol.SettingsSetParams{Patch: json.RawMessage(`{"oni":{"hands":"off"}}`)}, nil); err != nil {
		t.Fatalf("hands off: %v", err)
	}
	off := surface.await("the end by the setting", handsFrame(func(f *feature.HandsFrame) bool { return !f.Active })).(*feature.HandsFrame)
	if off.End != "stopped" || off.Stage != protocol.OniHandsOff || off.Reason != feature.ReasonFeatureOff {
		t.Fatalf("settings end frame = %+v", off)
	}
	if acts := r.page().acted(); len(acts) != 1 {
		t.Fatalf("acts = %+v", acts)
	}
}

// With Hands off nothing is found, started, read or armed: every call but
// stop is refused with its reason, and a spoken start is never taken.
func TestHandsOffStartsNothing(t *testing.T) {
	r := serveHands(t, protocol.OniHandsOff)
	app := r.dialApp(t)
	for _, action := range []string{protocol.OniHandsOpen, protocol.OniHandsTabs, protocol.OniHandsStatus} {
		if code, reason := reasonOf(app.call(protocol.MOniHands, protocol.OniHandsParams{Action: action}, nil)); code != protocol.CodeDisabled || reason != feature.ReasonFeatureOff {
			t.Fatalf("%s with Hands off: %s %s", action, code, reason)
		}
	}
	if res := handsCall(t, app, protocol.OniHandsStop); res.State != protocol.OniHandsStateClosed || res.Browser != "" {
		t.Fatalf("stop with Hands off = %+v", res)
	}
	router, tn := r.voiceRouter(t)
	id := tn.Barge()
	router.Route(route.Utterance{Text: "fill the form on this page", Source: route.Chord, Turn: id, At: time.Now()})
	if r.finds.Load() != 0 || len(r.launches) != 0 || r.clock.Armed() != 0 || r.d.handsResident() || r.d.oni.hands.active.Load() {
		t.Fatalf("Hands off started something: finds %d launches %d timers %d", r.finds.Load(), len(r.launches), r.clock.Armed())
	}
}
