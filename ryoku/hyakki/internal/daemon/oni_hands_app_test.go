package daemon

import (
	"context"
	"fmt"
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
	"ryoku-rashin/internal/hands/input"
	"ryoku-rashin/internal/hands/nest"
	"ryoku-rashin/internal/oni/capture"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/hands/task"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Phase 15 over the real socket: Hands in one picked live window and on
// Hyakki's own nested desktop, every executor a fake. Nothing here reads a
// real keyboard, starts a compositor or types anywhere.

const (
	appTarget  = window.ID("17")
	appSelf    = window.ID("7")
	appPrivate = window.ID("8")
	appOther   = window.ID("9")
	appEditor  = "org.gnome.TextEditor"
	appTyped   = "hello"
	appGoal    = "type hello into the notes"
	appTitle   = "notes.txt"
)

var appBox = window.Box{X: 200, Y: 150, W: 800, H: 600}

// appLive is the person's desktop: the picked editor (floating, placed),
// Hyakki's own window, a password manager and a terminal.
func appLive(focus window.ID, title string, box window.Box) window.Snapshot {
	b := box
	s := window.Snapshot{Outputs: []window.Output{dp1}, Windows: []window.Window{
		{ID: appTarget, App: appEditor, Title: title, Output: "DP-1", Visible: true, PID: 501, Floating: true, Box: &b},
		{ID: appSelf, App: "dev.ryoku.Hyakki", Title: "Hyakki", Output: "DP-1", Visible: true, PID: 777},
		{ID: appPrivate, App: "org.keepassxc.KeePassXC", Title: "Passwords", Output: "DP-1", Visible: true, PID: 888},
		{ID: appOther, App: "foot", Title: "~", Output: "DP-1", Visible: true, PID: 999},
	}}
	for i := range s.Windows {
		s.Windows[i].Focused = s.Windows[i].ID == focus
	}
	return s
}

// appNested is the nested desktop: one placed, focused file manager.
func appNested() window.Snapshot {
	return window.Snapshot{
		Outputs: []window.Output{{Name: "HEADLESS-1", W: 1280, H: 800, Scale: 1}},
		Windows: []window.Window{{ID: "1", App: "org.example.Files", Title: "Files", Output: "HEADLESS-1", Visible: true, PID: 31, Focused: true, Box: &window.Box{X: 0, Y: 0, W: 1280, H: 800}}},
	}
}

// appShots pictures a window with no screen behind it.
type appShots struct{}

func (appShots) Probe(context.Context) []capture.Capability { return nil }

func (appShots) Capture(context.Context, window.Window, []window.Box) (*capture.Shot, capture.Disclosure, error) {
	shot, err := capture.FromPNG(handsPNG())
	return shot, capture.Disclosure{}, err
}

// appWords reads what the rig says the screen shows: the typed word once it
// was typed, and a new word for each folder shown.
type appWords struct{ r *appRig }

func (w appWords) Words(context.Context, []byte) ([]screenask.Word, error) {
	w.r.mu.Lock()
	defer w.r.mu.Unlock()
	words := []screenask.Word{{Text: "notes", Box: feature.Box{X: 0.1, Y: 0.1, W: 0.2, H: 0.05}}}
	if slices.Contains(w.r.typed, appTyped) {
		words = append(words, screenask.Word{Text: appTyped, Box: feature.Box{X: 0.1, Y: 0.3, W: 0.2, H: 0.05}})
	}
	for i := range len(w.r.opened) + w.r.changed {
		words = append(words, screenask.Word{Text: "folder" + string(rune('a'+i)), Box: feature.Box{X: 0.5, Y: 0.6, W: 0.1, H: 0.05}})
	}
	return words, nil
}

// appNest is a nested desktop with no compositor behind it.
type appNest struct {
	r       *appRig
	dir     string
	done    chan struct{}
	once    sync.Once
	stopped atomic.Bool
}

func (n *appNest) Env() []string {
	return []string{"PATH=/usr/bin", "XDG_RUNTIME_DIR=" + n.dir, "WAYLAND_DISPLAY=wayland-1"}
}
func (n *appNest) Dir() string                             { return n.dir }
func (n *appNest) Display() string                         { return "wayland-1" }
func (n *appNest) Size() (int, int)                        { return 1280, 800 }
func (n *appNest) Done() <-chan struct{}                   { return n.done }
func (n *appNest) Capture(context.Context) ([]byte, error) { return handsPNG(), nil }
func (n *appNest) Open(argv []string) error {
	n.r.mu.Lock()
	defer n.r.mu.Unlock()
	n.r.opened = append(n.r.opened, slices.Clone(argv))
	return nil
}
func (n *appNest) Stop() {
	n.stopped.Store(true)
	n.once.Do(func() { close(n.done) })
}

// appRig is a serving daemon at the app or desktop stage.
type appRig struct {
	*handsRig
	live   *fakeDesktop
	nested *fakeDesktop

	mu     sync.Mutex
	typed  []string
	keys   [][]string
	envs   [][]string
	onKey  func()
	nests  []*appNest
	opened [][]string
	// changed counts the person's own steps that changed the screen
	changed int

	keyStarts, keyEnds atomic.Int32
}

type appOptions struct {
	journal string
	roots   []string
	apps    []string
	nest    func(*appRig) OniHandsNest
}

func serveAppHands(t *testing.T, stage string, o appOptions) *appRig {
	t.Helper()
	isolateDaemon(t)
	shortRuntime(t)
	t.Setenv("HYAKKI_SOCKET", "")
	settings, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	settings.Enabled, settings.Oni.Master, settings.Oni.Inspect, settings.Oni.Hands = true, true, true, stage
	if err := config.Save(settings); err != nil {
		t.Fatal(err)
	}
	if o.journal == "" {
		o.journal = filepath.Join(t.TempDir(), "tasks.json")
	}
	r := &appRig{
		handsRig: &handsRig{model: &handsModel{replies: make(chan string, 16)}, clock: &askClock{now: time.Unix(1_800_000_000, 0)}, app: &atomic.Bool{}},
		live:     newDesktop(appLive(appSelf, appTitle, appBox)),
		nested:   newDesktop(appNested()),
	}
	r.handsRig.desk = r.live
	deps := Deps{
		Version:    "oni-hands-app-test",
		Gateway:    r.model,
		OniWindows: func() (window.Provider, error) { return r.live, nil },
		OniCapture: func(window.Provider) capture.Service { return appShots{} },
		OniGuide:   OniGuideDeps{OCR: appWords{r}},
		OniHands: OniHandsDeps{
			Stop: func(func()) (func(), error) { return func() {}, nil },
			AnyKey: func(onKey func()) (func(), error) {
				r.keyStarts.Add(1)
				r.mu.Lock()
				r.onKey = onKey
				r.mu.Unlock()
				return func() { r.keyEnds.Add(1) }, nil
			},
			Nest: func(_ context.Context, opts nest.Options) (OniHandsNest, error) {
				if o.nest != nil {
					return o.nest(r), nil
				}
				n := &appNest{r: r, dir: t.TempDir(), done: make(chan struct{})}
				r.mu.Lock()
				r.nests = append(r.nests, n)
				r.mu.Unlock()
				return n, nil
			},
			NestMissing: func() []string { return nil },
			NestWindows: func(env []string) window.Provider { return r.nested },
			Keyboard: func(env []string) input.Runner {
				r.mu.Lock()
				r.envs = append(r.envs, slices.Clone(env))
				r.mu.Unlock()
				return func(_ context.Context, _ string, args ...string) error {
					r.mu.Lock()
					defer r.mu.Unlock()
					if i := slices.Index(args, "--"); i >= 0 && i+1 < len(args) {
						r.typed = append(r.typed, args[i+1])
					} else {
						r.keys = append(r.keys, slices.Clone(args))
					}
					return nil
				}
			},
			Journal: o.journal,
			Roots:   o.roots,
			AppDirs: o.apps,
			Clock:   r.clock,
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

func (r *appRig) typedChunks() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.typed)
}

func (r *appRig) pressedKeys() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.keys)
}

