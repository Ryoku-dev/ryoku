package daemon

import (
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/remind"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/teams"
)

// Timers and reminders are kept in the reminders store, so they outlive
// the daemon. A timer needs voice and nothing more, as it did before
// reminders came; a reminder needs reflex.reminders. Only the kinds that
// are on count: one timer is armed for the next of them due and no other;
// it fires as a bubble and, with voice on, aloud, and as a desktop
// notification while Oni is hidden or not on screen. One that falls due
// while the user is busy (a press, a line being said, a transcript or an
// answer on its way) waits up to reflex.reminders.busyTries times
// busyWaitS. One that fell due while the daemon was down fires once, marked
// late, when it starts again.
//
// The daemon stays up at rest only while the next one is due within
// reflex.reminders.residencyHorizonMin; past that the shared wake unit,
// held on while any is pending, starts it again in time. Where the wake
// unit cannot be armed, the daemon stays up while any is pending, and
// oni.reminders says so. With neither kind on, or none of them pending,
// nothing here is armed or held.

// oniRemind is the timers' and reminders' state.
type oniRemind struct {
	mu    sync.Mutex
	store *remind.Store
	// timer is the one armed timer, for the next one due; gen moves
	// whenever it is disarmed, so a fire already on its way does nothing
	timer presence.Timer
	gen   uint64
	// waiting is the due one a busy user holds back, tries how often it
	// has waited and until when it waits now
	waiting   string
	tries     int
	waitUntil time.Time

	// wakeMu serialises the hold on the wake unit; wakeKnown and wakeHeld
	// are the hold as last set, noWake that the unit could not be armed
	wakeMu    sync.Mutex
	wakeKnown bool
	wakeHeld  bool
	noWake    atomic.Bool
}

// remindKinds are the kinds that are on: timers with voice, reminders with
// reflex.reminders.
type remindKinds struct{ timer, reminder bool }

func remindKindsOf(settings protocol.OniSettings) remindKinds {
	on := func(id string) bool { return feature.Decide(feature.Request{Feature: id}, settings).Allow }
	return remindKinds{timer: on(feature.Voice), reminder: on(feature.Reminders)}
}

func (k remindKinds) any() bool { return k.timer || k.reminder }

func (k remindKinds) has(kind string) bool {
	return kind == remind.KindTimer && k.timer || kind == remind.KindReminder && k.reminder
}

// only keeps the ones of rs whose kind is on.
func (k remindKinds) only(rs []remind.Reminder) []remind.Reminder {
	return slices.DeleteFunc(rs, func(r remind.Reminder) bool { return !k.has(r.Kind) })
}

// remindKindsNow are the kinds on under the settings now.
func (d *Daemon) remindKindsNow() remindKinds {
	return remindKindsOf(protocol.EffectiveOni(d.Settings()))
}

// remindRefusal is why kind cannot be set now, or nil: a timer needs
// voice, a reminder reflex.reminders, and oni.reminders the reminders
// switch whatever it lists.
func (d *Daemon) remindRefusal(kind string) error {
	id, message := feature.Reminders, i18n.T("Reminders are off. Turn them on in Oni's settings.")
	if kind == remind.KindTimer {
		id, message = feature.Voice, i18n.T("Voice is off. Turn it on in Oni's settings.")
	}
	decision := feature.Decide(feature.Request{Feature: id}, protocol.EffectiveOni(d.Settings()))
	if decision.Allow {
		return nil
	}
	if decision.Reason == feature.ReasonOniOff {
		message = i18n.T("Oni is off.")
	}
	return &protocol.Error{Code: protocol.CodeDisabled, Message: message, Reason: decision.Reason}
}

// remindOffLine is what Oni says to kind asked for while it is off, or "".
func (d *Daemon) remindOffLine(kind string) string {
	var refused *protocol.Error
	if err := d.remindRefusal(kind); errors.As(err, &refused) {
		return refused.Message
	}
	return ""
}

// remindStoreLocked is the reminders store, opened on first use. The
// caller holds r.mu.
func (d *Daemon) remindStoreLocked() *remind.Store {
	r := &d.oni.remind
	if r.store == nil {
		r.store = d.deps.OniReflex.Reminders
		if r.store == nil {
			r.store = remind.NewStore("")
		}
	}
	return r.store
}

// remindPending is what is kept of the kinds on, soonest first, with a
// store that cannot be read holding nothing.
func (d *Daemon) remindPending(k remindKinds) []remind.Reminder {
	if !k.any() {
		return nil
	}
	r := &d.oni.remind
	r.mu.Lock()
	store := r.store
	r.mu.Unlock()
	if store == nil {
		return nil
	}
	rs, err := store.List()
	if err != nil {
		return nil
	}
	return k.only(rs)
}

