package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/remind"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func testWakeHorizon(t *testing.T) time.Duration {
	t.Helper()
	h, err := wakeHorizon()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func remindersOnSettings() protocol.Settings {
	return protocol.Settings{Enabled: true, Oni: protocol.OniSettings{Master: true, Reflex: true, ReflexSettings: protocol.OniReflexSettings{Reminders: true}}}
}

// keepReminder keeps one reminder due at due in the state directory's
// reminders file.
func keepReminder(t *testing.T, created, due time.Time) {
	t.Helper()
	loc := time.Local
	r, _, ok := remind.Parse("remind me in 5 minutes to stretch", created, loc)
	if !ok || r.Kind == "" {
		t.Fatalf("the parser claimed nothing: %+v", r)
	}
	r.Due = due
	if err := remind.NewStore("").Add(r); err != nil {
		t.Fatal(err)
	}
}

// The wake horizon is the spec's, not a number of the command's own.
func TestTheWakeHorizonIsTheSpecs(t *testing.T) {
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := testWakeHorizon(t), s.Reflex.Reminders.WakeHorizon(); got != want || want <= 0 {
		t.Fatalf("horizon %s, the spec's %s", got, want)
	}
}

// The wake command starts the daemon for a reminder due within the horizon,
// or already due, only while Hyakki, Oni, reflex and reminders are all on;
// never for one due later, nor with nothing kept.
func TestRoutinesWakeStartsTheDaemonOnlyForAReminderDueSoon(t *testing.T) {
	horizon := testWakeHorizon(t)
	now := time.Now()
	minute := time.Minute
	off := func(change func(*protocol.Settings)) protocol.Settings {
		s := remindersOnSettings()
		change(&s)
		return s
	}
	for _, check := range []struct {
		name     string
		settings protocol.Settings
		due      time.Duration // from now; 0 keeps nothing
		want     bool
	}{
		{"due within the horizon", remindersOnSettings(), horizon - minute, true},
		{"due at the horizon", remindersOnSettings(), horizon, true},
		{"already due", remindersOnSettings(), -2 * time.Hour, true},
		{"due past the horizon", remindersOnSettings(), horizon + minute, false},
		{"nothing kept", remindersOnSettings(), 0, false},
		{"hyakki off", off(func(s *protocol.Settings) { s.Enabled = false }), minute, false},
		{"oni off", off(func(s *protocol.Settings) { s.Oni.Master = false }), minute, false},
		{"reflex off", off(func(s *protocol.Settings) { s.Oni.Reflex = false }), minute, false},
		{"reminders off", off(func(s *protocol.Settings) { s.Oni.ReflexSettings.Reminders = false }), minute, false},
	} {
		t.Run(check.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			if check.due != 0 {
				created := now.Add(-3 * time.Hour)
				keepReminder(t, created, now.Add(check.due))
			}
			if got := wakeFor(check.settings, now, horizon); got != check.want {
				t.Fatalf("wake = %v, want %v", got, check.want)
			}
		})
	}
}

// A timer needs only voice: the wake command starts the daemon for one due
// within the horizon while Hyakki, Oni and voice are on, reflex and
// reminders off; never with voice, Oni or Hyakki off. A reminder in the
// same state wakes nothing.
func TestRoutinesWakeStartsTheDaemonForATimerWithVoiceAlone(t *testing.T) {
	horizon := testWakeHorizon(t)
	now := time.Now()
	voiceOn := func(change func(*protocol.Settings)) protocol.Settings {
		s := protocol.Settings{Enabled: true, Oni: protocol.OniSettings{Master: true, Voice: true}}
		if change != nil {
			change(&s)
		}
		return s
	}
	keepTimer := func(t *testing.T, due time.Time) {
		t.Helper()
		r, _, ok := remind.Parse("set a timer for 5 minutes", now.Add(-time.Hour), time.Local)
		if !ok || r.Kind != remind.KindTimer {
			t.Fatalf("the parser claimed no timer: %+v", r)
		}
		r.Due = due
		if err := remind.NewStore("").Add(r); err != nil {
			t.Fatal(err)
		}
	}
	for _, check := range []struct {
		name     string
		settings protocol.Settings
		timer    bool
		due      time.Duration
		want     bool
	}{
		{"timer due soon", voiceOn(nil), true, time.Minute, true},
		{"timer past the horizon", voiceOn(nil), true, horizon + time.Minute, false},
		{"timer with voice off", voiceOn(func(s *protocol.Settings) { s.Oni.Voice = false }), true, time.Minute, false},
		{"timer with oni off", voiceOn(func(s *protocol.Settings) { s.Oni.Master = false }), true, time.Minute, false},
		{"timer with hyakki off", voiceOn(func(s *protocol.Settings) { s.Enabled = false }), true, time.Minute, false},
		{"reminder with voice alone", voiceOn(nil), false, time.Minute, false},
	} {
		t.Run(check.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			if check.timer {
				keepTimer(t, now.Add(check.due))
			} else {
				keepReminder(t, now.Add(-time.Hour), now.Add(check.due))
			}
			if got := wakeFor(check.settings, now, horizon); got != check.want {
				t.Fatalf("wake = %v, want %v", got, check.want)
			}
		})
	}
}

// A team routine still wakes the daemon with reminders off, and Hyakki off
// wakes nothing at all.
func TestRoutinesWakeKeepsTheTeamsReasonAndHyakkiOffWins(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := os.MkdirAll(paths.TeamsDir(), 0700); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"teams":[{"id":"team-review","paused":false}],"routines":[{"team":"team-review","enabled":true}]}`)
	if err := os.WriteFile(filepath.Join(paths.TeamsDir(), "state.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	horizon := testWakeHorizon(t)
	now := time.Now()
	if !wakeFor(protocol.Settings{Enabled: true}, now, horizon) {
		t.Fatal("a team routine no longer wakes the daemon")
	}
	keepReminder(t, now.Add(-time.Hour), now.Add(time.Minute))
	if wakeFor(protocol.Settings{Oni: remindersOnSettings().Oni}, now, horizon) {
		t.Fatal("hyakki off still woke the daemon")
	}
}

// A damaged reminders file wakes nothing and is left as it is.
func TestRoutinesWakeIgnoresADamagedRemindersFile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := remind.DefaultStorePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}
	if wakeFor(remindersOnSettings(), time.Now(), testWakeHorizon(t)) {
		t.Fatal("a damaged file woke the daemon")
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != "{not json" {
		t.Fatalf("the damaged file changed: %q %v", raw, err)
	}
}