func (r *appRig) key() {
	r.mu.Lock()
	f := r.onKey
	r.mu.Unlock()
	f()
}

func appCall(t *testing.T, p *socketPeer, params protocol.OniHandsParams) (protocol.OniHandsResult, error) {
	t.Helper()
	var res protocol.OniHandsResult
	err := p.call(protocol.MOniHands, params, &res)
	return res, err
}

func mustApp(t *testing.T, p *socketPeer, params protocol.OniHandsParams) protocol.OniHandsResult {
	t.Helper()
	res, err := appCall(t, p, params)
	if err != nil {
		t.Fatalf("oni.hands %+v: %v", params, err)
	}
	return res
}

// answerStep answers the step's picture question from the app.
func answerStep(t *testing.T, app, surface *socketPeer, about string) {
	t.Helper()
	q := surface.await("the step's picture question", handsConsentFor(feature.Inspect)).(*feature.ConsentFrame)
	if !strings.HasPrefix(q.ID, feature.HandsStepCardPrefix) || !strings.Contains(q.Question, about) {
		t.Fatalf("the step's question = %+v", q)
	}
	if err := app.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: q.ID, Answer: "yes"}, nil); err != nil {
		t.Fatalf("the app's yes: %v", err)
	}
}

// startWindowTask picks the editor and starts goal there from the app,
// then clicks into the editor as the person would.
func (r *appRig) startWindowTask(t *testing.T, app *socketPeer) *socketPeer {
	t.Helper()
	surface := dialSocket(t, protocol.ClientSurface)
	if !surface.open(protocol.OniRendererNative).Presenter {
		t.Fatal("the surface is no presenter")
	}
	if res := mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsSelect, Window: string(appTarget)}); res.State != protocol.OniHandsStateAuthorized || res.App != appEditor {
		t.Fatalf("select = %+v", res)
	}
	res := mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsStart, Goal: appGoal})
	if res.State != protocol.OniHandsStateWorking || res.Scope != protocol.OniHandsScopeWindow || !protocol.ValidOniHandsTaskID(res.Task) {
		t.Fatalf("start = %+v", res)
	}
	if r.keyStarts.Load() != 1 {
		t.Fatalf("the key reader started %d times", r.keyStarts.Load())
	}
	r.live.set(appLive(appTarget, appTitle, appBox))
	answerStep(t, app, surface, "window you picked")
	return surface
}

