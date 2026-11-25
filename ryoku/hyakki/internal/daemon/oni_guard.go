package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/feedclose"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/guard"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/reflex/launch"
	"ryoku-rashin/internal/reflex/proc"
)

// The focus guard watches compositor events only while armed. A dedicated
// feed app gets a per-action app card before the Ryoku seam closes it;
// title-only matches in ordinary browsers are announced, never closed.
// The browser half can close only the current authorized tab in Hands'
// pipe-owned browser, with a fresh origin/title check. Its single retained
// address can be reopened briefly, never silently reauthorized.

// OniGuardDeps are what the focus guard closes windows and keeps time
// with; each nil field is the real one.
type OniGuardDeps struct {
	// Closer closes a window through the Ryoku seam.
	Closer GuardCloser
	// Clock times the guard's deadline and its cards.
	Clock presence.Clock
}

// GuardCloser closes one window for the guard: Supported reports whether
// any window can be closed in this session, and Close closes id only while
// it is still app's window. feedclose.Closer is the real one.
type GuardCloser interface {
	Supported(ctx context.Context) bool
	Close(ctx context.Context, id, app string) error
}

// guardCloseTimeout bounds one close: the seam's re-read and its act.
const (
	guardCloseTimeout = 5 * time.Second
	guardReopenWindow = 10 * time.Minute
)

// Why a guard ended.
const (
	guardByTime     = "time"
	guardByApp      = "app"
	guardByUser     = "user"
	guardByBack     = "back"
	guardBySettings = "settings"
	guardByRearm    = "rearm"
)

// oniGuard owns the armed watch and one closed Hands-tab return candidate.
type oniGuard struct {
	mu sync.Mutex
	// gen counts arms; a watch event or a timer from an older arm does
	// nothing
	gen   uint64
	armed bool
	words []string
	until time.Time
	// untilApp ends the guard once the app, seen open, has no window left
	untilApp  string
	untilSeen bool
	// timer is the one deadline timer; stop ends the window watch
	timer presence.Timer
	stop  context.CancelFunc
	// provider is the desktop watched; seam is whether its windows can be
	// closed through the Ryoku seam
	provider window.Provider
	seam     bool
	// browserCatalog is read once per explicit arm, never polled at rest.
	browserCatalog *launch.Catalog
	// seen is each window's feed already asked about or announced, so a
	// window is carded or said once per feed
	seen map[window.ID]string
	// cards wait in order; the first is on screen
	cards []*guardCard
	// closed and announced count this arm's acts
	closed, announced int
	// The browser half retains only the last tab it actually closed.
	browserSeen string
	reopen      *guardBrowserClosed
}

// guardCard is one close card: the window, its app and the feed it names.
type guardCard struct {
	id                 string
	win                window.ID
	app                string
	feed               string
	timer              presence.Timer
	browser            *handsBrowser
	tab, origin, title string
}

func (d *Daemon) guardClock() presence.Clock {
	if c := d.deps.OniGuard.Clock; c != nil {
		return c
	}
	return presence.SystemClock()
}

func (d *Daemon) guardCloser() GuardCloser {
	if c := d.deps.OniGuard.Closer; c != nil {
		return c
	}
	return feedclose.New(proc.Exec(os.Environ()))
}

// guardLimits is the spec's focus section.
func guardLimits() (spec.Focus, error) {
	s, err := spec.Load()
	if err != nil {
		return spec.Focus{}, err
	}
	return s.Focus, nil
}

// guardResident reports whether an armed guard keeps the daemon up.
func (d *Daemon) guardResident() bool {
	g := &d.oni.guard
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.armed
}

// oniFocusCall serves oni.focus.
func (d *Daemon) oniFocusCall(p protocol.OniFocusParams) (protocol.OniFocusResult, error) {
	switch p.Action {
	case protocol.OniFocusArm:
		return d.guardArm(p)
	case protocol.OniFocusStop:
		d.guardDisarm(0, guardByUser)
	case protocol.OniFocusBack:
		d.guardDisarm(0, guardByBack)
		d.guardReopen()
	}
	return d.guardResult(), nil
}

