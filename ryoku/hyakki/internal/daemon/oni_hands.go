package daemon

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"image"
	"io"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/hands/chrome"
	"ryoku-rashin/internal/hands/page"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/oni/capture"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/hands/consent"
	"ryoku-rashin/internal/oni/hands/loop"
	"ryoku-rashin/internal/oni/hands/proto"
	"ryoku-rashin/internal/oni/hands/task"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/oni/voice/wake"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/protocol"
)

// Hands works in one browser tab the person authorized, in a browser
// Hyakki started on its own profile and drives over a pipe. loop.Machine
// decides every move; this file carries out what it asks and hands back
// what came of it. A safe step (an ordinary fill, click, choice, scroll or
// wait) is announced and run; anything risky is handed to the person, who
// presses it on the page themselves, and Hands learns of that press only
// from the page's own trusted events. Nothing anyone says or answers makes
// Hands press it.
//
// Every Hands event runs on one serial queue (handsDo), so the machine sees
// one event at a time. The page, the model and the window watch are never
// called on the queue: their answers are posted back to it. Nothing of
// Hands runs while it is off: no browser, no timer, no key reader and no
// window watch. While the Hands browser is open the daemon stays up; while
// a task runs the stop key is read and the windows are watched for focus.

// oniHandsState is the Hands browser, its authorized tab and the task.
type oniHandsState struct {
	mu       sync.Mutex
	queue    []func()
	draining bool

	// ops serialises opening, authorizing, closing and forgetting; never
	// taken on the queue
	ops sync.Mutex

	// Desktop teardown stays off the queue, but shutdown must join it.
	nestStops sync.WaitGroup

	// open: the Hands browser runs; card: a task waits on a button (a
	// handoff card or a step's picture question); active: a task runs;
	// nested: the nested desktop runs
	open   atomic.Bool
	card   atomic.Bool
	active atomic.Bool
	nested atomic.Bool

	// queue only
	browser *handsBrowser
	run     *handsRun
	token   uint64
	tasks   uint64
	// pick is the live window the app picked for the next window task
	pick *handsPick

	journalMu sync.Mutex
	journal   *task.Journal

	rowsMu sync.Mutex
	rows   []protocol.OniHandsRow
}

// handsBrowser is one open Hands browser and the tab it authorized.
type handsBrowser struct {
	b       OniHandsBrowser
	drv     page.Driver
	class   string
	program string
	scope   *page.Scope
}

// handsRun is one task, from its start until its follow-up window ends.
type handsRun struct {
	m      *loop.Machine
	id     string
	hb     *handsBrowser
	conn   *connection
	plan   llm.Plan
	goal   string
	origin string
	active bool

	// stage and scope the task works at; ex carries out a task outside a
	// tab, nil in a tab
	stage, scope string
	ex           *handsExec

	// the journal row of a task outside a tab: its id, the steps it took
	// before a restart, and what was last written
	task     string
	stepBase int
	state    task.State
	written  int
	reason   string
	deadline presence.Timer

	// the last reading: its picture, controls and text, held in memory
	// until the next one or the end; pic is the picture outside a tab
	snap page.Snapshot
	pic  []byte
	// what the last parsed reply asked for, for the ledger
	tool string

	work   map[uint64]*handsWork
	timers map[uint64]presence.Timer
	parked uint64
	asked  string // the step's picture question waiting, by id

	card      string
	cardCtrl  string
	cardLine  string
	cardUntil time.Time
	cardSent  bool
	awaitStop context.CancelFunc

	frame feature.HandsFrame
	line  string

	keysStop  func()
	watchStop context.CancelFunc
	window    window.ID
	focus     window.ID

	stopAt  time.Time
	stopWhy string
}

type handsWork struct {
	cancel context.CancelFunc
	at     time.Time
}

// handsQuestion ties a step's picture to the request that wants it.
type handsQuestion struct {
	run     *handsRun
	token   uint64
	in      proto.Input
	correct string
	ctx     context.Context
}

// How a Hands task was stopped, as the ledger names it.
const (
	handsByApp       = "app"
	handsByCLI       = "cli"
	handsByPill      = "pill"
	handsByEsc       = "esc"
	handsByVoice     = "voice"
	handsBySettings  = "settings"
	handsByPresenter = "presenter"
	handsByClosed    = "closed"
	handsByDeclined  = "declined"
	handsByConsent   = "consent"
	handsByShutdown  = "shutdown"
)

// The ledger's own tool words besides the page's.
const (
	handsToolReply = "reply"
	handsToolUser  = "user"
	handsToolStop  = "stop"
	handsToolEnd   = "end"
)

// handsScroll is how far one scroll step moves, in CSS pixels.
const handsScroll = 600

// handsAnchors are the words that send a spoken press to Hands.
var handsAnchors = []string{"on this page", "on the page", "in the browser", "in this tab", "in the tab", "on this site"}

// handsResumes are spoken goings-on Hands hears while paused and never
// takes as its Resume button.
var handsResumes = []string{"keep going", "go on", "continue", "resume", "carry on"}

// The lines Hands' calls answer with.
var (
	handsDisclosure = capture.Disclosure{Path: "hands-tab", ScopeExact: true,
		Text: i18n.T("A picture of the authorized tab's visible part, taken by the browser itself, with the page's text and the names of its controls.")}
	handsLineNoWindow  = i18n.T("I can't tell which window is the Hands browser.")
	handsLineNoEvents  = i18n.T("This desktop does not report window focus, so Hands can't tell when you look away.")
	handsLineNoDraw    = i18n.T("Show Oni first: Hands works only where you can see it and stop it.")
	handsLineNoTab     = i18n.T("Authorize a tab first.")
	handsLineNoOpen    = i18n.T("Open the Hands browser first.")
	handsLineBusy      = i18n.T("Hands is busy. Stop it first.")
	handsLineNoModel   = i18n.T("No model is set up to look at pictures.")
	handsLineOpenNow   = i18n.T("Close the Hands browser first.")
	handsLineNoPause   = i18n.T("Hands is not paused.")
	handsLineForgotten = i18n.T("The Hands browser is forgotten.")
)

var handsStartURL = regexp.MustCompile(`^http://127\.0\.0\.1:[0-9]{1,5}/[!-~]*$`)

// handsDo runs f on the Hands queue: at once when nothing drains it, else
// after what is queued, on the goroutine draining it.
func (d *Daemon) handsDo(f func()) {
	h := &d.oni.hands
	h.mu.Lock()
	h.queue = append(h.queue, f)
	if h.draining {
		h.mu.Unlock()
		return
	}
	h.draining = true
	for len(h.queue) > 0 {
		next := h.queue[0]
		h.queue[0] = nil
		h.queue = h.queue[1:]
		h.mu.Unlock()
		next()
		h.mu.Lock()
	}
	h.queue = nil
	h.draining = false
	h.mu.Unlock()
}

// handsWait runs f on the queue and waits for what it returns.
func handsWait[T any](d *Daemon, f func() T) T {
	ch := make(chan T, 1)
	d.handsDo(func() { ch <- f() })
	return <-ch
}

