package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/oni/reflex/decide"
	"ryoku-rashin/internal/oni/remind"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/reflex/act"
	"ryoku-rashin/internal/reflex/audio"
	"ryoku-rashin/internal/reflex/browser"
	"ryoku-rashin/internal/reflex/bus"
	"ryoku-rashin/internal/reflex/launch"
	"ryoku-rashin/internal/reflex/notify"
	"ryoku-rashin/internal/reflex/proc"
	"ryoku-rashin/internal/reflex/shot"
	"ryoku-rashin/internal/reflex/wm"
	"ryoku-rashin/internal/sensitive"
)

// Quick commands run in the daemon: the router's Reflex slot plans an
// utterance through the reflex rules (and, with decisions on, the typed
// decision route for what the rules miss), and a Reflexed decision is
// carried out here, action by action, under the spec's caps. Each action is
// announced in at most five words and leaves one ledger row: intent, route,
// time, confidence and outcome, never the words said. With reflex off
// nothing here is built, started or armed.

// oniReflexState is the quick commands' state; everything is built on first use
// with reflex on and dropped when it goes off.
type oniReflexState struct {
	mu sync.Mutex
	// session keeps the bounds that span utterances: the waiting verb,
	// the launch cooldown and the per-minute cap of what is planned
	session reflex.Session
	built   bool
	exec    act.Executor
	cat     reflex.Catalog
	chooser *decide.Chooser
	// plans are the decided plans' route, time and confidence, by plan,
	// until the voice loop takes them
	plans map[*reflex.Plan]reflexMeta
	// ran are the actions carried out in the last minute, voice and
	// replay together
	ran []time.Time
	// rows is the ledger, oldest first, bounded by reflex.ledger.maxRows
	rows []protocol.OniReflexRow

	// bound is the reviewed account a Press answered the decision card
	// for, declined the one a no was given for (the card is not shown for
	// it again); card is the card waiting; hosted are the hosted decisions
	// asked in the last minute
	bound    string
	declined string
	card     *reflexCard
	hosted   []time.Time
}

// reflexMeta is how a plan was decided.
type reflexMeta struct {
	route string
	ms    int64
}

// reflexCard is the hosted-decision disclosure card on screen.
type reflexCard struct {
	id      string
	account string
	timer   presence.Timer
}

// reflexRunOpts says how a run of actions came about. Turn is the voice
// turn it answers (0 for a replay the app or the CLI asked for); Source is
// that turn's voice source; Route the decision route for the ledger; Gen
// the voice loop's generation when the run began (0 checks only the turn);
// Press is set only from a valid route.Press; Replay marks a routine
// replay, which never feeds the routine recorder and refuses what may not
// be replayed.
type reflexRunOpts struct {
	Turn   uint64
	Source string
	Route  string
	Gen    uint64
	Press  bool
	Replay bool
	// utter is the utterance a voice run answers; ms the time its plan
	// took
	utter route.Utterance
	ms    int64
}

// reflexStep is what one action did.
type reflexStep struct {
	Action  reflex.Action
	Done    bool
	Line    string
	Outcome string
}

// Lines said by the daemon's own actions. Each is one sentence of at most
// five words.
const (
	lineReflexEnough   = "That's enough for now."
	lineReflexPress    = route.ReflexNeedsPressLine
	lineReflexCant     = act.LineCantHere
	lineReflexMoved    = "Moved."
	lineReflexHome     = "Back home."
	lineReflexBigger   = "Bigger now."
	lineReflexSmaller  = "Smaller now."
	lineReflexBiggest  = "That's my biggest."
	lineReflexSmallest = "That's my smallest."
	lineReflexFailed   = act.LineFailed
	lineNoteKept       = "I'll remember that."
	lineNoteOff        = "Memory is off."
	lineNoteNotesOff   = "Notes are off."
	lineNoteRefused    = "I won't keep that. Card numbers, passwords and ID numbers stay with you."
	lineNoteWontKeep   = "I won't keep that."
)

// reflexFlips are the settings a spoken flip may change, each only
// narrowing what Oni does or changing how it looks, and the line said.
var reflexFlips = map[string]struct {
	patch map[string]any
	line  string
}{
	"sounds=off":       {map[string]any{"sounds": false}, "Sounds off."},
	"sounds=on":        {map[string]any{"sounds": true}, "Sounds on."},
	"reduceMotion=on":  {map[string]any{"reduceMotion": protocol.OniReduceOn}, "Less motion now."},
	"reduceMotion=off": {map[string]any{"reduceMotion": protocol.OniReduceOff}, "Full motion now."},
	"hidden=on":        {map[string]any{"hidden": true}, "Hiding now."},
	"voice=off":        {map[string]any{"voice": false}, "Voice off."},
	"reflex=off":       {map[string]any{"reflex": false}, "Quick commands off."},
}

