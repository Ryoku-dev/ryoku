package remind

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(filepath.Join(t.TempDir(), "state", "hyakki", "oni-reminders.json"))
}

func reminderAt(created time.Time, due time.Duration, kind, text string) Reminder {
	return Reminder{ID: newID(), Kind: kind, Created: created, Due: created.Add(due), Text: text}
}

func reason(err error) string {
	var pe *protocol.Error
	if errors.As(err, &pe) {
		return pe.Reason
	}
	return ""
}

func TestStoreDefaultPath(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/state-home")
	if got := NewStore("").Path(); got != "/tmp/state-home/hyakki/oni-reminders.json" {
		t.Fatalf("path %q", got)
	}
}

func TestStoreFileIsPrivate(t *testing.T) {
	s := newStore(t)
	now, _ := monday(t)
	if err := s.Add(reminderAt(now, time.Hour, KindReminder, "call the bank")); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(s.Path())
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode %v %v", fi.Mode(), err)
	}
	di, err := os.Stat(filepath.Dir(s.Path()))
	if err != nil || di.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode %v %v", di.Mode(), err)
	}
	left, _ := filepath.Glob(filepath.Join(filepath.Dir(s.Path()), ".*"))
	if len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
}

func TestStoreHoldsOnlyTheReminderWords(t *testing.T) {
	s := newStore(t)
	now, loc := monday(t)
	r, _, ok := Parse("Please could you remind me in ten minutes to stretch my back", now, loc)
	if !ok {
		t.Fatal("not claimed")
	}
	if err := s.Add(r); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"stretch my back"`)) {
		t.Fatalf("own words missing: %s", data)
	}
	for _, said := range []string{"please", "Please", "remind me", "ten minutes", "could you"} {
		if bytes.Contains(data, []byte(said)) {
			t.Errorf("file keeps %q from the request: %s", said, data)
		}
	}
	got, err := NewStore(s.Path()).List()
	if err != nil || len(got) != 1 || got[0].ID != r.ID || !got[0].Due.Equal(r.Due) || !got[0].Created.Equal(r.Created) || got[0].Text != r.Text {
		t.Fatalf("reopened: %+v %v", got, err)
	}
}

func TestStoreAddChecksBounds(t *testing.T) {
	s := newStore(t)
	now, _ := monday(t)
	long := strings.Repeat("x", MaxTextRunes+1)
	for name, r := range map[string]Reminder{
		"no id":            {Kind: KindReminder, Created: now, Due: now.Add(time.Hour)},
		"bad id":           {ID: "NOT-AN-ID-123456", Kind: KindReminder, Created: now, Due: now.Add(time.Hour)},
		"kind":             {ID: newID(), Kind: "alarm", Created: now, Due: now.Add(time.Hour)},
		"due before":       {ID: newID(), Kind: KindReminder, Created: now, Due: now.Add(-time.Second)},
		"due at creation":  {ID: newID(), Kind: KindReminder, Created: now, Due: now},
		"too far":          {ID: newID(), Kind: KindReminder, Created: now, Due: now.Add(MaxReminder + dstSlack + time.Second)},
		"timer too long":   {ID: newID(), Kind: KindTimer, Created: now, Due: now.Add(MaxTimer + time.Second)},
		"long words":       {ID: newID(), Kind: KindReminder, Created: now, Due: now.Add(time.Hour), Text: long},
		"control":          {ID: newID(), Kind: KindReminder, Created: now, Due: now.Add(time.Hour), Text: "call\x1b[2Jthe bank"},
		"format character": {ID: newID(), Kind: KindReminder, Created: now, Due: now.Add(time.Hour), Text: "call\u202ethe bank"},
		"untrimmed":        {ID: newID(), Kind: KindReminder, Created: now, Due: now.Add(time.Hour), Text: " call "},
		"bad utf8":         {ID: newID(), Kind: KindReminder, Created: now, Due: now.Add(time.Hour), Text: "\xff"},
	} {
		if err := s.Add(r); err == nil {
			t.Errorf("%s: kept", name)
		}
	}
	if _, err := os.Stat(s.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a refused reminder wrote the file")
	}
	r := reminderAt(now, time.Hour, KindReminder, "call")
	if err := s.Add(r); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(r); err == nil {
		t.Fatal("same id kept twice")
	}
	if err := s.Add(reminderAt(now, MaxReminder+time.Hour, KindReminder, "")); err != nil {
		t.Fatalf("a week on the wall clock across a change refused: %v", err)
	}
}

func TestStoreIsBounded(t *testing.T) {
	s := newStore(t)
	now, _ := monday(t)
	for i := range MaxPending {
		if err := s.Add(reminderAt(now, time.Duration(i+1)*time.Minute, KindReminder, "")); err != nil {
			t.Fatal(i, err)
		}
	}
	err := s.Add(reminderAt(now, time.Minute, KindTimer, ""))
	if reason(err) != ReasonFull {
		t.Fatalf("past MaxPending: %v", err)
	}
	if rs, _ := s.List(); len(rs) != MaxPending {
		t.Fatalf("%d kept", len(rs))
	}
}

func TestStoreDamagedFileIsNeverOverwritten(t *testing.T) {
	now, _ := monday(t)
	good := reminderAt(now, time.Hour, KindReminder, "call")
	for name, content := range map[string]string{
		"garbage":       "{not json",
		"version":       `{"v":2,"reminders":[]}`,
		"unknown field": `{"v":1,"reminders":[],"transcript":"remind me"}`,
		"trailing":      `{"v":1,"reminders":[]} {}`,
		"bad entry":     `{"v":1,"reminders":[{"id":"zz","kind":"reminder","due":"2026-11-23T16:00:00Z","created":"2026-11-23T15:00:00Z"}]}`,
		"duplicate id": `{"v":1,"reminders":[` +
			`{"id":"0123456789abcdef","kind":"timer","due":"2026-11-23T16:00:00Z","created":"2026-11-23T15:00:00Z"},` +
			`{"id":"0123456789abcdef","kind":"timer","due":"2026-11-23T16:01:00Z","created":"2026-11-23T15:00:00Z"}]}`,
		"too many": `{"v":1,"reminders":[` + strings.TrimSuffix(strings.Repeat(
			`{"id":"0123456789abcdef","kind":"timer","due":"2026-11-23T16:00:00Z","created":"2026-11-23T15:00:00Z"},`, MaxPending+1), ",") + `]}`,
		"too large": strings.Repeat(" ", MaxStoreBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			if err := os.MkdirAll(filepath.Dir(s.Path()), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(s.Path(), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := s.Add(good); reason(err) != ReasonDamaged {
				t.Fatalf("add: %v", err)
			}
			if _, err := s.List(); reason(err) != ReasonDamaged {
				t.Fatalf("list: %v", err)
			}
			if err := s.Done(good.ID); reason(err) != ReasonDamaged {
				t.Fatalf("done: %v", err)
			}
			if s.Cancel(good.ID) || s.CancelTimers() != 0 || s.CancelReminders() != 0 || s.CancelAll() != 0 || s.CancelBy(CancelReminder) != 0 {
				t.Fatal("cancelled from a damaged file")
			}
			if s.DueWithin(now, 365*24*time.Hour) || len(s.Due(now.Add(time.Hour))) != 0 {
				t.Fatal("a damaged file is due")
			}
			if _, ok := s.Next(now); ok {
				t.Fatal("a damaged file has a next reminder")
			}
			if data, _ := os.ReadFile(s.Path()); string(data) != content {
				t.Fatal("damaged file changed")
			}
			if err := s.Reset(); err != nil {
				t.Fatal(err)
			}
			if data, _ := os.ReadFile(s.DamagedPath()); string(data) != content {
				t.Fatal("reset did not move the file aside whole")
			}
			if err := s.Add(good); err != nil {
				t.Fatalf("after reset: %v", err)
			}
			if rs, err := s.List(); err != nil || len(rs) != 1 {
				t.Fatalf("after reset: %v %v", rs, err)
			}
		})
	}
	if err := newStore(t).Reset(); err != nil {
		t.Fatalf("reset with no file: %v", err)
	}
}

func TestStoreDueNextDone(t *testing.T) {
	s := newStore(t)
	now, _ := monday(t)
	set := now.Add(-time.Hour)
	early := reminderAt(set, 30*time.Minute, KindReminder, "early")
	missed := reminderAt(set, 50*time.Minute, KindTimer, "")
	soon := reminderAt(set, 70*time.Minute, KindReminder, "soon")
	later := reminderAt(set, 3*time.Hour, KindReminder, "later")
	for _, r := range []Reminder{later, soon, missed, early} {
		if err := s.Add(r); err != nil {
			t.Fatal(err)
		}
	}
	due := s.Due(now)
	if len(due) != 2 || due[0].ID != early.ID || due[1].ID != missed.ID {
		t.Fatalf("due %+v", due)
	}
	if !due[0].Late(now) || due[1].Late(due[1].Due.Add(time.Second)) {
		t.Fatal("late marking")
	}
	next, ok := s.Next(now)
	if !ok || next.ID != soon.ID {
		t.Fatalf("next %+v %v", next, ok)
	}
	for _, r := range due {
		if err := s.Done(r.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Done(early.ID); err == nil {
		t.Fatal("done twice")
	}
	if len(s.Due(now)) != 0 {
		t.Fatal("fired reminders still due")
	}
	rs, _ := s.List()
	if len(rs) != 2 || rs[0].ID != soon.ID || rs[1].ID != later.ID {
		t.Fatalf("left %+v", rs)
	}
	if _, ok := s.Next(later.Due); ok {
		t.Fatal("next past the last")
	}
}

func TestStoreCancels(t *testing.T) {
	s := newStore(t)
	now, _ := monday(t)
	t1 := reminderAt(now, time.Minute, KindTimer, "")
	t2 := reminderAt(now, 2*time.Minute, KindTimer, "eggs")
	r1 := reminderAt(now, time.Hour, KindReminder, "first")
	r2 := reminderAt(now, 2*time.Hour, KindReminder, "second")
	r3 := reminderAt(now, 3*time.Hour, KindReminder, "third")
	for _, r := range []Reminder{t1, t2, r1, r2, r3} {
		if err := s.Add(r); err != nil {
			t.Fatal(err)
		}
	}
	if n := s.CancelBy(CancelTimers); n != 2 {
		t.Fatalf("timers %d", n)
	}
	if n := s.CancelBy(CancelReminder); n != 1 {
		t.Fatalf("next reminder %d", n)
	}
	if rs, _ := s.List(); len(rs) != 2 || rs[0].ID != r2.ID {
		t.Fatalf("after next %+v", rs)
	}
	if !s.Cancel(r3.ID) || s.Cancel(r3.ID) {
		t.Fatal("cancel by id")
	}
	if n := s.CancelBy(CancelReminders); n != 1 {
		t.Fatalf("reminders %d", n)
	}
	if _, err := os.Stat(s.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a file is left with nothing pending")
	}
	for _, r := range []Reminder{reminderAt(now, time.Minute, KindTimer, ""), reminderAt(now, time.Hour, KindReminder, "")} {
		if err := s.Add(r); err != nil {
			t.Fatal(err)
		}
	}
	if n := s.CancelBy(CancelAll); n != 2 {
		t.Fatalf("all %d", n)
	}
	if _, err := os.Stat(s.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a file is left after cancelling all")
	}
	if s.CancelBy(CancelTimers) != 0 || s.CancelBy("unknown") != 0 {
		t.Fatal("cancelled from nothing")
	}
}

// DueWithin compares instants, so the wall clock jumping forward or back,
// or a midnight in between, changes nothing.
func TestStoreDueWithinAcrossDaylightSavingAndMidnight(t *testing.T) {
	loc := zone(t)
	s := newStore(t)
	if s.DueWithin(time.Now(), time.Hour) {
		t.Fatal("an empty store is due")
	}
	// 23:50 on the Saturday before clocks go forward; 3:10 on Sunday is
	// 2 h 20 min away, though the wall clocks are 3 h 20 min apart.
	now := wallAt(t, loc, "2026-03-07 23:50")
	r, _, ok := Parse("remind me at 3:10 am", now, loc)
	if !ok || r.Kind != KindReminder {
		t.Fatal("not set")
	}
	if err := s.Add(r); err != nil {
		t.Fatal(err)
	}
	if s.DueWithin(now, 2*time.Hour+19*time.Minute) || !s.DueWithin(now, 2*time.Hour+20*time.Minute) {
		t.Fatalf("spring forward: due in %v", r.Due.Sub(now))
	}
	s.CancelAll()
	// Clocks go back: from 0:50, 1:30 the second time is 1 h 40 min away.
	now = time.Date(2026, 11, 1, 4, 50, 0, 0, time.UTC) // 0:50 EDT
	if err := s.Add(reminderAt(now, 100*time.Minute, KindReminder, "")); err != nil {
		t.Fatal(err)
	}
	if s.DueWithin(now, 99*time.Minute) || !s.DueWithin(now, 100*time.Minute) {
		t.Fatal("fall back")
	}
	s.CancelAll()
	// Midnight: 23:50 to 00:15 is 25 minutes.
	now = wallAt(t, loc, "2026-11-23 23:50")
	r, _, _ = Parse("remind me at 12:15 am", now, loc)
	if err := s.Add(r); err != nil {
		t.Fatal(err)
	}
	if s.DueWithin(now, 24*time.Minute) || !s.DueWithin(now, 25*time.Minute) {
		t.Fatal("midnight")
	}
	// A reminder already due is due within any horizon.
	if !s.DueWithin(r.Due.Add(time.Hour), 0) {
		t.Fatal("overdue not due")
	}
}