// A window task: only pickable windows are listed, Hyakki's own and a
// password manager are refused, the CLI picks and starts nothing, the
// whole live desktop is no scope, and typing goes chunk by chunk between
// checks into the picked window's own display and is read back before
// "done" counts. The journal keeps no goal, title or typed word.
func TestHandsWindowTaskTypesBetweenChecksAndReadsBack(t *testing.T) {
	r := serveAppHands(t, protocol.OniHandsApp, appOptions{})
	app := r.dialApp(t)
	cli := dialSocket(t, protocol.ClientCLI)

	list := mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsWindows})
	if len(list.Windows) != 2 || list.Windows[0] != (protocol.OniHandsWindow{ID: string(appTarget), App: appEditor, Title: appTitle, Clickable: true}) || list.Windows[1].ID != string(appOther) {
		t.Fatalf("windows = %+v", list.Windows)
	}
	for _, id := range []window.ID{appSelf, appPrivate} {
		if _, err := appCall(t, app, protocol.OniHandsParams{Action: protocol.OniHandsSelect, Window: string(id)}); codeOf(err) != protocol.CodeForbidden {
			t.Fatalf("select %s: %v", id, err)
		}
	}
	for _, action := range []string{protocol.OniHandsWindows, protocol.OniHandsSelect} {
		if _, err := appCall(t, cli, protocol.OniHandsParams{Action: action, Window: map[bool]string{true: string(appTarget)}[action == protocol.OniHandsSelect]}); codeOf(err) != protocol.CodeForbidden {
			t.Fatalf("the CLI %s: %v", action, err)
		}
	}
	if _, err := appCall(t, app, protocol.OniHandsParams{Action: protocol.OniHandsStart, Goal: appGoal}); codeOf(err) != protocol.CodeConflict {
		t.Fatalf("a start before a pick: %v", err)
	}
	// the app stage reaches no browser tab and no other stage's scope; no
	// scope at all is the person's whole live desktop
	for _, scope := range []string{protocol.OniHandsScopeBrowser, protocol.OniHandsScopeDesktop} {
		if _, err := appCall(t, app, protocol.OniHandsParams{Action: protocol.OniHandsStart, Goal: appGoal, Scope: scope}); codeOf(err) != protocol.CodeForbidden {
			t.Fatalf("a start in scope %s at the app stage: %v", scope, err)
		}
	}
	for _, action := range []string{protocol.OniHandsOpen, protocol.OniHandsTabs} {
		if _, err := appCall(t, app, protocol.OniHandsParams{Action: action}); codeOf(err) != protocol.CodeConflict {
			t.Fatalf("%s at the app stage: %v", action, err)
		}
	}
	if r.keyStarts.Load() != 0 || len(r.typedChunks()) != 0 {
		t.Fatal("something read keys or typed before a task began")
	}

	r.model.replies <- handsReply("type", map[string]any{"text": appTyped})
	r.model.replies <- handsReply("done", map[string]any{"text": "Typed."})
	surface := r.startWindowTask(t, app)
	active := surface.await("the window task's frame", handsFrame(func(f *feature.HandsFrame) bool { return f.Active })).(*feature.HandsFrame)
	if active.Stage != protocol.OniHandsApp || active.Scope != protocol.OniHandsScopeWindow || active.App != appEditor || active.Output != "DP-1" {
		t.Fatalf("active frame = %+v", active)
	}
	end := surface.await("the task's end", handsFrame(func(f *feature.HandsFrame) bool { return !f.Active && f.End != "" })).(*feature.HandsFrame)
	if end.Stage != protocol.OniHandsApp || end.End != protocol.OniHandsEndDone {
		t.Fatalf("end frame = %+v", end)
	}
	if got := r.typedChunks(); !slices.Equal(got, []string{appTyped}) {
		t.Fatalf("typed %q", got)
	}
	r.mu.Lock()
	env := slices.Clone(r.envs[0])
	r.mu.Unlock()
	if !slices.Contains(env, "XDG_RUNTIME_DIR="+os.Getenv("XDG_RUNTIME_DIR")) || !slices.Contains(env, "WAYLAND_DISPLAY=wayland-0") || len(env) != 4 {
		t.Fatalf("the keyboard's environment is %q", env)
	}
	eventually(t, "the key reader stopped", func() bool { return r.keyEnds.Load() == 1 })

	var log protocol.OniHandsLogResult
	if err := app.call(protocol.MOniHandsLog, protocol.Empty{}, &log); err != nil {
		t.Fatal(err)
	}
	var tools []string
	for i := len(log.Rows) - 1; i >= 0; i-- {
		row := log.Rows[i]
		tools = append(tools, row.Tool+":"+row.Outcome)
		if strings.Contains(row.Reason, appTyped) || strings.Contains(row.Tool, appTyped) {
			t.Fatalf("a ledger row holds the typed word: %+v", row)
		}
	}
	if want := []string{"check:done", "type:done", "check:done", "readback:done", "end:done"}; !slices.Equal(tools, want) {
		t.Fatalf("ledger %q, want %q", tools, want)
	}

	tasks := mustApp(t, cli, protocol.OniHandsParams{Action: protocol.OniHandsTasks})
	if len(tasks.Tasks) != 1 || tasks.Tasks[0].State != string(task.Done) || tasks.Tasks[0].Scope != protocol.OniHandsScopeWindow ||
		tasks.Tasks[0].App != appEditor || tasks.Tasks[0].Origin != task.OriginApp || tasks.Tasks[0].Reason != protocol.OniHandsEndDone {
		t.Fatalf("tasks = %+v", tasks.Tasks)
	}
	raw, err := os.ReadFile(r.d.handsJournalPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{appTyped, appGoal, appTitle, "notes"} {
		if strings.Contains(string(raw), word) {
			t.Fatalf("the journal keeps %q: %s", word, raw)
		}
	}
	if info, err := os.Stat(r.d.handsJournalPath()); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the journal file is %v, %v", info, err)
	}
	if _, err := appCall(t, cli, protocol.OniHandsParams{Action: protocol.OniHandsClear}); codeOf(err) != protocol.CodeForbidden {
		t.Fatalf("the CLI cleared the journal: %v", err)
	}
	if res := mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsClear}); len(res.Tasks) != 0 {
		t.Fatalf("clear = %+v", res)
	}
}