// handsResident reports whether Hands keeps the daemon up: the Hands
// browser is open, a task runs, or the nested desktop runs.
func (d *Daemon) handsResident() bool {
	h := &d.oni.hands
	return h.open.Load() || h.active.Load() || h.nested.Load()
}

// handsLive says run is the task that runs. On the queue.
func (d *Daemon) handsLive(run *handsRun) bool {
	return run != nil && d.oni.hands.run == run && run.active
}

// handsDecide is the decision of the Hands row of the stored stage; off
// and no stage are decided as the browser's, which is then off.
func handsDecide(settings protocol.OniSettings) feature.Decision {
	id := feature.HandsStageFeature(settings.Hands)
	if id == "" {
		id = feature.HandsBrowser
	}
	return feature.Decide(feature.Request{Feature: id}, settings)
}

// handsRefusal says why Hands may not work now, or nil.
func handsRefusal(settings protocol.OniSettings) error {
	decision := handsDecide(settings)
	if decision.Allow {
		return nil
	}
	message := i18n.T("Hands is off. Turn it and looking at windows on in Oni's settings.")
	if decision.Reason == feature.ReasonOniOff {
		message = i18n.T("Oni is off.")
	}
	return &protocol.Error{Code: protocol.CodeDisabled, Message: message, Reason: decision.Reason}
}

// oniHandsCall answers oni.hands. The gate has already kept each action to
// its callers.
func (d *Daemon) oniHandsCall(c *connection, p protocol.OniHandsParams) (protocol.OniHandsResult, error) {
	settings := protocol.EffectiveOni(d.Settings())
	stage := settings.Hands
	refused := handsRefusal(settings)
	var (
		res protocol.OniHandsResult
		err error
	)
	switch p.Action {
	case protocol.OniHandsStop:
		why := handsByCLI
		switch {
		case c.app:
			why = handsByApp
		case d.oniIsPresenter(c):
			why = handsByPill
		}
		res = handsWait(d, func() protocol.OniHandsResult {
			d.handsStopRun(d.oni.hands.run, why)
			return d.handsResult()
		})
	case protocol.OniHandsForget:
		res, err = d.handsForget()
	case protocol.OniHandsTasks:
		res, err = d.handsTasksCall()
	case protocol.OniHandsClear:
		res, err = d.handsClearCall()
	default:
		if refused != nil {
			return protocol.OniHandsResult{}, refused
		}
		res, err = d.handsAction(c, p, stage)
	}
	if err != nil {
		return protocol.OniHandsResult{}, err
	}
	switch {
	case refused != nil:
	case stage == protocol.OniHandsBrowser:
		if res.Browser == "" {
			res.Browser = d.handsBrowserName()
		}
	case stage == protocol.OniHandsApp || stage == protocol.OniHandsDesktop:
		res.Missing = d.handsNestMissing()
		if stage == protocol.OniHandsDesktop && len(res.Missing) > 0 && res.State == protocol.OniHandsStateAuthorized {
			res.State = protocol.OniHandsStateClosed
		}
	}
	return res, nil
}

// handsAction answers the actions that need Hands on, each at its own stage.
func (d *Daemon) handsAction(c *connection, p protocol.OniHandsParams, stage string) (protocol.OniHandsResult, error) {
	browser := stage == protocol.OniHandsBrowser
	switch p.Action {
	case protocol.OniHandsOpen, protocol.OniHandsTabs, protocol.OniHandsAuthorize:
		if !browser {
			return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeConflict, "%s", handsLineNoBrowser)
		}
		switch p.Action {
		case protocol.OniHandsOpen:
			return d.handsOpen()
		case protocol.OniHandsTabs:
			return d.handsTabs()
		}
		return d.handsAuthorize(p.Tab)
	case protocol.OniHandsWindows, protocol.OniHandsSelect:
		if stage != protocol.OniHandsApp {
			return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeConflict, "%s", handsLineNoStage)
		}
		if p.Action == protocol.OniHandsWindows {
			return d.handsWindowsCall()
		}
		return d.handsSelect(p.Window)
	case protocol.OniHandsStart:
		scope := handsScopeOf(stage, p.Scope)
		if browser && scope == protocol.OniHandsScopeBrowser && p.Task == "" {
			return d.handsBegin(c, p.Goal, loop.ByButton, false)
		}
		origin := task.OriginApp
		if !c.app {
			origin = task.OriginPresenter
		}
		return d.handsBeginApp(handsAppStart{scope: scope, goal: p.Goal, origin: origin, task: p.Task, conn: c})
	case protocol.OniHandsResume:
		return d.handsResume()
	}
	return handsWait(d, d.handsResult), nil
}

// handsBrowserName is the open browser's code name, or the one an open
// would start; empty when none can be.
func (d *Daemon) handsBrowserName() string {
	if name := handsWait(d, func() string {
		if hb := d.oni.hands.browser; hb != nil {
			return hb.program
		}
		return ""
	}); name != "" {
		return name
	}
	prog, err := d.handsFind()
	if err != nil {
		return ""
	}
	return prog.Name
}

// handsResult is where Hands stands. On the queue.
func (d *Daemon) handsResult() protocol.OniHandsResult {
	h := &d.oni.hands
	res := protocol.OniHandsResult{State: protocol.OniHandsStateClosed}
	switch protocol.EffectiveOni(d.Settings()).Hands {
	case protocol.OniHandsApp:
		res.State = protocol.OniHandsStateOpen
		if h.pick != nil {
			res.State, res.App = protocol.OniHandsStateAuthorized, h.pick.app
		}
	case protocol.OniHandsDesktop:
		res.State = protocol.OniHandsStateAuthorized
	}
	if hb := h.browser; hb != nil {
		res = protocol.OniHandsResult{State: protocol.OniHandsStateOpen, Browser: hb.program}
		if hb.scope != nil {
			res.State, res.Origin = protocol.OniHandsStateAuthorized, hb.scope.Origin
		}
	}
	if run := h.run; run != nil {
		if run.active {
			res.State, res.Step, res.Max, res.Pause = run.frame.Phase, run.frame.Step, run.frame.Max, run.frame.Pause
			if run.ex != nil {
				res.Scope, res.Task = run.scope, run.id
				if run.ex.scope == protocol.OniHandsScopeWindow {
					res.App = run.ex.base.App
				}
			}
		}
		res.Line = oniText(run.line, oniBubbleMax)
	}
	return res
}

// handsOpen starts the Hands browser, or answers with the one open.
func (d *Daemon) handsOpen() (protocol.OniHandsResult, error) {
	h := &d.oni.hands
	h.ops.Lock()
	defer h.ops.Unlock()
	if handsWait(d, func() bool { return h.browser != nil }) {
		return handsWait(d, d.handsResult), nil
	}
	prog, err := d.handsFind()
	if err != nil {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", err.Error())
	}
	class, err := handsClass()
	if err != nil {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeInternal, "%s", err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
	defer cancel()
	b, err := d.handsLaunch(ctx, chrome.Options{Exec: prog.Exec, ProfileDir: handsProfile(), Class: class, StartURL: handsStart(os.Getenv), Env: reflexEnv(os.Environ())})
	if err != nil {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The Hands browser did not start: %v.", err))
	}
	hb := &handsBrowser{b: b, drv: b.Driver(), class: class, program: prog.Name}
	h.open.Store(true)
	d.handsDo(func() { h.browser = hb })
	go func() {
		for ev := range hb.drv.Events() {
			d.handsDo(func() { d.handsPageEvent(hb, ev) })
		}
	}()
	go func() {
		<-b.Done()
		d.handsDo(func() { d.handsBrowserGone(hb) })
	}()
	d.stateChanged()
	return handsWait(d, d.handsResult), nil
}

// handsClass is a fresh window class for one Hands browser.
func handsClass() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "hyakki-hands-" + hex.EncodeToString(b[:]), nil
}

