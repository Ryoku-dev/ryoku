package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/remind"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/teams"
)

// remindEpoch is the reminders clock's start, the voice clock's too.
var remindEpoch = time.Unix(1_800_000_000, 0)

// remindProbe records the reminders' hold on the wake unit and their
// notifications; wakeErr is what arming the unit returns.
type remindProbe struct {
	mu      sync.Mutex
	wakes   []bool
	wakeErr error
	notes   [][2]string
}

func (p *remindProbe) wake(on bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.wakes = append(p.wakes, on)
	if on {
		return p.wakeErr
	}
	return nil
}

func (p *remindProbe) notify(summary, body string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.notes = append(p.notes, [2]string{summary, body})
	return nil
}

func (p *remindProbe) wakeCalls() []bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.wakes)
}

func (p *remindProbe) notices() [][2]string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.notes)
}

// remindRig serves voice with reflex and reminders on (edit may change
// that), the reminders on their own fake clock, store, notifier and wake
// unit, and keep already kept.
func remindRig(t *testing.T, edit func(*protocol.Settings), probe *remindProbe, keep ...remind.Reminder) (*voiceRig, *askClock, *remind.Store) {
	t.Helper()
	store := remind.NewStore(filepath.Join(t.TempDir(), "oni-reminders.json"))
	for _, r := range keep {
		if err := store.Add(r); err != nil {
			t.Fatal(err)
		}
	}
	rc := &askClock{now: remindEpoch}
	r := serveVoice(t, func(s *protocol.Settings) {
		s.Oni.Reflex = true
		s.Oni.ReflexSettings.Reminders = true
		if edit != nil {
			edit(s)
		}
	}, nil, func(d *Deps) {
		d.OniReflex.Clock = rc
		d.OniReflex.Reminders = store
		d.OniReflex.Notify = probe.notify
		d.OniReflex.Wake = probe.wake
	})
	return r, rc, store
}

func remindEventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("never: %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func remindKept(t *testing.T, store *remind.Store) []remind.Reminder {
	t.Helper()
	rs, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func remindLimits(t *testing.T) spec.ReflexReminders {
	t.Helper()
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return sp.Reflex.Reminders
}

func kept(id, kind, text string, created, due time.Time) remind.Reminder {
	return remind.Reminder{ID: id, Kind: kind, Created: created, Due: due, Text: text}
}

// A timer set by voice is kept in the store with the same confirmation as
// before, one timer is armed for it, it rings once when due, and the wake
// unit is held while it is pending and let go once it has rung.
func TestOniRemindVoiceTimerIsKeptRingsOnceAndLetsTheWakeUnitGo(t *testing.T) {
	probe := &remindProbe{}
	r, rc, store := remindRig(t, nil, probe)
	if rc.Armed() != 0 || len(probe.wakeCalls()) != 0 {
		t.Fatalf("with nothing kept: %d armed, wake %v", rc.Armed(), probe.wakeCalls())
	}
	surface, _ := r.presenting(t)
	log := &frameLog{p: surface}
	r.chord(t, surface, "set a timer for five seconds", false)
	log.until("the timer confirmed", spoken("Timer set, 5 seconds."))
	rs := remindKept(t, store)
	if len(rs) != 1 || rs[0].Kind != remind.KindTimer || rs[0].Due.Sub(rs[0].Created) != 5*time.Second || rs[0].Text != "" {
		t.Fatalf("kept %+v", rs)
	}
	remindEventually(t, "the wake unit held", func() bool { return slices.Equal(probe.wakeCalls(), []bool{true}) })
	if rc.Armed() != 1 {
		t.Fatalf("%d timers armed for one timer", rc.Armed())
	}
	remindEventually(t, "voice at rest", func() bool { return !r.d.voiceBusy() })
	rc.Advance(4 * time.Second)
	if len(remindKept(t, store)) != 1 {
		t.Fatal("the timer rang early")
	}
	rc.Advance(time.Second)
	log.until("the timer's end said", spoken("Time's up."))
	remindEventually(t, "the timer done", func() bool { return len(remindKept(t, store)) == 0 })
	remindEventually(t, "the wake unit let go", func() bool { return slices.Equal(probe.wakeCalls(), []bool{true, false}) })
	if rc.Armed() != 0 {
		t.Fatalf("%d timers armed with nothing kept", rc.Armed())
	}
	rc.Advance(time.Hour)
	log.quiet("a second ring", spoken("Time's up."))
	if n := len(probe.notices()); n != 0 {
		t.Fatalf("%d notifications while Oni is on screen and speaks", n)
	}
}

// Reminders and timers are listed and cancelled by voice and through
// oni.reminders; only the next one due is ever armed.
func TestOniRemindCancelAndListByVoiceAndCall(t *testing.T) {
	probe := &remindProbe{}
	r, rc, store := remindRig(t, nil, probe)
	surface, _ := r.presenting(t)
	log := &frameLog{p: surface}
	r.chord(t, surface, "set a timer for ten minutes", false)
	log.until("the timer confirmed", spoken("Timer set, 10 minutes."))
	remindEventually(t, "voice at rest", func() bool { return !r.d.voiceBusy() })
	r.chord(t, surface, "Remind me in 20 minutes to stretch", false)
	log.until("the reminder confirmed", spoken("Reminder in 20 minutes."))
	if n := len(remindKept(t, store)); n != 2 || rc.Armed() != 1 {
		t.Fatalf("%d kept, %d armed", n, rc.Armed())
	}
	cli := dialSocket(t, protocol.ClientCLI)
	var res protocol.OniRemindersResult
	if err := cli.call(protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersList}, &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Reminders) != 2 || res.Reminders[0].Kind != protocol.OniReminderKindTimer || res.Reminders[1].Text != "stretch" || res.NoWake || res.Reminders[0].Late {
		t.Fatalf("listed %+v", res)
	}
	remindEventually(t, "voice at rest", func() bool { return !r.d.voiceBusy() })
	r.chord(t, surface, "cancel the timer", false)
	log.until("the timer cancelled", spoken("Timer cancelled."))
	if rs := remindKept(t, store); len(rs) != 1 || rs[0].Kind != remind.KindReminder || rc.Armed() != 1 {
		t.Fatalf("after cancelling the timer: %+v, %d armed", rs, rc.Armed())
	}
	remindEventually(t, "voice at rest", func() bool { return !r.d.voiceBusy() })
	r.chord(t, surface, "what are my reminders", false)
	log.until("the list said", func(f feature.Payload) bool {
		v, ok := f.(*feature.VoiceFrame)
		return ok && v.Phase == feature.VoiceSpeaking && strings.HasPrefix(v.Text, "1 reminder.") && strings.Contains(v.Text, "Stretch")
	})
	id := res.Reminders[1].ID
	if err := cli.call(protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersCancel, ID: id}, &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Reminders) != 0 || rc.Armed() != 0 || len(remindKept(t, store)) != 0 {
		t.Fatalf("after cancelling the last: %+v, %d armed", res, rc.Armed())
	}
	remindEventually(t, "the wake unit let go", func() bool { return slices.Equal(probe.wakeCalls(), []bool{true, false}) })
	err := cli.call(protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersCancel, ID: id}, &res)
	if code, _ := reasonOf(err); code != protocol.CodeNotFound {
		t.Fatalf("cancelling a gone reminder: %v", err)
	}
	if err := cli.call(protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersCancelAll}, &res); err != nil || len(res.Reminders) != 0 {
		t.Fatalf("cancel all of nothing: %+v %v", res, err)
	}
}