// Every change the rule watches pauses a window task before its next
// action: the title, the box, focus on Hyakki's own window or another one,
// and any key the person presses. Nothing is typed after the pause, no
// voice and no CLI resumes it, and a resume from the app checks afresh.
func TestHandsWindowTaskPausesOnEveryChange(t *testing.T) {
	moved := appBox
	moved.X += 40
	for _, c := range []struct {
		name  string
		pause string
		do    func(r *appRig)
	}{
		{"title", protocol.OniHandsPauseTitle, func(r *appRig) { r.live.set(appLive(appTarget, "secret.txt", appBox)) }},
		{"geometry", protocol.OniHandsPauseGeometry, func(r *appRig) { r.live.set(appLive(appTarget, appTitle, moved)) }},
		{"self", protocol.OniHandsPauseSelf, func(r *appRig) { r.live.set(appLive(appSelf, appTitle, appBox)) }},
		{"focus", protocol.OniHandsPauseFocus, func(r *appRig) { r.live.set(appLive(appOther, appTitle, appBox)) }},
		{"private", protocol.OniHandsPausePrivate, func(r *appRig) { r.live.set(appLive(appPrivate, appTitle, appBox)) }},
		{"key", protocol.OniHandsPauseKey, func(r *appRig) { r.key() }},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := serveAppHands(t, protocol.OniHandsApp, appOptions{})
			app := r.dialApp(t)
			surface := r.startWindowTask(t, app)
			// the model is asked and waits: the change comes now
			eventually(t, "the step's request", func() bool { return r.model.sent() == 1 })
			c.do(r)
			paused := surface.await("the pause", handsFrame(func(f *feature.HandsFrame) bool { return f.Active && f.Phase == protocol.OniHandsStatePaused })).(*feature.HandsFrame)
			if paused.Pause != c.pause {
				t.Fatalf("paused for %q, want %q", paused.Pause, c.pause)
			}
			eventually(t, "the request cut", func() bool { return r.model.aborted.Load() == 1 })
			r.model.replies <- handsReply("type", map[string]any{"text": appTyped})

			// no spoken resume, from any source, and no CLI resume
			router, tn := r.voiceRouter(t)
			id := tn.Barge()
			for _, s := range []route.Source{route.Chord, route.Toggle, route.Wake, route.Open} {
				router.Route(route.Utterance{Text: "keep going", Source: s, Turn: id, At: time.Now()})
			}
			cli := dialSocket(t, protocol.ClientCLI)
			if _, err := appCall(t, cli, protocol.OniHandsParams{Action: protocol.OniHandsResume}); codeOf(err) != protocol.CodeForbidden {
				t.Fatalf("the CLI resumed: %v", err)
			}
			time.Sleep(50 * time.Millisecond)
			if res := mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsStatus}); res.State != protocol.OniHandsStatePaused || res.Pause != c.pause {
				var log protocol.OniHandsLogResult
				_ = app.call(protocol.MOniHandsLog, protocol.Empty{}, &log)
				t.Fatalf("after the refused resumes: %+v, ledger %+v", res, log.Rows)
			}
			if len(r.typedChunks()) != 0 || len(r.pressedKeys()) != 0 {
				t.Fatalf("input after the pause: %q %q", r.typedChunks(), r.pressedKeys())
			}
			// the app's Resume checks afresh: a lasting change pauses again
			// before anything is typed
			mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsResume})
			time.Sleep(100 * time.Millisecond)
			if c.name == "key" {
				// a key is a moment, not a state: the resumed task goes on
				return
			}
			if res := mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsStatus}); res.State != protocol.OniHandsStatePaused || len(r.typedChunks()) != 0 {
				t.Fatalf("after the app's resume: %+v, typed %q", res, r.typedChunks())
			}
			mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsStop})
			surface.await("the stop", handsFrame(func(f *feature.HandsFrame) bool { return f.End == protocol.OniHandsEndStopped }))
		})
	}
}

