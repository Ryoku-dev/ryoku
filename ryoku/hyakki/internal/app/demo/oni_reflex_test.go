package demo

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// reflexDemo is the Oni demo with Reflex and every reflex switch on
func reflexDemo() *server {
	s := &server{data: makeFixtures(FeatureOni)}
	s.data.snapshot.Settings.Oni.Reflex = true
	s.data.snapshot.Settings.Oni.ReflexSettings = protocol.OniReflexSettings{Routines: true, Reminders: true, Notes: true, Decisions: true}
	return s
}

func TestDemoReflexCallsNeedTheirFeatures(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOni)}
	calls := []struct {
		method string
		params any
	}{
		{protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersList}},
		{protocol.MOniRoutines, protocol.OniRoutinesParams{Action: protocol.OniRoutinesList}},
		{protocol.MOniReflexLog, protocol.Empty{}},
	}
	for _, c := range calls {
		if refused := oniCallErr(t, s, c.method, c.params); refused.Code != protocol.CodeDisabled || refused.Reason != feature.ReasonFeatureOff {
			t.Errorf("%s with reflex off: %+v", c.method, refused)
		}
	}
	// a reflex switch without Reflex is still off
	s.data.snapshot.Settings.Oni.ReflexSettings = protocol.OniReflexSettings{Routines: true, Reminders: true}
	for _, c := range calls[:2] {
		if refused := oniCallErr(t, s, c.method, c.params); refused.Code != protocol.CodeDisabled || refused.Reason != feature.ReasonFeatureOff {
			t.Errorf("%s with Reflex off: %+v", c.method, refused)
		}
	}
	s = reflexDemo()
	s.data.snapshot.Settings.Oni.Master = false
	for _, c := range calls {
		if refused := oniCallErr(t, s, c.method, c.params); refused.Code != protocol.CodeDisabled || refused.Reason != feature.ReasonOniOff {
			t.Errorf("%s with Oni off: %+v", c.method, refused)
		}
	}
	s = reflexDemo()
	for _, c := range [][2]string{
		{protocol.MOniReminders, `{}`},
		{protocol.MOniReminders, `{"action":"cancel","id":"x"}`},
		{protocol.MOniRoutines, `{"action":"forget"}`},
		{protocol.MOniRoutines, `{"action":"save","name":"Morning"}`},
		{protocol.MOniReflexLog, `{"words":true}`},
	} {
		var refused *protocol.Error
		if _, err := s.call(c[0], json.RawMessage(c[1])); !errors.As(err, &refused) || refused.Code != protocol.CodeBadRequest {
			t.Errorf("%s %s: %v", c[0], c[1], err)
		}
	}
}

// Two demos answer the same calls the same way, whenever they run.
func TestDemoReflexAnswersAreDeterministic(t *testing.T) {
	run := func() []any {
		s := reflexDemo()
		return []any{
			call(t, s, protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersList}),
			call(t, s, protocol.MOniRoutines, protocol.OniRoutinesParams{Action: protocol.OniRoutinesList}),
			call(t, s, protocol.MOniReflexLog, protocol.Empty{}),
			call(t, s, protocol.MOniRoutines, protocol.OniRoutinesParams{Action: protocol.OniRoutinesRun, Name: "wind  DOWN"}),
			call(t, s, protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersCancel, ID: "5d2c9a0e7b41f386"}),
		}
	}
	first, second := run(), run()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("the demo answered\n%#v\nthen\n%#v", first, second)
	}
	listed := first[0].(protocol.OniRemindersResult)
	if len(listed.Reminders) != 2 || !listed.Reminders[0].Due.Before(listed.Reminders[1].Due) || listed.Reminders[0].Kind != protocol.OniReminderKindTimer {
		t.Fatalf("reminders %+v", listed)
	}
	for _, r := range listed.Reminders {
		if !protocol.ValidOniReminderID(r.ID) || !r.Due.After(Epoch) || r.Late {
			t.Errorf("reminder %+v", r)
		}
	}
	routines := first[1].(protocol.OniRoutinesResult)
	if len(routines.Routines) != 2 || routines.Routines[0].Name != "Morning" || routines.Ran {
		t.Fatalf("routines %+v", routines)
	}
	rows := first[2].(protocol.OniReflexLogResult).Rows
	if len(rows) == 0 {
		t.Fatal("the demo ledger is empty")
	}
	for i, row := range rows {
		if row.Route != protocol.DecisionRules && row.Route != protocol.DecisionLocal && row.Route != protocol.DecisionEmber {
			t.Errorf("row %d route %q", i, row.Route)
		}
		if i > 0 && row.At.After(rows[i-1].At) {
			t.Errorf("row %d is newer than the row before it", i)
		}
		found := false
		for _, o := range protocol.OniReflexOutcomes {
			found = found || o == row.Outcome
		}
		if !found {
			t.Errorf("row %d outcome %q", i, row.Outcome)
		}
	}
	ran := first[3].(protocol.OniRoutinesResult)
	if !ran.Ran || ran.Routines[0].Name != "Wind down" || !ran.Routines[0].Used.Equal(Epoch) {
		t.Fatalf("a run %+v", ran)
	}
	if cancelled := first[4].(protocol.OniRemindersResult); len(cancelled.Reminders) != 1 || cancelled.Reminders[0].Text != "call the dentist" {
		t.Fatalf("after a cancel %+v", cancelled)
	}
}

