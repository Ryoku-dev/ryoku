package daemon

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/hands/app"
	"ryoku-rashin/internal/hands/nest"
	"ryoku-rashin/internal/oni/capture"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/hands/apprules"
	"ryoku-rashin/internal/oni/hands/consent"
	"ryoku-rashin/internal/oni/hands/discipline"
	"ryoku-rashin/internal/oni/hands/loop"
	"ryoku-rashin/internal/oni/hands/proto"
	"ryoku-rashin/internal/oni/hands/task"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/reflex/launch"
)

// Hands outside a browser tab: in one live window the person picked (the
// app stage's window scope), or on Hyakki's own nested desktop (the app
// stage's nested scope, the desktop stage's desktop scope). The person's
// whole live desktop is never a scope.
//
// The loop asks for every action to be checked before and after; this
// file answers those checks from a fresh read of the windows (the picked
// window's baseline, or the nested desktop's away rule), and app.Driver
// checks again around each chunk, key and click it sends. Input goes only
// to the picked window's display, or to the nested desktop's own display,
// through the app driver; nothing here runs a file. A file the model wants
// shown has its folder opened on the nested desktop by the file manager,
// never the file itself. Startup starts nothing: a task begins only from
// the app's or the presenter's Start (or a team's approved request), and
// what it started dies with it.

// handsSelf are Hyakki's own app ids: never a target, and their focus
// pauses a task.
var handsSelf = []string{"dev.ryoku.Hyakki"}

// The ledger's tool words for the checks and the read-back.
const (
	handsToolCheck    = "check"
	handsToolReadBack = "readback"
)

// How a task outside a tab is stopped besides the browser's ways.
const (
	handsByGrant = "grant"
	handsByTime  = "time"
	handsByEnded = "ended"
)

// The most files one find_files lists, and how deep and how long it looks.
const (
	handsFindMax   = 20
	handsFindDepth = 6
	handsFindTime  = 3 * time.Second
)

var (
	handsLineNoPick     = i18n.T("Pick the window Hands works in first.")
	handsLineNoStage    = i18n.T("Hands is not at that stage. Choose it in Oni's settings.")
	handsLineNoBrowser  = i18n.T("The Hands browser is used only when Hands works in a browser.")
	handsLineNoDesktop  = i18n.T("Hyakki's separate desktop cannot run here.")
	handsLineQueued     = i18n.T("Another task is running. This one waits for its Start button.")
	handsLineNoKeys     = i18n.T("The keyboards cannot be read, so a key you press could not pause Hands. It does not start.")
	handsDisclosWindow  = capture.Disclosure{Path: "hands-window", ScopeExact: true, Text: i18n.T("A picture of the window you picked, with Oni's own pixels painted out, and the words read in it.")}
	handsDisclosDesktop = capture.Disclosure{Path: "hands-desktop", ScopeExact: true, Text: i18n.T("A picture of Hyakki's separate desktop, and the words read in it.")}
)

// handsPick is the live window the app picked for the next window task:
// only its id and app id, never its title.
type handsPick struct {
	id  window.ID
	app string
}

// handsExec carries out a task outside a tab. Its fields are set before the
// run starts; regions, files and the picture are on the queue only.
type handsExec struct {
	scope string
	drv   *app.Driver
	base  discipline.Baseline

	// the live desktop, read only: the picked window's, and for the nested
	// scopes where Hyakki's own window is
	live window.Provider

	// the nested desktop
	ws     OniHandsNest
	nested window.Provider
	desk   discipline.Desktop

	// grant, for a team's approved task, says whether its approval still
	// holds; nil for the app's and the presenter's own tasks
	grant func() bool

	// the last reading: the regions read, and for find_files the paths
	// found, in memory until the next one or the end
	regions []handsRegion
	files   []string
	focus   window.Window
	roots   []string

	// ready closes once the picked window first has the person's focus:
	// they press Start in Hyakki, then click into the window. Until then
	// nothing acts and only the window closing counts.
	ready     chan struct{}
	readyOnce sync.Once

	// the latest read of each desktop from its events, on the queue
	lastLive, lastNested *window.Snapshot

	watchStop context.CancelFunc
	closed    atomic.Bool
}

// handsRegion is one line of words read on the screen, in fractions of the
// picture.
type handsRegion struct {
	id   string
	name string
	box  feature.Box
}

// handsStageOf is the hands stage a scope belongs to.
func handsStageOf(scope string) string {
	for stage, scopes := range protocol.OniHandsStageScopes {
		if slices.Contains(scopes, scope) {
			return stage
		}
	}
	return protocol.OniHandsOff
}

// handsScopeOf is the scope a start at stage asks for: scope itself, or the
// stage's own when empty.
func handsScopeOf(stage, scope string) string {
	if scope != "" {
		return scope
	}
	switch stage {
	case protocol.OniHandsBrowser:
		return protocol.OniHandsScopeBrowser
	case protocol.OniHandsApp:
		return protocol.OniHandsScopeWindow
	case protocol.OniHandsDesktop:
		return protocol.OniHandsScopeDesktop
	}
	return ""
}

// handsAppLimits are the spec's hands.app numbers for the driver.
func handsAppLimits() app.Limits {
	s, err := oniGuideSpec()
	if err != nil {
		return app.Limits{}
	}
	a := s.Hands.App
	return app.Limits{GeometryTolPx: a.GeometryTolPx, Verify: a.Verify(), Chunk: a.Chunk(), ChunkRunes: a.TypeChunkRunes, MaxTypeRunes: a.MaxTypeRunes, Readback: a.Readback()}
}

// handsTolPx is the spec's geometry tolerance.
func handsTolPx() int {
	if s, err := oniGuideSpec(); err == nil {
		return s.Hands.App.GeometryTolPx
	}
	return 0
}