// Enter is the person's to press: it becomes a handoff card with no yes.
// No spoken yes, from any source, and no answer on the wire presses it;
// the app's button says the person did it.
func TestHandsWindowHandoffHasNoYes(t *testing.T) {
	r := serveAppHands(t, protocol.OniHandsApp, appOptions{})
	app := r.dialApp(t)
	r.model.replies <- handsReply("press", map[string]any{"keys": "Return"})
	surface := r.startWindowTask(t, app)
	card := surface.await("the handoff card", handsConsentFor(feature.HandsApp)).(*feature.ConsentFrame)
	if !strings.HasPrefix(card.ID, feature.HandsCardPrefix) || card.Destination != feature.EgressLocal {
		t.Fatalf("the handoff card = %+v", card)
	}
	router, tn := r.voiceRouter(t)
	id := tn.Barge()
	for _, s := range []route.Source{route.Chord, route.Toggle, route.Wake, route.Open} {
		for _, text := range []string{"yes", "go ahead", "approve it"} {
			if d := router.Route(route.Utterance{Text: text, Source: s, Turn: id, At: time.Now()}); d.Kind != route.NeedsPress {
				t.Fatalf("%s %q = %+v", s, text, d)
			}
		}
	}
	for _, p := range []*socketPeer{app, dialSocket(t, protocol.ClientCLI)} {
		if code := codeOf(p.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: "yes"}, nil)); code != protocol.CodeBadRequest {
			t.Fatalf("a yes to the handoff card: %s", code)
		}
	}
	if len(r.pressedKeys()) != 0 || len(r.typedChunks()) != 0 {
		t.Fatalf("a yes pressed a key: %q", r.pressedKeys())
	}
	r.model.replies <- handsReply("done", map[string]any{"text": "Sent."})
	// the person pressed Enter themselves: the screen moved
	r.mu.Lock()
	r.changed++
	r.mu.Unlock()
	if res := mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsResume}); res.State != protocol.OniHandsStateWorking {
		t.Fatalf("the person's done = %+v", res)
	}
	surface.await("the next step", handsFrame(func(f *feature.HandsFrame) bool { return !f.Active && f.End != "" }))
	if len(r.pressedKeys()) != 0 {
		t.Fatalf("Hands pressed %q", r.pressedKeys())
	}
}