// remindWake holds the reminders' share of the wake unit or lets it go.
func (d *Daemon) remindWake(on bool) error {
	if hold := d.deps.OniReflex.Wake; hold != nil {
		return hold(on)
	}
	return teams.HoldWakeForReminders(on)
}

// remindSettings follows the settings: a kind turned on arms the next one
// due and fires any already due; with neither on the timer is disarmed and
// the wake unit let go.
func (d *Daemon) remindSettings(settings protocol.Settings) {
	kinds := remindKindsOf(protocol.EffectiveOni(settings))
	r := &d.oni.remind
	r.mu.Lock()
	if !kinds.any() {
		built := r.store != nil
		d.remindDisarmLocked()
		r.store = nil
		r.mu.Unlock()
		if built {
			go d.remindHoldWake()
		}
		return
	}
	d.remindStoreLocked()
	d.remindArmLocked(kinds)
	r.mu.Unlock()
	go d.remindHoldWake()
}

// remindDisarmLocked stops the armed timer. The caller holds r.mu.
func (d *Daemon) remindDisarmLocked() {
	r := &d.oni.remind
	r.gen++
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}

// remindArmLocked arms the one timer for the next one of kinds due: at its
// time, at once for one already due, or when a busy wait ends. Nothing is
// armed with none pending or a store that cannot be read. The caller holds
// r.mu.
func (d *Daemon) remindArmLocked(kinds remindKinds) {
	r := &d.oni.remind
	d.remindDisarmLocked()
	if r.store == nil {
		return
	}
	rs, err := r.store.List()
	if err == nil {
		rs = kinds.only(rs)
	}
	if err != nil || len(rs) == 0 {
		r.waiting, r.tries, r.waitUntil = "", 0, time.Time{}
		return
	}
	at := rs[0].Due
	if rs[0].ID == r.waiting && r.waitUntil.After(at) {
		at = r.waitUntil
	}
	clock := d.reflexClock()
	gen := r.gen
	r.timer = clock.AfterFunc(max(at.Sub(clock.Now()), 0), func() { d.remindFire(gen) })
}

// remindFire fires the one due first, unless a busy user holds it back,
// and arms the next.
func (d *Daemon) remindFire(gen uint64) {
	r := &d.oni.remind
	kinds := d.remindKindsNow()
	r.mu.Lock()
	if gen != r.gen || r.store == nil {
		r.mu.Unlock()
		return
	}
	r.timer = nil
	now := d.reflexClock().Now()
	due := kinds.only(r.store.Due(now))
	if len(due) == 0 {
		d.remindArmLocked(kinds)
		r.mu.Unlock()
		return
	}
	rem := due[0]
	if rem.ID != r.waiting {
		r.waiting, r.tries, r.waitUntil = rem.ID, 0, time.Time{}
	}
	r.mu.Unlock()
	// the voice loop's lock is never taken under the reminders'
	busy := d.voiceBusy()
	limits := reflexLimits().Reminders
	r.mu.Lock()
	if gen != r.gen || r.store == nil {
		r.mu.Unlock()
		return
	}
	if busy && r.tries < limits.BusyTries {
		r.tries++
		r.waitUntil = now.Add(limits.BusyWait())
		d.remindArmLocked(kinds)
		r.mu.Unlock()
		return
	}
	r.waiting, r.tries, r.waitUntil = "", 0, time.Time{}
	err := r.store.Done(rem.ID)
	var refused *protocol.Error
	gone := errors.As(err, &refused) && refused.Code == protocol.CodeNotFound
	if err == nil || gone {
		// one that cannot be marked done is not armed again: it would
		// ring without end
		d.remindArmLocked(kinds)
	} else {
		d.warn("oni reminders: %v", err)
	}
	r.mu.Unlock()
	if !gone {
		d.remindShow(rem, now)
	}
	d.remindHoldWake()
	d.stateChanged()
}

