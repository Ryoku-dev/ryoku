package demo

import (
	"slices"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

// Settings' Phase 17 pages read the demo's optional parts and remove what
// Hyakki put there; a part Ryoku installed, an unknown one and a bad id are
// refused and nothing changes.
func TestDemoComponentsListAndRemove(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOniPhase17)}
	listed := call(t, s, protocol.MComponents, protocol.Empty{}).(protocol.ComponentsResult).Components
	if len(listed) == 0 {
		t.Fatal("the demo has no parts")
	}
	for _, c := range listed {
		if !protocol.ValidComponentID(c.ID) || !slices.Contains(protocol.ComponentKinds, c.Kind) || !slices.Contains(protocol.ComponentStates, c.State) {
			t.Errorf("part %+v", c)
		}
		if c.Kind == protocol.ComponentProgram && (c.Removable || len(c.Needs) == 0 || c.Why == "") {
			t.Errorf("a program part %+v", c)
		}
	}
	after := call(t, s, protocol.MComponentRemove, protocol.ComponentRemoveParams{ID: "stt:base.en"}).(protocol.ComponentsResult).Components
	i := slices.IndexFunc(after, func(c protocol.Component) bool { return c.ID == "stt:base.en" })
	if i < 0 || after[i].State != protocol.ComponentAbsent || after[i].Bytes != 0 || after[i].Path != "" {
		t.Fatalf("after removing, %+v", after)
	}
	for id, code := range map[string]string{
		"stt:small.en":      protocol.CodeForbidden,
		"program:tesseract": protocol.CodeForbidden,
		"stt:nope":          protocol.CodeNotFound,
		"../x":              protocol.CodeBadRequest,
	} {
		if refused := oniCallErr(t, s, protocol.MComponentRemove, protocol.ComponentRemoveParams{ID: id}); refused.Code != code {
			t.Errorf("removing %s: %+v", id, refused)
		}
	}
}

// Activity counts what the stores hold, newest rows first, and deleting a
// store empties it alone; all empties every one.
func TestDemoActivityCountsAndClears(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOniPhase17)}
	got := call(t, s, protocol.MOniActivity, protocol.Empty{}).(protocol.OniActivityResult)
	if len(got.Counts) != len(protocol.OniActivityStores) || got.Counts[protocol.OniActivityReminders] != len(s.data.oni.reflex.reminders) || got.Counts[protocol.OniActivityReflexLog] != len(s.data.oni.reflex.rows) {
		t.Fatalf("counts %v", got.Counts)
	}
	if len(got.Rows) == 0 || len(got.Rows) > protocol.OniActivityRowsMax || !slices.IsSortedFunc(got.Rows, func(a, b protocol.OniActivityRow) int { return b.At.Compare(a.At) }) || got.Since != got.Rows[len(got.Rows)-1].At {
		t.Fatalf("rows %+v since %v", got.Rows, got.Since)
	}
	routines := got.Counts[protocol.OniActivityRoutines]
	if refused := oniCallErr(t, s, protocol.MOniActivityClear, protocol.OniActivityClearParams{Stores: []string{protocol.OniActivityReminders}}); refused.Code != protocol.CodeConflict {
		t.Fatalf("clearing while Oni is on: %+v", refused)
	}
	s.data.snapshot.Settings.Oni.Master = false
	one := call(t, s, protocol.MOniActivityClear, protocol.OniActivityClearParams{Stores: []string{protocol.OniActivityReminders}}).(protocol.OniActivityResult)
	if one.Counts[protocol.OniActivityReminders] != 0 || one.Counts[protocol.OniActivityRoutines] != routines || len(s.data.oni.reflex.reminders) != 0 {
		t.Fatalf("clearing reminders left %v", one.Counts)
	}
	all := call(t, s, protocol.MOniActivityClear, protocol.OniActivityClearParams{Stores: []string{protocol.OniActivityAll}}).(protocol.OniActivityResult)
	for store, n := range all.Counts {
		if n != 0 {
			t.Errorf("after all, %s holds %d", store, n)
		}
	}
	if len(all.Rows) != 0 || !all.Since.IsZero() {
		t.Errorf("after all, rows %+v", all.Rows)
	}
	for _, stores := range [][]string{nil, {"memory"}, {protocol.OniActivityAll, protocol.OniActivityRoutines}} {
		if refused := oniCallErr(t, s, protocol.MOniActivityClear, protocol.OniActivityClearParams{Stores: stores}); refused.Code != protocol.CodeBadRequest {
			t.Errorf("clearing %q: %+v", stores, refused)
		}
	}
	s.data.snapshot.Settings.Oni.Master = true
	s.data.snapshot.Settings.Oni.Assignment.Enabled = true
	s.data.snapshot.Settings.Oni.Hands = protocol.OniHandsBrowser
	if log := call(t, s, protocol.MOniAssignmentLog, protocol.Empty{}).(protocol.OniAssignmentLogResult); len(log.Rows) != 0 || log.Day != demoAssignmentDay() {
		t.Fatalf("assignment history came back, or its budget was reset: %+v", log)
	}
	if log := call(t, s, protocol.MOniHandsLog, protocol.Empty{}).(protocol.OniHandsLogResult); len(log.Rows) != 0 {
		t.Fatalf("Hands history came back: %+v", log)
	}
}