// handsStart is the address the Hands browser opens: a blank page, or in
// the Oni lab only the loopback page HYAKKI_ONI_HANDS_START_URL names.
func handsStart(getenv func(string) string) string {
	if getenv("HYAKKI_ONI_LAB") == "1" {
		if u := getenv("HYAKKI_ONI_HANDS_START_URL"); handsStartURL.MatchString(u) {
			return u
		}
	}
	return "about:blank"
}

// handsCurrent is the open browser, on the queue.
func (d *Daemon) handsCurrent() *handsBrowser {
	return handsWait(d, func() *handsBrowser { return d.oni.hands.browser })
}

// handsTabs lists the Hands browser's tabs.
func (d *Daemon) handsTabs() (protocol.OniHandsResult, error) {
	hb := d.handsCurrent()
	if hb == nil {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeConflict, "%s", handsLineNoOpen)
	}
	ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
	defer cancel()
	tabs, err := hb.drv.Tabs(ctx)
	if err != nil {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The Hands browser's tabs could not be read: %v.", err))
	}
	res := handsWait(d, d.handsResult)
	res.Tabs = []protocol.OniHandsTab{}
	for _, t := range tabs {
		tab := protocol.OniHandsTab{ID: t.ID, Origin: t.Origin, Title: oniText(strings.Join(strings.Fields(t.Title), " "), protocol.OniHandsTitleMax)}
		if protocol.ValidOniHandsTab(tab) {
			res.Tabs = append(res.Tabs, tab)
		}
	}
	return res, nil
}

// handsAuthorize binds the tab id names, by its target and its origin.
func (d *Daemon) handsAuthorize(id string) (protocol.OniHandsResult, error) {
	h := &d.oni.hands
	h.ops.Lock()
	defer h.ops.Unlock()
	hb := d.handsCurrent()
	if hb == nil {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeConflict, "%s", handsLineNoOpen)
	}
	if d.oni.hands.active.Load() {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeConflict, "%s", handsLineBusy)
	}
	ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
	defer cancel()
	scope, err := hb.drv.Authorize(ctx, id)
	if err != nil {
		code := protocol.CodeUnavailable
		if errors.Is(err, page.ErrUnknownTab) {
			code = protocol.CodeNotFound
		}
		return protocol.OniHandsResult{}, protocol.Errorf(code, "%s", i18n.Tf("That tab could not be authorized: %v.", err))
	}
	return handsWait(d, func() protocol.OniHandsResult {
		if d.oni.hands.browser == hb {
			hb.scope = &scope
			if d.guardResident() {
				go d.guardBrowser(0)
			}
		}
		return d.handsResult()
	}), nil
}

// handsBegin starts a task toward goal in the authorized tab: from the app
// or the presenter (c, by the button), or from a spoken press (c nil).
// Only an unanchored spoken follow-up continues the prior task's history.
func (d *Daemon) handsBegin(c *connection, goal, by string, follow bool) (protocol.OniHandsResult, error) {
	if line := d.guardOpenRefusal(goal); line != "" {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeForbidden, "%s", line)
	}
	settings := protocol.EffectiveOni(d.Settings())
	if err := handsRefusal(settings); err != nil {
		return protocol.OniHandsResult{}, err
	}
	hb := d.handsCurrent()
	switch {
	case hb == nil:
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeConflict, "%s", handsLineNoOpen)
	case hb.scope == nil:
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeConflict, "%s", handsLineNoTab)
	case d.oni.hands.active.Load():
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeConflict, "%s", handsLineBusy)
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
	provider, err := d.oniWindows()
	if err != nil {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The windows could not be read: %v.", err))
	}
	// away is heard from the compositor's events alone; a desktop that
	// would have to be polled is refused, and Geometry is never asked
	if !provider.Caps().Events {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", handsLineNoEvents)
	}
	ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
	snap, err := provider.Snapshot(ctx)
	cancel()
	if err != nil {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The windows could not be read: %v.", err))
	}
	win, ok := handsWindow(snap, hb.b.PID(), hb.class)
	if !ok {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeConflict, "%s", handsLineNoWindow)
	}
	ctx, cancel = context.WithTimeout(context.Background(), oniAskTimeout)
	plan, err := d.deps.Gateway.Plan(ctx, oniPlanRequest())
	cancel()
	if err != nil {
		return protocol.OniHandsResult{}, noRouteError(err)
	}
	if c != nil {
		if err := hostedNeedsApp(c, plan.Route()); err != nil {
			return protocol.OniHandsResult{}, err
		}
	}
	a := handsWait(d, func() handsAnswer {
		h := &d.oni.hands
		if h.browser != hb || hb.scope == nil || h.active.Load() {
			return handsAnswer{err: protocol.Errorf(protocol.CodeConflict, "%s", handsLineBusy)}
		}
		run := h.run
		if !follow || run == nil || run.m.Phase() != loop.PhaseFollowing || run.hb != hb {
			if run != nil {
				d.handsDrop(run)
			}
			cfg := handsConfig(os.Getenv)
			cfg.Base = h.token
			run = &handsRun{m: loop.New(cfg, d.handsClock().Now), hb: hb, stage: protocol.OniHandsBrowser, scope: protocol.OniHandsScopeBrowser}
			h.run = run
		}
		h.tasks++
		run.id = "t" + strconv.FormatUint(h.tasks, 10)
		run.conn, run.plan, run.goal, run.origin = c, plan, strings.TrimSpace(goal), hb.scope.Origin
		for token, t := range run.timers {
			t.Stop()
			delete(run.timers, token)
		}
		run.work, run.timers = map[uint64]*handsWork{}, map[uint64]presence.Timer{}
		run.window, run.focus, run.stopAt, run.stopWhy, run.line = win.ID, handsFocused(snap), time.Time{}, "", ""
		run.active = true
		h.active.Store(true)
		// Esc stops it; a keyboard that cannot be read leaves the pill
		if stop, err := d.handsStopKeys(func() { d.handsStop(handsByEsc) }); err == nil {
			run.keysStop = stop
		}
		watch, stopWatch := context.WithCancel(context.Background())
		run.watchStop = stopWatch
		go func() {
			_ = provider.Watch(watch, func(s window.Snapshot) {
				d.handsDo(func() { d.handsWindows(run, s) })
			})
		}()
		d.handsHandle(run, loop.Start{Goal: run.goal, Origin: run.origin, By: by})
		return handsAnswer{res: d.handsResult()}
	})
	return a.res, a.err
}