// remindShow fires rem at at: aloud while voice is on and a voice is
// installed, in the bubble while Oni is on screen, and as a desktop
// notification while Oni is hidden or nothing else showed it.
func (d *Daemon) remindShow(rem remind.Reminder, at time.Time) {
	line := remind.FireLine(rem, at)
	settings := protocol.EffectiveOni(d.Settings())
	spoken := false
	if feature.Decide(feature.Request{Feature: feature.Voice}, settings).Allow {
		v := &d.oni.voice
		v.mu.Lock()
		if d.voiceEnsureLocked(settings) == nil && d.voiceSpeakerLocked(settings) != nil {
			spoken = d.voiceSayLocked(settings, v.turn.ID(), "", line)
		}
		v.mu.Unlock()
	}
	shown := false
	if svc := d.oni.service.Load(); svc != nil && !settings.Hidden {
		shown = svc.Bubble(0, "answer", oniText(line, oniBubbleMax)) == nil
	}
	if settings.Hidden || !spoken && !shown {
		d.reflexNotify(remindNoticeTitle(rem, at), rem.Text)
	}
}

// remindNoticeTitle heads the notification of rem fired at at.
func remindNoticeTitle(rem remind.Reminder, at time.Time) string {
	late := rem.Late(at)
	switch {
	case rem.Kind == remind.KindTimer && late:
		return i18n.T("Your timer ended earlier.")
	case rem.Kind == remind.KindTimer:
		return i18n.T("Time's up.")
	case late:
		return i18n.T("Reminder, a bit late")
	}
	return i18n.T("Reminder")
}

// remindHoldWake holds the wake unit while one of the kinds on is pending,
// and lets it go otherwise. A unit never held is never touched; one that
// could not be armed is tried again.
func (d *Daemon) remindHoldWake() {
	r := &d.oni.remind
	r.wakeMu.Lock()
	defer r.wakeMu.Unlock()
	want := len(d.remindPending(d.remindKindsNow())) > 0
	if !r.wakeKnown && !want || r.wakeKnown && r.wakeHeld == want && !(want && r.noWake.Load()) {
		return
	}
	err := d.remindWake(want)
	r.wakeKnown, r.wakeHeld = true, want
	if !want {
		r.noWake.Store(false)
		return
	}
	if was := r.noWake.Swap(err != nil); err != nil && !was {
		d.warn("oni reminders: the wake unit cannot be armed, so Hyakki stays up while a timer or reminder is pending: %v", err)
	}
}

// remindResident reports whether a timer or reminder keeps the daemon up
// at rest: the next one of the kinds on is due within
// reflex.reminders.residencyHorizonMin, or any is pending while the wake
// unit cannot be armed.
func (d *Daemon) remindResident() bool {
	rs := d.remindPending(d.remindKindsNow())
	if len(rs) == 0 {
		return false
	}
	if d.oni.remind.noWake.Load() {
		return true
	}
	return !rs[0].Due.After(d.reflexClock().Now().Add(reflexLimits().Reminders.ResidencyHorizon()))
}

// remindChangedLocked arms the timer again after the store changed, then
// sets the wake unit's hold and looks again at the idle deadline. The
// caller holds r.mu.
func (d *Daemon) remindChangedLocked() {
	d.remindArmLocked(d.remindKindsNow())
	go func() {
		d.remindHoldWake()
		d.stateChanged()
	}()
}

// remindAnswer is what Oni says for a decision the router answered: a
// timer or reminder kept, cancelled or listed, or the router's own line.
// It runs under the voice loop's lock and takes the reminders'.
func (d *Daemon) remindAnswer(dec route.Decision) string {
	switch dec.Card {
	case route.CardTimer, route.CardReminder:
		if dec.Reminder != nil {
			line, _ := d.remindKeep(*dec.Reminder, dec.Line)
			return line
		}
		if dec.Card == route.CardReminder {
			if off := d.remindOffLine(remind.KindReminder); off != "" {
				return off
			}
		}
		return dec.Line
	case route.CardRemindCancel:
		line, _ := d.remindCancel(dec.Cancel)
		return line
	case route.CardRemindList:
		line, _ := d.remindList()
		return line
	}
	return dec.Line
}

// remindSet keeps the timer or reminder clause asks for, said at at, and
// returns what Oni says: its confirmation, or why it was not kept. refused
// is set when its kind is off.
func (d *Daemon) remindSet(clause string, at time.Time) (line string, refused bool) {
	rem, line, ok := remind.Parse(clause, at, time.Local)
	switch {
	case !ok:
		return i18n.T("I didn't catch when."), false
	case rem.Kind == "":
		return line, false
	}
	return d.remindKeep(rem, line)
}

// remindKeep keeps rem and returns line, which confirms it, or why it was
// not kept; refused is set when its kind is off.
func (d *Daemon) remindKeep(rem remind.Reminder, line string) (string, bool) {
	if off := d.remindOffLine(rem.Kind); off != "" {
		return off, true
	}
	r := &d.oni.remind
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := d.remindStoreLocked().Add(rem); err != nil {
		return remindErrorLine(err), false
	}
	d.remindChangedLocked()
	return line, false
}