func TestHandsWindowClosureEndsPausedAndWaitingTasks(t *testing.T) {
	for _, paused := range []bool{false, true} {
		t.Run(fmt.Sprintf("paused=%t", paused), func(t *testing.T) {
			r := serveAppHands(t, protocol.OniHandsApp, appOptions{})
			app := r.dialApp(t)
			if !paused {
				r.model.replies <- handsReply("press", map[string]any{"keys": "Return"})
			}
			surface := r.startWindowTask(t, app)
			if paused {
				eventually(t, "the step's request", func() bool { return r.model.sent() == 1 })
				r.live.set(appLive(appSelf, appTitle, appBox))
				surface.await("the pause", handsFrame(func(f *feature.HandsFrame) bool {
					return f.Active && f.Phase == protocol.OniHandsStatePaused
				}))
			} else {
				surface.await("the handoff card", handsConsentFor(feature.HandsApp))
			}
			closed := appLive(appSelf, appTitle, appBox)
			closed.Windows = slices.DeleteFunc(closed.Windows, func(w window.Window) bool { return w.ID == appTarget })
			r.live.set(closed)
			surface.await("the closed target", handsFrame(func(f *feature.HandsFrame) bool {
				return !f.Active && f.End == protocol.OniHandsEndClosed
			}))
			tasks := mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsTasks}).Tasks
			if len(tasks) != 1 || tasks[0].Reason != protocol.OniHandsEndClosed {
				t.Fatalf("closed target journal: %+v", tasks)
			}
			if len(r.typedChunks()) != 0 || len(r.pressedKeys()) != 0 {
				t.Fatal("closing the target delivered input")
			}
			eventually(t, "the key reader stopped", func() bool { return r.keyEnds.Load() == r.keyStarts.Load() })
		})
	}
}

// A restart holds: a task the dead daemon left running comes back
// interrupted, startup starts no key reader, keyboard or nested desktop,
// and only the app's or the presenter's Start, with the goal said again,
// runs it.
func TestHandsRestartHoldsAndStartsNothing(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "tasks.json")
	j := task.Open(journal)
	left, err := j.Enqueue(task.Task{ID: task.NewID(), Scope: protocol.OniHandsScopeWindow, App: appEditor, Origin: task.OriginApp}, time.Unix(1_799_999_000, 0))
	if err != nil || left.State != task.Working {
		t.Fatalf("the dead daemon's task: %+v, %v", left, err)
	}
	r := serveAppHands(t, protocol.OniHandsApp, appOptions{journal: journal})
	app := r.dialApp(t)
	cli := dialSocket(t, protocol.ClientCLI)
	tasks := mustApp(t, cli, protocol.OniHandsParams{Action: protocol.OniHandsTasks})
	if len(tasks.Tasks) != 1 || tasks.Tasks[0].State != string(task.Interrupted) || tasks.Tasks[0].Reason != task.ReasonRestart {
		t.Fatalf("after the restart: %+v", tasks.Tasks)
	}
	if res := mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsStatus}); res.State != protocol.OniHandsStateOpen || res.Task != "" {
		t.Fatalf("status after the restart: %+v", res)
	}
	r.mu.Lock()
	nests, envs := len(r.nests), len(r.envs)
	r.mu.Unlock()
	if r.keyStarts.Load() != 0 || nests != 0 || envs != 0 || r.model.sent() != 0 {
		t.Fatal("the restart started an input program, a nested desktop or a request")
	}
	start := protocol.OniHandsParams{Action: protocol.OniHandsStart, Goal: appGoal, Task: left.ID}
	if _, err := appCall(t, cli, start); codeOf(err) != protocol.CodeForbidden {
		t.Fatalf("the CLI started the held task: %v", err)
	}
	router, tn := r.voiceRouter(t)
	id := tn.Barge()
	for _, s := range []route.Source{route.Chord, route.Wake, route.Open} {
		router.Route(route.Utterance{Text: "keep going in the notes", Source: s, Turn: id, At: time.Now()})
	}
	time.Sleep(50 * time.Millisecond)
	if r.keyStarts.Load() != 0 || r.model.sent() != 0 {
		t.Fatal("a voice started the held task")
	}
	mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsSelect, Window: string(appTarget)})
	surface := dialSocket(t, protocol.ClientSurface)
	surface.open(protocol.OniRendererNative)
	res := mustApp(t, app, start)
	if res.State != protocol.OniHandsStateWorking || res.Task != left.ID {
		t.Fatalf("the app's Start: %+v", res)
	}
	tasks = mustApp(t, cli, protocol.OniHandsParams{Action: protocol.OniHandsTasks})
	if tasks.Tasks[0].State != string(task.Working) {
		t.Fatalf("the held task did not start: %+v", tasks.Tasks)
	}
	mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsStop})
	eventually(t, "the task stopped", func() bool {
		ts := mustApp(t, cli, protocol.OniHandsParams{Action: protocol.OniHandsTasks}).Tasks
		return ts[0].State == string(task.Stopped)
	})
}

