package feature

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

// the Hands rows as Phases 10 to 13 shipped them, unavailable
func unavailableHandsRow(id string) string {
	return `{"id":"` + id + `","stage":"unavailable","egress":"disclosed","consent":"firm","allow":false,"reason":"unavailable"}`
}

// the Hands rows now, guarded and denied with reason
func guardedHandsRow(id, reason string) string {
	return `{"id":"` + id + `","stage":"guarded","egress":"disclosed","consent":"firm","allow":false,"reason":"` + reason + `"}`
}

// withHandsRow is capabilities as an earlier phase shipped them with the one
// change the Hands stages make: each row is guarded and gives the reason
// the order names.
func withHandsRow(capabilities, reason string) string {
	for _, id := range handsIDs {
		capabilities = strings.Replace(capabilities, unavailableHandsRow(id), guardedHandsRow(id, reason), 1)
	}
	return capabilities
}

// withoutHandsRow undoes withHandsRow.
func withoutHandsRow(capabilities, reason string) string {
	for _, id := range handsIDs {
		capabilities = strings.Replace(capabilities, guardedHandsRow(id, reason), unavailableHandsRow(id), 1)
	}
	return capabilities
}

func TestHandsNumbersAreTheSpecs(t *testing.T) {
	if HandoffTTL() != 120*time.Second || HandoffTTL() != ConsentTTL {
		t.Errorf("a handoff card waits %v, the spec and every consent card say 120s", HandoffTTL())
	}
	if HandsMaxSteps() != 25 {
		t.Errorf("a task takes %d steps, the spec says 15 and 10 more", HandsMaxSteps())
	}
}