// handsConfig is the machine's numbers from the spec, with the card and
// resume windows the Oni lab may shorten.
func handsConfig(getenv func(string) string) loop.Config {
	var c loop.Config
	if s, err := oniGuideSpec(); err == nil {
		c = loop.FromSpec(s)
	}
	if getenv("HYAKKI_ONI_LAB") != "1" {
		return c
	}
	for _, o := range []struct {
		key string
		to  *time.Duration
	}{{"HYAKKI_ONI_HANDS_CARD_MS", &c.CardTTL}, {"HYAKKI_ONI_HANDS_RESUME_MS", &c.Resume}} {
		if ms, err := strconv.Atoi(getenv(o.key)); err == nil && ms >= 1000 {
			if d := time.Duration(ms) * time.Millisecond; d < *o.to {
				*o.to = d
			}
		}
	}
	return c
}

// handsWindow is the Hands browser's one window: by its process, else by
// its own class; never a guess between two.
func handsWindow(s window.Snapshot, pid int, class string) (window.Window, bool) {
	for _, match := range []func(window.Window) bool{
		func(w window.Window) bool { return pid > 0 && w.PID == pid },
		func(w window.Window) bool { return class != "" && w.App == class },
	} {
		var found []window.Window
		for _, w := range s.Windows {
			if match(w) {
				found = append(found, w)
			}
		}
		if len(found) == 1 {
			return found[0], true
		}
	}
	return window.Window{}, false
}

func handsFocused(s window.Snapshot) window.ID {
	for _, w := range s.Windows {
		if w.Focused {
			return w.ID
		}
	}
	return ""
}

// handsApproving recognizes a live authenticated app answering a pending
// picture question, never a matching label alone or a later focus change.
func (d *Daemon) handsApproving(run *handsRun, s window.Snapshot) bool {
	if run.asked == "" {
		return false
	}
	focus := handsFocused(s)
	pid := 0
	for _, w := range s.Windows {
		if w.ID == focus && w.Visible && !w.Minimized {
			pid = w.PID
			break
		}
	}
	if pid <= 0 {
		return false
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	for peer := range d.clients {
		if peer.app && peer.pid > 0 && int(peer.pid) == pid {
			peer.mu.RLock()
			live := !peer.closed
			peer.mu.RUnlock()
			if live {
				return true
			}
		}
	}
	return false
}

// the app approves pictures even when voice or the pill started the task
func (d *Daemon) handsWindows(run *handsRun, s window.Snapshot) {
	if !d.handsLive(run) {
		return
	}
	var own *window.Window
	focus := handsFocused(s)
	for i := range s.Windows {
		w := &s.Windows[i]
		if w.ID == run.window {
			own = w
		}
	}
	if own == nil {
		d.handsStopRun(run, handsByClosed)
		return
	}
	approving := d.handsApproving(run, s)
	switch {
	case own.Minimized || !own.Visible:
		d.handsHandle(run, loop.Away{Why: protocol.OniHandsPauseHidden})
	case focus != run.focus && focus != run.window && !approving:
		d.handsHandle(run, loop.Away{Why: protocol.OniHandsPauseFocus})
	}
	run.focus = focus
}

// handsPageEvent hears what the authorized tab did by itself. On the queue.
func (d *Daemon) handsPageEvent(hb *handsBrowser, ev page.Event) {
	h := &d.oni.hands
	if h.browser != hb {
		return
	}
	if d.guardResident() {
		go d.guardBrowser(0)
	}
	run := h.run
	switch ev.Kind {
	case page.Closed, page.Crashed:
		hb.scope = nil
		d.handsStopRun(run, handsByClosed)
	case page.Pressed:
		if d.handsLive(run) && run.card != "" && ev.Control == run.cardCtrl {
			d.handsRow(run, handsToolUser, protocol.OniHandsOutcomeDone, "", 0)
			d.handsHandle(run, loop.UserDid{ID: run.card})
		}
	case page.Navigated:
		if ev.Away && d.handsLive(run) {
			d.handsHandle(run, loop.Away{Why: protocol.OniHandsPauseNavigation})
		}
	case page.Hidden, page.Dialog, page.Popup, page.Download:
		if d.handsLive(run) {
			d.handsHandle(run, loop.Away{Why: string(ev.Kind)})
		}
	}
}

// handsBrowserGone forgets a browser that exited. On the queue.
func (d *Daemon) handsBrowserGone(hb *handsBrowser) {
	h := &d.oni.hands
	if h.browser != hb {
		return
	}
	if run := h.run; run != nil && run.hb == hb {
		d.handsStopRun(run, handsByClosed)
		d.handsDrop(run)
	}
	h.browser = nil
	h.open.Store(false)
	go d.stateChanged()
}

// handsStop stops the running task, from anyone and anything: Esc, the
// pill, the CLI, a spoken stop, a changed setting, a lost presenter. It
// never waits, so a caller may hold a lock.
func (d *Daemon) handsStop(why string) {
	if !d.oni.hands.active.Load() {
		return
	}
	go d.handsDo(func() { d.handsStopRun(d.oni.hands.run, why) })
}

// handsStopRun stops run. On the queue.
func (d *Daemon) handsStopRun(run *handsRun, why string) {
	if !d.handsLive(run) {
		return
	}
	run.stopAt, run.stopWhy = d.handsClock().Now(), why
	d.handsHandle(run, loop.Stop{Why: why})
}

// handsSettings follows the settings: a task started under other rules
// ends, which takes its nested desktop with it. An Inspect opt-out ends
// all work but leaves the person's owned browser window open; deselecting
// the browser stage or turning Oni/Hyakki off retires that window too.
func (d *Daemon) handsSettings(current, next protocol.Settings) {
	h := &d.oni.hands
	was, now := protocol.EffectiveOni(current), protocol.EffectiveOni(next)
	if was.Hands != now.Hands {
		go d.handsDo(func() { h.pick = nil })
	}
	if !h.open.Load() && !h.active.Load() {
		return
	}
	before, after := handsDecide(was), handsDecide(now)
	if before == after && was.Master == now.Master && was.Hands == now.Hands && was.Disclosure == now.Disclosure {
		return
	}
	d.handsStop(handsBySettings)
	if now.Master && now.Hands == protocol.OniHandsBrowser {
		return
	}
	if h.open.Load() {
		go d.handsClose()
	}
}

// handsResume goes on with the paused task: the app's or the presenter's
// Resume button, the only resume there is. Outside a tab, where Hands
// cannot see the person's own press, the button pressed while a handoff
// card waits says they did the step themselves.
func (d *Daemon) handsResume() (protocol.OniHandsResult, error) {
	a := handsWait(d, func() handsAnswer {
		run := d.oni.hands.run
		if !d.handsLive(run) {
			return handsAnswer{err: protocol.Errorf(protocol.CodeConflict, "%s", handsLineNoPause)}
		}
		switch phase := run.m.Phase(); {
		case phase == loop.PhasePaused:
			if run.ex != nil && run.ex.isReady() {
				// keys pressed while paused no longer count; the next
				// action checks the window afresh
				run.ex.drv.Resume()
			}
			d.handsHandle(run, loop.Resume{By: loop.ByButton})
		case phase == loop.PhaseWaiting && run.ex != nil && run.card != "":
			if run.ex.isReady() {
				run.ex.drv.Resume()
			}
			d.handsRow(run, handsToolUser, protocol.OniHandsOutcomeDone, "", 0)
			d.handsHandle(run, loop.UserDid{ID: run.card})
		default:
			return handsAnswer{err: protocol.Errorf(protocol.CodeConflict, "%s", handsLineNoPause)}
		}
		return handsAnswer{res: d.handsResult()}
	})
	return a.res, a.err
}

// handsAnswer is a call's answer from the queue.
type handsAnswer struct {
	res protocol.OniHandsResult
	err error
}

// handsClose stops any task and closes the Hands browser.
func (d *Daemon) handsClose() {
	h := &d.oni.hands
	h.ops.Lock()
	defer h.ops.Unlock()
	hb := handsWait(d, func() *handsBrowser {
		hb := h.browser
		if run := h.run; run != nil {
			d.handsStopRun(run, handsBySettings)
			d.handsDrop(run)
		}
		h.browser = nil
		return hb
	})
	if hb == nil {
		return
	}
	if err := hb.b.Close(); err != nil {
		d.warn("oni hands browser: %v", err)
	}
	h.open.Store(false)
	go d.stateChanged()
}

// handsShutdown stops the task, which ends its nested desktop, and closes
// the Hands browser as the daemon exits.
func (d *Daemon) handsShutdown() {
	h := &d.oni.hands
	h.ops.Lock()
	if h.open.Load() || h.active.Load() || h.nested.Load() {
		handsWait(d, func() struct{} {
			d.handsStopRun(h.run, handsByShutdown)
			return struct{}{}
		})
	}
	h.nestStops.Wait()
	h.ops.Unlock()
	d.handsClose()
}

// handsForget deletes the Hands browser's profile, and nothing else, while
// no Hands browser runs.
func (d *Daemon) handsForget() (protocol.OniHandsResult, error) {
	h := &d.oni.hands
	h.ops.Lock()
	defer h.ops.Unlock()
	if h.open.Load() {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeConflict, "%s", handsLineOpenNow)
	}
	if err := os.RemoveAll(handsProfile()); err != nil {
		return protocol.OniHandsResult{}, protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("The Hands browser could not be forgotten: %v.", err))
	}
	res := protocol.OniHandsResult{State: protocol.OniHandsStateClosed, Line: handsLineForgotten}
	if handsRefusal(protocol.EffectiveOni(d.Settings())) == nil {
		res.Browser = d.handsBrowserName()
	}
	return res, nil
}