// A reminder due while the user is busy waits, up to busyTries times
// busyWaitS, and then rings anyway, not marked late.
func TestOniRemindBusyUserDelaysTheFireAtMostBusyTriesWaits(t *testing.T) {
	limits := remindLimits(t)
	probe := &remindProbe{}
	r, rc, store := remindRig(t, nil, probe, kept("0123456789abcdef", remind.KindReminder, "stretch", remindEpoch.Add(-time.Minute), remindEpoch.Add(time.Minute)))
	surface, _ := r.presenting(t)
	log := &frameLog{p: surface}
	talk(t, surface, protocol.OniVoiceStart)
	r.mic.next(t)
	t.Cleanup(func() {
		_ = surface.call(protocol.MOniTalk, protocol.OniTalkParams{Action: protocol.OniVoiceCancel}, nil)
	})
	rc.Advance(time.Minute)
	for i := range limits.BusyTries {
		if n := len(remindKept(t, store)); n != 1 || rc.Armed() != 1 {
			t.Fatalf("wait %d: %d kept, %d armed", i, n, rc.Armed())
		}
		rc.Advance(limits.BusyWait())
	}
	log.until("the reminder said, not late", spoken("Reminder. Stretch."))
	remindEventually(t, "the reminder done", func() bool { return len(remindKept(t, store)) == 0 })
	if rc.Armed() != 0 {
		t.Fatalf("%d armed after the last reminder rang", rc.Armed())
	}
}

// A reminder that fell due while the daemon was down rings once, marked
// late, as soon as the daemon starts, and is then done.
func TestOniRemindDueWhileTheDaemonWasDownRingsOnceMarkedLate(t *testing.T) {
	probe := &remindProbe{}
	_, rc, store := remindRig(t, func(s *protocol.Settings) { s.Oni.Hidden = true }, probe,
		kept("0123456789abcdef", remind.KindReminder, "call the bank", remindEpoch.Add(-2*time.Hour), remindEpoch.Add(-time.Hour)))
	if rc.Armed() != 1 {
		t.Fatalf("%d armed for a reminder already due", rc.Armed())
	}
	rc.Advance(0)
	remindEventually(t, "the late notice", func() bool { return len(probe.notices()) == 1 })
	if got := probe.notices()[0]; got != [2]string{"Reminder, a bit late", "call the bank"} {
		t.Fatalf("notice %q", got)
	}
	remindEventually(t, "the reminder done", func() bool { return len(remindKept(t, store)) == 0 })
	rc.Advance(time.Hour)
	time.Sleep(50 * time.Millisecond)
	if n := len(probe.notices()); n != 1 || rc.Armed() != 0 {
		t.Fatalf("%d notices, %d armed", n, rc.Armed())
	}
}

// With reminders off a pending reminder is not armed or held and does not
// keep the daemon up, a reminder asked for by voice is refused aloud and
// not kept, and oni.reminders answers disabled.
func TestOniRemindOffArmsNothingHoldsNothingAndSaysSo(t *testing.T) {
	probe := &remindProbe{}
	r, rc, store := remindRig(t, func(s *protocol.Settings) { s.Oni.ReflexSettings.Reminders = false }, probe,
		kept("0123456789abcdef", remind.KindReminder, "stretch", remindEpoch.Add(-time.Minute), remindEpoch.Add(10*time.Minute)))
	if rc.Armed() != 0 || r.d.remindResident() {
		t.Fatalf("reminders off: %d armed, resident %v", rc.Armed(), r.d.remindResident())
	}
	remindEventually(t, "the daemon idle", r.d.Idle)
	surface, _ := r.presenting(t)
	log := &frameLog{p: surface}
	r.chord(t, surface, "Remind me in 20 minutes to stretch", false)
	log.until("the refusal", spoken("Reminders are off. Turn them on in Oni's settings."))
	if n := len(remindKept(t, store)); n != 1 || rc.Armed() != 0 {
		t.Fatalf("%d kept, %d armed", n, rc.Armed())
	}
	cli := dialSocket(t, protocol.ClientCLI)
	err := cli.call(protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersList}, nil)
	if code, reason := reasonOf(err); code != protocol.CodeDisabled || reason != feature.ReasonFeatureOff {
		t.Fatalf("oni.reminders with reminders off: %v", err)
	}
	rc.Advance(time.Hour)
	if n := len(remindKept(t, store)); n != 1 || len(probe.wakeCalls()) != 0 || len(probe.notices()) != 0 {
		t.Fatalf("off yet %d kept, wake %v, notices %v", n, probe.wakeCalls(), probe.notices())
	}
}