// oni.hands decodes strictly: each action carries exactly what it needs, a
// start a one-line goal (with a scope and a journal task at most), an
// authorize one tab id, a select one window id, and nothing else.
func TestHandsCallsDecodeStrictly(t *testing.T) {
	task := strings.Repeat("ab", protocol.OniHandsTaskIDBytes/2)
	good := map[string]protocol.OniHandsParams{
		`{"action":"open"}`:                             {Action: protocol.OniHandsOpen},
		`{"action":"tabs"}`:                             {Action: protocol.OniHandsTabs},
		`{"action":"authorize","tab":"9F3A01C2"}`:       {Action: protocol.OniHandsAuthorize, Tab: "9F3A01C2"},
		`{"action":"start","goal":"fill the form"}`:     {Action: protocol.OniHandsStart, Goal: "fill the form"},
		`{"action":"stop"}`:                             {Action: protocol.OniHandsStop},
		`{"action":"resume"}`:                           {Action: protocol.OniHandsResume},
		`{"action":"status"}`:                           {Action: protocol.OniHandsStatus},
		`{"action":"forget"}`:                           {Action: protocol.OniHandsForget},
		`{"action":"windows"}`:                          {Action: protocol.OniHandsWindows},
		`{"action":"select","window":"0x5a1e2b3c9e20"}`: {Action: protocol.OniHandsSelect, Window: "0x5a1e2b3c9e20"},
		`{"action":"tasks"}`:                            {Action: protocol.OniHandsTasks},
		`{"action":"clear"}`:                            {Action: protocol.OniHandsClear},
		` {"action":"start","goal":"` + strings.Repeat("é", protocol.OniHandsGoalMax) + `"} `: {Action: protocol.OniHandsStart, Goal: strings.Repeat("é", protocol.OniHandsGoalMax)},
		`{"action":"start","goal":"type hello","scope":"nested","task":"` + task + `"}`:       {Action: protocol.OniHandsStart, Goal: "type hello", Scope: protocol.OniHandsScopeNested, Task: task},
	}
	for raw, want := range good {
		v, err := DecodeCall(protocol.MOniHands, json.RawMessage(raw))
		if err != nil {
			t.Errorf("%s: %v", raw, err)
			continue
		}
		if p, ok := v.(*protocol.OniHandsParams); !ok || *p != want {
			t.Errorf("%s decoded as %#v", raw, v)
		}
	}
	if len(good) != len(protocol.OniHandsActions)+2 {
		t.Fatalf("the test covers %d calls for %d actions", len(good), len(protocol.OniHandsActions))
	}
	for _, raw := range []string{
		``, `{}`, `null`, `[]`, `{"action":null}`,
		`{"action":"STOP"}`, `{"action":"yes"}`, `{"action":"no"}`, `{"action":"press"}`, `{"action":"Forget"}`,
		`{"action":"stop","goal":"x"}`, `{"action":"status","tab":"1"}`, `{"action":"open","tab":"1","goal":"x"}`,
		`{"action":"forget","tab":"9F3A01C2"}`, `{"action":"forget","goal":"x"}`,
		`{"action":"authorize"}`, `{"action":"authorize","tab":""}`, `{"action":"authorize","tab":"a-b"}`,
		`{"action":"authorize","tab":"` + strings.Repeat("a", protocol.OniHandsTabIDMax+1) + `"}`,
		`{"action":"authorize","tab":"1","goal":"x"}`,
		`{"action":"start"}`, `{"action":"start","goal":"  "}`, `{"action":"start","goal":"fill\nsend"}`,
		`{"action":"start","goal":"fill\u202ethis"}`, `{"action":"start","goal":"x","tab":"1"}`,
		`{"action":"start","goal":"` + strings.Repeat("a", protocol.OniHandsGoalMax+1) + `"}`,
		`{"action":"stop","by":"voice"}`, `{"action":"stop","action":"start","goal":"x"}`, `{"Action":"stop"}`,
		`{"action":"select"}`, `{"action":"select","window":"a b"}`, `{"action":"select","window":"1","goal":"x"}`,
		`{"action":"windows","window":"1"}`, `{"action":"stop","window":"1"}`, `{"action":"tasks","task":"` + task + `"}`,
		`{"action":"clear","scope":"window"}`, `{"action":"start","goal":"x","scope":"everywhere"}`,
		`{"action":"start","goal":"x","scope":"Window"}`, `{"action":"start","goal":"x","task":"1"}`,
		`{"action":"start","goal":"x","task":"` + strings.ToUpper(task) + `"}`, `{"action":"start","task":"` + task + `"}`,
		`{"action":"start","goal":"x","window":"1"}`, `{"action":"resume","task":"` + task + `"}`,
		`{"action":"stop"} {}`,
	} {
		if _, err := DecodeCall(protocol.MOniHands, json.RawMessage(raw)); err == nil {
			t.Errorf("oni.hands %s decoded", raw)
		}
	}
	for _, raw := range []string{``, `{}`} {
		if _, err := DecodeCall(protocol.MOniHandsLog, json.RawMessage(raw)); err != nil {
			t.Errorf("oni.hands.log %q: %v", raw, err)
		}
	}
	for _, raw := range []string{`{"words":true}`, `null`, `{"task":"1"}`} {
		if _, err := DecodeCall(protocol.MOniHandsLog, json.RawMessage(raw)); err == nil {
			t.Errorf("oni.hands.log %s decoded", raw)
		}
	}
	if slices.Contains(ReservedCalls(), protocol.MOniHands) || slices.Contains(ReservedCalls(), protocol.MOniHandsLog) || len(ReservedCalls()) != 0 {
		t.Fatalf("reserved calls are %q", ReservedCalls())
	}
}

