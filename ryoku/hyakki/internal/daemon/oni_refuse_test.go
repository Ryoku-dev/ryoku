package daemon

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/hands/chrome"
	"ryoku-rashin/internal/hands/input"
	"ryoku-rashin/internal/hands/nest"
	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/oni/sound"
	"ryoku-rashin/internal/oni/voice/audio"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/reflex/act"
)

// refuseRealDesktop returns d with every dep whose nil is the user's own
// session filled with a fake that fails t when used: the wake unit (the
// user manager), notifications (the session bus), the executors (app
// launch, the window manager, the sink), the microphone, the speaker, the
// typing tool, the focus guard's window closer, and a rewrite's selection
// reader and undo key. The desktop-entry
// catalog becomes an empty one, so no real directory is read. Deps a test
// injects stay as they are; production keeps nil as the real one.
func refuseRealDesktop(t testing.TB, d Deps) Deps {
	t.Helper()
	fail := func(what string) error {
		t.Errorf("a test reached the real %s; inject a fake", what)
		return fmt.Errorf("refused in tests: the real %s", what)
	}
	r := &d.OniReflex
	if r.Wake == nil {
		r.Wake = func(bool) error { return fail("wake unit") }
	}
	if r.Notify == nil {
		r.Notify = func(string, string) error { return fail("notification service") }
	}
	if r.Exec == nil {
		r.Exec = refusingExec{fail}
	}
	if r.Catalog == nil {
		r.Catalog = emptyCatalog{}
	}
	v := &d.OniVoice
	if v.Capture == nil {
		v.Capture = refusingCapture{fail}
	}
	if v.Sink == nil {
		v.Sink = func() (sound.Sink, error) { return nil, fail("speaker") }
	}
	if v.Type == nil {
		v.Type = func(context.Context, string, ...string) error { return fail("typing tool") }
	}
	h := &d.OniHands
	if h.Find == nil {
		h.Find = func() (OniHandsProgram, error) { return OniHandsProgram{}, fail("desktop browsers") }
	}
	if h.Launch == nil {
		h.Launch = func(context.Context, chrome.Options) (OniHandsBrowser, error) { return nil, fail("Hands browser") }
	}
	if h.Stop == nil {
		h.Stop = func(func()) (func(), error) { return nil, fail("stop key reader") }
	}
	if h.AnyKey == nil {
		h.AnyKey = func(func()) (func(), error) { return nil, fail("key-edge reader") }
	}
	if h.Nest == nil {
		h.Nest = func(context.Context, nest.Options) (OniHandsNest, error) { return nil, fail("nested desktop") }
	}
	if h.NestWindows == nil {
		h.NestWindows = func([]string) window.Provider { _ = fail("nested desktop's windows"); return nil }
	}
	if h.Keyboard == nil {
		h.Keyboard = func([]string) input.Runner {
			return func(context.Context, string, ...string) error { return fail("keyboard tool") }
		}
	}
	if h.Pointer == nil {
		h.Pointer = func(string) (input.Pointer, error) { return nil, fail("virtual pointer") }
	}
	if h.NestMissing == nil {
		h.NestMissing = func() []string { return nil }
	}
	if h.Journal == "" {
		h.Journal = filepath.Join(t.TempDir(), "oni-hands-tasks.json")
	}
	if h.Roots == nil {
		h.Roots = []string{}
	}
	if h.AppDirs == nil {
		h.AppDirs = []string{}
	}
	if d.OniGuard.Closer == nil {
		d.OniGuard.Closer = refusingCloser{fail}
	}
	w := &d.OniRewrite
	if w.Select == nil {
		w.Select = func(context.Context, int, string, ...string) ([]byte, error) { return nil, fail("selection reader") }
	}
	if w.Keys == nil {
		w.Keys = func(context.Context, string, ...string) error { return fail("undo key") }
	}
	return d
}

// refusingExec carries out nothing and can do nothing; running an action
// fails the test.
type refusingExec struct{ fail func(string) error }

func (e refusingExec) Do(_ context.Context, a reflex.Action) (act.Outcome, error) {
	return act.Outcome{}, e.fail("executor for " + string(a.Intent))
}

func (refusingExec) Caps() map[reflex.Intent]bool { return nil }

// refusingCloser closes no window; asking it to fails the test.
type refusingCloser struct{ fail func(string) error }

func (c refusingCloser) Supported(context.Context) bool {
	_ = c.fail("window closer")
	return false
}

func (c refusingCloser) Close(context.Context, string, string) error {
	return c.fail("window closer")
}

// emptyCatalog knows no app.
type emptyCatalog struct{}

func (emptyCatalog) Resolve(string) (string, string, float64, bool) { return "", "", 0, false }

// refusingCapture opens no microphone.
type refusingCapture struct{ fail func(string) error }

func (c refusingCapture) Start(context.Context, string) (<-chan []int16, func() error, error) {
	return nil, nil, c.fail("microphone")
}

var _ audio.Capture = refusingCapture{}

// recordTB keeps the errors a refusing fake reports.
type recordTB struct {
	testing.TB
	errs []string
}

func (r *recordTB) Helper() {}

func (r *recordTB) Errorf(format string, args ...any) {
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}

// With a rig's defaults, every one of those deps refuses and fails the
// test that reaches it, and an injected dep is kept.
func TestRefuseRealDesktopFailsWhatIsNotInjected(t *testing.T) {
	woke := false
	rec := &recordTB{TB: t}
	d := refuseRealDesktop(rec, Deps{OniReflex: OniReflexDeps{Wake: func(bool) error { woke = true; return nil }}})
	if err := d.OniReflex.Wake(true); err != nil || !woke || len(rec.errs) != 0 {
		t.Fatalf("an injected wake unit was replaced: %v %v", err, rec.errs)
	}
	ctx := context.Background()
	reached := []func() error{
		func() error { return d.OniReflex.Notify("Reminder", "stretch") },
		func() error { _, err := d.OniReflex.Exec.Do(ctx, reflex.Action{Intent: reflex.OpenApp}); return err },
		func() error { _, _, err := d.OniVoice.Capture.Start(ctx, ""); return err },
		func() error { _, err := d.OniVoice.Sink(); return err },
		func() error { return d.OniVoice.Type(ctx, "wtype", "x") },
		func() error { return refuseRealDesktop(rec, Deps{}).OniReflex.Wake(true) },
		func() error { _, err := d.OniHands.Find(); return err },
		func() error { _, err := d.OniHands.Launch(ctx, chrome.Options{}); return err },
		func() error { _, err := d.OniHands.Stop(func() {}); return err },
		func() error { return d.OniGuard.Closer.Close(ctx, "21", "tiktok") },
		func() error { _, err := d.OniRewrite.Select(ctx, 10, "wl-paste"); return err },
		func() error { return d.OniRewrite.Keys(ctx, "wtype", "x") },
	}
	for i, call := range reached {
		before := len(rec.errs)
		if err := call(); err == nil || len(rec.errs) != before+1 {
			t.Errorf("dep %d: err %v, %d test errors", i, err, len(rec.errs)-before)
		}
	}
	if _, _, _, ok := d.OniReflex.Catalog.Resolve("firefox"); ok {
		t.Error("the test catalog knows an app")
	}
	if caps := d.OniReflex.Exec.Caps(); len(caps) != 0 {
		t.Errorf("the refusing executor claims %v", caps)
	}
}