// A timer needs voice and nothing more, as before reminders came: it is
// kept, armed, held and rings with reflex and reminders off, while a
// reminder asked for in the same state is refused and not kept.
func TestOniRemindTimerNeedsOnlyVoiceAndAReminderItsSwitch(t *testing.T) {
	probe := &remindProbe{}
	r, rc, store := remindRig(t, func(s *protocol.Settings) {
		s.Oni.Reflex = false
		s.Oni.ReflexSettings.Reminders = false
	}, probe)
	surface, _ := r.presenting(t)
	log := &frameLog{p: surface}
	r.chord(t, surface, "set a timer for five seconds", false)
	log.until("the timer confirmed", spoken("Timer set, 5 seconds."))
	if rs := remindKept(t, store); len(rs) != 1 || rs[0].Kind != remind.KindTimer || rc.Armed() != 1 {
		t.Fatalf("kept %+v, %d armed", rs, rc.Armed())
	}
	remindEventually(t, "the wake unit held", func() bool { return slices.Equal(probe.wakeCalls(), []bool{true}) })
	if !r.d.remindResident() {
		t.Fatal("a timer due in seconds does not keep the daemon up")
	}
	remindEventually(t, "voice at rest", func() bool { return !r.d.voiceBusy() })
	r.chord(t, surface, "Remind me in 20 minutes to stretch", false)
	log.until("the reminder refused", spoken("Reminders are off. Turn them on in Oni's settings."))
	if n := len(remindKept(t, store)); n != 1 {
		t.Fatalf("%d kept after a refused reminder", n)
	}
	remindEventually(t, "voice at rest", func() bool { return !r.d.voiceBusy() })
	rc.Advance(5 * time.Second)
	log.until("the timer's end said", spoken("Time's up."))
	remindEventually(t, "the timer done", func() bool { return len(remindKept(t, store)) == 0 })
	remindEventually(t, "the wake unit let go", func() bool { return slices.Equal(probe.wakeCalls(), []bool{true, false}) })
	if rc.Armed() != 0 {
		t.Fatalf("%d armed with nothing kept", rc.Armed())
	}
}