// reflexClock is the quick commands' clock.
func (d *Daemon) reflexClock() presence.Clock {
	if c := d.deps.OniReflex.Clock; c != nil {
		return c
	}
	return presence.SystemClock()
}

// reflexLimits is the spec's reflex section.
func reflexLimits() spec.Reflex {
	sp, err := spec.Load()
	if err != nil {
		return spec.Reflex{}
	}
	return sp.Reflex
}

// reflexOn reports whether quick commands are allowed now.
func (d *Daemon) reflexOn() bool {
	return feature.Decide(feature.Request{Feature: feature.Reflex}, protocol.EffectiveOni(d.Settings())).Allow
}

// reflexBuildLocked builds the executors and the catalog on first use. The
// caller holds r.mu.
func (d *Daemon) reflexBuildLocked() {
	r := &d.oni.reflex
	if r.built {
		return
	}
	r.built = true
	r.exec, r.cat = d.deps.OniReflex.Exec, d.deps.OniReflex.Catalog
	if r.exec != nil && r.cat != nil {
		return
	}
	env := reflexEnv(os.Environ())
	lim := reflexLimits()
	desktops := strings.Split(lookupEnv(env, "XDG_CURRENT_DESKTOP"), ":")
	cat := launch.Scan(launch.ScanOptions{Dirs: launch.Dirs(env), Max: lim.CatalogMax, Desktops: desktops, LookPath: exec.LookPath})
	launcher := launch.New(cat, launch.Options{Env: env, Cooldown: lim.LaunchCooldown()})
	deps := act.Deps{
		Launcher: launcher,
		Volume:   audio.NewVolume(proc.Exec(env)),
		Media:    audio.NewMedia(bus.Session),
		WM:       wm.New(env, proc.Exec(env)),
	}
	if e, ok := browser.Default(cat, browser.Mimeapps(env)); ok {
		if b, err := browser.New(launcher, e.ID, ""); err == nil {
			deps.Browser = b
			cat.Alias("browser", e.ID)
		}
	}
	if pics := shot.PicturesDir(env); pics != "" {
		home := lookupEnv(env, "HOME")
		deps.Shot = shot.New(shot.Options{Dir: filepath.Join(pics, "Screenshots"), LookPath: exec.LookPath, Run: proc.Exec(env), Spawn: proc.Detached(env, home)})
	}
	if r.exec == nil {
		r.exec = act.New(deps)
	}
	if r.cat == nil {
		r.cat = cat
	}
}

// reflexEnv is the environment launched apps and the tools start with: the
// daemon's own, keeping the session bus and the display so an app opens on
// the user's desktop, without the daemon's and its service manager's own
// markers.
func reflexEnv(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		key, _, _ := strings.Cut(kv, "=")
		switch {
		case strings.HasPrefix(key, "HYAKKI_"),
			slices.Contains([]string{"NOTIFY_SOCKET", "LISTEN_FDS", "LISTEN_PID", "LISTEN_FDNAMES", "INVOCATION_ID", "JOURNAL_STREAM", "WATCHDOG_PID", "WATCHDOG_USEC", "MANAGERPID"}, key):
			continue
		}
		out = append(out, kv)
	}
	return out
}

func lookupEnv(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if k, v, ok := strings.Cut(env[i], "="); ok && k == key {
			return v
		}
	}
	return ""
}

// reflexCatalog is the catalog the executors open apps from, built on
// first use; nil with reflex off.
func (d *Daemon) reflexCatalog() reflex.Catalog {
	if !d.reflexOn() {
		return nil
	}
	r := &d.oni.reflex
	r.mu.Lock()
	defer r.mu.Unlock()
	d.reflexBuildLocked()
	return r.cat
}

// reflexSettings follows the settings: reflex off drops everything built,
// the waiting card and the account a card bound; decisions off drop the
// card and the binding.
func (d *Daemon) reflexSettings(next protocol.Settings) {
	eff := protocol.EffectiveOni(next)
	r := &d.oni.reflex
	r.mu.Lock()
	defer r.mu.Unlock()
	if !feature.Decide(feature.Request{Feature: feature.Decisions}, eff).Allow {
		d.reflexDropCardLocked()
		r.bound, r.declined, r.hosted, r.chooser = "", "", nil, nil
	}
	if feature.Decide(feature.Request{Feature: feature.Reflex}, eff).Allow {
		return
	}
	r.session = reflex.Session{}
	r.built, r.exec, r.cat = false, nil, nil
	r.plans, r.ran = nil, nil
}