// handsDrop forgets run for good: its follow-up window and timers. On the
// queue.
func (d *Daemon) handsDrop(run *handsRun) {
	for token, t := range run.timers {
		t.Stop()
		delete(run.timers, token)
	}
	if d.oni.hands.run == run {
		d.oni.hands.run = nil
	}
}

func (d *Daemon) handsHandle(run *handsRun, ev loop.Event) {
	d.handsApply(run, run.m.Handle(ev))
	h := &d.oni.hands
	h.token = max(h.token, run.m.Token())
	if !run.active && run.m.Phase() != loop.PhaseFollowing && h.run == run {
		d.handsDrop(run)
	}
	h.card.Store(h.run != nil && h.run.active && (h.run.card != "" || h.run.parked != 0))
}

// handsApply carries out the machine's effects in order. On the queue.
func (d *Daemon) handsApply(run *handsRun, effects []loop.Effect) {
	for _, e := range effects {
		switch e := e.(type) {
		case loop.Frame:
			d.handsFrame(run, e)
		case loop.Refused:
			d.handsRow(run, e.Tool, protocol.OniHandsOutcomeRefused, e.Reason, 0)
		case loop.ReadPage:
			ctx := d.handsWork(run, e.Token, e.Until)
			if run.ex != nil {
				go d.handsReadApp(run, e.Token, ctx)
			} else {
				go d.handsRead(run, e.Token, ctx)
			}
		case loop.Ask:
			ctx := d.handsWork(run, e.Token, e.Until)
			q := &handsQuestion{run: run, token: e.Token, correct: e.Correct, ctx: ctx}
			pic := run.snap.Picture
			if run.ex != nil {
				q.in, pic = d.handsAppInput(run, e), run.pic
			} else {
				q.in = d.handsInput(run, e)
			}
			go d.handsAsk(q, bytes.Clone(pic))
		case loop.Act:
			ctx := d.handsWork(run, e.Token, e.Until)
			run.line = e.Line
			if ex := run.ex; ex != nil {
				go d.handsActApp(run, e, ctx, slices.Clone(ex.regions), slices.Clone(ex.files), ex.focus)
			} else {
				go d.handsAct(run, e, ctx)
			}
		case loop.Verify:
			ctx := d.handsWork(run, e.Token, e.Until)
			go d.handsVerifyApp(run, e, ctx)
		case loop.ReadBack:
			ctx := d.handsWork(run, e.Token, e.Until)
			go d.handsReadBackApp(run, e, ctx, run.ex.focus)
		case loop.ShowCard:
			d.handsCard(run, e)
		case loop.HideCard:
			if run.card == e.ID {
				d.handsHideCard(run)
			}
		case loop.Arm:
			d.handsArm(run, e.Token, e.Until)
		case loop.Cancel:
			d.handsCancel(run, e.Token)
		case loop.End:
			d.handsEnded(run, e)
		}
	}
}

// handsFrame shows the task's active frame, and the handoff card once its
// waiting frame is out.
func (d *Daemon) handsFrame(run *handsRun, f loop.Frame) {
	if !run.active {
		return
	}
	hf := feature.HandsFrame{
		Stage: run.stage, Active: true, Reason: feature.ReasonAllowed, Scope: run.scope,
		Phase: f.Phase, Step: f.Step, Max: f.Max, Pause: f.Pause,
	}
	if ex := run.ex; ex != nil && ex.scope == protocol.OniHandsScopeWindow {
		if protocol.ValidOniHandsApp(ex.base.App) {
			hf.App = ex.base.App
		}
		if protocol.ValidOniHandsApp(ex.base.Output) {
			hf.Output = ex.base.Output
		}
	}
	if hf.Step > hf.Max {
		hf.Max = min(hf.Step, feature.HandsMaxSteps())
		hf.Step = min(hf.Step, hf.Max)
	}
	run.frame = hf
	if f.Line != "" {
		run.line = f.Line
	}
	d.handsJournalFrame(run, f.Phase, f.Step, f.Pause)
	svc := d.oni.service.Load()
	if svc == nil {
		return
	}
	if err := svc.Hands(hf); err != nil {
		d.warn("oni hands frame: %v", err)
	}
	// the card shows while its waiting frame does, and again when a resume
	// brings it back, with what is left of its time
	switch {
	case run.card == "":
	case f.Phase != protocol.OniHandsStateWaiting:
		run.cardSent = false
	case !run.cardSent:
		left := max(run.cardUntil.Sub(d.handsClock().Now()), 0)
		if err := svc.Consent(*feature.HandoffCard(feature.HandsStageFeature(run.stage), run.card, run.cardLine, left)); err != nil {
			d.warn("oni hands card: %v", err)
		}
		run.cardSent = true
	}
}