// guardResult is the guard as it stands.
func (d *Daemon) guardResult() protocol.OniFocusResult {
	enabled := feature.Decide(feature.Request{Feature: feature.FeedGuard}, protocol.EffectiveOni(d.Settings())).Allow
	g := &d.oni.guard
	g.mu.Lock()
	defer g.mu.Unlock()
	now := d.guardClock().Now()
	reopen := 0
	if r := g.reopen; enabled && r != nil && r.gen == g.gen {
		if age := now.Sub(r.at); age >= 0 && age <= guardReopenWindow {
			reopen = 1
		}
	}
	if !g.armed {
		return protocol.OniFocusResult{Reopen: reopen}
	}
	left := g.until.Sub(now)
	if left < 0 {
		left = 0
	}
	return protocol.OniFocusResult{Armed: true, LeftMs: left.Milliseconds(), UntilApp: g.untilApp, Words: append([]string(nil), g.words...), Closed: g.closed, Announced: g.announced, Reopen: reopen}
}

// guardArm arms the guard, replacing any armed one. A refusal is the
// result's Refused, and leaves nothing armed.
func (d *Daemon) guardArm(p protocol.OniFocusParams) (protocol.OniFocusResult, error) {
	refuse := func(why string) (protocol.OniFocusResult, error) {
		return protocol.OniFocusResult{Refused: why}, nil
	}
	settings := protocol.EffectiveOni(d.Settings())
	if !feature.Decide(feature.Request{Feature: feature.FeedGuard}, settings).Allow {
		return refuse(protocol.OniFocusRefusedOff)
	}
	words := settings.Feeds.Words
	if len(p.Words) > 0 {
		words = p.Words
	}
	words, _ = protocol.CleanOniFeedWords(words)
	if len(words) == 0 {
		return refuse(protocol.OniFocusRefusedWords)
	}
	limits, err := guardLimits()
	if err != nil {
		return protocol.OniFocusResult{}, protocol.Errorf(protocol.CodeInternal, "%s", i18n.Tf("Oni's spec could not be read: %v.", err))
	}
	most := limits.Max()
	span := most
	if p.UntilApp == "" {
		span = time.Duration(p.Minutes) * time.Minute
		if span > most {
			return refuse(protocol.OniFocusRefusedTooLong)
		}
	}
	provider, err := d.oniProvider()
	if err != nil {
		return refuse(protocol.OniFocusRefusedDesktop)
	}
	caps := provider.Caps()
	if !caps.Events {
		return refuse(protocol.OniFocusRefusedEvents)
	}
	seam := false
	if caps.Provider == "ryoku" {
		ctx, cancel := context.WithTimeout(context.Background(), guardCloseTimeout)
		seam = d.guardCloser().Supported(ctx)
		cancel()
	}
	browserCatalog := d.handsCatalog()
	d.guardDisarm(0, guardByRearm)

	g := &d.oni.guard
	g.mu.Lock()
	g.gen++
	gen := g.gen
	clock := d.guardClock()
	g.armed, g.words, g.until = true, words, clock.Now().Add(span)
	g.untilApp, g.untilSeen = strings.TrimSpace(p.UntilApp), false
	g.provider, g.seam = provider, seam
	g.browserCatalog = browserCatalog
	g.seen, g.cards, g.closed, g.announced = map[window.ID]string{}, nil, 0, 0
	g.timer = clock.AfterFunc(span, func() { d.guardDisarm(gen, guardByTime) })
	ctx, stop := context.WithCancel(context.Background())
	g.reopen = nil
	g.browserSeen = ""
	g.stop = stop
	g.mu.Unlock()

	go func() {
		if s, err := provider.Snapshot(ctx); err == nil {
			d.guardWindows(gen, s)
		}
		_ = provider.Watch(ctx, func(s window.Snapshot) { d.guardWindows(gen, s) })
	}()
	d.guardFrame(feature.FocusArmed, span, 0)
	return d.guardResult(), nil
}

