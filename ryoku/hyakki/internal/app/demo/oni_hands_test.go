package demo

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// handsDemo is the Oni demo with Hands in a browser on
func handsDemo() *server {
	s := &server{data: makeFixtures(FeatureOni)}
	s.data.snapshot.Settings.Oni.Hands = protocol.OniHandsBrowser
	return s
}

func TestDemoHandsCallsNeedHandsInABrowser(t *testing.T) {
	calls := []struct {
		method string
		params any
	}{
		{protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsStatus}},
		{protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsOpen}},
		{protocol.MOniHandsLog, protocol.Empty{}},
	}
	s := &server{data: makeFixtures(FeatureOni)}
	for _, c := range calls {
		if refused := oniCallErr(t, s, c.method, c.params); refused.Code != protocol.CodeDisabled || refused.Reason != feature.ReasonFeatureOff {
			t.Errorf("%s with hands off: %+v", c.method, refused)
		}
	}
	// the browser stage builds on looking at windows
	s = handsDemo()
	s.data.snapshot.Settings.Oni.Inspect = false
	for _, c := range calls {
		if refused := oniCallErr(t, s, c.method, c.params); refused.Code != protocol.CodeDisabled || refused.Reason != feature.ReasonFeatureOff {
			t.Errorf("%s with looking at windows off: %+v", c.method, refused)
		}
	}
	s = handsDemo()
	s.data.snapshot.Settings.Oni.Master = false
	for _, c := range calls {
		if refused := oniCallErr(t, s, c.method, c.params); refused.Code != protocol.CodeDisabled || refused.Reason != feature.ReasonOniOff {
			t.Errorf("%s with Oni off: %+v", c.method, refused)
		}
	}
	s = handsDemo()
	for _, c := range [][2]string{
		{protocol.MOniHands, `{}`},
		{protocol.MOniHands, `{"action":"yes"}`},
		{protocol.MOniHands, `{"action":"start"}`},
		{protocol.MOniHands, `{"action":"authorize","tab":"D1A2-B3C4"}`},
		{protocol.MOniHandsLog, `{"words":true}`},
	} {
		var refused *protocol.Error
		if _, err := s.call(c[0], json.RawMessage(c[1])); !errors.As(err, &refused) || refused.Code != protocol.CodeBadRequest {
			t.Errorf("%s %s: %v", c[0], c[1], err)
		}
	}
}

// Two demos answer the same Hands calls the same way: the browser opens on
// its fixture tabs, a tab is authorized, a task starts at its first step
// and stops, and the ledger holds codes only.
func TestDemoHandsAnswersAreDeterministic(t *testing.T) {
	run := func() []any {
		s := handsDemo()
		return []any{
			call(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsStatus}),
			call(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsOpen}),
			call(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsTabs}),
			call(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsAuthorize, Tab: demoHandsTabs[0].ID}),
			call(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsStart, Goal: "fill the contact form"}),
			call(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsStop}),
			call(t, s, protocol.MOniHandsLog, protocol.Empty{}),
		}
	}
	first, second := run(), run()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("the demo answered\n%#v\nthen\n%#v", first, second)
	}
	b := demoHandsBrowser
	want := []protocol.OniHandsResult{
		{State: protocol.OniHandsStateClosed, Browser: b},
		{State: protocol.OniHandsStateOpen, Browser: b},
		{State: protocol.OniHandsStateOpen, Browser: b, Tabs: demoHandsTabs},
		{State: protocol.OniHandsStateAuthorized, Browser: b, Origin: demoHandsTabs[0].Origin},
		{State: protocol.OniHandsStateWorking, Browser: b, Origin: demoHandsTabs[0].Origin, Step: 1, Max: feature.HandsMaxSteps()},
		{State: protocol.OniHandsStateAuthorized, Browser: b, Origin: demoHandsTabs[0].Origin},
	}
	for i, w := range want {
		if got := first[i].(protocol.OniHandsResult); !reflect.DeepEqual(got, w) {
			t.Errorf("answer %d is %+v, want %+v", i, got, w)
		}
	}
	for _, tab := range demoHandsTabs {
		if !protocol.ValidOniHandsTab(tab) {
			t.Errorf("tab %+v cannot go on the wire", tab)
		}
	}
	rows := first[6].(protocol.OniHandsLogResult).Rows
	if len(rows) == 0 {
		t.Fatal("the demo Hands ledger is empty")
	}
	for i, row := range rows {
		if !protocol.ValidOniHandsRow(row) {
			t.Errorf("row %d %+v holds more than codes", i, row)
		}
		if i > 0 && row.At.After(rows[i-1].At) {
			t.Errorf("row %d is newer than the row before it", i)
		}
	}
}

