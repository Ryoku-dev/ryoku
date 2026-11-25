package daemon

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/protocol"
)

// These tests drive the focus guard's window half over the daemon's own
// socket. The desktop, the seam's closer, the clock and the notifier are
// fakes: no compositor is asked and no window is closed.

// guardCloser stands in for the seam: it records every probe and close.
type guardCloser struct {
	mu        sync.Mutex
	supported bool
	probes    int
	closes    [][2]string
}

func (c *guardCloser) Supported(context.Context) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.probes++
	return c.supported
}

func (c *guardCloser) Close(_ context.Context, id, app string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closes = append(c.closes, [2]string{id, app})
	return nil
}

func (c *guardCloser) seen() (probes int, closes [][2]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.probes, slices.Clone(c.closes)
}

// guardDesk is the fake desktop with the caps a test picks.
type guardDesk struct {
	*fakeDesktop
	caps window.Caps
}

func (g guardDesk) Caps() window.Caps { return g.caps }

type guardRig struct {
	*voiceRig
	desk   *fakeDesktop
	closer *guardCloser
	clock  *askClock
	probe  *remindProbe
}

var (
	guardSeam    = window.Caps{Provider: "ryoku", Geometry: window.GeomAll, Events: true}
	guardNiri    = window.Caps{Provider: "niri", Geometry: window.GeomAll, Events: true}
	guardNoEvent = window.Caps{Provider: "wlr", Geometry: window.GeomNone}
)

// serveGuard serves a daemon with the guard switched on, the feed words
// tiktok and reddit, and a desktop with caps; edit changes the settings.
func serveGuard(t *testing.T, caps window.Caps, edit func(*protocol.Settings)) *guardRig {
	t.Helper()
	g := &guardRig{
		desk:   newDesktop(window.Snapshot{Outputs: []window.Output{dp1}}),
		closer: &guardCloser{supported: true},
		clock:  &askClock{now: time.Unix(1_800_000_000, 0)},
		probe:  &remindProbe{},
	}
	g.voiceRig = serveVoice(t, func(s *protocol.Settings) {
		s.Oni.Feeds.Guard = true
		s.Oni.Feeds.Words = []string{"tiktok", "reddit"}
		if edit != nil {
			edit(s)
		}
	}, nil, func(d *Deps) {
		d.OniWindows = func() (window.Provider, error) { return guardDesk{g.desk, caps}, nil }
		d.OniGuard = OniGuardDeps{Closer: g.closer, Clock: g.clock}
		d.OniReflex.Notify = g.probe.notify
	})
	return g
}

func guardFocus(t *testing.T, p *socketPeer, params protocol.OniFocusParams) protocol.OniFocusResult {
	t.Helper()
	var res protocol.OniFocusResult
	if err := p.call(protocol.MOniFocus, params, &res); err != nil {
		t.Fatalf("oni.focus %s: %v", params.Action, err)
	}
	return res
}

func guardArm20(t *testing.T, p *socketPeer) protocol.OniFocusResult {
	t.Helper()
	return guardFocus(t, p, protocol.OniFocusParams{Action: protocol.OniFocusArm, Minutes: 20})
}

// noticed waits for a notification whose body holds every part.
func (g *guardRig) noticed(t *testing.T, parts ...string) string {
	t.Helper()
	var body string
	remindEventually(t, "a notice with "+strings.Join(parts, ", "), func() bool {
		for _, n := range g.probe.notices() {
			hit := true
			for _, p := range parts {
				hit = hit && strings.Contains(n[1], p)
			}
			if hit {
				body = n[1]
				return true
			}
		}
		return false
	})
	return body
}

const (
	guardFeedApp = "com.example.TikTok"
	guardTitle   = "Reddit - Dive into anything"
)

func guardFeeds() window.Snapshot {
	return window.Snapshot{Outputs: []window.Output{dp1}, Windows: []window.Window{
		{ID: "21", App: guardFeedApp, Title: "For You", Output: "DP-1", Visible: true, Focused: true},
		{ID: "25", App: "firefox", Title: guardTitle, Output: "DP-1", Visible: true, FocusOrder: 1},
		{ID: "30", App: "org.gnome.TextEditor", Title: "notes.txt", Output: "DP-1", Visible: true, FocusOrder: 2},
	}}
}