func TestDemoReflexCancelsAndForgets(t *testing.T) {
	s := reflexDemo()
	if refused := oniCallErr(t, s, protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersCancel, ID: "ffffffffffffffff"}); refused.Code != protocol.CodeNotFound {
		t.Fatalf("cancelling a reminder that is not set: %+v", refused)
	}
	if refused := oniCallErr(t, s, protocol.MOniRoutines, protocol.OniRoutinesParams{Action: protocol.OniRoutinesForget, Name: "Evening"}); refused.Code != protocol.CodeNotFound {
		t.Fatalf("forgetting a routine that is not kept: %+v", refused)
	}
	if refused := oniCallErr(t, s, protocol.MOniRoutines, protocol.OniRoutinesParams{Action: protocol.OniRoutinesRun, Name: "Evening"}); refused.Code != protocol.CodeNotFound {
		t.Fatalf("running a routine that is not kept: %+v", refused)
	}
	ran := call(t, s, protocol.MOniRoutines, protocol.OniRoutinesParams{Action: protocol.OniRoutinesRun}).(protocol.OniRoutinesResult)
	if !ran.Ran || ran.Routines[0].Name != "Morning" {
		t.Fatalf("running the latest routine: %+v", ran)
	}
	forgot := call(t, s, protocol.MOniRoutines, protocol.OniRoutinesParams{Action: protocol.OniRoutinesForget, Name: "morning"}).(protocol.OniRoutinesResult)
	if len(forgot.Routines) != 1 || forgot.Routines[0].Name != "Wind down" || forgot.Ran {
		t.Fatalf("after forgetting one: %+v", forgot)
	}
	if all := call(t, s, protocol.MOniRoutines, protocol.OniRoutinesParams{Action: protocol.OniRoutinesForgetAll}).(protocol.OniRoutinesResult); all.Routines == nil || len(all.Routines) != 0 {
		t.Fatalf("after forgetting every one: %#v", all)
	}
	if refused := oniCallErr(t, s, protocol.MOniRoutines, protocol.OniRoutinesParams{Action: protocol.OniRoutinesRun}); refused.Code != protocol.CodeNotFound {
		t.Fatalf("running with nothing kept: %+v", refused)
	}
	if all := call(t, s, protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersCancelAll}).(protocol.OniRemindersResult); all.Reminders == nil || len(all.Reminders) != 0 {
		t.Fatalf("after cancelling every one: %#v", all)
	}
	raw, err := json.Marshal(call(t, s, protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersList}))
	if err != nil || string(raw) != `{"reminders":[]}` {
		t.Fatalf("an empty list is %s, %v", raw, err)
	}
}

// The reflex sessions start with Oni, talking and quick commands on with
// routines, reminders and notes, decisions left off; the no-wake one says
// Oni stays running for a reminder. Neither switch leaks into the features.
func TestDemoReflexSessions(t *testing.T) {
	for _, session := range []string{FeatureOniReflex, FeatureOniReflexNoWake} {
		s := &server{data: makeFixtures(session)}
		if s.data.snapshot.Settings.Features[session] {
			t.Fatalf("the demo switch %s leaked into the feature flags", session)
		}
		oni := s.data.snapshot.Settings.Oni
		if !oni.Master || !oni.Voice || !oni.Reflex || oni.ReflexSettings != (protocol.OniReflexSettings{Routines: true, Reminders: true, Notes: true}) {
			t.Fatalf("%s starts with %+v", session, oni)
		}
		listed := call(t, s, protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersList}).(protocol.OniRemindersResult)
		if listed.NoWake != (session == FeatureOniReflexNoWake) || len(listed.Reminders) != 2 {
			t.Fatalf("%s lists %+v", session, listed)
		}
		cancelled := call(t, s, protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersCancelAll}).(protocol.OniRemindersResult)
		if cancelled.NoWake != listed.NoWake {
			t.Fatalf("%s: a cancel answers %+v", session, cancelled)
		}
	}
}