// The desktop stage works only on Hyakki's own nested desktop: its input
// goes to the nested display alone, the presenter starts nothing there, a
// found executable is shown by opening its folder and never run, and the
// nested desktop ends with the task.
func TestHandsDesktopTaskShowsFoldersAndNeverRunsFiles(t *testing.T) {
	roots := t.TempDir()
	script := filepath.Join(roots, "report.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(roots, "report.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	apps := t.TempDir()
	entry := "[Desktop Entry]\nType=Application\nName=Files\nExec=/usr/bin/true %U\nMimeType=inode/directory;\nCategories=System;FileManager;\n"
	if err := os.WriteFile(filepath.Join(apps, "org.example.Files.desktop"), []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}
	r := serveAppHands(t, protocol.OniHandsDesktop, appOptions{roots: []string{roots}, apps: []string{apps}})
	app := r.dialApp(t)
	surface := dialSocket(t, protocol.ClientSurface)
	surface.open(protocol.OniRendererNative)
	if _, err := appCall(t, surface, protocol.OniHandsParams{Action: protocol.OniHandsStart, Goal: "show the report"}); codeOf(err) != protocol.CodeForbidden {
		t.Fatalf("the presenter started a desktop task: %v", err)
	}
	for _, scope := range []string{protocol.OniHandsScopeWindow, protocol.OniHandsScopeNested, protocol.OniHandsScopeBrowser} {
		if _, err := appCall(t, app, protocol.OniHandsParams{Action: protocol.OniHandsStart, Goal: "x", Scope: scope}); codeOf(err) != protocol.CodeForbidden {
			t.Fatalf("a start in scope %s at the desktop stage: %v", scope, err)
		}
	}
	if _, err := appCall(t, app, protocol.OniHandsParams{Action: protocol.OniHandsWindows}); codeOf(err) != protocol.CodeConflict {
		t.Fatalf("live windows listed at the desktop stage: %v", err)
	}
	r.model.replies <- handsReply("find_files", map[string]any{"query": "report"})
	r.model.replies <- handsReply("show_file", map[string]any{"index": 1})
	r.model.replies <- handsReply("done", map[string]any{"text": "It is in that folder."})
	res := mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsStart, Goal: "show the report"})
	if res.State != protocol.OniHandsStateWorking || res.Scope != protocol.OniHandsScopeDesktop || !r.d.handsResident() {
		t.Fatalf("start = %+v", res)
	}
	// the person turns from Hyakki, where they pressed Start
	r.live.set(appLive(appOther, appTitle, appBox))
	answerStep(t, app, surface, "separate desktop")
	end := surface.await("the task's end", handsFrame(func(f *feature.HandsFrame) bool { return !f.Active && f.End != "" })).(*feature.HandsFrame)
	if end.Stage != protocol.OniHandsDesktop || end.End != protocol.OniHandsEndDone {
		t.Fatalf("end frame = %+v", end)
	}
	r.mu.Lock()
	opened := slices.Clone(r.opened)
	nests := slices.Clone(r.nests)
	envs := slices.Clone(r.envs)
	r.mu.Unlock()
	if len(opened) != 1 || !slices.Equal(opened[0], []string{"/usr/bin/true", roots}) {
		t.Fatalf("opened %q, want the folder alone", opened)
	}
	for _, argv := range opened {
		if slices.Contains(argv, script) {
			t.Fatalf("the executable was opened: %q", argv)
		}
	}
	if len(nests) != 1 || len(envs) != 1 {
		t.Fatalf("%d nested desktops, %d keyboards", len(nests), len(envs))
	}
	if !slices.Contains(envs[0], "XDG_RUNTIME_DIR="+nests[0].dir) || !slices.Contains(envs[0], "WAYLAND_DISPLAY=wayland-1") || slices.Contains(envs[0], "XDG_RUNTIME_DIR="+os.Getenv("XDG_RUNTIME_DIR")) {
		t.Fatalf("the nested keyboard's environment is %q", envs[0])
	}
	eventually(t, "the nested desktop ended with the task", func() bool { return nests[0].stopped.Load() && !r.d.handsResident() })
	if r.keyStarts.Load() != 0 {
		t.Fatal("the nested desktop read the person's keys")
	}
}

// A team's request reaches Hands only in scope nested or window, never the
// desktop scope, and never once its approval no longer holds.
func TestHandsGrantedNeverReachesTheDesktop(t *testing.T) {
	r := serveAppHands(t, protocol.OniHandsApp, appOptions{})
	live := func() bool { return true }
	for _, scope := range []string{protocol.OniHandsScopeDesktop, protocol.OniHandsScopeBrowser, ""} {
		if _, err := r.d.handsGranted(t.Context(), "file it", scope, "", live); codeOf(err) != protocol.CodeForbidden {
			t.Fatalf("a team's request in scope %q: %v", scope, err)
		}
	}
	if _, err := r.d.handsGranted(t.Context(), "file it", protocol.OniHandsScopeNested, "", func() bool { return false }); codeOf(err) != protocol.CodeForbidden {
		t.Fatalf("a stale approval: %v", err)
	}
	r.mu.Lock()
	nests := len(r.nests)
	r.mu.Unlock()
	if nests != 0 || r.keyStarts.Load() != 0 {
		t.Fatal("a refused request started something")
	}
}