// reflexAccountChanged drops the account a card bound when the signed-in
// account is no longer it, and lifts the out-of-credit pin.
func (d *Daemon) reflexAccountChanged() {
	r := &d.oni.reflex
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.bound == "" && r.declined == "" && r.card == nil && r.chooser == nil {
		return
	}
	current := d.reflexAccount()
	if r.bound != "" && r.bound != current {
		r.bound, r.hosted = "", nil
	}
	if r.declined != current {
		r.declined = ""
	}
	if r.card != nil && r.card.account != current {
		d.reflexDropCardLocked()
	}
	if r.chooser != nil {
		r.chooser.Restore()
	}
}

// reflexAccount is the signed-in account's id, "" for none.
func (d *Daemon) reflexAccount() string {
	id, _ := d.reflexAccountShown()
	return id
}

func (d *Daemon) reflexAccountShown() (id, display string) {
	if d.deps.Account == nil {
		return "", ""
	}
	st := d.deps.Account.Status()
	if st.Account == nil {
		return "", ""
	}
	display = st.Account.Display
	if display == "" {
		display = st.Account.ID
	}
	return st.Account.ID, display
}

// reflexPlan is the router's Reflex slot: it plans, and never acts. The
// rules answer first; with decisions on, what they miss goes to the typed
// decision route. It reports false, leaving the utterance on the slow path,
// for anything neither takes.
func (d *Daemon) reflexPlan(u route.Utterance, p route.Press) (route.Decision, bool) {
	if !d.reflexOn() {
		return route.Decision{}, false
	}
	r := &d.oni.reflex
	at := u.At
	if at.IsZero() {
		at = d.reflexClock().Now()
	}
	started := time.Now()
	r.mu.Lock()
	d.reflexBuildLocked()
	cat := r.cat
	requested := guardReflexCatalog{daemon: d, catalog: cat}
	if !p.Valid() {
		// a plan only a press may start is handed back for the router to
		// refuse, without counting against the session's bounds
		if dry, ok := reflex.Parse(u.Text, at, nil, &requested); ok && slices.ContainsFunc(dry.Actions, pressOnly) {
			r.mu.Unlock()
			return route.Decision{Plan: &dry}, true
		}
	}
	plan, ok := r.session.Plan(u.Text, at, &requested)
	if ok {
		if requested.line != "" {
			r.mu.Unlock()
			return route.Decision{Line: requested.line}, true
		}
		if len(plan.Actions) > 0 {
			d.reflexMetaLocked(&plan, reflexMeta{route: protocol.DecisionRules, ms: time.Since(started).Milliseconds()})
		}
		r.mu.Unlock()
		if len(plan.Actions) == 0 && plan.Line == "" {
			// a verb waits for its object: nothing is said or asked
			return route.Decision{}, true
		}
		return route.Decision{Plan: &plan, Line: plan.Line}, true
	}
	r.mu.Unlock()
	if requested.line != "" {
		return route.Decision{Line: requested.line}, true
	}
	a, meta, target, ok := d.reflexDecide(u, p, cat)
	if !ok {
		return route.Decision{}, false
	}
	if line := d.guardDecidedRequestRefusal(a, target); line != "" {
		return route.Decision{Line: line}, true
	}
	decided := reflex.Plan{Actions: []reflex.Action{a}}
	r.mu.Lock()
	d.reflexMetaLocked(&decided, meta)
	r.mu.Unlock()
	return route.Decision{Plan: &decided}, true
}

// reflexMetaLocked keeps how plan was decided until the voice loop takes
// it. Plans the loop dropped are let go once a few are waiting.
func (d *Daemon) reflexMetaLocked(plan *reflex.Plan, meta reflexMeta) {
	r := &d.oni.reflex
	if r.plans == nil || len(r.plans) >= 16 {
		r.plans = map[*reflex.Plan]reflexMeta{}
	}
	r.plans[plan] = meta
}