// handsMasks are Oni's own rectangles on output, painted out of a live
// window's picture.
func (d *Daemon) handsMasks(output string) []window.Box {
	ctx, cancel := context.WithTimeout(context.Background(), oniOverlayWait)
	defer cancel()
	return d.oniMasks(ctx, output)
}

// handsWindowsCall lists the live windows a window task may work in.
func (d *Daemon) handsWindowsCall() (protocol.OniHandsResult, error) {
	provider, err := d.oniWindows()
	if err != nil {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The windows could not be read: %v.", err))
	}
	ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
	snap, err := provider.Snapshot(ctx)
	cancel()
	if err != nil {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The windows could not be read: %v.", err))
	}
	res := handsWait(d, d.handsResult)
	res.Windows = []protocol.OniHandsWindow{}
	for _, w := range snap.Windows {
		if _, why := discipline.Capture(snap, w.ID, handsSelf); why != discipline.Accepted {
			continue
		}
		row := protocol.OniHandsWindow{ID: string(w.ID), App: w.App, Title: oniText(strings.Join(strings.Fields(w.Title), " "), protocol.OniHandsTitleMax), Clickable: w.Box != nil}
		if protocol.ValidOniHandsWindowID(row.ID) && protocol.ValidOniHandsApp(row.App) {
			res.Windows = append(res.Windows, row)
		}
	}
	return res, nil
}

// handsSelect picks the live window id names for the next window task. It
// only remembers it: the key reader and the baseline start with the task.
func (d *Daemon) handsSelect(id string) (protocol.OniHandsResult, error) {
	pick, err := d.handsCheckPick(window.ID(id))
	if err != nil {
		return protocol.OniHandsResult{}, err
	}
	return handsWait(d, func() protocol.OniHandsResult {
		d.oni.hands.pick = &pick
		return d.handsResult()
	}), nil
}

