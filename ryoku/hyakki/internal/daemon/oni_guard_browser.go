package daemon

import (
	"context"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/hands/page"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/guard"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/protocol"
)

// guardTabDriver exists only on the pipe-backed Hands page session. A user
// browser is never discovered, attached, or addressed by this interface.
type guardTabDriver interface {
	CloseGuardTab(context.Context, page.Scope, string) error
	ReopenGuardTab(context.Context, string) error
}

type guardBrowserClosed struct {
	browser *handsBrowser
	tab     string
	at      time.Time
	gen     uint64
}

// guardBrowser is called by window/tab events and authorization, not a timer.
// Only the current authorized target is eligible, even in our own browser.
func (d *Daemon) guardBrowser(gen uint64) {
	hb := d.handsCurrent()
	if hb == nil {
		return
	}
	scope := handsWait(d, func() page.Scope {
		if d.oni.hands.browser != hb || hb.scope == nil {
			return page.Scope{}
		}
		return *hb.scope
	})
	if scope.TabID == "" {
		return
	}
	if _, ok := hb.drv.(guardTabDriver); !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), guardCloseTimeout)
	defer cancel()
	tabs, err := hb.drv.Tabs(ctx)
	if err != nil {
		return
	}
	g := &d.oni.guard
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.armed || gen != 0 && g.gen != gen {
		return
	}
	for _, tab := range tabs {
		if tab.ID != scope.TabID || tab.Origin != scope.Origin {
			continue
		}
		hit := guard.Match("", tab.Title, g.words)
		if hit.Reach == guard.None {
			g.browserSeen = ""
			return
		}
		key := tab.ID + "\x00" + tab.Title
		if g.browserSeen == key || !d.guardCanAsk() {
			return
		}
		card, err := newGuardCard(window.Window{App: "Hands browser"}, hit.Feed)
		if err != nil {
			return
		}
		card.browser, card.tab, card.origin, card.title = hb, tab.ID, tab.Origin, tab.Title
		g.browserSeen = key
		g.cards = append(g.cards, card)
		if g.cards[0].timer == nil {
			d.guardShowLocked(g.gen)
		}
		return
	}
}

func (d *Daemon) guardCloseBrowser(ctx context.Context, gen uint64, card *guardCard) {
	// Serialize against browser replacement/authorization. The session performs
	// the final exact title/origin check immediately before Target.closeTarget.
	h := &d.oni.hands
	h.ops.Lock()
	defer h.ops.Unlock()
	if d.handsCurrent() != card.browser {
		return
	}
	g := &d.oni.guard
	g.mu.Lock()
	live := g.armed && g.gen == gen && d.guardClock().Now().Before(g.until) && guard.Match("", card.title, g.words).Feed == card.feed
	if !live {
		g.mu.Unlock()
		return
	}
	driver, ok := card.browser.drv.(guardTabDriver)
	if !ok {
		g.mu.Unlock()
		return
	}
	scope := page.Scope{TabID: card.tab, Origin: card.origin}
	if err := driver.CloseGuardTab(ctx, scope, card.title); err != nil {
		g.mu.Unlock()
		d.guardSay(i18n.T("The Hands tab changed before I could close it, so I left it."))
		return
	}
	g.reopen = &guardBrowserClosed{browser: card.browser, tab: card.tab, at: d.guardClock().Now(), gen: gen}
	g.closed++
	closed, left := g.closed, g.until.Sub(d.guardClock().Now())
	g.mu.Unlock()
	handsWait(d, func() bool {
		if h.browser == card.browser {
			card.browser.scope = nil
			d.handsStopRun(h.run, handsByClosed)
		}
		return true
	})
	d.guardSay(i18n.T("Closed the blocked feed in the Hands browser."))
	d.guardFrame(feature.FocusArmed, left, closed)
}

func (d *Daemon) guardReopen() {
	if !feature.Decide(feature.Request{Feature: feature.FeedGuard}, protocol.EffectiveOni(d.Settings())).Allow {
		return
	}
	g := &d.oni.guard
	g.mu.Lock()
	r := g.reopen
	g.reopen = nil
	g.mu.Unlock()
	if r == nil {
		return
	}
	age := d.guardClock().Now().Sub(r.at)
	if age < 0 || age > guardReopenWindow {
		return
	}
	h := &d.oni.hands
	h.ops.Lock()
	defer h.ops.Unlock()
	if d.handsCurrent() != r.browser {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.armed || g.gen != r.gen {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), guardCloseTimeout)
	defer cancel()
	if driver, ok := r.browser.drv.(guardTabDriver); ok {
		if err := driver.ReopenGuardTab(ctx, r.tab); err == nil {
			d.guardSay(i18n.T("Reopened the Hands tab. Authorize it again before Hands can act."))
		}
	}
}