// With the guard off, or Oni off, an arm is refused: no watch, no timer,
// no seam probe, nothing resident.
func TestOniGuardArmsNothingWhileOff(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(*protocol.Settings)
	}{
		{"guard off", func(s *protocol.Settings) { s.Oni.Feeds.Guard = false }},
		{"oni off", func(s *protocol.Settings) { s.Oni.Master = false }},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := serveGuard(t, guardSeam, c.edit)
			cli := dialSocket(t, protocol.ClientCLI)
			if res := guardArm20(t, cli); res.Armed || res.Refused != protocol.OniFocusRefusedOff {
				t.Fatalf("arm while off: %+v", res)
			}
			if res := guardFocus(t, cli, protocol.OniFocusParams{Action: protocol.OniFocusStatus}); res.Armed {
				t.Fatalf("status: %+v", res)
			}
			if w, _ := g.desk.state(); w != 0 || g.clock.Armed() != 0 || g.d.guardResident() {
				t.Fatalf("watching %d, %d timers, resident %v", w, g.clock.Armed(), g.d.guardResident())
			}
			if probes, closes := g.closer.seen(); probes != 0 || len(closes) != 0 {
				t.Fatalf("the seam was asked: %d probes, closes %v", probes, closes)
			}
		})
	}
}

// A desktop without window events, or with no feed words, arms nothing.
func TestOniGuardRefusesWithoutWindowEventsOrWords(t *testing.T) {
	t.Run("no window events", func(t *testing.T) {
		g := serveGuard(t, guardNoEvent, nil)
		cli := dialSocket(t, protocol.ClientCLI)
		if res := guardArm20(t, cli); res.Armed || res.Refused != protocol.OniFocusRefusedEvents {
			t.Fatalf("arm without events: %+v", res)
		}
		if w, _ := g.desk.state(); w != 0 || g.clock.Armed() != 0 || g.d.guardResident() {
			t.Fatalf("watching %d, %d timers, resident %v", w, g.clock.Armed(), g.d.guardResident())
		}
		if probes, _ := g.closer.seen(); probes != 0 {
			t.Fatalf("the seam was asked %d times", probes)
		}
	})
	t.Run("no feed words", func(t *testing.T) {
		g := serveGuard(t, guardSeam, func(s *protocol.Settings) { s.Oni.Feeds.Words = nil })
		cli := dialSocket(t, protocol.ClientCLI)
		if res := guardArm20(t, cli); res.Armed || res.Refused != protocol.OniFocusRefusedWords {
			t.Fatalf("arm without words: %+v", res)
		}
		if w, _ := g.desk.state(); w != 0 || g.clock.Armed() != 0 {
			t.Fatalf("watching %d, %d timers", w, g.clock.Armed())
		}
		// words given with the arm stand in for the Settings ones
		res := guardFocus(t, cli, protocol.OniFocusParams{Action: protocol.OniFocusArm, Minutes: 5, Words: []string{"reels"}})
		if !res.Armed || !slices.Equal(res.Words, []string{"reels"}) {
			t.Fatalf("arm with words: %+v", res)
		}
	})
}

