package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/keep"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/protocol"
)

// Proactive hints. With oni.guide.hints on, Oni may offer a walkthrough
// the user kept for the app of the window it is perched on: Show me
// replays it with no model, Not now snoozes that app for the rest of the
// day, and a hint nobody touches retracts after the spec's time. A hint is
// decided on the perch and focus events the perch already follows; nothing
// watches the front app, and no picture is taken to decide one. At most
// one hint shows every cooldown and each kept walkthrough at most once a
// day, as keep.Hints records. With hints off nothing here runs and nothing
// is armed; with a hint out, its retract is the one timer.

// What a hint says, the kicker over it, and its consent frame's question.
var (
	oniHintLine     = i18n.T("I kept a walkthrough for this app. Want to see it again?")
	oniHintKicker   = i18n.T("Kept from before")
	oniHintQuestion = i18n.T("Show the kept walkthrough again?")
)

// guideHint is the hint a walk offer carries: its consent frame's id, and
// the kept walkthrough, app and window it was offered for.
type guideHint struct {
	id     string
	key    string
	app    string
	window window.ID
}

// oniHints is the hint record beside the kept walkthroughs, opened on first
// use.
func (d *Daemon) oniHints() *keep.Hints {
	keeps := d.oniKeeps()
	g := &d.oni.guide
	g.keepsMu.Lock()
	defer g.keepsMu.Unlock()
	if g.hints == nil {
		g.hints = keeps.Hints()
	}
	return g.hints
}

func oniHintID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "h-" + hex.EncodeToString(b[:]), nil
}

func hintsOn(settings protocol.OniSettings) bool {
	return feature.Decide(feature.Request{Feature: feature.GuideHints}, settings).Allow
}

// hintPerched considers a hint for w, the window Oni just perched on or
// the perched window taking focus again. It never waits, so a caller may
// hold a lock; with hints off it does nothing at all.
func (d *Daemon) hintPerched(w window.Window) {
	if w.ID == "" || strings.TrimSpace(w.App) == "" || !hintsOn(protocol.EffectiveOni(d.Settings())) {
		return
	}
	go d.guideDo(func() { d.hintOffer(w) })
}

// hintOffer shows a hint for a walkthrough kept for w's app, when one is
// due and nothing else is on the guide card. On the queue.
func (d *Daemon) hintOffer(w window.Window) {
	g := &d.oni.guide
	// a hint for another window goes: Oni perched elsewhere
	if g.walk != nil && g.walk.hint != nil && g.walk.hint.window != w.ID {
		d.guideDropWalk(true)
	}
	svc := d.oni.service.Load()
	if g.run != nil || g.walk != nil || svc == nil || !hintsOn(protocol.EffectiveOni(d.Settings())) || !d.hintPerchedOn(w.ID) {
		return
	}
	entries, err := d.oniKeeps().List()
	if err != nil {
		d.warn("oni hint: %v", err)
		return
	}
	app := strings.TrimSpace(w.App)
	var keys []string
	goals := map[string]string{}
	for _, e := range entries {
		if strings.EqualFold(e.App, app) {
			keys = append(keys, e.Key)
			goals[e.Key] = e.Goal
		}
	}
	if len(keys) == 0 {
		return
	}
	clock := d.oniGuideClock()
	key, err := d.oniHints().Offer(app, keys, clock.Now())
	if err != nil {
		d.warn("oni hint: %v", err)
		return
	}
	if key == "" {
		return
	}
	id, err := oniHintID()
	if err != nil {
		return
	}
	g.seq++
	if err := svc.Guide(feature.GuideFrame{Seq: g.seq, Phase: feature.GuideNudging, Kicker: oniHintKicker, Text: oniHintLine, Actions: []string{feature.GuideActionNext, feature.GuideActionSkip}}); err != nil {
		return
	}
	_ = svc.Consent(*feature.HintOffer(id, oniHintQuestion))
	walk := &guideWalk{goal: goals[key], hint: &guideHint{id: id, key: key, app: app, window: w.ID}}
	walk.timer = clock.AfterFunc(feature.HintOfferTTL(), func() {
		d.guideDo(func() {
			if g.walk == walk {
				d.guideDropWalk(true)
			}
		})
	})
	g.walk = walk
}

// hintPerchedOn says Oni is perched on window id right now.
func (d *Daemon) hintPerchedOn(id window.ID) bool {
	o := &d.oni.perch
	o.mu.Lock()
	current := o.window.ID
	o.mu.Unlock()
	if current != id {
		return false
	}
	svc := d.oni.service.Load()
	if svc == nil {
		return false
	}
	_, ok := svc.Perched()
	return ok
}

// hintSkip lets the hint id go with the user's Not now: its app is
// snoozed for the rest of the day. It says whether id named the hint out.
// On the queue.
func (d *Daemon) hintSkip(id string) bool {
	g := &d.oni.guide
	if g.walk == nil || g.walk.hint == nil || g.walk.hint.id != id {
		return false
	}
	if err := d.oniHints().Snooze(g.walk.hint.app, d.oniGuideClock().Now()); err != nil {
		d.warn("oni hint snooze: %v", err)
	}
	d.guideDropWalk(true)
	return true
}

// hintAnswer closes w's hint on the frames: yes when it was taken up, no
// when it went. On the queue.
func (d *Daemon) hintAnswer(w *guideWalk, taken bool) {
	if w.hint == nil || d.oni.service.Load() == nil {
		return
	}
	answer := "no"
	if taken {
		answer = "yes"
	}
	d.oniEmit(&feature.ConsentAnswerFrame{ID: w.hint.id, Answer: answer})
}

// hintsOff drops a hint out when hints were turned off.
func (d *Daemon) hintsOff() {
	go d.guideDo(func() {
		if w := d.oni.guide.walk; w != nil && w.hint != nil {
			d.guideDropWalk(true)
		}
	})
}