// handsCard points at the control the step is handed off on, and in a tab
// watches the page for the person's own press on it. Outside a tab Hands
// cannot see the press: the person says they did it with the Resume
// button.
func (d *Daemon) handsCard(run *handsRun, e loop.ShowCard) {
	d.handsHideCard(run)
	run.card, run.cardCtrl, run.cardSent = e.ID, e.Control, false
	run.cardLine, run.cardUntil = oniText(e.Line, 400), e.Until
	run.line = e.Line
	d.handsRow(run, run.tool, protocol.OniHandsOutcomeHandoff, string(e.Reason), 0)
	if e.Control == "" || run.hb == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	run.awaitStop = cancel
	drv := run.hb.drv
	go func() {
		if err := drv.Await(ctx, e.Control); err != nil && ctx.Err() == nil {
			d.warn("oni hands await: %v", err)
		}
	}()
}

func (d *Daemon) handsHideCard(run *handsRun) {
	if run.awaitStop != nil {
		run.awaitStop()
		run.awaitStop = nil
	}
	run.card, run.cardCtrl, run.cardLine, run.cardSent = "", "", "", false
}

// handsWork records a read, request or step with token, armed with its
// deadline.
func (d *Daemon) handsWork(run *handsRun, token uint64, until time.Time) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	run.work[token] = &handsWork{cancel: cancel, at: d.handsClock().Now()}
	d.handsArm(run, token, until)
	return ctx
}

// handsArm arms the deadline with token at until; a zero until holds it.
func (d *Daemon) handsArm(run *handsRun, token uint64, until time.Time) {
	if t := run.timers[token]; t != nil {
		t.Stop()
		delete(run.timers, token)
	}
	if until.IsZero() {
		return
	}
	clock := d.handsClock()
	run.timers[token] = clock.AfterFunc(until.Sub(clock.Now()), func() {
		d.handsDo(func() {
			if d.oni.hands.run != run || run.timers[token] == nil {
				return
			}
			delete(run.timers, token)
			d.handsHandle(run, loop.Deadline{Token: token})
		})
	})
}

// finishWork forgets the work with token once its answer is in; false when
// it was abandoned.
func (r *handsRun) finishWork(token uint64) bool {
	w := r.work[token]
	if w == nil {
		return false
	}
	if t := r.timers[token]; t != nil {
		t.Stop()
		delete(r.timers, token)
	}
	w.cancel()
	delete(r.work, token)
	return true
}

// handsCancel abandons the work with token: a model stream is cut, and a
// picture waiting on its question is dropped.
func (d *Daemon) handsCancel(run *handsRun, token uint64) {
	run.finishWork(token)
	if run.parked == token {
		d.handsDropQuestion(run)
	}
}

// handsEnded drops everything the task holds and takes every indication
// down in one frame. On the queue.
func (d *Daemon) handsEnded(run *handsRun, e loop.End) {
	if !run.active {
		return
	}
	run.active = false
	d.oni.hands.active.Store(false)
	for token := range run.work {
		run.finishWork(token)
	}
	for token, t := range run.timers {
		t.Stop()
		delete(run.timers, token)
	}
	d.handsDropQuestion(run)
	d.handsHideCard(run)
	if run.keysStop != nil {
		run.keysStop()
		run.keysStop = nil
	}
	if run.watchStop != nil {
		run.watchStop()
		run.watchStop = nil
	}
	if run.deadline != nil {
		run.deadline.Stop()
		run.deadline = nil
	}
	step := run.frame.Step
	if run.ex != nil {
		// the key reader, the watches and the nested desktop with every
		// program on it end with the task
		d.handsCloseExec(run.ex)
		clear(run.pic)
		run.pic = nil
		run.ex.regions, run.ex.files = nil, nil
		d.handsJournalEnd(run, e, step)
	}
	clear(run.snap.Picture)
	run.snap = page.Snapshot{}
	run.frame, run.line = feature.HandsFrame{}, e.Line
	settings := protocol.EffectiveOni(d.Settings())
	decision := handsDecide(settings)
	stage := settings.Hands
	if !slices.Contains(protocol.OniHandsStages, stage) {
		stage = protocol.OniHandsOff
	}
	if svc := d.oni.service.Load(); svc != nil {
		if err := svc.Hands(feature.HandsFrame{Stage: stage, Reason: decision.Reason, End: e.Reason}); err != nil && !errors.Is(err, presence.ErrOff) && !errors.Is(err, presence.ErrClosed) {
			d.warn("oni hands frame: %v", err)
		}
		if line := oniText(e.Line, oniBubbleMax); line != "" && e.Reason != loop.EndStopped {
			_ = svc.Bubble(0, "say", line)
		}
	}
	if !run.stopAt.IsZero() {
		ms := d.handsClock().Now().Sub(run.stopAt).Milliseconds()
		d.handsRow(run, handsToolStop, protocol.OniHandsOutcomeDone, run.stopWhy, ms)
	}
	outcome := protocol.OniHandsOutcomeDone
	switch e.Reason {
	case loop.EndDone, loop.EndAsked, loop.EndAnswered, loop.EndStopped:
	default:
		outcome = protocol.OniHandsOutcomeFailed
	}
	d.handsRow(run, handsToolEnd, outcome, e.Reason, 0)
}

// handsSubject is what the loop sorts a control by: its role and name,
// raised by what the page's own first sorting found, never lowered.
func handsSubject(c page.Control) consent.Subject {
	s := consent.Subject{Role: c.Role, Name: c.Name}
	switch c.Reason {
	case consent.ReasonSubmits:
		s.Submits = true
	case consent.ReasonPaymentForm:
		s.PaymentForm = true
	case consent.ReasonPrivate:
		s.InputType = "password"
	case consent.ReasonFileInput:
		s.InputType = "file"
	case consent.ReasonOtherOrigin:
		s.OtherOrigin = true
	case consent.ReasonHidden:
		s.Hidden = true
	}
	return s
}

// handsRead reads the authorized tab for the machine.
func (d *Daemon) handsRead(run *handsRun, token uint64, ctx context.Context) {
	var snap page.Snapshot
	var err error
	// A person's navigation can invalidate a snapshot mid-read. Refresh only
	// that reading, under the original deadline and cancellation.
	for {
		if err = ctx.Err(); err != nil {
			break
		}
		snap, err = run.hb.drv.Page(ctx)
		if !errors.Is(err, page.ErrStale) {
			break
		}
	}
	d.handsDo(func() {
		if !d.handsLive(run) {
			clear(snap.Picture)
			return
		}
		if !run.finishWork(token) {
			clear(snap.Picture)
			return
		}
		read := loop.PageRead{Token: token, Err: err}
		if err == nil {
			clear(run.snap.Picture)
			run.snap = snap
			read.Origin, read.Print = snap.Origin, snap.Print
			for _, c := range snap.Controls {
				read.Controls = append(read.Controls, loop.Control{ID: c.ID, Subject: handsSubject(c)})
			}
		}
		d.handsHandle(run, read)
	})
}