// reflexDecide asks the typed decision route about what the rules missed.
// The hosted route is reachable only when providers.decisions pins it or
// allows it, reflexSettings.decisions is on, the utterance came with a
// Press, a Press answered the disclosure card for the account signed in
// now, and fewer than reflex.decisions.hostedPerMin hosted decisions were
// asked this minute; otherwise no account rides the request and nothing
// can be spent.
func (d *Daemon) reflexDecide(u route.Utterance, p route.Press, cat reflex.Catalog) (reflex.Action, reflexMeta, string, bool) {
	settings := d.Settings()
	eff := protocol.EffectiveOni(settings)
	if !feature.Decide(feature.Request{Feature: feature.Decisions}, eff).Allow || d.deps.FastDecisions == nil {
		return reflex.Action{}, reflexMeta{}, "", false
	}
	cfg := settings.Providers.Decisions
	via := cfg.Features[decide.Feature]
	if via == "" {
		via = cfg.Route
	}
	if via == "" || via == protocol.DecisionRules {
		return reflex.Action{}, reflexMeta{}, "", false
	}
	hostedOK := via == protocol.DecisionEmber || via == protocol.DecisionAuto && cfg.AllowHosted
	lim := reflexLimits().Decisions
	r := &d.oni.reflex
	now := d.reflexClock().Now()
	account := ""
	r.mu.Lock()
	if r.chooser == nil {
		r.chooser = &decide.Chooser{Tier0: decide.Rules(reflexTier0(cat)), Route: d.deps.FastDecisions, Limits: lim}
	}
	chooser := r.chooser
	if hostedOK && p.Valid() {
		current, display := d.reflexAccountShown()
		switch {
		case current == "":
		case r.bound != current:
			d.reflexOfferCardLocked(current, display)
		default:
			kept := r.hosted[:0]
			for _, t := range r.hosted {
				if now.Sub(t) < time.Minute && !now.Before(t) {
					kept = append(kept, t)
				}
			}
			r.hosted = kept
			if len(r.hosted) < lim.HostedPerMin {
				r.hosted = append(r.hosted, now)
				account = current
			}
		}
	}
	r.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), lim.Budget()+250*time.Millisecond)
	defer cancel()
	out := chooser.Choose(ctx, decide.Ask{Utterance: u.Text, AccountID: account})
	if out.Line != "" {
		d.reflexAnnounce(reflexRunOpts{Turn: u.Turn, Source: voiceSourceOf(u.Source)}, out.Line)
	}
	if !out.Act {
		return reflex.Action{}, reflexMeta{}, "", false
	}
	a, target, ok := decidedAction(out.Intent, out.Confidence, u.Text, cat)
	if !ok {
		return reflex.Action{}, reflexMeta{}, "", false
	}
	return a, reflexMeta{route: out.Route, ms: out.ElapsedMS}, target, true
}

// reflexTier0 is the rules as the decision policy's tier 0: one claimed
// action, at its rule's confidence.
func reflexTier0(cat reflex.Catalog) func(string) (reflex.Intent, float64, bool) {
	return func(text string) (reflex.Intent, float64, bool) {
		p, ok := reflex.Parse(text, time.Time{}, nil, cat)
		if !ok || len(p.Actions) != 1 {
			return "", 0, false
		}
		return p.Actions[0].Intent, p.Actions[0].Confidence, true
	}
}

// decidedAction is the action for an intent a decision named. Its argument
// comes from code, never from the model: an app only when a span of the
// words names one in the catalog, a timer the whole sentence for the
// reminder parser. An intent whose argument code cannot find is not acted
// on.
func decidedAction(intent reflex.Intent, confidence float64, text string, cat reflex.Catalog) (reflex.Action, string, bool) {
	a := reflex.Action{Intent: intent, Confidence: confidence}
	target := ""
	switch intent {
	case reflex.NewTab, reflex.SnapLeft, reflex.SnapRight, reflex.Maximize, reflex.Minimize,
		reflex.VolumeUp, reflex.VolumeDown, reflex.Screenshot, reflex.SelfHome, reflex.ListReminders:
	case reflex.Mute:
		a.Arg = ""
	case reflex.PlayPause:
		a.Arg = reflex.MediaToggle
	case reflex.SetTimer, reflex.SetReminder:
		a.Arg = strings.TrimSpace(text)
	case reflex.OpenApp:
		var ok bool
		a.Arg, target, ok = catalogSpan(text, cat)
		if !ok {
			return reflex.Action{}, "", false
		}
	default:
		return reflex.Action{}, "", false
	}
	return a, target, true
}

// catalogSpan finds the catalog app a run of up to four of text's words
// names best, retaining that exact requested span before catalogue matching.
func catalogSpan(text string, cat reflex.Catalog) (string, string, bool) {
	if cat == nil {
		return "", "", false
	}
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '\'')
	})
	best, target, score := "", "", 0.0
	for n := 1; n <= 4; n++ {
		for i := 0; i+n <= len(words); i++ {
			spoken := strings.Join(words[i:i+n], " ")
			if name, _, s, ok := cat.Resolve(spoken); ok && s > score {
				best, target, score = name, spoken, s
			}
		}
	}
	return best, target, best != ""
}

func voiceSourceOf(s route.Source) string {
	switch s {
	case route.Chord:
		return feature.VoiceSourceChord
	case route.Wake:
		return feature.VoiceSourceWake
	case route.Open:
		return feature.VoiceSourceOpen
	}
	return feature.VoiceSourceToggle
}