// Through the seam, each window whose app id names a feed gets its own
// card naming the feed and the app, and closes only on yes; a browser
// window naming a feed only in its title is announced and never closed.
func TestOniGuardCardsEachFeedAppAndNeverClosesATitleMatch(t *testing.T) {
	g := serveGuard(t, guardSeam, nil)
	surface, _ := g.presenting(t)
	log := &frameLog{p: surface}
	cli := dialSocket(t, protocol.ClientCLI)
	app := g.dialApp(t)

	res := guardArm20(t, cli)
	if !res.Armed || res.LeftMs != (20*time.Minute).Milliseconds() || !slices.Equal(res.Words, []string{"tiktok", "reddit"}) {
		t.Fatalf("armed: %+v", res)
	}
	log.until("the armed frame", func(f feature.Payload) bool {
		ff, ok := f.(*feature.FocusFrame)
		return ok && ff.Phase == feature.FocusArmed
	})
	g.desk.await(t, 1, "")
	if g.clock.Armed() != 1 {
		t.Fatalf("%d timers while armed, want the one deadline", g.clock.Armed())
	}

	g.desk.set(guardFeeds())
	card := log.until("the close card", consentFor(feature.FeedGuard)).(*feature.ConsentFrame)
	if !strings.HasPrefix(card.ID, feature.GuardCardPrefix) || card.Class != feature.ConsentPerAction || card.Destination != "local" {
		t.Fatalf("card: %+v", card)
	}
	if !strings.Contains(card.Question, guardFeedApp) || !strings.Contains(card.Question, "tiktok") || strings.Contains(card.Question, "For You") {
		t.Fatalf("the card does not name the feed and the app alone: %q", card.Question)
	}
	body := g.noticed(t, "reddit", "firefox")
	if strings.Contains(body, "Dive") {
		t.Fatalf("the announcement holds the title: %q", body)
	}
	if _, closes := g.closer.seen(); len(closes) != 0 {
		t.Fatalf("closed before a yes: %v", closes)
	}

	var ask protocol.OniAskResult
	if err := cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: "yes"}, &ask); err == nil {
		t.Fatal("CLI approved an app-only feed close")
	}
	if err := app.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: "yes"}, &ask); err != nil || ask.Status != protocol.OniAskAnswered {
		t.Fatalf("yes: %+v %v", ask, err)
	}
	remindEventually(t, "the feed app closed", func() bool { _, c := g.closer.seen(); return len(c) == 1 })
	if _, closes := g.closer.seen(); closes[0] != [2]string{"21", guardFeedApp} {
		t.Fatalf("closed %v", closes)
	}
	g.noticed(t, "Closed "+guardFeedApp)

	// a second window of the feed app gets its own card; a no leaves it
	snap := guardFeeds()
	snap.Windows = append(snap.Windows, window.Window{ID: "22", App: guardFeedApp, Title: "Live", Output: "DP-1", Visible: true})
	g.desk.set(snap)
	second := log.until("the second card", consentFor(feature.FeedGuard)).(*feature.ConsentFrame)
	if second.ID == card.ID {
		t.Fatal("the second window reused the first card")
	}
	if err := app.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: second.ID, Answer: "no"}, &ask); err != nil || ask.Status != protocol.OniAskDeclined {
		t.Fatalf("no: %+v %v", ask, err)
	}
	// the same windows again ask nothing more
	g.desk.set(snap)
	log.quiet("a card for a window already asked about", consentFor(feature.FeedGuard))
	if _, closes := g.closer.seen(); len(closes) != 1 {
		t.Fatalf("closes %v", closes)
	}
	res = guardFocus(t, cli, protocol.OniFocusParams{Action: protocol.OniFocusStatus})
	if !res.Armed || res.Closed != 1 || res.Announced != 1 {
		t.Fatalf("status: %+v", res)
	}
	if err := app.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: second.ID, Answer: "yes"}, &ask); err == nil {
		t.Fatal("an answered card was answered again")
	}
}

// Without the seam, or with a seam that does not answer, a feed app's
// window is only announced; title matches are announced as everywhere.
func TestOniGuardOnlyAnnouncesWhereNoSeamCanClose(t *testing.T) {
	for _, c := range []struct {
		name      string
		caps      window.Caps
		supported bool
		probes    int
	}{
		{"niri without the seam", guardNiri, true, 0},
		{"seam not answering", guardSeam, false, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := serveGuard(t, c.caps, nil)
			g.closer.supported = c.supported
			surface, _ := g.presenting(t)
			log := &frameLog{p: surface}
			cli := dialSocket(t, protocol.ClientCLI)
			if res := guardArm20(t, cli); !res.Armed {
				t.Fatalf("arm: %+v", res)
			}
			g.desk.await(t, 1, "")
			g.desk.set(guardFeeds())
			g.noticed(t, "tiktok", guardFeedApp, "no way to close")
			g.noticed(t, "reddit", "firefox")
			log.quiet("a close card", consentFor(feature.FeedGuard))
			probes, closes := g.closer.seen()
			if probes != c.probes || len(closes) != 0 {
				t.Fatalf("%d probes, closes %v", probes, closes)
			}
			if res := guardFocus(t, cli, protocol.OniFocusParams{Action: protocol.OniFocusStatus}); res.Announced != 2 || res.Closed != 0 {
				t.Fatalf("status: %+v", res)
			}
		})
	}
}