// handsInput is one model request's input from the last reading. On the
// queue.
func (d *Daemon) handsInput(run *handsRun, e loop.Ask) proto.Input {
	in := proto.Input{Scope: proto.ScopeBrowser, Goal: e.Goal, Origin: run.origin, Text: run.snap.Text, History: e.History, Truncated: run.snap.Truncated}
	for _, c := range run.snap.Controls {
		in.Controls = append(in.Controls, proto.Control{ID: c.ID, Role: c.Role, Name: c.Name})
	}
	if s, err := oniGuideSpec(); err == nil {
		h := s.Hands
		in.Bounds = proto.Bounds{Controls: h.Page.MaxControls, NameRunes: h.Page.MaxNameRunes, TextRunes: h.Page.MaxTextRunes, HistoryLines: h.HistoryCarry}
	}
	return in
}

// handsAsk takes the step's picture as the tab drew it, then asks before
// it is sent exactly as an Inspect picture is, or sends it.
func (d *Daemon) handsAsk(hq *handsQuestion, png []byte) {
	run := hq.run
	fail := func(err error) {
		d.handsDo(func() { d.handsReplied(hq, "", err) })
	}
	shot, err := capture.FromPNG(png)
	if err != nil {
		fail(err)
		return
	}
	disc := handsDisclosure
	if run.ex != nil {
		disc = handsAppDisclosure(run.scope)
	}
	q := &oniQuestion{conn: run.conn, text: run.goal, shot: shot, disc: disc, plan: run.plan, hands: hq}
	if q.pic, err = shot.JPEG(capture.AskLongSide, capture.AskQuality); err != nil {
		q.clear()
		fail(err)
		return
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(q.pic)); err == nil {
		q.size = screenask.Size{W: cfg.Width, H: cfg.Height}
	}
	if hq.ctx.Err() != nil {
		q.clear()
		return
	}
	settings := protocol.EffectiveOni(d.Settings())
	a := &d.oni.ask
	a.mu.Lock()
	if !feature.MustAsk(feature.Inspect, settings, a.asked[q.plan.Route()]) {
		a.mu.Unlock()
		d.handsSend(q)
		return
	}
	consent, err := q.consent()
	if err != nil {
		a.mu.Unlock()
		q.clear()
		fail(err)
		return
	}
	old := a.pending
	a.gen++
	gen := a.gen
	q.timer = a.now().AfterFunc(feature.ConsentTTL, func() { d.oniConsentExpired(gen) })
	a.pending = q
	a.mu.Unlock()
	if old != nil {
		d.oniQuestionDropped(old)
	}
	d.handsDo(func() {
		if !d.handsLive(run) || run.work[hq.token] == nil {
			d.handsDropQuestion(run)
			return
		}
		// the request's deadline waits with the picture; the question's
		// own expiry bounds the wait
		d.handsHandle(run, loop.Asking{Token: hq.token})
		if !d.handsLive(run) {
			return
		}
		run.parked, run.asked = hq.token, consent.ID
		d.oni.hands.card.Store(true)
		if svc := d.oni.service.Load(); svc != nil {
			_ = svc.Consent(feature.ConsentFrame{
				ID: consent.ID, Feature: feature.Inspect, Class: feature.ConsentDisclosure,
				Destination: consent.Destination, Route: consent.Route, Question: consent.Question,
				ExpiresMs: consent.ExpiresMs,
			})
		}
	})
}

// handsSend sends the step's picture, page text and control names to the
// planned route alone, and hands the reply to the machine. Stopping the
// task cancels the request's context, which cuts the model stream.
func (d *Daemon) handsSend(q *oniQuestion) {
	hq := q.hands
	system, user := proto.Prompt(hq.in)
	if hq.correct != "" {
		user += "\n\n" + hq.correct
	}
	req := oniRequest(system, user, q.pic)
	req.Model, req.Strict, req.LocalOnly = q.plan.Route(), true, q.plan.Local
	reply, err := d.handsModel(hq.ctx, req)
	q.clear()
	d.handsDo(func() { d.handsReplied(hq, reply, err) })
}

func (d *Daemon) handsModel(ctx context.Context, req llm.Request) (string, error) {
	stream, err := d.deps.Gateway.Stream(ctx, req)
	if err != nil {
		return "", err
	}
	defer stream.Close()
	var reply strings.Builder
	for {
		delta, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if reply.Len()+len(delta.Text) > oniReplyMax {
			break
		}
		reply.WriteString(delta.Text)
	}
	return reply.String(), nil
}

// handsReplied hands a request's reply to the machine, unless it was
// abandoned. A reply that breaks the contract is a ledger row. On the
// queue.
func (d *Daemon) handsReplied(hq *handsQuestion, reply string, err error) {
	run := hq.run
	if !d.handsLive(run) {
		return
	}
	if !run.finishWork(hq.token) {
		return
	}
	if err == nil {
		offer := proto.Offer{Scope: proto.ScopeBrowser}
		if ex := run.ex; ex != nil {
			offer = proto.Offer{Scope: run.scope, Files: len(ex.files)}
			for _, r := range ex.regions {
				offer.Controls = append(offer.Controls, r.id)
			}
		} else {
			for _, c := range run.snap.Controls {
				offer.Controls = append(offer.Controls, c.ID)
			}
		}
		resp, perr := proto.Parse(reply, offer)
		switch {
		case perr != nil:
			d.handsRow(run, handsToolReply, protocol.OniHandsOutcomeRefused, handsParseCode(perr), 0)
		case resp.Type == proto.TypeToolCall:
			run.tool = resp.Tool
		}
	}
	d.handsHandle(run, loop.Reply{Token: hq.token, Text: reply, Err: err})
}

// handsParseCode names a contract break.
func handsParseCode(err error) string {
	for _, c := range []struct {
		err  error
		code string
	}{
		{proto.ErrMultiple, "multiple"}, {proto.ErrInvented, "invented"}, {proto.ErrPermission, "permission"},
		{proto.ErrArgs, "args"}, {proto.ErrShape, "shape"}, {proto.ErrJSON, "json"},
	} {
		if errors.Is(err, c.err) {
			return c.code
		}
	}
	return "error"
}