// reflexOfferCardLocked shows the hosted-decision disclosure card for the
// account signed in now, once; a Press answering yes binds that account.
// The caller holds r.mu.
func (d *Daemon) reflexOfferCardLocked(account, display string) {
	r := &d.oni.reflex
	svc := d.oni.service.Load()
	if r.card != nil || svc == nil || r.declined == account {
		return
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return
	}
	card := &reflexCard{id: "d-" + hex.EncodeToString(b[:]), account: account}
	question := i18n.Tf("Let Ryoku Ember sort quick commands the rules miss, on %s's credits? What you said leaves this computer.", oniText(display, 80))
	if err := svc.Consent(*feature.DecisionCard(card.id, protocol.ProviderEmber, oniText(question, 400))); err != nil {
		return
	}
	card.timer = d.reflexClock().AfterFunc(feature.ConsentTTL, func() { d.reflexCardExpired(card) })
	r.card = card
}

func (d *Daemon) reflexCardExpired(card *reflexCard) {
	r := &d.oni.reflex
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.card == card {
		r.card = nil
	}
}

// reflexDropCardLocked takes the card off its slot. The caller holds r.mu.
func (d *Daemon) reflexDropCardLocked() {
	r := &d.oni.reflex
	if r.card != nil && r.card.timer != nil {
		r.card.timer.Stop()
	}
	r.card = nil
}

// reflexCardWaiting is the id of the decision card waiting, or "".
func (d *Daemon) reflexCardWaiting() string {
	r := &d.oni.reflex
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.card == nil {
		return ""
	}
	return r.card.id
}

// reflexCardTake answers the decision card id names: yes binds the account
// the card showed, if it is still the one signed in; no binds nothing. It
// reports false when id is not the card.
func (d *Daemon) reflexCardTake(id string, allow bool) (bool, error) {
	r := &d.oni.reflex
	r.mu.Lock()
	defer r.mu.Unlock()
	card := r.card
	if card == nil || id == "" || card.id != id {
		return false, nil
	}
	d.reflexDropCardLocked()
	if !allow {
		r.declined = card.account
		return true, nil
	}
	if card.account == "" || card.account != d.reflexAccount() {
		return true, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The account changed; nothing was allowed."))
	}
	r.bound, r.hosted = card.account, nil
	if r.chooser != nil {
		r.chooser.Restore()
	}
	return true, nil
}

// reflexCardAnswer serves oni.consent.answer for the decision card: only
// the app, which shows the account, answers it. It reports false when id
// is not the card.
func (d *Daemon) reflexCardAnswer(c *connection, p protocol.OniConsentAnswerParams) (protocol.OniAskResult, bool, error) {
	if !strings.HasPrefix(p.ID, "d-") || d.reflexCardWaiting() != p.ID {
		return protocol.OniAskResult{}, false, nil
	}
	if !c.app {
		return protocol.OniAskResult{}, true, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Hyakki app answers this question."))
	}
	allow := p.Answer == "yes"
	if _, err := d.reflexCardTake(p.ID, allow); err != nil {
		return protocol.OniAskResult{}, true, err
	}
	if !allow {
		return protocol.OniAskResult{Status: protocol.OniAskDeclined}, true, nil
	}
	return protocol.OniAskResult{Status: protocol.OniAskAnswered}, true, nil
}