// A handoff card has no yes: it is local, starts h-, waits the spec's time
// at most, and no oni.consent.answer, yes or no, reaches it.
func TestHandoffCardsHaveNoYes(t *testing.T) {
	card := HandoffCard(HandsBrowser, "h-3", "Press Send yourself when you are ready.", HandoffTTL())
	out, err := EncodeFrame(card)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"kind":"consent","data":{"id":"h-3","feature":"hands.browser","class":"firm","destination":"local","question":"Press Send yourself when you are ready.","expiresMs":120000}}`; string(out) != want {
		t.Fatalf("the card is %s", out)
	}
	// the app and desktop stages hand off the same way, under their own row
	for _, id := range []string{HandsApp, HandsDesktop} {
		if _, err := EncodeFrame(HandoffCard(id, "h-4", "Press Enter yourself.", time.Second)); err != nil {
			t.Errorf("%s handoff card: %v", id, err)
		}
	}
	for _, bad := range []*ConsentFrame{
		HandoffCard(HandsBrowser, "h-3", "Press Send.", HandoffTTL()+time.Millisecond),
		HandoffCard(HandsBrowser, "h-3", "Press Send.", 0),
		HandoffCard(HandsBrowser, "c-3", "Press Send.", time.Second),
		HandoffCard(HandsBrowser, "h-", "Press Send.", time.Second),
		HandoffCard(HandsApp, "h-3", "Press Send.", HandoffTTL()+time.Millisecond),
		HandoffCard(HandsDesktop, "c-3", "Press Send.", time.Second),
		{ID: "h-3", Feature: HandsBrowser, Class: ConsentFirm, Destination: "local", Route: "openrouter/x", Question: "Press Send.", ExpiresMs: 1000},
		{ID: "h-3", Feature: HandsBrowser, Class: ConsentPerAction, Destination: "local", Question: "Press Send.", ExpiresMs: 1000},
		{ID: "h-3", Feature: HandsApp, Class: ConsentFirm, Destination: "hosted", Route: "openrouter/x", Question: "Press Send.", ExpiresMs: 1000},
		{ID: "h-3", Feature: HandsDesktop, Class: ConsentPerAction, Destination: "local", Question: "Press Send.", ExpiresMs: 1000},
	} {
		if out, err := EncodeFrame(bad); err == nil {
			t.Errorf("encoded %s", out)
		}
	}
	for _, answer := range []string{"yes", "no"} {
		raw := json.RawMessage(`{"id":"h-3","answer":"` + answer + `"}`)
		if _, err := DecodeCall(protocol.MOniConsentAnswer, raw); err == nil {
			t.Errorf("oni.consent.answer %s decoded for a handoff card", raw)
		}
	}
	if _, err := DecodeCall(protocol.MOniConsentAnswer, json.RawMessage(`{"id":"c-3","answer":"yes"}`)); err != nil {
		t.Fatalf("a picture's card no longer takes a yes: %v", err)
	}
	// the frame that takes a card down still decodes
	if _, err := DecodeFrame([]byte(`{"kind":"consent.answer","data":{"id":"h-3","answer":"no"}}`)); err != nil {
		t.Fatal(err)
	}
}

func TestHandsFramesFollowTheTask(t *testing.T) {
	good := []HandsFrame{
		{Stage: protocol.OniHandsOff, Reason: ReasonOniOff},
		{Stage: protocol.OniHandsOff, Reason: ReasonFeatureOff},
		{Stage: protocol.OniHandsBrowser, Reason: ReasonFeatureOff},
		{Stage: protocol.OniHandsBrowser, Reason: ReasonAllowed},
		{Stage: protocol.OniHandsBrowser, Active: true, Reason: ReasonAllowed, Scope: protocol.OniHandsScopeBrowser, Phase: protocol.OniHandsStateWorking, Max: 1},
		{Stage: protocol.OniHandsBrowser, Active: true, Reason: ReasonAllowed, Scope: protocol.OniHandsScopeBrowser, Phase: protocol.OniHandsStateWaiting, Step: 25, Max: 25},
	}
	for _, pause := range protocol.OniHandsBrowserPauses {
		good = append(good, HandsFrame{Stage: protocol.OniHandsBrowser, Active: true, Reason: ReasonAllowed, Scope: protocol.OniHandsScopeBrowser, Phase: protocol.OniHandsStatePaused, Step: 1, Max: 15, Pause: pause})
	}
	for _, end := range protocol.OniHandsEnds {
		good = append(good, HandsFrame{Stage: protocol.OniHandsBrowser, Reason: ReasonAllowed, End: end})
	}
	// turning Hands off ends a running task, at stage off
	good = append(good, HandsFrame{Stage: protocol.OniHandsOff, Reason: ReasonFeatureOff, End: protocol.OniHandsEndStopped})
	for _, f := range good {
		if _, err := EncodeFrame(&f); err != nil {
			t.Errorf("%+v: %v", f, err)
		}
	}
	// the app stage works in a picked window, naming its app and output,
	// or on the nested desktop; the desktop stage only on the nested one
	for _, f := range []HandsFrame{
		{Stage: protocol.OniHandsApp, Reason: ReasonFeatureOff},
		{Stage: protocol.OniHandsApp, Reason: ReasonAllowed, End: protocol.OniHandsEndUnread},
		{Stage: protocol.OniHandsDesktop, Reason: ReasonAllowed},
		{Stage: protocol.OniHandsApp, Active: true, Reason: ReasonAllowed, Scope: protocol.OniHandsScopeWindow, Phase: protocol.OniHandsStateWorking, Max: 15, App: "org.gnome.TextEditor", Output: "eDP-1"},
		{Stage: protocol.OniHandsApp, Active: true, Reason: ReasonAllowed, Scope: protocol.OniHandsScopeNested, Phase: protocol.OniHandsStateWorking, Max: 15},
		{Stage: protocol.OniHandsDesktop, Active: true, Reason: ReasonAllowed, Scope: protocol.OniHandsScopeDesktop, Phase: protocol.OniHandsStateWaiting, Step: 2, Max: 15},
	} {
		if _, err := EncodeFrame(&f); err != nil {
			t.Errorf("%+v: %v", f, err)
		}
	}
	for _, pause := range protocol.OniHandsAppPauses {
		for _, f := range []HandsFrame{
			{Stage: protocol.OniHandsApp, Active: true, Reason: ReasonAllowed, Scope: protocol.OniHandsScopeWindow, Phase: protocol.OniHandsStatePaused, Step: 1, Max: 15, Pause: pause},
			{Stage: protocol.OniHandsDesktop, Active: true, Reason: ReasonAllowed, Scope: protocol.OniHandsScopeDesktop, Phase: protocol.OniHandsStatePaused, Step: 1, Max: 15, Pause: pause},
		} {
			if _, err := EncodeFrame(&f); err != nil {
				t.Errorf("%+v: %v", f, err)
			}
		}
	}
	active := HandsFrame{Stage: protocol.OniHandsBrowser, Active: true, Reason: ReasonAllowed, Scope: protocol.OniHandsScopeBrowser, Phase: protocol.OniHandsStatePaused, Step: 1, Max: 15}
	var bad []HandsFrame
	for _, pause := range protocol.OniHandsPauses {
		if !slices.Contains(protocol.OniHandsBrowserPauses, pause) {
			f := active
			f.Pause = pause
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		t.Fatal("no later scope's pause was tried")
	}
	// a tab's own pauses never belong to a window or a desktop
	for _, pause := range []string{protocol.OniHandsPauseNavigation, protocol.OniHandsPauseDialog, protocol.OniHandsPausePopup, protocol.OniHandsPauseDownload} {
		bad = append(bad, HandsFrame{Stage: protocol.OniHandsApp, Active: true, Reason: ReasonAllowed, Scope: protocol.OniHandsScopeWindow, Phase: protocol.OniHandsStatePaused, Step: 1, Max: 15, Pause: pause})
	}
	// no stage reaches another's scope: no ladder, and the desktop scope is
	// the nested desktop's alone
	for stage, scopes := range map[string][]string{
		protocol.OniHandsBrowser: {protocol.OniHandsScopeWindow, protocol.OniHandsScopeNested, protocol.OniHandsScopeDesktop},
		protocol.OniHandsApp:     {protocol.OniHandsScopeBrowser, protocol.OniHandsScopeDesktop},
		protocol.OniHandsDesktop: {protocol.OniHandsScopeBrowser, protocol.OniHandsScopeWindow, protocol.OniHandsScopeNested},
	} {
		for _, scope := range scopes {
			bad = append(bad, HandsFrame{Stage: stage, Active: true, Reason: ReasonAllowed, Scope: scope, Phase: protocol.OniHandsStateWorking, Max: 15})
		}
	}
	bad = append(bad,
		// the app and output come only with a picked window, as codes
		HandsFrame{Stage: protocol.OniHandsApp, Active: true, Reason: ReasonAllowed, Scope: protocol.OniHandsScopeNested, Phase: protocol.OniHandsStateWorking, Max: 15, App: "foot"},
		HandsFrame{Stage: protocol.OniHandsApp, Active: true, Reason: ReasonAllowed, Scope: protocol.OniHandsScopeWindow, Phase: protocol.OniHandsStateWorking, Max: 15, App: "notes.txt — Text Editor"},
		HandsFrame{Stage: protocol.OniHandsApp, Reason: ReasonAllowed, App: "foot"},
		HandsFrame{Stage: protocol.OniHandsDesktop, Active: true, Reason: ReasonAllowed, Scope: protocol.OniHandsScopeDesktop, Phase: protocol.OniHandsStateWorking, Max: 15, Output: "eDP-1"},
		HandsFrame{Stage: "everything", Reason: ReasonFeatureOff},
	)
	bad = append(bad,
		HandsFrame{Stage: protocol.OniHandsOff, Reason: ReasonUnavailable},
		HandsFrame{Stage: protocol.OniHandsBrowser, Reason: ReasonUnavailable},
		HandsFrame{Stage: protocol.OniHandsBrowser, Reason: ReasonAllowed, Max: 15},
		HandsFrame{Stage: protocol.OniHandsOff, Active: true, Reason: ReasonAllowed, Scope: protocol.OniHandsScopeBrowser, Phase: protocol.OniHandsStateWorking, Max: 15},
		HandsFrame{Stage: protocol.OniHandsBrowser, Active: true, Reason: ReasonAllowed, Scope: protocol.OniHandsScopeBrowser, Phase: protocol.OniHandsStateAuthorized, Max: 15},
		HandsFrame{Stage: protocol.OniHandsBrowser, Active: true, Reason: ReasonAllowed, Scope: protocol.OniHandsScopeBrowser, Phase: protocol.OniHandsStateWorking, Max: HandsMaxSteps() + 1},
		HandsFrame{Stage: protocol.OniHandsBrowser, Active: true, Reason: ReasonAllowed, Scope: protocol.OniHandsScopeBrowser, Phase: protocol.OniHandsStateWorking, Max: 15, End: protocol.OniHandsEndDone},
		HandsFrame{Stage: protocol.OniHandsBrowser, Reason: ReasonAllowed, End: "finished"},
		HandsFrame{Stage: protocol.OniHandsBrowser, Reason: ReasonAllowed, End: "Done"},
	)
	for _, raw := range []string{
		`{"kind":"hands","data":{"stage":"browser","active":false,"reason":"allowed","end":"sent the form"}}`,
		`{"kind":"hands","data":{"stage":"browser","active":false,"reason":"allowed","end":7}}`,
	} {
		if _, err := DecodeFrame([]byte(raw)); err == nil {
			t.Errorf("decoded %s", raw)
		}
	}
	if p, err := DecodeFrame([]byte(`{"kind":"hands","data":{"stage":"browser","active":false,"reason":"allowed","end":"done"}}`)); err != nil || p.(*HandsFrame).End != protocol.OniHandsEndDone {
		t.Errorf("a finished task's frame decoded to %+v, %v", p, err)
	}
	for _, f := range bad {
		if out, err := EncodeFrame(&f); err == nil {
			t.Errorf("encoded %s", out)
		}
	}
}