// The deadline disarms the guard: the watch and the timer end, the user is
// told, and with no client left the daemon idles again.
func TestOniGuardExpiryDisarmsAndTheDaemonIdles(t *testing.T) {
	g := serveGuard(t, guardSeam, nil)
	cli := dialSocket(t, protocol.ClientCLI)
	if res := guardArm20(t, cli); !res.Armed {
		t.Fatalf("arm: %+v", res)
	}
	g.desk.await(t, 1, "")
	if g.clock.Armed() != 1 || !g.d.guardResident() {
		t.Fatalf("%d timers, resident %v", g.clock.Armed(), g.d.guardResident())
	}
	cli.conn.Close()
	remindEventually(t, "no client left", g.d.restingBesideMemory)
	if g.d.Idle() {
		t.Fatal("an armed guard let the daemon idle")
	}
	g.clock.Advance(20*time.Minute - time.Second)
	if !g.d.guardResident() {
		t.Fatal("disarmed before its time")
	}
	g.clock.Advance(time.Second)
	if g.d.guardResident() {
		t.Fatal("still armed after its time")
	}
	g.desk.await(t, 0, "")
	if g.clock.Armed() != 0 {
		t.Fatalf("%d timers after expiry", g.clock.Armed())
	}
	g.noticed(t, "Focus time is over")
	remindEventually(t, "the daemon idles", g.d.Idle)

	cli = dialSocket(t, protocol.ClientCLI)
	if res := guardFocus(t, cli, protocol.OniFocusParams{Action: protocol.OniFocusStatus}); res.Armed || res.LeftMs != 0 {
		t.Fatalf("status after expiry: %+v", res)
	}
	// the desktop changing afterwards asks and says nothing
	before := len(g.probe.notices())
	g.desk.set(guardFeeds())
	time.Sleep(50 * time.Millisecond)
	if after := len(g.probe.notices()); after != before {
		t.Fatalf("a disarmed guard spoke: %v", g.probe.notices()[before:])
	}
}

// The user's stop, "let me back", the settings and the app it lasts until
// each disarm the guard and leave nothing running.
func TestOniGuardDisarmsOnStopBackSettingsAndItsApp(t *testing.T) {
	g := serveGuard(t, guardSeam, nil)
	cli := dialSocket(t, protocol.ClientCLI)
	disarmed := func(what string) {
		t.Helper()
		g.desk.await(t, 0, "")
		if g.clock.Armed() != 0 || g.d.guardResident() {
			t.Fatalf("%s: %d timers, resident %v", what, g.clock.Armed(), g.d.guardResident())
		}
	}
	for _, action := range []string{protocol.OniFocusStop, protocol.OniFocusBack} {
		guardArm20(t, cli)
		g.desk.await(t, 1, "")
		if res := guardFocus(t, cli, protocol.OniFocusParams{Action: action}); res.Armed || res.Reopen != 0 {
			t.Fatalf("%s: %+v", action, res)
		}
		disarmed(action)
	}

	guardArm20(t, cli)
	g.desk.await(t, 1, "")
	g.patch(t, `{"oni":{"feeds":{"guard":false}}}`)
	disarmed("the guard switched off")
	g.patch(t, `{"oni":{"feeds":{"guard":true}}}`)
	guardArm20(t, cli)
	g.desk.await(t, 1, "")
	g.patch(t, `{"oni":{"master":false}}`)
	disarmed("Oni switched off")
	g.patch(t, `{"oni":{"master":true}}`)

	res := guardFocus(t, cli, protocol.OniFocusParams{Action: protocol.OniFocusArm, UntilApp: "org.example.Writer"})
	if !res.Armed || res.UntilApp != "org.example.Writer" {
		t.Fatalf("arm until an app: %+v", res)
	}
	g.desk.await(t, 1, "")
	writer := window.Window{ID: "40", App: "org.example.Writer", Title: "essay", Output: "DP-1", Visible: true}
	g.desk.set(window.Snapshot{Outputs: []window.Output{dp1}, Windows: []window.Window{writer}})
	if !g.d.guardResident() {
		t.Fatal("the app it lasts until is open, and the guard ended")
	}
	g.desk.set(window.Snapshot{Outputs: []window.Output{dp1}})
	disarmed("the app closed")
	g.noticed(t, "Focus time is over")
}