// reflexDecidedLocked carries out a Reflexed decision for the voice loop.
// The actions run in the background, outside the loop's lock; a run whose
// voice session ended or whose turn moved on stops and says nothing. The
// caller holds v.mu.
func (d *Daemon) reflexDecidedLocked(u route.Utterance, source string, gen uint64, dec route.Decision) {
	v := &d.oni.voice
	r := &d.oni.reflex
	o := reflexRunOpts{Turn: u.Turn, Source: source, Route: protocol.DecisionRules, Gen: gen, Press: dec.Press.Valid(), utter: u}
	var actions []reflex.Action
	if dec.Plan != nil {
		actions = slices.Clone(dec.Plan.Actions)
		r.mu.Lock()
		if meta, ok := r.plans[dec.Plan]; ok {
			o.Route, o.ms = meta.route, meta.ms
			delete(r.plans, dec.Plan)
		}
		r.mu.Unlock()
	}
	if len(actions) == 0 && strings.TrimSpace(dec.Line) == "" {
		return
	}
	v.work++
	go func() {
		defer func() {
			v.mu.Lock()
			v.work--
			v.mu.Unlock()
			d.stateChanged()
		}()
		if len(actions) == 0 {
			d.reflexAnnounce(o, dec.Line)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
		defer cancel()
		d.reflexRun(ctx, actions, o)
	}()
}

// reflexLive reports whether a run may still act and speak: always for a
// replay the app asked for; for a voice run, while its voice session and
// turn last.
func (d *Daemon) reflexLive(o reflexRunOpts) bool {
	if o.Turn == 0 {
		return true
	}
	v := &d.oni.voice
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.turn != nil && v.turn.Accept(o.Turn) && (o.Gen == 0 || o.Gen == v.gen)
}

// reflexAnnounce says line for o: spoken or in the bubble for a voice turn
// still current, in the bubble otherwise, and as a desktop notification
// while Oni is hidden or not on screen. A late line, for a voice turn that
// is over, is not said at all.
func (d *Daemon) reflexAnnounce(o reflexRunOpts, line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	line = i18n.T(line)
	settings := protocol.EffectiveOni(d.Settings())
	svc := d.oni.service.Load()
	hidden := settings.Hidden || svc == nil
	if o.Turn != 0 {
		v := &d.oni.voice
		v.mu.Lock()
		live := v.turn != nil && v.turn.Accept(o.Turn) && (o.Gen == 0 || o.Gen == v.gen)
		if live {
			d.voiceSayLocked(settings, o.Turn, o.Source, line)
		}
		v.mu.Unlock()
		if !live {
			return
		}
	} else if !hidden {
		_ = svc.Bubble(0, "answer", oniText(line, oniBubbleMax))
	}
	if hidden {
		d.reflexNotify(oniActiveProfile(settings.Character).Name, line)
	}
}

// reflexNotify posts a desktop notification.
func (d *Daemon) reflexNotify(summary, body string) {
	if summary == "" {
		summary = "Oni"
	}
	summary, body = oniText(summary, notify.MaxSummary), oniText(body, notify.MaxBody)
	if post := d.deps.OniReflex.Notify; post != nil {
		if err := post(summary, body); err != nil {
			d.warn("oni reflex notify: %v", err)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := notify.New(bus.Session).Post(ctx, summary, body); err != nil {
		d.warn("oni reflex notify: %v", err)
	}
}

// reflexAdmit counts one action against reflex.maxActionsPerMinute, voice
// and replay together, and reports whether it may run.
func (d *Daemon) reflexAdmit(now time.Time) bool {
	r := &d.oni.reflex
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.ran[:0]
	for _, t := range r.ran {
		if now.Sub(t) < time.Minute && !now.Before(t) {
			kept = append(kept, t)
		}
	}
	r.ran = kept
	if len(r.ran) >= reflexLimits().MaxActionsPerMinute {
		return false
	}
	r.ran = append(r.ran, now)
	return true
}

// reflexRun carries out actions one by one under the caps: each is checked
// against the registry, the Press it needs and the per-minute cap, done
// through the executors or the daemon's own handlers, announced, and
// written to the ledger. A voice run stops once its turn is over. Called
// without the voice loop's lock.
func (d *Daemon) reflexRun(ctx context.Context, actions []reflex.Action, o reflexRunOpts) []reflexStep {
	clock := d.reflexClock()
	var caps map[reflex.Intent]bool
	capped := false
	steps := make([]reflexStep, 0, len(actions))
	for i, a := range actions {
		if !d.reflexLive(o) || ctx.Err() != nil || !d.reflexOn() {
			break
		}
		started := clock.Now()
		info, known := reflex.Lookup(a.Intent)
		step := reflexStep{Action: a}
		blocked := d.guardActionRefusal(a)
		switch {
		case blocked != "":
			step.Outcome, step.Line = protocol.OniReflexRefused, blocked
		case !known || o.Replay && (info.PressOnly || !info.Replayable):
			step.Outcome, step.Line = protocol.OniReflexRefused, lineReflexCant
		case info.PressOnly && !o.Press:
			step.Outcome, step.Line = protocol.OniReflexRefused, lineReflexPress
		case !d.reflexAdmit(started):
			step.Outcome = protocol.OniReflexSkipped
			if !capped {
				step.Line, capped = lineReflexEnough, true
			}
		default:
			step = d.reflexDo(ctx, a, o, &caps)
		}
		ms := clock.Now().Sub(started).Milliseconds()
		if i == 0 {
			ms += o.ms
		}
		d.reflexRecord(protocol.OniReflexRow{At: started, Intent: string(a.Intent), Route: o.Route, Ms: max(0, ms), Confidence: a.Confidence, Outcome: step.Outcome})
		steps = append(steps, step)
		// a done action belongs to the run even when the next press moved
		// the turn on; the recorder itself resets when the voice session ends
		if step.Done && !o.Replay && o.Turn != 0 {
			d.routineObserve(a, o.utter)
		}
		if !d.reflexLive(o) {
			// the voice session ended while it ran: say nothing more
			break
		}
		d.reflexAnnounce(o, step.Line)
	}
	return steps
}

// reflexDo carries out one admitted action.
func (d *Daemon) reflexDo(ctx context.Context, a reflex.Action, o reflexRunOpts, caps *map[reflex.Intent]bool) reflexStep {
	step := reflexStep{Action: a}
	done := func(line string) reflexStep {
		step.Done, step.Line, step.Outcome = true, line, protocol.OniReflexDone
		return step
	}
	result := func(line, outcome string) reflexStep {
		step.Line, step.Outcome = line, outcome
		return step
	}
	settings := protocol.EffectiveOni(d.Settings())
	allowed := func(id string) bool { return feature.Decide(feature.Request{Feature: id}, settings).Allow }
	switch a.Intent {
	case reflex.SetTimer, reflex.SetReminder:
		line, refused := d.remindSet(a.Arg, o.utter.At)
		if refused {
			return result(line, protocol.OniReflexRefused)
		}
		return done(line)
	case reflex.CancelTimer:
		line, refused := d.remindCancel(remind.CancelKind(a.Arg))
		if refused {
			return result(line, protocol.OniReflexRefused)
		}
		return done(line)
	case reflex.ListReminders:
		line, refused := d.remindList()
		if refused {
			return result(line, protocol.OniReflexRefused)
		}
		return done(line)
	case reflex.RunRoutine:
		line := d.routineRunSpoken(o.utter, o.Source, a.Arg)
		if !allowed(feature.Routines) {
			return result(line, protocol.OniReflexRefused)
		}
		return done(line)
	case reflex.RememberNote:
		line, outcome := d.reflexNote(ctx, a.Arg)
		step.Done = outcome == protocol.OniReflexDone
		return result(line, outcome)
	case reflex.SelfMove:
		if !slices.Contains(protocol.OniCorners, a.Arg) {
			return result(lineReflexCant, protocol.OniReflexRefused)
		}
		if err := d.oniPlace(protocol.OniPlaceParams{Corner: a.Arg}); err != nil {
			return result(lineReflexFailed, protocol.OniReflexFailed)
		}
		return done(lineReflexMoved)
	case reflex.SelfHome:
		if err := d.oniPlace(protocol.OniPlaceParams{Corner: presence.DefaultCorner}); err != nil {
			return result(lineReflexFailed, protocol.OniReflexFailed)
		}
		return done(lineReflexHome)
	case reflex.SelfSize:
		return d.reflexSize(a, settings.Size)
	case reflex.SettingFlip:
		flip, ok := reflexFlips[a.Arg]
		if !ok || !slices.Contains(reflex.Flips(), a.Arg) || !o.Press {
			return result(lineReflexCant, protocol.OniReflexRefused)
		}
		// say it before a flip that would end the voice session
		d.reflexAnnounce(o, flip.line)
		if err := d.reflexPatch(flip.patch); err != nil {
			return result(lineReflexFailed, protocol.OniReflexFailed)
		}
		step.Done, step.Outcome = true, protocol.OniReflexDone
		return step
	}
	r := &d.oni.reflex
	r.mu.Lock()
	d.reflexBuildLocked()
	exe := r.exec
	r.mu.Unlock()
	if exe == nil {
		return result(lineReflexCant, protocol.OniReflexUnsupported)
	}
	if *caps == nil {
		*caps = exe.Caps()
	}
	if !(*caps)[a.Intent] {
		return result(lineReflexCant, protocol.OniReflexUnsupported)
	}
	out, err := exe.Do(ctx, a)
	switch {
	case err == nil && out.Done:
		return done(out.Line)
	case errors.Is(err, launch.ErrCooldown):
		return result(out.Line, protocol.OniReflexCooldown)
	case errors.Is(err, act.ErrUnsupported), errors.Is(err, act.ErrNotExecutor), errors.Is(err, shot.ErrUnavailable):
		return result(firstText(out.Line, lineReflexCant), protocol.OniReflexUnsupported)
	case err == nil:
		return result(out.Line, protocol.OniReflexSkipped)
	}
	d.warn("oni reflex %s: %v", a.Intent, err)
	return result(firstText(out.Line, lineReflexFailed), protocol.OniReflexFailed)
}

// reflexSize makes Oni one size bigger or smaller.
func (d *Daemon) reflexSize(a reflex.Action, size string) reflexStep {
	step := reflexStep{Action: a}
	i := slices.Index(protocol.OniSizes, size)
	if i < 0 {
		i = slices.Index(protocol.OniSizes, protocol.OniSizeNormal)
	}
	line := lineReflexBigger
	switch a.Arg {
	case reflex.SizeBigger:
		if i == len(protocol.OniSizes)-1 {
			step.Line, step.Outcome = lineReflexBiggest, protocol.OniReflexSkipped
			return step
		}
		i++
	case reflex.SizeSmaller:
		if i == 0 {
			step.Line, step.Outcome = lineReflexSmallest, protocol.OniReflexSkipped
			return step
		}
		i--
		line = lineReflexSmaller
	default:
		step.Line, step.Outcome = lineReflexCant, protocol.OniReflexRefused
		return step
	}
	if err := d.reflexPatch(map[string]any{"size": protocol.OniSizes[i]}); err != nil {
		step.Line, step.Outcome = lineReflexFailed, protocol.OniReflexFailed
		return step
	}
	step.Done, step.Line, step.Outcome = true, line, protocol.OniReflexDone
	return step
}

// reflexPatch changes Oni's settings as the user would in Settings.
func (d *Daemon) reflexPatch(patch map[string]any) error {
	raw, err := json.Marshal(map[string]any{"oni": patch})
	if err != nil {
		return err
	}
	if _, err := d.applySettings(context.Background(), raw); err != nil {
		d.warn("oni reflex settings: %v", err)
		return err
	}
	return nil
}

// reflexNote keeps a spoken note in memory as the user, through memory.add
// and its refusals. What sensitive.Note refuses (a password, a PIN, a card,
// account or identity number, a key, a one-time code, any long run of
// digits) is refused before memory sees it; memory off says so.
func (d *Daemon) reflexNote(ctx context.Context, text string) (string, string) {
	text = strings.TrimSpace(text)
	switch {
	case !feature.Decide(feature.Request{Feature: feature.Notes}, protocol.EffectiveOni(d.Settings())).Allow:
		return lineNoteNotesOff, protocol.OniReflexRefused
	case text == "":
		return lineReflexCant, protocol.OniReflexRefused
	case sensitive.Note(text):
		return lineNoteRefused, protocol.OniReflexRefused
	case d.deps.Memory == nil:
		return lineNoteOff, protocol.OniReflexRefused
	}
	raw, err := json.Marshal(protocol.MemoryAddParams{Scope: protocol.MemoryYou, Text: text})
	if err != nil {
		return lineReflexFailed, protocol.OniReflexFailed
	}
	res, err := d.deps.Memory.Handle(ctx, core.MemoryCaller{}, protocol.MMemoryAdd, raw)
	var perr *protocol.Error
	switch {
	case err == nil:
		// "no, I meant" may correct it within the spec's window
		if change, ok := res.(protocol.MemoryChange); ok {
			d.memoryNoted(change)
		}
		return lineNoteKept, protocol.OniReflexDone
	case errors.As(err, &perr) && perr.Code == protocol.CodeDisabled:
		return lineNoteOff, protocol.OniReflexRefused
	case errors.As(err, &perr) && perr.Code == protocol.CodeBadRequest:
		return lineNoteWontKeep, protocol.OniReflexRefused
	}
	d.warn("oni reflex note: %v", err)
	return lineReflexFailed, protocol.OniReflexFailed
}

// reflexRecord adds one row to the ledger, dropping the oldest past
// reflex.ledger.maxRows.
func (d *Daemon) reflexRecord(row protocol.OniReflexRow) {
	r := &d.oni.reflex
	limit := reflexLimits().Ledger.MaxRows
	if limit <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.rows) >= limit {
		r.rows = slices.Delete(r.rows, 0, len(r.rows)-limit+1)
	}
	r.rows = append(r.rows, row)
}

// reflexLedger writes the row for an action that did not run through
// reflexRun, such as a routine step skipped before it began.
func (d *Daemon) reflexLedger(o reflexRunOpts, a reflex.Action, outcome string) {
	d.reflexRecord(protocol.OniReflexRow{At: d.reflexClock().Now(), Intent: string(a.Intent), Route: o.Route, Confidence: a.Confidence, Outcome: outcome})
}

// reflexLogCall serves oni.reflex.log: the ledger, newest first. It holds
// no words. Rows are kept oldest first and reversed on the way out.
func (d *Daemon) reflexLogCall() (protocol.OniReflexLogResult, error) {
	decision := feature.Decide(feature.Request{Feature: feature.Reflex}, protocol.EffectiveOni(d.Settings()))
	if !decision.Allow {
		message := i18n.T("Quick commands are off. Turn them on in Oni's settings.")
		if decision.Reason == feature.ReasonOniOff {
			message = i18n.T("Oni is off.")
		}
		return protocol.OniReflexLogResult{}, &protocol.Error{Code: protocol.CodeDisabled, Message: message, Reason: decision.Reason}
	}
	r := &d.oni.reflex
	r.mu.Lock()
	defer r.mu.Unlock()
	rows := append([]protocol.OniReflexRow{}, r.rows...)
	slices.Reverse(rows)
	return protocol.OniReflexLogResult{Rows: rows}, nil
}

// pressOnly reports whether a needs a Press, or is no registry intent.
func pressOnly(a reflex.Action) bool {
	info, ok := reflex.Lookup(a.Intent)
	return !ok || info.PressOnly
}