// handsCheckPick reads the windows and accepts id only when a task could
// work in it: shown, pinned, not Hyakki's own and not private.
func (d *Daemon) handsCheckPick(id window.ID) (handsPick, error) {
	provider, err := d.oniWindows()
	if err != nil {
		return handsPick{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The windows could not be read: %v.", err))
	}
	ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
	snap, err := provider.Snapshot(ctx)
	cancel()
	if err != nil {
		return handsPick{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The windows could not be read: %v.", err))
	}
	b, why := discipline.Capture(snap, id, handsSelf)
	switch why {
	case discipline.Accepted:
		return handsPick{id: id, app: b.App}, nil
	case discipline.ReasonMissing:
		return handsPick{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That window is gone."))
	case discipline.ReasonSelf:
		return handsPick{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Hands never works in Hyakki's own windows."))
	case discipline.ReasonPrivate:
		return handsPick{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Hands never works in a password manager, a password prompt or a lock screen."))
	case discipline.ReasonHidden:
		return handsPick{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("That window is not shown."))
	}
	return handsPick{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Hands cannot tell that window apart from others, so it cannot work in it."))
}

// handsAppStart is what starting a task outside a tab needs, gathered off
// the queue.
type handsAppStart struct {
	scope, goal, origin string
	task                string // a journal row to start again
	conn                *connection
	grant               func() bool
}

// handsBeginApp starts a task in a picked window or on the nested desktop:
// from the app or the presenter by the button, or from a team's approved
// request. A start while a task runs waits in the journal for its own
// button.
func (d *Daemon) handsBeginApp(s handsAppStart) (protocol.OniHandsResult, error) {
	if line := d.guardOpenRefusal(s.goal); line != "" {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeForbidden, "%s", line)
	}
	settings := protocol.EffectiveOni(d.Settings())
	if err := handsRefusal(settings); err != nil {
		return protocol.OniHandsResult{}, err
	}
	stage := settings.Hands
	if !protocol.OniHandsStageScope(stage, s.scope) || s.scope == protocol.OniHandsScopeBrowser {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeForbidden, "%s", handsLineNoStage)
	}
	h := &d.oni.hands
	h.ops.Lock()
	defer h.ops.Unlock()
	j, err := d.handsJournal()
	if err != nil {
		return protocol.OniHandsResult{}, err
	}
	pick := handsWait(d, func() *handsPick { return h.pick })
	app := ""
	if s.scope == protocol.OniHandsScopeWindow {
		if pick == nil {
			return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeConflict, "%s", handsLineNoPick)
		}
		app = pick.app
	} else if missing := d.handsNestMissing(); len(missing) > 0 {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("Hyakki's separate desktop needs %s, which this computer does not have.", strings.Join(missing, ", ")))
	}
	if h.active.Load() {
		if s.task != "" || s.grant != nil {
			return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeConflict, "%s", handsLineBusy)
		}
		t, err := j.Enqueue(task.Task{ID: task.NewID(), Scope: s.scope, App: app, Origin: s.origin}, d.handsClock().Now())
		if err != nil {
			return protocol.OniHandsResult{}, err
		}
		res := handsWait(d, d.handsResult)
		res.Line = handsLineQueued
		res.Task = t.ID
		return res, nil
	}
	// a drawn presenter is the visible indication and the pill's Stop, a
	// stop path that never needs a voice
	svc := d.oni.service.Load()
	d.oni.clientsMu.Lock()
	drawn := d.oni.presenter != nil
	d.oni.clientsMu.Unlock()
	if svc == nil || !drawn {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeConflict, "%s", handsLineNoDraw)
	}
	if d.deps.Gateway == nil {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", handsLineNoModel)
	}
	live, err := d.oniWindows()
	if err != nil {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The windows could not be read: %v.", err))
	}
	// away is heard from the compositor's events alone; Geometry is never
	// asked
	if !live.Caps().Events {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", handsLineNoEvents)
	}
	ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
	plan, err := d.deps.Gateway.Plan(ctx, oniPlanRequest())
	cancel()
	if err != nil {
		return protocol.OniHandsResult{}, noRouteError(err)
	}
	if s.conn != nil {
		if err := hostedNeedsApp(s.conn, plan.Route()); err != nil {
			return protocol.OniHandsResult{}, err
		}
	}
	// the journal row: a new one, or the waiting or interrupted one the
	// button starts again
	now := d.handsClock().Now()
	var row task.Task
	if s.task != "" {
		row, err = j.Start(s.task, task.ByButton, now)
		if err == nil && (row.Scope != s.scope || s.scope == protocol.OniHandsScopeWindow && row.App != app) {
			_, _ = j.Update(row.ID, task.Stopped, row.Step, loop.EndStopped, now)
			err = protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("That task worked somewhere else. Start a new one there."))
		}
	} else {
		row, err = j.Enqueue(task.Task{ID: task.NewID(), Scope: s.scope, App: app, Origin: s.origin}, now)
		if err == nil && row.State == task.UpNext {
			res := handsWait(d, d.handsResult)
			res.Line, res.Task = handsLineQueued, row.ID
			return res, nil
		}
	}
	if err != nil {
		return protocol.OniHandsResult{}, err
	}
	fail := func(err error) (protocol.OniHandsResult, error) {
		_, _ = j.Update(row.ID, task.Failed, row.Step, loop.EndFailed, d.handsClock().Now())
		return protocol.OniHandsResult{}, err
	}
	turns := d.handsTurnsLeft(row.Step)
	if turns < 1 {
		_, _ = j.Update(row.ID, task.Failed, row.Step, loop.EndBudget, now)
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("That task used all its steps. Start a new one."))
	}
	ex := &handsExec{scope: s.scope, live: live, grant: s.grant}
	// every task waits for the person to turn from Hyakki, where they
	// pressed Start, to the window or the desktop it works on
	ex.ready = make(chan struct{})
	var origin string
	if s.scope == protocol.OniHandsScopeWindow {
		if err := d.handsExecWindow(ex, pick.id); err != nil {
			return fail(err)
		}
		origin = ex.base.App
	} else {
		if err := d.handsExecNested(ex); err != nil {
			return fail(err)
		}
	}
	a := handsWait(d, func() handsAnswer {
		if h.active.Load() {
			return handsAnswer{err: protocol.Errorf(protocol.CodeConflict, "%s", handsLineBusy)}
		}
		if run := h.run; run != nil {
			d.handsDrop(run)
		}
		cfg := handsConfig(os.Getenv)
		cfg.Base = h.token
		cfg.Steps = min(cfg.Steps, turns)
		cfg.Extra = min(cfg.Extra, max(turns-cfg.Steps, 0))
		run := &handsRun{m: loop.New(cfg, d.handsClock().Now), ex: ex, stage: handsStageOf(s.scope), scope: s.scope, task: row.ID, stepBase: row.Step}
		run.state = row.State
		h.run = run
		run.id = row.ID
		run.conn, run.plan, run.goal, run.origin = s.conn, plan, strings.TrimSpace(s.goal), origin
		run.work, run.timers = map[uint64]*handsWork{}, map[uint64]presence.Timer{}
		run.active = true
		h.active.Store(true)
		// Esc stops it; a keyboard that cannot be read leaves the pill
		if stop, err := d.handsStopKeys(func() { d.handsStop(handsByEsc) }); err == nil {
			run.keysStop = stop
		}
		d.handsWatchApp(run)
		d.handsArmTime(run)
		d.handsHandle(run, loop.Start{Goal: run.goal, Origin: origin, Scope: s.scope, By: loop.ByButton})
		res := d.handsResult()
		return handsAnswer{res: res}
	})
	if a.err != nil {
		d.handsCloseExec(ex)
		return fail(a.err)
	}
	d.stateChanged()
	return a.res, nil
}

// handsTurnsLeft is how many steps a task that already took step may still
// take: the spec's task turns, which a restart never refills.
func (d *Daemon) handsTurnsLeft(step int) int {
	turns := 40
	if s, err := oniGuideSpec(); err == nil && s.Hands.Task.Turns > 0 {
		turns = s.Hands.Task.Turns
	}
	return turns - step
}

// handsExecWindow binds ex to the live window id: the driver reads the
// person's windows, takes masked pictures of the window and starts the key
// reader, which pauses the task at any key the person presses.
func (d *Daemon) handsExecWindow(ex *handsExec, id window.ID) error {
	display, err := app.LiveDisplay(os.Getenv)
	if err != nil {
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("This session's display cannot be named, so Hands cannot type into it."))
	}
	drv, err := app.New(app.Config{
		Display: display, Windows: ex.live, Pictures: app.Pictures(d.oniCapture(ex.live)), OCR: d.oniOCR(),
		Masks: d.handsMasks, Keys: d.handsAnyKey, Self: handsSelf, Limits: handsAppLimits(),
		OnPause: func(p discipline.Pause) { d.handsDo(func() { d.handsExecPaused(ex, string(p)) }) },
		Run:     d.deps.OniHands.Keyboard, Dial: d.deps.OniHands.Pointer,
	})
	if err != nil {
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("Hands cannot drive that window: %v.", err))
	}
	ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
	defer cancel()
	b, err := drv.Select(ctx, id)
	if err != nil {
		drv.Close()
		var refused *app.Refused
		switch {
		case errors.As(err, &refused):
			_, perr := d.handsCheckPick(id)
			if perr == nil {
				perr = protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Hands cannot work in that window now."))
			}
			return perr
		case errors.Is(err, app.ErrNoKeys):
			return protocol.Errorf(protocol.CodeUnavailable, "%s", handsLineNoKeys)
		}
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The windows could not be read: %v.", err))
	}
	ex.drv, ex.base = drv, b
	return nil
}