// guardDisarm ends the guard: arm gen's, or whichever is armed when gen is
// 0. It stops the deadline timer, the watch and any card, says why where
// the user did not ask for it, and lets the daemon idle again.
func (d *Daemon) guardDisarm(gen uint64, why string) {
	g := &d.oni.guard
	g.mu.Lock()
	if !g.armed || gen != 0 && gen != g.gen {
		g.mu.Unlock()
		return
	}
	closed := g.closed
	if g.timer != nil {
		g.timer.Stop()
	}
	if g.stop != nil {
		g.stop()
	}
	shown := ""
	for i, c := range g.cards {
		if c.timer != nil {
			c.timer.Stop()
			if i == 0 {
				shown = c.id
			}
		}
	}
	g.armed, g.words, g.until, g.untilApp, g.untilSeen = false, nil, time.Time{}, "", false
	g.timer, g.stop, g.provider, g.seam = nil, nil, nil, false
	g.seen, g.cards, g.closed, g.announced = nil, nil, 0, 0
	g.browserCatalog = nil
	g.mu.Unlock()
	if shown != "" {
		d.guardCardClosed(shown, "no")
	}
	switch why {
	case guardByRearm:
		return
	case guardByTime, guardByApp:
		d.guardSay(i18n.T("Focus time is over. Feeds are open to you again."))
	}
	d.guardFrame(feature.FocusDone, 0, closed)
	d.stateChanged()
}

// guardSettings disarms a guard armed under other rules.
func (d *Daemon) guardSettings(current, next protocol.Settings) {
	if !d.guardResident() {
		return
	}
	was := feature.Decide(feature.Request{Feature: feature.FeedGuard}, protocol.EffectiveOni(current))
	now := feature.Decide(feature.Request{Feature: feature.FeedGuard}, protocol.EffectiveOni(next))
	if was != now || !now.Allow || strings.Join(protocol.EffectiveOni(current).Feeds.Words, "\x00") != strings.Join(protocol.EffectiveOni(next).Feeds.Words, "\x00") {
		d.guardDisarm(0, guardBySettings)
	}
}

// guardWindows hears one window event of arm gen's watch.
func (d *Daemon) guardWindows(gen uint64, s window.Snapshot) {
	g := &d.oni.guard
	g.mu.Lock()
	if !g.armed || gen != g.gen {
		g.mu.Unlock()
		return
	}
	if g.untilApp != "" {
		open := false
		for _, w := range s.Windows {
			open = open || strings.EqualFold(w.App, g.untilApp)
		}
		if open {
			g.untilSeen = true
		} else if g.untilSeen {
			g.mu.Unlock()
			d.guardDisarm(gen, guardByApp)
			return
		}
	}
	present := map[window.ID]bool{}
	var says []string
	for _, w := range s.Windows {
		present[w.ID] = true
		hit := guard.Match(w.App, w.Title, g.words)
		if hit.Reach == guard.Close && guardBrowserIdentity(w.App, g.browserCatalog) {
			hit.Reach = guard.Announce
		}
		if hit.Reach == guard.None || g.seen[w.ID] == hit.Feed {
			continue
		}
		g.seen[w.ID] = hit.Feed
		switch {
		case hit.Reach == guard.Close && g.seam && d.guardCanAsk():
			card, err := newGuardCard(w, hit.Feed)
			if err != nil {
				continue
			}
			g.cards = append(g.cards, card)
		case hit.Reach == guard.Close && !g.seam:
			g.announced++
			says = append(says, i18n.Tf("%s is open, and %s is on your blocked list. This desktop gives me no way to close windows.", guardApp(w.App), hit.Feed))
		default:
			g.announced++
			says = append(says, i18n.Tf("%s is open in %s, and it's on your blocked list.", hit.Feed, guardApp(w.App)))
		}
	}
	for id := range g.seen {
		if !present[id] {
			delete(g.seen, id)
		}
	}
	// a waiting card whose window is gone is never shown
	kept := g.cards[:0]
	for i, c := range g.cards {
		if c.browser != nil || i == 0 && c.timer != nil || present[c.win] {
			kept = append(kept, c)
		}
	}
	g.cards = kept
	if len(g.cards) > 0 && g.cards[0].timer == nil {
		d.guardShowLocked(gen)
	}
	// Browser discovery is a bounded read on this window event, never polling.
	g.mu.Unlock()
	for _, line := range says {
		d.guardSay(line)
	}
	d.guardBrowser(gen)
}