// remindCancel takes away what k names, of the kinds on, and says what
// went; refused is set when no kind k names is on.
func (d *Daemon) remindCancel(k remind.CancelKind) (string, bool) {
	kinds := d.remindKindsNow()
	switch {
	case k == remind.CancelTimers && !kinds.timer:
		return d.remindOffLine(remind.KindTimer), true
	case (k == remind.CancelReminder || k == remind.CancelReminders) && !kinds.reminder:
		return d.remindOffLine(remind.KindReminder), true
	case k == remind.CancelAll && !kinds.any():
		return d.remindOffLine(remind.KindTimer), true
	case k == remind.CancelAll && !kinds.reminder:
		k = remind.CancelTimers
	case k == remind.CancelAll && !kinds.timer:
		k = remind.CancelReminders
	}
	r := &d.oni.remind
	r.mu.Lock()
	defer r.mu.Unlock()
	store := d.remindStoreLocked()
	if _, err := store.List(); err != nil {
		return remindErrorLine(err), false
	}
	n := store.CancelBy(k)
	if n == 0 {
		return i18n.T("Nothing to cancel."), false
	}
	d.remindChangedLocked()
	switch {
	case k == remind.CancelTimers && n == 1:
		return i18n.T("Timer cancelled."), false
	case k == remind.CancelTimers:
		return i18n.Tf("%d timers cancelled.", n), false
	case k == remind.CancelAll:
		return i18n.Tf("All %d cancelled.", n), false
	case n == 1:
		return i18n.T("Reminder cancelled."), false
	}
	return i18n.Tf("%d reminders cancelled.", n), false
}

// remindList reads out what is kept of the kinds on; refused is set when
// neither is on.
func (d *Daemon) remindList() (string, bool) {
	kinds := d.remindKindsNow()
	if !kinds.any() {
		return d.remindOffLine(remind.KindTimer), true
	}
	r := &d.oni.remind
	r.mu.Lock()
	store := d.remindStoreLocked()
	r.mu.Unlock()
	rs, err := store.List()
	if err != nil {
		return remindErrorLine(err), false
	}
	return remind.ListLine(kinds.only(rs), d.reflexClock().Now(), time.Local), false
}

// remindErrorLine says why the store refused.
func remindErrorLine(err error) string {
	var refused *protocol.Error
	if errors.As(err, &refused) {
		switch refused.Reason {
		case remind.ReasonFull:
			return refused.Message
		case remind.ReasonDamaged:
			return i18n.T("I can't read your reminders.")
		}
	}
	return i18n.T("I couldn't keep that.")
}

// remindersCall serves oni.reminders: list, add one (remindAddCall), cancel
// one, or cancel all of what is kept, timers included. It needs
// reflex.reminders.
func (d *Daemon) remindersCall(p protocol.OniRemindersParams) (protocol.OniRemindersResult, error) {
	if err := d.remindRefusal(remind.KindReminder); err != nil {
		return protocol.OniRemindersResult{}, err
	}
	if p.Action == protocol.OniRemindersAdd {
		return d.remindAddCall(p)
	}
	r := &d.oni.remind
	r.mu.Lock()
	store := d.remindStoreLocked()
	rs, err := store.List()
	if err != nil {
		r.mu.Unlock()
		return protocol.OniRemindersResult{}, err
	}
	switch p.Action {
	case protocol.OniRemindersCancel:
		if !store.Cancel(p.ID) {
			r.mu.Unlock()
			return protocol.OniRemindersResult{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That reminder is not there any more."))
		}
	case protocol.OniRemindersCancelAll:
		store.CancelAll()
	}
	if p.Action != protocol.OniRemindersList {
		if rs, err = store.List(); err != nil {
			r.mu.Unlock()
			return protocol.OniRemindersResult{}, err
		}
		d.remindArmLocked(d.remindKindsNow())
	}
	r.mu.Unlock()
	d.remindHoldWake()
	if p.Action != protocol.OniRemindersList {
		d.stateChanged()
	}
	now := d.reflexClock().Now()
	out := protocol.OniRemindersResult{Reminders: make([]protocol.OniReminderInfo, 0, len(rs)), NoWake: r.noWake.Load()}
	for _, rem := range rs {
		out.Reminders = append(out.Reminders, protocol.OniReminderInfo{ID: rem.ID, Kind: rem.Kind, Due: rem.Due, Text: rem.Text, Late: rem.Late(now)})
	}
	return out, nil
}