// handsExecNested starts Hyakki's own nested desktop for ex. Its input goes
// to its own display only, its windows are read with its own environment,
// and the person's desktop is only read, for where Hyakki's window is.
func (d *Daemon) handsExecNested(ex *handsExec) error {
	o := nest.Options{Dir: handsNestDir(os.Getenv), Home: handsNestHome()}
	if s, err := oniGuideSpec(); err == nil {
		n := s.Hands.Desktop
		o.Width, o.Height, o.Ready, o.StopGrace = n.WidthPx, n.HeightPx, n.Ready(), n.StopGrace()
	}
	if err := os.MkdirAll(o.Home, 0o700); err != nil {
		return protocol.Errorf(protocol.CodeInternal, "%s", err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), o.Ready+oniAskTimeout)
	defer cancel()
	ws, err := d.handsNestStart(ctx, o)
	if err != nil {
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("Hyakki's separate desktop did not start: %v.", err))
	}
	d.oni.hands.nested.Store(true)
	ex.ws = ws
	ex.nested = d.handsNestWindows(ws.Env())
	ex.roots = d.handsRoots()
	snap, err := ex.nested.Snapshot(ctx)
	if err != nil {
		d.handsCloseExec(ex)
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The separate desktop's windows could not be read: %v.", err))
	}
	desk, why := discipline.CaptureDesktop(snap)
	if why != discipline.Accepted {
		d.handsCloseExec(ex)
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("The separate desktop shows no screen."))
	}
	ex.lastNested = &snap
	if live, err := ex.live.Snapshot(ctx); err == nil {
		ex.lastLive = &live
		if !handsSelfFocused(live) {
			ex.readyOnce.Do(func() { close(ex.ready) })
		}
	}
	ex.desk = desk
	drv, err := app.New(app.Config{
		Display: app.NestDisplay(ws), Windows: ex.nested, Pictures: nestShooter{ws}, OCR: d.oniOCR(),
		// the person's keys never reach the nested desktop, so they race
		// nothing there; its away rule is discipline.Desktop's
		Keys: func(func()) (func(), error) { return func() {}, nil },
		Self: handsSelf, Limits: handsAppLimits(),
		Run: d.deps.OniHands.Keyboard, Dial: d.deps.OniHands.Pointer,
	})
	if err != nil {
		d.handsCloseExec(ex)
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("Hands cannot drive the separate desktop: %v.", err))
	}
	ex.drv = drv
	return nil
}

// nestShooter pictures the whole nested desktop: nothing of Oni or of the
// person's desktop is on it.
type nestShooter struct{ ws OniHandsNest }

func (n nestShooter) Shoot(ctx context.Context, _ window.Window, _ []window.Box) ([]byte, func(), error) {
	png, err := n.ws.Capture(ctx)
	if err != nil {
		return nil, nil, err
	}
	return png, func() { clear(png) }, nil
}

// handsCloseExec ends everything ex started: the key reader, the watches
// and the nested desktop with every program on it.
func (d *Daemon) handsCloseExec(ex *handsExec) {
	if ex == nil || ex.closed.Swap(true) {
		return
	}
	if ex.watchStop != nil {
		ex.watchStop()
	}
	if ex.drv != nil {
		ex.drv.Close()
	}
	if ex.ws != nil {
		ws := ex.ws
		h := &d.oni.hands
		h.nestStops.Add(1)
		go func() {
			ws.Stop()
			h.nested.Store(false)
			h.nestStops.Done()
			d.stateChanged()
		}()
	}
}

// handsWatchApp hears the compositors while the task runs: the person's
// desktop always, the nested one for a nested task. On the queue.
func (d *Daemon) handsWatchApp(run *handsRun) {
	ex := run.ex
	watch, stop := context.WithCancel(context.Background())
	ex.watchStop = stop
	go func() {
		_ = ex.live.Watch(watch, func(s window.Snapshot) {
			d.handsDo(func() { d.handsAppEvent(run, &s, nil) })
		})
	}()
	if ex.nested != nil {
		go func() {
			_ = ex.nested.Watch(watch, func(s window.Snapshot) {
				d.handsDo(func() { d.handsAppEvent(run, nil, &s) })
			})
		}()
		go func() {
			select {
			case <-ex.ws.Done():
				d.handsDo(func() { d.handsStopRun(run, handsByEnded) })
			case <-watch.Done():
			}
		}()
	}
}

// handsArmTime ends the task when the spec's task time is up.
func (d *Daemon) handsArmTime(run *handsRun) {
	s, err := oniGuideSpec()
	if err != nil || s.Hands.Task.Ms <= 0 {
		return
	}
	run.deadline = d.handsClock().AfterFunc(s.Hands.Task.Time(), func() {
		d.handsDo(func() { d.handsStopRun(run, handsByTime) })
	})
}