type shutdownWaitNest struct {
	*appNest
	entered chan struct{}
	release <-chan struct{}
	begin   sync.Once
}

func (n *shutdownWaitNest) Stop() {
	n.begin.Do(func() { close(n.entered) })
	<-n.release
	n.appNest.Stop()
}

func TestHandsShutdownWaitsForNestedDesktop(t *testing.T) {
	for _, stopped := range []bool{false, true} {
		name := "active"
		if stopped {
			name = "cleanup-already-running"
		}
		t.Run(name, func(t *testing.T) {
			release := make(chan struct{})
			var released sync.Once
			finish := func() { released.Do(func() { close(release) }) }
			n := &shutdownWaitNest{
				appNest: &appNest{dir: t.TempDir(), done: make(chan struct{})},
				entered: make(chan struct{}), release: release,
			}
			r := serveAppHands(t, protocol.OniHandsDesktop, appOptions{nest: func(r *appRig) OniHandsNest {
				n.r = r
				return n
			}})
			t.Cleanup(finish)
			app := r.dialApp(t)
			surface := dialSocket(t, protocol.ClientSurface)
			surface.open(protocol.OniRendererNative)
			mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsStart, Goal: "wait here"})
			if stopped {
				mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsStop})
			}
			done := make(chan struct{})
			go func() { r.d.Shutdown(); close(done) }()
			select {
			case <-n.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("shutdown never began ending the nested desktop")
			}
			select {
			case <-done:
				t.Fatal("shutdown returned while the nested desktop still owned its resources")
			case <-time.After(100 * time.Millisecond):
			}
			finish()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("shutdown did not finish after the nested desktop ended")
			}
			if !n.stopped.Load() {
				t.Fatal("shutdown finished before nested desktop cleanup")
			}
		})
	}
}

func TestHandsDisclosureSurvivesFocusOnHyakki(t *testing.T) {
	for _, stage := range []string{protocol.OniHandsApp, protocol.OniHandsDesktop} {
		for _, test := range []struct {
			name string
			pid  int
			own  bool
		}{
			{"authenticated app", os.Getpid(), true},
			{"same label, foreign process", os.Getpid() + 1, false},
			{"same label, missing process", 0, false},
		} {
			t.Run(stage+"/"+test.name, func(t *testing.T) {
				r := serveAppHands(t, stage, appOptions{})
				app := r.dialApp(t)
				surface := dialSocket(t, protocol.ClientSurface)
				if !surface.open(protocol.OniRendererNative).Presenter {
					t.Fatal("the surface is no presenter")
				}
				if stage == protocol.OniHandsApp {
					mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsSelect, Window: string(appTarget)})
				}
				mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsStart, Goal: appGoal})
				target := appLive(appTarget, appTitle, appBox)
				r.live.set(target)
				card := surface.await("the step's picture question", handsConsentFor(feature.Inspect)).(*feature.ConsentFrame)
				focus := func(s window.Snapshot) {
					r.live.set(s)
					handsWait(r.d, func() struct{} {
						r.d.handsAppEvent(r.d.oni.hands.run, &s, nil)
						return struct{}{}
					})
				}
				self := appLive(appSelf, appTitle, appBox)
				for i := range self.Windows {
					if self.Windows[i].ID == appSelf {
						self.Windows[i].PID = test.pid
					}
				}
				focus(self)
				if r.model.sent() != 0 {
					t.Fatal("the picture left before approval")
				}
				res := mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsStatus})
				if !test.own {
					if res.State != protocol.OniHandsStatePaused {
						t.Fatalf("foreign focus did not pause the task: %+v", res)
					}
					if err := app.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: "yes"}, nil); err == nil {
						t.Fatal("foreign focus left the picture approval usable")
					}
					return
				}
				if res.State != protocol.OniHandsStateWorking {
					t.Fatalf("focusing the authenticated app to answer its disclosure interrupted the task: %+v", res)
				}
				if err := app.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: "yes"}, nil); err != nil {
					t.Fatalf("the disclosure stopped accepting the person's answer: %v", err)
				}
				eventually(t, "the approved picture reaches the model", func() bool { return r.model.sent() == 1 })
				focus(target)
				focus(self)
				if res := mustApp(t, app, protocol.OniHandsParams{Action: protocol.OniHandsStatus}); res.State != protocol.OniHandsStatePaused {
					t.Fatalf("the app retained its focus exception after approval: %+v", res)
				}
				eventually(t, "unapproved focus aborts the model", func() bool { return r.model.aborted.Load() == 1 })
			})
		}
	}
}