// Only the kinds that are on are armed, held and keep the daemon up: with
// reminders off a pending reminder neither rings nor holds anything while
// a timer beside it does; with voice off a pending timer is left alone
// while a reminder beside it rings.
func TestOniRemindArmsOnlyTheKindsThatAreOn(t *testing.T) {
	reminder := kept("0123456789abcdef", remind.KindReminder, "stretch", remindEpoch.Add(-time.Minute), remindEpoch.Add(time.Minute))
	timer := kept("fedcba9876543210", remind.KindTimer, "", remindEpoch.Add(-time.Minute), remindEpoch.Add(10*time.Minute))
	t.Run("reminders off", func(t *testing.T) {
		probe := &remindProbe{}
		r, rc, store := remindRig(t, func(s *protocol.Settings) { s.Oni.ReflexSettings.Reminders = false }, probe, reminder, timer)
		remindEventually(t, "the wake unit held for the timer", func() bool { return slices.Equal(probe.wakeCalls(), []bool{true}) })
		if rc.Armed() != 1 {
			t.Fatalf("%d armed", rc.Armed())
		}
		rc.Advance(time.Minute)
		if n := len(remindKept(t, store)); n != 2 || rc.Armed() != 1 {
			t.Fatalf("the reminder rang with reminders off: %d kept, %d armed", n, rc.Armed())
		}
		rc.Advance(9 * time.Minute)
		remindEventually(t, "the timer done", func() bool {
			rs := remindKept(t, store)
			return len(rs) == 1 && rs[0].Kind == remind.KindReminder
		})
		remindEventually(t, "the wake unit let go", func() bool { return slices.Equal(probe.wakeCalls(), []bool{true, false}) })
		if rc.Armed() != 0 || r.d.remindResident() {
			t.Fatalf("a reminder with reminders off: %d armed, resident %v", rc.Armed(), r.d.remindResident())
		}
	})
	t.Run("voice off", func(t *testing.T) {
		probe := &remindProbe{}
		r, rc, store := remindRig(t, func(s *protocol.Settings) { s.Oni.Voice = false }, probe, reminder, timer)
		remindEventually(t, "the wake unit held for the reminder", func() bool { return slices.Equal(probe.wakeCalls(), []bool{true}) })
		rc.Advance(time.Minute)
		remindEventually(t, "the reminder done", func() bool {
			rs := remindKept(t, store)
			return len(rs) == 1 && rs[0].Kind == remind.KindTimer
		})
		if got := probe.notices(); len(got) != 1 || got[0] != [2]string{"Reminder", "stretch"} {
			t.Fatalf("notices %q", got)
		}
		remindEventually(t, "the wake unit let go", func() bool { return slices.Equal(probe.wakeCalls(), []bool{true, false}) })
		rc.Advance(time.Hour)
		if n := len(remindKept(t, store)); n != 1 || rc.Armed() != 0 || r.d.remindResident() {
			t.Fatalf("a timer with voice off: %d kept, %d armed, resident %v", n, rc.Armed(), r.d.remindResident())
		}
	})
	t.Run("both off", func(t *testing.T) {
		probe := &remindProbe{}
		r, rc, store := remindRig(t, func(s *protocol.Settings) {
			s.Oni.Voice = false
			s.Oni.Reflex = false
		}, probe, reminder, timer)
		remindEventually(t, "the daemon idle", r.d.Idle)
		rc.Advance(time.Hour)
		if n := len(remindKept(t, store)); n != 2 || rc.Armed() != 0 || len(probe.wakeCalls()) != 0 || len(probe.notices()) != 0 {
			t.Fatalf("both off: %d kept, %d armed, wake %v, notices %v", n, rc.Armed(), probe.wakeCalls(), probe.notices())
		}
	})
}

// Voice turned off disarms a pending timer and lets the wake unit go;
// turned on again it arms and holds it.
func TestOniRemindVoiceSwitchArmsAndHoldsATimer(t *testing.T) {
	probe := &remindProbe{}
	r, rc, _ := remindRig(t, func(s *protocol.Settings) { s.Oni.Reflex = false }, probe,
		kept("fedcba9876543210", remind.KindTimer, "", remindEpoch.Add(-time.Minute), remindEpoch.Add(time.Hour)))
	remindEventually(t, "the wake unit held", func() bool { return slices.Equal(probe.wakeCalls(), []bool{true}) })
	r.patch(t, `{"oni":{"voice":false}}`)
	remindEventually(t, "the wake unit let go", func() bool { return slices.Equal(probe.wakeCalls(), []bool{true, false}) })
	if rc.Armed() != 0 {
		t.Fatalf("%d armed with voice off", rc.Armed())
	}
	r.patch(t, `{"oni":{"voice":true}}`)
	remindEventually(t, "the wake unit held again", func() bool { return slices.Equal(probe.wakeCalls(), []bool{true, false, true}) })
	if rc.Armed() != 1 {
		t.Fatalf("%d armed with voice on", rc.Armed())
	}
}