// handsAppEvent hears one compositor event: a change the rule does not
// allow pauses the task, a closed window ends it. Each event updates the
// latest read of its desktop, and the rule judges both latest reads. Nothing
// acts while a card waits on the person or a pause holds, and every action
// checks afresh before it runs, so those are not interrupted. On the queue.
func (d *Daemon) handsAppEvent(run *handsRun, live, nested *window.Snapshot) {
	if !d.handsLive(run) {
		return
	}
	ex := run.ex
	if live != nil {
		ex.lastLive = live
	}
	if nested != nil {
		ex.lastNested = nested
	}
	if phase := run.m.Phase(); phase == loop.PhaseWaiting || phase == loop.PhasePaused {
		if ex.scope == protocol.OniHandsScopeWindow && live != nil &&
			ex.base.Check(*live, handsSelf, handsTolPx()) == discipline.PauseClosed {
			d.handsHandle(run, loop.Away{Why: string(discipline.PauseClosed)})
		}
		return
	}
	var why string
	switch {
	case ex.scope == protocol.OniHandsScopeWindow:
		if live == nil {
			return
		}
		why = string(ex.base.Check(*live, handsSelf, handsTolPx()))
		if !ex.isReady() {
			// before the window first has focus only its closing counts
			if handsTargetFocused(*live, ex.base.ID) && why == "" {
				// keys pressed to get there do not count
				ex.readyOnce.Do(func() {
					ex.drv.Resume()
					close(ex.ready)
				})
			}
			if why != string(discipline.PauseClosed) {
				return
			}
		}
	default:
		if !ex.isReady() {
			// the person pressed Start in Hyakki: until they first turn
			// away from it, Hyakki's own focus is no away
			if live != nil && !handsSelfFocused(*live) {
				ex.readyOnce.Do(func() { close(ex.ready) })
			}
			return
		}
		if ex.lastLive == nil || ex.lastNested == nil {
			return
		}
		why = string(ex.desk.Check(discipline.Scene{Presenter: d.handsDrawn(), Live: ex.lastLive, Nested: ex.lastNested}, handsSelf))
	}
	if why != "" {
		if (why == string(discipline.PauseSelf) || why == string(discipline.PauseFocus)) &&
			ex.lastLive != nil && d.handsApproving(run, *ex.lastLive) {
			return
		}
		d.handsHandle(run, loop.Away{Why: why})
	}
}

// handsDrawn says whether a presenter draws Oni.
func (d *Daemon) handsDrawn() bool {
	d.oni.clientsMu.Lock()
	defer d.oni.clientsMu.Unlock()
	return d.oni.presenter != nil
}

// handsSelfFocused says one of Hyakki's own windows has the person's focus.
func handsSelfFocused(s window.Snapshot) bool {
	for _, w := range s.Windows {
		if w.Focused && discipline.Self(w, handsSelf) {
			return true
		}
	}
	return false
}

// isReady says the picked window has had the person's focus.
func (ex *handsExec) isReady() bool {
	if ex.ready == nil {
		return true
	}
	select {
	case <-ex.ready:
		return true
	default:
		return false
	}
}

// handsTargetFocused says id is the one focused window of s.
func handsTargetFocused(s window.Snapshot, id window.ID) bool {
	for _, w := range s.Windows {
		if w.ID == id {
			return w.Focused
		}
	}
	return false
}

// handsExecPaused hears the driver's own pause: a key the person pressed,
// or a check that failed around its action. On the queue.
func (d *Daemon) handsExecPaused(ex *handsExec, why string) {
	run := d.oni.hands.run
	if !d.handsLive(run) || run.ex != ex {
		return
	}
	switch run.m.Phase() {
	case loop.PhaseWaiting, loop.PhasePaused:
		return
	}
	if !ex.isReady() {
		return
	}
	d.handsHandle(run, loop.Away{Why: why})
}

// handsPresenterLost follows a presenter that went away: a tab's task
// stops, as Phase 14 does; a task outside a tab pauses, and nothing acts
// until the Resume button.
func (d *Daemon) handsPresenterLost() {
	if !d.oni.hands.active.Load() {
		return
	}
	go d.handsDo(func() {
		run := d.oni.hands.run
		if !d.handsLive(run) {
			return
		}
		if run.ex == nil {
			d.handsStopRun(run, handsByPresenter)
			return
		}
		d.handsHandle(run, loop.Away{Why: string(discipline.PausePresenter)})
	})
}

// handsReadApp reads the window or the nested desktop for the machine: a
// masked picture, the words in it as regions, and the focused window's app
// id and title (matched, never kept).
func (d *Daemon) handsReadApp(run *handsRun, token uint64, ctx context.Context) {
	ex := run.ex
	read := loop.PageRead{Token: token}
	var pic []byte
	var regions []handsRegion
	var focus window.Window
	// the person pressed Start in Hyakki: wait for them to turn to the
	// window or the desktop, at most half the read's time, then look anyway
	if !ex.isReady() {
		t := time.NewTimer(handsFocusWait())
		select {
		case <-ex.ready:
		case <-t.C:
		case <-ctx.Done():
		}
		t.Stop()
	}
	switch ex.scope {
	case protocol.OniHandsScopeWindow:
		f, err := ex.drv.Look(ctx)
		if err != nil {
			if why := handsPauseOf(err); why != "" {
				d.handsDo(func() {
					if d.handsLive(run) && run.finishWork(token) {
						d.handsHandle(run, loop.Away{Why: why})
					}
				})
				return
			}
			read.Err = err
			break
		}
		pic = bytes.Clone(f.PNG)
		f.Clear()
		regions = handsRegions(f.Words, handsMaxRegions())
		read.Origin, read.Title = ex.base.App, ex.base.Title
		read.Private = handsPrivate(f.Words)
	default:
		snap, err := ex.nested.Snapshot(ctx)
		if err != nil {
			read.Err = err
			break
		}
		for _, w := range snap.Windows {
			if w.Focused {
				focus = w
			}
		}
		png, err := ex.ws.Capture(ctx)
		if err != nil {
			read.Err = err
			break
		}
		words, werr := d.oniOCR().Words(ctx, png)
		if werr != nil {
			words = nil
		}
		pic = png
		regions = handsRegions(words, handsMaxRegions())
		read.Origin, read.Title = focus.App, focus.Title
		read.Private = handsPrivate(words) || focus.ID != "" && discipline.Private(focus)
	}
	if read.Err == nil {
		read.Print = handsPrint(regions)
		for _, r := range regions {
			read.Controls = append(read.Controls, loop.Control{ID: r.id, Subject: consent.Subject{Role: "text", Name: r.name}})
		}
	}
	d.handsDo(func() {
		if !d.handsLive(run) || !run.finishWork(token) {
			clear(pic)
			return
		}
		clear(run.pic)
		run.pic, ex.regions, ex.focus = pic, regions, focus
		d.handsHandle(run, read)
	})
}