// A guard card is answered only while it is on screen and only by its id.
func TestOniGuardIgnoresAnUnknownCard(t *testing.T) {
	g := serveGuard(t, guardSeam, nil)
	app := g.dialApp(t)
	var ask protocol.OniAskResult
	err := app.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: feature.GuardCardPrefix + "0011223344556677", Answer: "yes"}, &ask)
	var perr *protocol.Error
	if !errors.As(err, &perr) || perr.Code != protocol.CodeNotFound {
		t.Fatalf("unknown card: %+v %v", ask, err)
	}
	if _, closes := g.closer.seen(); len(closes) != 0 {
		t.Fatalf("closes %v", closes)
	}
}

// Every oni.focus action passes the gate for the app and the CLI; the
// presenter only stops the guard; nobody else, an agent's tools or a team
// session's socket included, makes the call.
func TestOniGuardCallPassesTheGateByAction(t *testing.T) {
	d := oniDaemon(t)
	app := gateConn(t, d, protocol.ClientApp, true, "")
	cli := gateConn(t, d, protocol.ClientCLI, false, "")
	presenter := gateConn(t, d, protocol.ClientSurface, false, "")
	second := gateConn(t, d, protocol.ClientSurface, false, "")
	observer := gateConn(t, d, protocol.ClientCLI, false, "")
	bar := gateConn(t, d, protocol.ClientBar, false, "")
	hub := gateConn(t, d, protocol.ClientHub, false, "")
	notApp := gateConn(t, d, protocol.ClientApp, false, "")
	team := gateConn(t, d, protocol.ClientCLI, false, "agent-1")
	d.oni.clientsMu.Lock()
	d.oni.clients = map[*connection]*oniClient{
		presenter: {renderer: protocol.OniRendererNative},
		second:    {renderer: protocol.OniRendererNative},
		observer:  {renderer: protocol.OniRendererObserver},
	}
	d.oni.presenter = presenter
	d.oni.clientsMu.Unlock()

	calls := map[string]protocol.OniFocusParams{
		protocol.OniFocusStatus: {Action: protocol.OniFocusStatus},
		protocol.OniFocusArm:    {Action: protocol.OniFocusArm, Minutes: 20},
		protocol.OniFocusStop:   {Action: protocol.OniFocusStop},
		protocol.OniFocusBack:   {Action: protocol.OniFocusBack},
	}
	if len(calls) != len(protocol.OniFocusActions) {
		t.Fatalf("the test covers %d of %d actions", len(calls), len(protocol.OniFocusActions))
	}
	for action, p := range calls {
		raw := mustMarshal(t, p)
		for name, c := range map[string]*connection{"the app": app, "the CLI": cli} {
			if err := d.mainGate(c, protocol.MOniFocus, raw); err != nil {
				t.Errorf("%s was refused %s: %v", name, action, err)
			}
		}
		err := d.mainGate(presenter, protocol.MOniFocus, raw)
		if may := action == protocol.OniFocusStop; may && err != nil || !may && !forbidden(err) {
			t.Errorf("the presenter sent %s: %v", action, err)
		}
		for name, c := range map[string]*connection{
			"a surface that is not the presenter": second,
			"an observer":                         observer,
			"the bar":                             bar,
			"the hub":                             hub,
			"a program that is not the app":       notApp,
			"a team session's connection":         team,
		} {
			if err := d.mainGate(c, protocol.MOniFocus, raw); !forbidden(err) {
				t.Errorf("%s sent %s: %v", name, action, err)
			}
		}
		for _, info := range []protocol.SessionInfo{{ID: "ask-1", Kind: protocol.KindAsk}, {ID: "agent-1", Kind: protocol.KindAgent, Team: "t-1"}} {
			if err := d.hostGate(info, protocol.MOniFocus, raw); !forbidden(err) {
				t.Errorf("an agent's tools (team %q) sent %s: %v", info.Team, action, err)
			}
		}
	}
	if err := agentGate(protocol.MOniFocus); !forbidden(err) {
		t.Errorf("a team session's socket may call %s", protocol.MOniFocus)
	}
}