func TestDemoHandsRefusesWhatTheDaemonRefuses(t *testing.T) {
	s := handsDemo()
	refuse := func(params protocol.OniHandsParams, code string) {
		t.Helper()
		if refused := oniCallErr(t, s, protocol.MOniHands, params); refused.Code != code {
			t.Errorf("%+v answered %+v, want %s", params, refused, code)
		}
	}
	refuse(protocol.OniHandsParams{Action: protocol.OniHandsTabs}, protocol.CodeUnavailable)
	refuse(protocol.OniHandsParams{Action: protocol.OniHandsAuthorize, Tab: demoHandsTabs[0].ID}, protocol.CodeUnavailable)
	call(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsOpen})
	refuse(protocol.OniHandsParams{Action: protocol.OniHandsStart, Goal: "fill the form"}, protocol.CodeConflict)
	refuse(protocol.OniHandsParams{Action: protocol.OniHandsAuthorize, Tab: "FFFF0000"}, protocol.CodeNotFound)
	call(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsAuthorize, Tab: demoHandsTabs[1].ID})
	refuse(protocol.OniHandsParams{Action: protocol.OniHandsResume}, protocol.CodeConflict)
	call(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsStart, Goal: "fill the form"})
	refuse(protocol.OniHandsParams{Action: protocol.OniHandsStart, Goal: "fill it again"}, protocol.CodeConflict)
	// the settings refuse a stage that does not exist, as the daemon does,
	// and take the app and desktop stages
	var refused *protocol.Error
	if _, err := s.call(protocol.MSettingsSet, json.RawMessage(`{"patch":{"oni":{"hands":"everywhere"}}}`)); !errors.As(err, &refused) || refused.Code != protocol.CodeBadRequest {
		t.Errorf("hands everywhere: %v", err)
	}
	if s.data.snapshot.Settings.Oni.Hands != protocol.OniHandsBrowser {
		t.Fatalf("a refused patch stored hands %q", s.data.snapshot.Settings.Oni.Hands)
	}
	for _, stage := range []string{protocol.OniHandsApp, protocol.OniHandsDesktop} {
		if _, err := s.call(protocol.MSettingsSet, json.RawMessage(`{"patch":{"oni":{"hands":"`+stage+`"}}}`)); err != nil {
			t.Fatalf("hands %s: %v", stage, err)
		}
		if got := s.data.snapshot.Settings.Oni.Hands; got != stage {
			t.Fatalf("hands %s stored %q", stage, got)
		}
	}
	if _, err := s.call(protocol.MSettingsSet, json.RawMessage(`{"patch":{"oni":{"hands":"off"}}}`)); err != nil {
		t.Fatal(err)
	}
	if refused := oniCallErr(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsStatus}); refused.Reason != feature.ReasonFeatureOff {
		t.Fatalf("hands turned off answered %+v", refused)
	}
}

// Forgetting the Hands browser is refused while it is open and answers
// with where Hands stands and a line saying what went; a machine with no
// Chromium-family browser says so on every answer and cannot open one.
func TestDemoHandsForgetsAndKnowsItsBrowser(t *testing.T) {
	s := handsDemo()
	forgot := call(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsForget}).(protocol.OniHandsResult)
	if forgot.State != protocol.OniHandsStateClosed || forgot.Browser != demoHandsBrowser || forgot.Line == "" {
		t.Fatalf("forget answered %+v", forgot)
	}
	call(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsOpen})
	if refused := oniCallErr(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsForget}); refused.Code != protocol.CodeConflict {
		t.Fatalf("forget while open answered %+v", refused)
	}
	if !protocol.ValidOniHandsBrowser(demoHandsBrowser) {
		t.Fatalf("%q cannot go on the wire", demoHandsBrowser)
	}

	s = &server{data: makeFixtures(FeatureOniHandsNoBrowser)}
	status := call(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsStatus}).(protocol.OniHandsResult)
	if status.State != protocol.OniHandsStateClosed || status.Browser != "" {
		t.Fatalf("a machine with no browser answered %+v", status)
	}
	if refused := oniCallErr(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsOpen}); refused.Code != protocol.CodeUnavailable {
		t.Fatalf("opening with no browser answered %+v", refused)
	}
}

// The paused demo starts on its first tab at step 4, paused by focus;
// Resume goes on from there and a second Resume is refused.
func TestDemoHandsPausedResumes(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOniHandsPaused)}
	if s.data.snapshot.Settings.Oni.Hands != protocol.OniHandsBrowser {
		t.Fatalf("the paused demo starts with hands %q", s.data.snapshot.Settings.Oni.Hands)
	}
	status := call(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsStatus}).(protocol.OniHandsResult)
	if status.State != protocol.OniHandsStatePaused || status.Pause != protocol.OniHandsPauseFocus || status.Step != demoHandsPausedStep || status.Origin != demoHandsTabs[0].Origin {
		t.Fatalf("the paused demo answered %+v", status)
	}
	resumed := call(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsResume}).(protocol.OniHandsResult)
	if resumed.State != protocol.OniHandsStateWorking || resumed.Pause != "" || resumed.Step != demoHandsPausedStep {
		t.Fatalf("resume answered %+v", resumed)
	}
	if refused := oniCallErr(t, s, protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsResume}); refused.Code != protocol.CodeConflict {
		t.Fatalf("a second resume answered %+v", refused)
	}
}