// handsMaxRegions bounds the regions listed, the spec's page bound.
func handsMaxRegions() int {
	if s, err := oniGuideSpec(); err == nil && s.Hands.Page.MaxControls > 0 {
		return s.Hands.Page.MaxControls
	}
	return 60
}

// handsPrivate says whether the words read show a private field: a
// password, passphrase or one-time code asked for.
func handsPrivate(words []screenask.Word) bool {
	_, refused := screenask.Sensitive("", words, feature.Box{W: 1, H: 1})
	return refused
}

// handsRegions groups the words read into lines, top to bottom, at most max
// of them, each a region the model may name.
func handsRegions(words []screenask.Word, max int) []handsRegion {
	ws := slices.Clone(words)
	slices.SortStableFunc(ws, func(a, b screenask.Word) int {
		return cmp.Or(cmp.Compare(a.Box.Y+a.Box.H/2, b.Box.Y+b.Box.H/2), cmp.Compare(a.Box.X, b.Box.X))
	})
	var lines [][]screenask.Word
	for _, w := range ws {
		if strings.TrimSpace(w.Text) == "" {
			continue
		}
		n := len(lines)
		if n > 0 {
			last := lines[n-1][len(lines[n-1])-1]
			cy, ly := w.Box.Y+w.Box.H/2, last.Box.Y+last.Box.H/2
			if abs(cy-ly) <= max64(last.Box.H, w.Box.H)/2 && w.Box.X-(last.Box.X+last.Box.W) <= 3*max64(last.Box.H, w.Box.H) {
				lines[n-1] = append(lines[n-1], w)
				continue
			}
		}
		lines = append(lines, []screenask.Word{w})
	}
	var out []handsRegion
	for i, line := range lines {
		if i >= max {
			break
		}
		box := line[0].Box
		texts := make([]string, 0, len(line))
		for _, w := range line {
			x1, y1 := max64(box.X+box.W, w.Box.X+w.Box.W), max64(box.Y+box.H, w.Box.Y+w.Box.H)
			box.X, box.Y = min(box.X, w.Box.X), min(box.Y, w.Box.Y)
			box.W, box.H = x1-box.X, y1-box.Y
			texts = append(texts, w.Text)
		}
		out = append(out, handsRegion{id: "r" + strconv.Itoa(i+1), name: strings.Join(texts, " "), box: box})
	}
	return out
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func max64(a, b float64) float64 { return max(a, b) }

// handsPrint fingerprints what the screen shows by its words: a blinking
// caret or a moving clock hand is no change.
func handsPrint(regions []handsRegion) string {
	h := sha256.New()
	for _, r := range regions {
		h.Write([]byte(r.name))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)[:8])
}

// handsPauseOf names the pause in an error from the driver, or "".
func handsPauseOf(err error) string {
	var p *app.Paused
	if errors.As(err, &p) {
		return string(p.Reason)
	}
	return ""
}

// handsVerifyApp checks the window, or the nested desktop, before or after
// one action, and answers the machine. A team's task whose approval no
// longer holds ends here.
func (d *Daemon) handsVerifyApp(run *handsRun, e loop.Verify, ctx context.Context) {
	ex := run.ex
	if ex.grant != nil && !ex.grant() {
		d.handsDo(func() {
			if d.handsLive(run) && run.finishWork(e.Token) {
				d.handsStopRun(run, handsByGrant)
			}
		})
		return
	}
	why, err := d.handsCheck(ex, ctx)
	if err != nil {
		why = string(discipline.PauseWindow)
	}
	d.handsDo(func() {
		if !d.handsLive(run) || !run.finishWork(e.Token) {
			return
		}
		outcome, reason := protocol.OniHandsOutcomeDone, ""
		if why != "" {
			outcome, reason = protocol.OniHandsOutcomeRefused, why
		}
		d.handsRow(run, handsToolCheck, outcome, reason, 0)
		d.handsHandle(run, loop.Verified{Token: e.Token, Pause: why})
	})
}

// handsCheck is the rule's verdict on fresh reads: "" when it holds.
func (d *Daemon) handsCheck(ex *handsExec, ctx context.Context) (string, error) {
	live, err := ex.live.Snapshot(ctx)
	if err != nil {
		return "", err
	}
	if ex.scope == protocol.OniHandsScopeWindow {
		if why := ex.base.Check(live, handsSelf, handsTolPx()); why != discipline.OK {
			return string(why), nil
		}
		if !d.handsDrawn() {
			return string(discipline.PausePresenter), nil
		}
		return string(ex.drv.Paused()), nil
	}
	nested, err := ex.nested.Snapshot(ctx)
	if err != nil {
		return "", err
	}
	return string(ex.desk.Check(discipline.Scene{Presenter: d.handsDrawn(), Live: &live, Nested: &nested}, handsSelf)), nil
}

