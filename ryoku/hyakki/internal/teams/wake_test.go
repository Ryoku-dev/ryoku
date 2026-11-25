package teams

import (
	"errors"
	"slices"
	"testing"
)

type unitRecorder struct {
	calls []bool
	fail  error
}

func (u *unitRecorder) set(on bool) error {
	u.calls = append(u.calls, on)
	if on {
		return u.fail
	}
	return nil
}

// The unit stays on while either the teams or the reminders hold it, goes
// off only when both let go, and is set only when that changes.
func TestTheWakeUnitIsOnWhileAnyReasonHoldsIt(t *testing.T) {
	u := &unitRecorder{}
	w := NewWakeOwner(u.set)
	steps := []struct {
		reason WakeReason
		on     bool
		unit   []bool
	}{
		{WakeTeams, false, []bool{false}},
		{WakeTeams, true, []bool{false, true}},
		{WakeReminders, true, []bool{false, true}},
		{WakeTeams, false, []bool{false, true}},
		{WakeReminders, true, []bool{false, true}},
		{WakeReminders, false, []bool{false, true, false}},
		{WakeReminders, false, []bool{false, true, false}},
		{WakeReminders, true, []bool{false, true, false, true}},
		{WakeTeams, true, []bool{false, true, false, true}},
		{WakeReminders, false, []bool{false, true, false, true}},
		{WakeTeams, false, []bool{false, true, false, true, false}},
	}
	for i, s := range steps {
		if err := w.Hold(s.reason, s.on); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		if !slices.Equal(u.calls, s.unit) {
			t.Fatalf("step %d (%s %v): unit set %v, want %v", i, s.reason, s.on, u.calls, s.unit)
		}
		if w.Held(s.reason) != s.on {
			t.Fatalf("step %d: %s held %v", i, s.reason, w.Held(s.reason))
		}
	}
}

// A unit that cannot be armed is reported to the reason holding it, and
// every later hold tries again until it is armed; letting go never fails.
func TestAWakeUnitThatCannotBeArmedIsReportedAndRetried(t *testing.T) {
	u := &unitRecorder{fail: ErrNoWakeUnit}
	w := NewWakeOwner(u.set)
	if err := w.Hold(WakeReminders, true); !errors.Is(err, ErrNoWakeUnit) {
		t.Fatalf("hold: %v", err)
	}
	if err := w.Hold(WakeTeams, true); !errors.Is(err, ErrNoWakeUnit) {
		t.Fatalf("second hold: %v", err)
	}
	u.fail = nil
	if err := w.Hold(WakeReminders, true); err != nil {
		t.Fatalf("armed hold: %v", err)
	}
	if err := w.Hold(WakeReminders, true); err != nil || len(u.calls) != 3 {
		t.Fatalf("an armed unit was set again: %v %v", err, u.calls)
	}
	u.fail = ErrNoWakeUnit
	if err := w.Hold(WakeTeams, false); err != nil {
		t.Fatal(err)
	}
	if err := w.Hold(WakeReminders, false); err != nil {
		t.Fatalf("letting go failed: %v", err)
	}
	if want := []bool{true, true, true, false}; !slices.Equal(u.calls, want) {
		t.Fatalf("unit set %v, want %v", u.calls, want)
	}
}

// The teams' scheduler holds the shared unit only for the teams reason.
func TestTheTeamsHoldOnlyTheirOwnReason(t *testing.T) {
	u := &unitRecorder{}
	saved := wakeUnit
	wakeUnit = NewWakeOwner(u.set)
	t.Cleanup(func() { wakeUnit = saved })
	if err := HoldWakeForReminders(true); err != nil {
		t.Fatal(err)
	}
	holdWakeForTeams(false)
	if !wakeUnit.Held(WakeReminders) || wakeUnit.Held(WakeTeams) || !slices.Equal(u.calls, []bool{true}) {
		t.Fatalf("the teams letting go turned the unit off under a reminder: %v", u.calls)
	}
	holdWakeForTeams(true)
	if err := HoldWakeForReminders(false); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(u.calls, []bool{true}) {
		t.Fatalf("a reminder letting go turned the unit off under a routine: %v", u.calls)
	}
	holdWakeForTeams(false)
	if !slices.Equal(u.calls, []bool{true, false}) {
		t.Fatalf("unit set %v", u.calls)
	}
}