// The bridge's state follows Settings: the tools turned on, the approved
// program and the one waiting; off, no tool and no waiting program.
func TestDemoMCPStatusFollowsSettings(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOniPhase17)}
	got := call(t, s, protocol.MOniMCP, protocol.Empty{}).(protocol.OniMCPStatus)
	if !got.Enabled || !slices.Equal(got.Tools, []string{protocol.OniMCPRemindersList, protocol.OniMCPNotify}) || len(got.Clients) != 2 ||
		got.Clients[0] != (protocol.OniMCPClient{Name: demoMCPApproved, State: protocol.OniMCPClientApproved}) ||
		got.Clients[1] != (protocol.OniMCPClient{Name: demoMCPWaiting, State: protocol.OniMCPClientWaiting, Connected: true}) {
		t.Fatalf("status %+v", got)
	}
	s.data.snapshot.Settings.Oni.MCP.Enabled = false
	if off := call(t, s, protocol.MOniMCP, protocol.Empty{}).(protocol.OniMCPStatus); off.Enabled || len(off.Tools) != 0 || len(off.Clients) != 1 {
		t.Fatalf("off %+v", off)
	}
	// a plain Oni demo has the bridge off
	plain := &server{data: makeFixtures(FeatureOni)}
	if st := call(t, plain, protocol.MOniMCP, protocol.Empty{}).(protocol.OniMCPStatus); st.Enabled || len(st.Clients) != 0 {
		t.Fatalf("plain %+v", st)
	}
}

// A line goes on the pill while Oni is on and shown; a bad line is refused.
func TestDemoNotifyAndReminderAdd(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOniPhase17)}
	call(t, s, protocol.MOniNotify, protocol.OniNotifyParams{Text: "Build finished"})
	if !slices.Equal(s.data.oni.phase17.notified, []string{"Build finished"}) {
		t.Fatalf("notified %q", s.data.oni.phase17.notified)
	}
	if refused := oniCallErr(t, s, protocol.MOniNotify, protocol.OniNotifyParams{Text: "two\nlines"}); refused.Code != protocol.CodeBadRequest {
		t.Errorf("a bad line: %+v", refused)
	}
	s.data.snapshot.Settings.Oni.Hidden = true
	if refused := oniCallErr(t, s, protocol.MOniNotify, protocol.OniNotifyParams{Text: "Build finished"}); refused.Code != protocol.CodeDisabled {
		t.Errorf("a line while hidden: %+v", refused)
	}
	before := len(s.data.oni.reflex.reminders)
	due := Epoch.Add(2 * time.Hour)
	got := call(t, s, protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersAdd, Kind: protocol.OniReminderKindReminder, Due: due, Text: "Stretch"}).(protocol.OniRemindersResult)
	if len(got.Reminders) != before+1 || !slices.IsSortedFunc(got.Reminders, func(a, b protocol.OniReminderInfo) int { return a.Due.Compare(b.Due) }) ||
		!slices.ContainsFunc(got.Reminders, func(r protocol.OniReminderInfo) bool {
			return r.Text == "Stretch" && r.Due.Equal(due) && protocol.ValidOniReminderID(r.ID)
		}) {
		t.Fatalf("after an add %+v", got.Reminders)
	}
}