// handsActApp runs one safe step outside a tab, through the driver.
func (d *Daemon) handsActApp(run *handsRun, e loop.Act, ctx context.Context, regions []handsRegion, files []string, focus window.Window) {
	ex := run.ex
	r, found, paths := d.handsDoApp(ex, e, ctx, regions, files, focus)
	d.handsDo(func() {
		if !d.handsLive(run) {
			return
		}
		start := run.work[e.Token]
		if !run.finishWork(e.Token) {
			return
		}
		var ms int64
		if start != nil {
			ms = d.handsClock().Now().Sub(start.at).Milliseconds()
		}
		outcome, reason := protocol.OniHandsOutcomeDone, ""
		switch {
		case r.Refused:
			outcome, reason = protocol.OniHandsOutcomeRefused, r.Err
		case r.Err != "":
			outcome, reason = protocol.OniHandsOutcomeFailed, r.Err
		}
		if e.Tool == proto.ToolFindFiles && r.Err == "" {
			ex.files = paths
			r.Found = found
		}
		d.handsRow(run, e.Tool, outcome, reason, ms)
		d.handsHandle(run, loop.Acted{Token: e.Token, Result: r})
	})
}

// handsDoApp carries out e and says what came of it: Err a code, never a
// word typed or read.
func (d *Daemon) handsDoApp(ex *handsExec, e loop.Act, ctx context.Context, regions []handsRegion, files []string, focus window.Window) (loop.Result, int, []string) {
	refused := func(why string) (loop.Result, int, []string) { return loop.Result{Refused: true, Err: why}, 0, nil }
	failed := func(err error) (loop.Result, int, []string) {
		var ref *app.Refused
		switch {
		case errors.As(err, &ref):
			return refused(ref.Why)
		case handsPauseOf(err) != "":
			return loop.Result{Err: "paused"}, 0, nil
		case errors.Is(err, context.DeadlineExceeded):
			return loop.Result{Err: "timeout"}, 0, nil
		case errors.Is(err, context.Canceled):
			return loop.Result{Err: "cancelled"}, 0, nil
		}
		return loop.Result{Err: "error"}, 0, nil
	}
	nested := ex.scope != protocol.OniHandsScopeWindow
	// on the nested desktop the step goes to the window that has focus,
	// bound afresh with its own checks
	bind := func() error {
		if !nested {
			return nil
		}
		if focus.ID == "" {
			return &app.Refused{Why: "no-window"}
		}
		_, err := ex.drv.Select(ctx, focus.ID)
		return err
	}
	a := e.Args
	switch e.Tool {
	case proto.ToolWait:
		t := time.NewTimer(time.Duration(max(a.Ms, 0)) * time.Millisecond)
		defer t.Stop()
		select {
		case <-t.C:
		case <-ctx.Done():
			return failed(ctx.Err())
		}
		return loop.Result{}, 0, nil
	case proto.ToolType:
		if err := bind(); err != nil {
			return failed(err)
		}
		typed, err := ex.drv.Type(ctx, a.Text)
		if err != nil {
			return failed(err)
		}
		if typed.Rest != "" {
			return loop.Result{Err: "partial"}, 0, nil
		}
		return loop.Result{}, 0, nil
	case proto.ToolPress:
		if err := bind(); err != nil {
			return failed(err)
		}
		if err := ex.drv.Press(ctx, a.Keys); err != nil {
			return failed(err)
		}
		return loop.Result{}, 0, nil
	case proto.ToolScroll:
		if err := bind(); err != nil {
			return failed(err)
		}
		key := "Page_Down"
		if a.Dir == proto.DirUp {
			key = "Page_Up"
		}
		if err := ex.drv.Press(ctx, key); err != nil {
			return failed(err)
		}
		return loop.Result{}, 0, nil
	case proto.ToolClick:
		i := slices.IndexFunc(regions, func(r handsRegion) bool { return r.id == a.ID })
		if i < 0 {
			return refused("region")
		}
		box := regions[i].box
		fx, fy := box.X+box.W/2, box.Y+box.H/2
		if nested {
			// the picture is the whole nested output; the click lands in
			// the focused window, in fractions of its own box
			if focus.Box == nil {
				return refused("unplaced")
			}
			w, h := ex.ws.Size()
			b := focus.Box
			fx, fy = (fx*float64(w)-b.X)/b.W, (fy*float64(h)-b.Y)/b.H
			if !(fx >= 0 && fx < 1 && fy >= 0 && fy < 1) {
				return refused("region")
			}
			if err := bind(); err != nil {
				return failed(err)
			}
		}
		if err := ex.drv.Click(ctx, fx, fy); err != nil {
			return failed(err)
		}
		return loop.Result{}, 0, nil
	case proto.ToolOpenApp:
		if line := d.guardOpenRefusal(a.Name); line != "" {
			d.guardSay(line)
			return refused("focus-guard")
		}
		if !nested {
			return refused("scope")
		}
		entry, ok := d.handsCatalog().Lookup(a.Name)
		if !ok {
			return refused("no-app")
		}
		if err := ex.ws.Open(entry.Argv()); err != nil {
			return loop.Result{Err: "open"}, 0, nil
		}
		return loop.Result{}, 0, nil
	case proto.ToolFindFiles:
		if !nested {
			return refused("scope")
		}
		if err := apprules.Query(a.Query); err != nil {
			return refused("query")
		}
		paths := handsFind(ctx, ex.roots, a.Query)
		return loop.Result{}, len(paths), paths
	case proto.ToolShowFile:
		if !nested {
			return refused("scope")
		}
		if a.Index < 1 || a.Index > len(files) {
			return refused("index")
		}
		dir, err := handsShowDir(files[a.Index-1], ex.roots)
		if err != nil {
			return refused("path")
		}
		fm, ok := handsFileManager(d.handsCatalog())
		if !ok {
			return refused("no-file-manager")
		}
		// the folder alone, never the file: an executable is shown, not run
		if err := ex.ws.Open(fm.WithURL(dir)); err != nil {
			return loop.Result{Err: "open"}, 0, nil
		}
		return loop.Result{}, 0, nil
	}
	return refused("tool")
}