// handsConsented sends the step's picture the person said yes to.
func (d *Daemon) handsConsented(q *oniQuestion) (protocol.OniAskResult, error) {
	hq := q.hands
	type answer struct {
		res protocol.OniAskResult
		err error
	}
	a := handsWait(d, func() answer {
		run := hq.run
		if !d.handsLive(run) || run.parked != hq.token || run.work[hq.token] == nil {
			q.clear()
			return answer{err: protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That question has expired or was answered."))}
		}
		run.parked, run.asked = 0, ""
		d.handsAnswered(q.id, "yes")
		// the request starts now, with its whole timeout
		d.handsHandle(run, loop.Allowed{Token: hq.token})
		go d.handsSend(q)
		dest := "hosted"
		if q.plan.Local {
			dest = "local"
		}
		return answer{res: protocol.OniAskResult{Status: protocol.OniAskAnswered, Answer: &protocol.OniAnswer{Route: q.plan.Route(), Destination: dest, Text: i18n.T("Allowed. Hands goes on.")}}}
	})
	return a.res, a.err
}

// handsDeclined stops the task whose step's picture the person said no to.
func (d *Daemon) handsDeclined(q *oniQuestion) {
	q.clear()
	hq := q.hands
	d.handsDo(func() {
		run := hq.run
		if d.handsLive(run) && run.parked == hq.token {
			run.parked, run.asked = 0, ""
			d.handsAnswered(q.id, "no")
			d.handsStopRun(run, handsByDeclined)
		}
	})
}

// handsQuestionLost stops the task whose step's question went unanswered
// or was replaced.
func (d *Daemon) handsQuestionLost(q *oniQuestion) {
	hq := q.hands
	go d.handsDo(func() {
		run := hq.run
		if d.handsLive(run) && run.parked == hq.token {
			run.parked, run.asked = 0, ""
			d.handsAnswered(q.id, "no")
			d.handsStopRun(run, handsByConsent)
		}
	})
}

// handsDropQuestion clears the step's question run left waiting. On the
// queue.
func (d *Daemon) handsDropQuestion(run *handsRun) {
	a := &d.oni.ask
	a.mu.Lock()
	q := a.pending
	if q == nil || q.hands == nil || q.hands.run != run {
		a.mu.Unlock()
		if run.asked != "" {
			d.handsAnswered(run.asked, "no")
		}
		run.parked, run.asked = 0, ""
		return
	}
	a.pending = nil
	a.gen++
	a.mu.Unlock()
	q.clear()
	d.handsAnswered(q.id, "no")
	run.parked, run.asked = 0, ""
}

// handsAnswered takes a step's question off the frames.
func (d *Daemon) handsAnswered(id, answer string) {
	if id != "" && d.oni.service.Load() != nil {
		d.oniEmit(&feature.ConsentAnswerFrame{ID: id, Answer: answer})
	}
}

// handsAct runs one safe step; the page sorts it again against the element
// itself and refuses anything not safe.
func (d *Daemon) handsAct(run *handsRun, e loop.Act, ctx context.Context) {
	a := page.Act{Verb: e.Tool, Control: e.Args.ID, Text: e.Args.Text, Option: e.Args.Option, Ms: e.Args.Ms}
	if e.Tool == proto.ToolScroll {
		a.DY = handsScroll
		if e.Args.Dir == proto.DirUp {
			a.DY = -handsScroll
		}
	}
	res, err := run.hb.drv.Act(ctx, a)
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
		r := loop.Result{ReadBack: res.ReadBack}
		outcome, reason := protocol.OniHandsOutcomeDone, ""
		switch {
		case err == nil:
		case errors.Is(err, page.ErrNotSafe):
			r.Refused, r.Err = true, string(res.Reason)
			outcome, reason = protocol.OniHandsOutcomeRefused, string(res.Reason)
		default:
			r.Err = handsErrCode(err)
			outcome, reason = protocol.OniHandsOutcomeFailed, r.Err
		}
		d.handsRow(run, e.Tool, outcome, reason, ms)
		d.handsHandle(run, loop.Acted{Token: e.Token, Result: r})
	})
}

// handsErrCode names why a step did not run.
func handsErrCode(err error) string {
	for _, c := range []struct {
		err  error
		code string
	}{
		{page.ErrStale, "stale"}, {page.ErrCovered, "covered"}, {page.ErrText, "text"}, {page.ErrNoOption, "no-option"},
		{page.ErrOrigin, "origin"}, {page.ErrAway, "away"}, {page.ErrNotAuthorized, "not-authorized"},
		{context.DeadlineExceeded, "timeout"}, {context.Canceled, "cancelled"},
	} {
		if errors.Is(err, c.err) {
			return c.code
		}
	}
	return "error"
}

// handsRow keeps one ledger row: codes and numbers, never a word.
func (d *Daemon) handsRow(run *handsRun, tool, outcome, reason string, ms int64) {
	row := protocol.OniHandsRow{At: d.handsClock().Now(), Task: run.id, Step: run.frame.Step, Tool: tool, Outcome: outcome, Reason: reason, Ms: max(ms, 0)}
	if row.Tool == "" {
		row.Tool = "step"
	}
	if !protocol.ValidOniHandsRow(row) {
		row.Reason = ""
		if !protocol.ValidOniHandsRow(row) {
			return
		}
	}
	limit := 200
	if s, err := oniGuideSpec(); err == nil && s.Hands.Ledger.MaxRows > 0 {
		limit = s.Hands.Ledger.MaxRows
	}
	h := &d.oni.hands
	h.rowsMu.Lock()
	defer h.rowsMu.Unlock()
	h.rows = append(h.rows, row)
	if over := len(h.rows) - limit; over > 0 {
		h.rows = slices.Delete(h.rows, 0, over)
	}
}

// handsLogCall answers oni.hands.log, newest first.
func (d *Daemon) handsLogCall() (protocol.OniHandsLogResult, error) {
	h := &d.oni.hands
	h.rowsMu.Lock()
	rows := slices.Clone(h.rows)
	h.rowsMu.Unlock()
	slices.Reverse(rows)
	if rows == nil {
		rows = []protocol.OniHandsRow{}
	}
	return protocol.OniHandsLogResult{Rows: rows}, nil
}

// voiceHands is the router's Hands slot: a spoken press starts a task when
// it names the page with an anchor phrase and a tab is authorized, or
// continues the finished one within the follow-up window. A spoken "keep
// going" while paused is heard and never resumes: only the button does.
func (d *Daemon) voiceHands(p route.Press, text string) bool {
	// a paused task of any stage hears a spoken "keep going" and resumes
	// nothing; only a tab's task ever starts from a press
	if !p.Valid() || !d.oni.hands.open.Load() && !d.oni.hands.active.Load() {
		return false
	}
	norm := wake.Normalize(text)
	type state struct{ live, paused, following, tab bool }
	s := handsWait(d, func() state {
		h := &d.oni.hands
		run := h.run
		return state{
			live:      d.handsLive(run),
			paused:    d.handsLive(run) && run.m.Phase() == loop.PhasePaused,
			following: run != nil && !run.active && run.m.Phase() == loop.PhaseFollowing,
			tab:       h.browser != nil && h.browser.scope != nil,
		}
	})
	if s.paused && slices.Contains(handsResumes, norm) {
		return true
	}
	if s.live || !s.tab {
		return false
	}
	anchored := false
	for _, a := range handsAnchors {
		if strings.Contains(" "+norm+" ", " "+a+" ") {
			anchored = true
		}
	}
	if !anchored && !s.following {
		return false
	}
	goal := strings.TrimSpace(text)
	if rs := []rune(goal); len(rs) > protocol.OniHandsGoalMax {
		goal = strings.TrimSpace(string(rs[:protocol.OniHandsGoalMax]))
	}
	go func() {
		if _, err := d.handsBegin(nil, goal, "press", !anchored && s.following); err != nil {
			d.warn("oni hands: %v", err)
		}
	}()
	return true
}