// guardCanAsk reports whether a card can be shown: Oni is on screen.
func (d *Daemon) guardCanAsk() bool {
	return d.oni.service.Load() != nil && !protocol.EffectiveOni(d.Settings()).Hidden
}

func newGuardCard(w window.Window, feed string) (*guardCard, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return nil, err
	}
	return &guardCard{id: feature.GuardCardPrefix + hex.EncodeToString(b[:]), win: w.ID, app: w.App, feed: feed}, nil
}

// guardApp is an app id as a line names it.
func guardApp(app string) string {
	if app == "" {
		return i18n.T("a window")
	}
	return oniText(app, protocol.OniFocusAppMax)
}

// guardShowLocked puts the first waiting card on screen with its own
// expiry; a card that cannot be shown is dropped and the next tried. The
// caller holds g.mu.
func (d *Daemon) guardShowLocked(gen uint64) {
	g := &d.oni.guard
	for len(g.cards) > 0 {
		card := g.cards[0]
		svc := d.oni.service.Load()
		question := i18n.Tf("Close %s? It's one of your blocked feeds: %s.", guardApp(card.app), card.feed)
		if svc != nil && svc.Consent(*feature.GuardCard(card.id, oniText(question, 400))) == nil {
			card.timer = d.guardClock().AfterFunc(feature.ConsentTTL, func() { d.guardCardExpired(gen, card) })
			return
		}
		g.cards = g.cards[1:]
	}
}

func (d *Daemon) guardCardExpired(gen uint64, card *guardCard) {
	g := &d.oni.guard
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.armed || gen != g.gen || len(g.cards) == 0 || g.cards[0] != card {
		return
	}
	g.cards = g.cards[1:]
	d.guardCardClosed(card.id, "no")
	d.guardShowLocked(gen)
}

// guardCardClosed takes the card id names off the renderers' screen with
// its answer, an expired or dropped card's being no.
func (d *Daemon) guardCardClosed(id, answer string) {
	if svc := d.oni.service.Load(); svc != nil {
		_ = svc.ConsentAnswer(feature.ConsentAnswerFrame{ID: id, Answer: answer})
	}
}

// guardCardWaiting is the id of the guard card on screen, or "".
func (d *Daemon) guardCardWaiting() string {
	g := &d.oni.guard
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.armed || len(g.cards) == 0 || g.cards[0].timer == nil {
		return ""
	}
	return g.cards[0].id
}

// guardCardTake answers the card id names. Yes closes its window, on its
// own goroutine; no leaves it open. The next card, if any, takes its
// place. It reports false when id is not the card on screen.
func (d *Daemon) guardCardTake(id string, allow bool) bool {
	g := &d.oni.guard
	g.mu.Lock()
	if !g.armed || id == "" || len(g.cards) == 0 || g.cards[0].id != id || g.cards[0].timer == nil {
		g.mu.Unlock()
		return false
	}
	card, gen, provider := g.cards[0], g.gen, g.provider
	card.timer.Stop()
	g.cards = g.cards[1:]
	answer := "no"
	if allow {
		answer = "yes"
	}
	d.guardCardClosed(card.id, answer)
	d.guardShowLocked(gen)
	g.mu.Unlock()
	if allow {
		go d.guardClose(gen, provider, card)
	}
	return true
}