// handsCatalog is the launcher catalog the nested desktop opens apps from,
// whether or not reflexes are on.
func (d *Daemon) handsCatalog() *launch.Catalog {
	dirs := d.deps.OniHands.AppDirs
	if dirs == nil {
		dirs = launch.Dirs(os.Environ())
	}
	var desktops []string
	if v := os.Getenv("XDG_CURRENT_DESKTOP"); v != "" {
		desktops = strings.Split(v, ":")
	}
	return launch.Scan(launch.ScanOptions{Dirs: dirs, Max: reflexLimits().CatalogMax, Desktops: desktops, LookPath: exec.LookPath})
}

// handsFileManager is the catalog's file manager: an entry that opens
// folders and calls itself a file manager.
func handsFileManager(cat *launch.Catalog) (launch.Entry, bool) {
	for _, e := range cat.Entries() {
		if slices.Contains(e.MimeTypes, "inode/directory") && slices.Contains(e.Categories, "FileManager") {
			return e, true
		}
	}
	return launch.Entry{}, false
}

// handsFind lists at most handsFindMax regular files under roots whose name
// holds query, ignoring case; hidden folders are not searched.
func handsFind(ctx context.Context, roots []string, query string) []string {
	ctx, cancel := context.WithTimeout(ctx, handsFindTime)
	defer cancel()
	q := strings.ToLower(strings.TrimSpace(query))
	var found []string
	stop := errors.New("enough")
	for _, root := range roots {
		depth0 := strings.Count(filepath.Clean(root), string(filepath.Separator))
		err := filepath.WalkDir(root, func(p string, de fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if ctx.Err() != nil || len(found) >= handsFindMax {
				return stop
			}
			if de.IsDir() {
				if p != root && (strings.HasPrefix(de.Name(), ".") || strings.Count(p, string(filepath.Separator))-depth0 >= handsFindDepth) {
					return filepath.SkipDir
				}
				return nil
			}
			if de.Type().IsRegular() && strings.Contains(strings.ToLower(de.Name()), q) {
				found = append(found, p)
			}
			return nil
		})
		if errors.Is(err, stop) {
			break
		}
	}
	return found
}

// handsShowDir is the folder a found file is shown in, by the app rules:
// within the roots, never the file itself.
func handsShowDir(path string, roots []string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	f := apprules.File{Path: path, Mode: info.Mode()}
	if info.Mode().IsRegular() {
		if fh, err := os.Open(path); err == nil {
			head := make([]byte, 64)
			n, _ := fh.Read(head)
			fh.Close()
			f.Head = head[:n]
		}
	}
	return apprules.Show(f, roots)
}

// handsReadBackApp looks for what one type action typed, and answers the
// machine; the words read never leave the driver.
func (d *Daemon) handsReadBackApp(run *handsRun, e loop.ReadBack, ctx context.Context, focus window.Window) {
	ex := run.ex
	seen := loop.SeenUnknown
	var why string
	var err error
	if ex.scope != protocol.OniHandsScopeWindow && focus.ID != "" {
		_, err = ex.drv.Select(ctx, focus.ID)
	}
	var rb app.Readback
	if err == nil {
		rb, err = ex.drv.ReadBack(ctx, e.Text)
	}
	switch {
	case handsPauseOf(err) != "":
		why = handsPauseOf(err)
	case rb == app.Yes:
		seen = loop.SeenYes
	case rb == app.No:
		seen = loop.SeenNo
	}
	d.handsDo(func() {
		if !d.handsLive(run) || !run.finishWork(e.Token) {
			return
		}
		if why != "" {
			d.handsHandle(run, loop.Away{Why: why})
			return
		}
		outcome, reason := protocol.OniHandsOutcomeDone, ""
		switch seen {
		case loop.SeenNo:
			outcome, reason = protocol.OniHandsOutcomeFailed, "no"
		case loop.SeenUnknown:
			outcome, reason = protocol.OniHandsOutcomeFailed, "unknown"
		}
		d.handsRow(run, handsToolReadBack, outcome, reason, 0)
		d.handsHandle(run, loop.ReadBackResult{Token: e.Token, Seen: seen})
	})
}

// handsAppInput is one model request's input outside a tab.
func (d *Daemon) handsAppInput(run *handsRun, e loop.Ask) proto.Input {
	ex := run.ex
	in := proto.Input{Scope: run.scope, Goal: e.Goal, Origin: run.origin, History: e.History}
	if ex.scope == protocol.OniHandsScopeWindow {
		in.Title = ex.base.Title
	} else {
		in.Origin, in.Title = ex.focus.App, ex.focus.Title
	}
	for _, r := range ex.regions {
		in.Controls = append(in.Controls, proto.Control{ID: r.id, Role: "text", Name: r.name})
	}
	for _, p := range ex.files {
		in.Files = append(in.Files, filepath.Base(p))
	}
	if s, err := oniGuideSpec(); err == nil {
		h := s.Hands
		in.Bounds = proto.Bounds{Controls: h.Page.MaxControls, NameRunes: h.Page.MaxNameRunes, TextRunes: h.Page.MaxTextRunes, HistoryLines: h.HistoryCarry}
	}
	return in
}

// handsAppDisclosure is what a step's picture shows outside a tab.
func handsAppDisclosure(scope string) capture.Disclosure {
	if scope == protocol.OniHandsScopeWindow {
		return handsDisclosWindow
	}
	return handsDisclosDesktop
}

// handsFocusWait is how long the first look waits for the picked window to
// get the person's focus: half a page read's time.
func handsFocusWait() time.Duration {
	return handsConfig(os.Getenv).PageTimeout / 2
}