// Turning reminders off disarms the timer and lets the wake unit go;
// turning them on again arms and holds it; Oni turned off lets it go too.
func TestOniRemindSwitchesArmAndHoldTheWakeUnit(t *testing.T) {
	probe := &remindProbe{}
	r, rc, _ := remindRig(t, nil, probe, kept("0123456789abcdef", remind.KindReminder, "stretch", remindEpoch.Add(-time.Minute), remindEpoch.Add(48*time.Hour)))
	remindEventually(t, "the wake unit held", func() bool { return slices.Equal(probe.wakeCalls(), []bool{true}) })
	if rc.Armed() != 1 {
		t.Fatalf("%d armed", rc.Armed())
	}
	for _, step := range []struct {
		patch string
		armed int
		wakes []bool
	}{
		{`{"oni":{"reflexSettings":{"reminders":false}}}`, 0, []bool{true, false}},
		{`{"oni":{"reflexSettings":{"reminders":true}}}`, 1, []bool{true, false, true}},
		{`{"oni":{"reflex":false}}`, 0, []bool{true, false, true, false}},
		{`{"oni":{"reflex":true}}`, 1, []bool{true, false, true, false, true}},
		{`{"oni":{"master":false}}`, 0, []bool{true, false, true, false, true, false}},
	} {
		r.patch(t, step.patch)
		remindEventually(t, step.patch, func() bool { return slices.Equal(probe.wakeCalls(), step.wakes) })
		if rc.Armed() != step.armed {
			t.Fatalf("%s: %d armed", step.patch, rc.Armed())
		}
	}
}

// The daemon stays up at rest only while the next reminder is due within
// the residency horizon, or while any is pending and the wake unit cannot
// be armed, which oni.reminders reports.
func TestOniRemindKeepsTheDaemonUpOnlyWithinTheResidencyHorizon(t *testing.T) {
	horizon := remindLimits(t).ResidencyHorizon()
	for _, c := range []struct {
		name     string
		due      time.Duration
		noWake   bool
		resident bool
	}{
		{"within the horizon", horizon - time.Minute, false, true},
		{"already due", -time.Second, false, true},
		{"past the horizon", horizon + time.Minute, false, false},
		{"no wake unit", 48 * time.Hour, true, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			probe := &remindProbe{}
			if c.noWake {
				probe.wakeErr = teams.ErrNoWakeUnit
			}
			due := remindEpoch.Add(c.due)
			r, _, _ := remindRig(t, nil, probe, kept("0123456789abcdef", remind.KindReminder, "stretch", due.Add(-time.Hour), due))
			remindEventually(t, "the wake unit asked", func() bool { return len(probe.wakeCalls()) > 0 })
			remindEventually(t, "residency settled", func() bool { return r.d.remindResident() == c.resident })
			remindEventually(t, "no client left", r.d.restingBesideMemory)
			if r.d.Idle() == c.resident {
				t.Fatalf("idle %v with resident %v", r.d.Idle(), c.resident)
			}
			cli := dialSocket(t, protocol.ClientCLI)
			var res protocol.OniRemindersResult
			if err := cli.call(protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersList}, &res); err != nil {
				t.Fatal(err)
			}
			if res.NoWake != c.noWake {
				t.Fatalf("noWake %v, want %v", res.NoWake, c.noWake)
			}
			if !c.noWake {
				return
			}
			res = protocol.OniRemindersResult{}
			if err := cli.call(protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersCancelAll}, &res); err != nil || res.NoWake {
				t.Fatalf("cancel all: %+v %v", res, err)
			}
			if r.d.remindResident() {
				t.Fatal("resident with nothing pending")
			}
		})
	}
}

// A store that cannot be read arms nothing, holds nothing and is said so.
func TestOniRemindDamagedStoreArmsNothing(t *testing.T) {
	probe := &remindProbe{}
	r, rc, store := remindRig(t, nil, probe)
	cli := dialSocket(t, protocol.ClientCLI)
	if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path(), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := cli.call(protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersList}, nil)
	var refused *protocol.Error
	if !errors.As(err, &refused) || refused.Reason != remind.ReasonDamaged {
		t.Fatalf("list of a damaged store: %v", err)
	}
	if line, refused := r.d.remindSet("set a timer for five minutes", remindEpoch); line != "I can't read your reminders." || refused || rc.Armed() != 0 || r.d.remindResident() {
		t.Fatalf("set on a damaged store: %q, %d armed", line, rc.Armed())
	}
	if raw, err := os.ReadFile(store.Path()); err != nil || string(raw) != "{not json" || len(probe.wakeCalls()) != 0 {
		t.Fatalf("the damaged store changed: %q %v, wake %v", raw, err, probe.wakeCalls())
	}
}