// guardCardAnswer serves oni.consent.answer for a guard card. It reports
// false when id is not the guard card on screen.
func (d *Daemon) guardCardAnswer(p protocol.OniConsentAnswerParams) (protocol.OniAskResult, bool) {
	if !strings.HasPrefix(p.ID, feature.GuardCardPrefix) {
		return protocol.OniAskResult{}, false
	}
	allow := p.Answer == "yes"
	if !d.guardCardTake(p.ID, allow) {
		return protocol.OniAskResult{}, false
	}
	if !allow {
		return protocol.OniAskResult{Status: protocol.OniAskDeclined}, true
	}
	return protocol.OniAskResult{Status: protocol.OniAskAnswered}, true
}

// guardClose closes the card's window after a yes: only while arm gen is
// still armed, and only after a fresh read still shows the window under
// the app whose id names the feed. The seam reads it once more itself.
func (d *Daemon) guardClose(gen uint64, provider window.Provider, card *guardCard) {
	ctx, cancel := context.WithTimeout(context.Background(), guardCloseTimeout)
	defer cancel()
	if card.browser != nil {
		d.guardCloseBrowser(ctx, gen, card)
		return
	}
	g := &d.oni.guard
	g.mu.Lock()
	words := g.words
	live := g.armed && gen == g.gen
	g.mu.Unlock()
	if !live {
		return
	}
	app := guardApp(card.app)
	s, err := provider.Snapshot(ctx)
	if err != nil {
		d.guardSay(i18n.Tf("I couldn't close %s.", app))
		return
	}
	still := false
	for _, w := range s.Windows {
		if w.ID == card.win && w.App == card.app {
			hit := guard.Match(w.App, "", words)
			still = hit.Reach == guard.Close && hit.Feed == card.feed
		}
	}
	// Re-read desktop metadata at the action boundary: a queued card never
	// grants lasting permission over an app now identified as a browser.
	if still && !guardBrowserIdentity(card.app, d.handsCatalog()) {
		err = d.guardCloser().Close(ctx, string(card.win), card.app)
	} else {
		err = feedclose.ErrChanged
	}
	switch {
	case errors.Is(err, feedclose.ErrChanged):
		d.guardSay(i18n.Tf("%s changed before I could close it, so I left it.", app))
		return
	case err != nil:
		d.guardSay(i18n.Tf("I couldn't close %s.", app))
		return
	}
	g.mu.Lock()
	if !g.armed || gen != g.gen {
		g.mu.Unlock()
		return
	}
	g.closed++
	closed, left := g.closed, g.until.Sub(d.guardClock().Now())
	g.mu.Unlock()
	d.guardSay(i18n.Tf("Closed %s.", app))
	d.guardFrame(feature.FocusArmed, left, closed)
}

// guardSay puts one of the guard's lines in Oni's bubble, or in a desktop
// notification while Oni is hidden, not on screen or has no strand out to
// speak from.
func (d *Daemon) guardSay(line string) {
	settings := protocol.EffectiveOni(d.Settings())
	if svc := d.oni.service.Load(); svc != nil && !settings.Hidden && svc.Bubble(0, "answer", oniText(line, oniBubbleMax)) == nil {
		return
	}
	d.reflexNotify(oniActiveProfile(settings.Character).Name, line)
}

// guardFrame tells the renderers where the guard stands: armed with the
// time left and the windows closed, or done. An armed guard with no time
// left is about to end and its done frame follows.
func (d *Daemon) guardFrame(phase string, left time.Duration, closed int) {
	if d.oni.service.Load() == nil || phase == feature.FocusArmed && left < time.Millisecond {
		return
	}
	if phase != feature.FocusArmed {
		left = 0
	}
	d.oniEmit(&feature.FocusFrame{Phase: phase, LeftMs: left.Milliseconds(), Closed: closed})
}
