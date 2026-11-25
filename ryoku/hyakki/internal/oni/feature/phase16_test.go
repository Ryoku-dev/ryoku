package feature

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

// the four Phase 16 rows, denied with reason, as they sit after the guide's
// and before the hands rows
func phase16Rows(reason string) string {
	return `{"id":"memory.forget","stage":"reversible","egress":"local","consent":"per-action","allow":false,"reason":"` + reason + `"},` +
		`{"id":"recall","stage":"read-only","egress":"disclosed","consent":"disclosure","allow":false,"reason":"` + reason + `"},` +
		`{"id":"rewrite","stage":"reversible","egress":"disclosed","consent":"disclosure","allow":false,"reason":"` + reason + `"},` +
		`{"id":"assignment","stage":"read-only","egress":"disclosed","consent":"disclosure","allow":false,"reason":"` + reason + `"},`
}

// withPhase16Rows is capabilities as an earlier phase shipped them with the
// Phase 16 rows in their place; every earlier row keeps its bytes.
func withPhase16Rows(capabilities, reason string) string {
	return strings.Replace(capabilities, `{"id":"hands.browser"`, phase16Rows(reason)+`{"id":"hands.browser"`, 1)
}

// withoutPhase16Rows undoes withPhase16Rows.
func withoutPhase16Rows(capabilities, reason string) string {
	return strings.Replace(capabilities, phase16Rows(reason), "", 1)
}

// The new rows come right after guide.hints and right before the hands
// rows, in this order, and nothing else moved.
func TestPhase16RowsSitAfterTheGuideHints(t *testing.T) {
	var ids []string
	for _, f := range Features() {
		ids = append(ids, f.ID)
	}
	at := slices.Index(ids, GuideHints)
	if at < 0 || !slices.Equal(ids[at+1:at+5], []string{MemoryForget, Recall, Rewrite, Assignment}) || ids[at+5] != HandsBrowser {
		t.Fatalf("contract order %v", ids)
	}
	for _, c := range []struct{ id, setting string }{
		{MemoryForget, "oni.reflexSettings.notes"},
		{Recall, "oni.recall.enabled"},
		{Rewrite, "oni.rewrite.enabled"},
		{Assignment, "oni.assignment.enabled"},
	} {
		if f, _ := Lookup(c.id); f.Setting != c.setting {
			t.Errorf("%s turns on with %q, want %q", c.id, f.Setting, c.setting)
		}
	}
	for _, s := range []protocol.OniSettings{config.Defaults().Oni, protocol.DefaultOniSettings()} {
		if s.Recall.Enabled || s.Recall.Vault || s.Rewrite.Enabled || s.Rewrite.Voice || s.Assignment.Enabled || s.Assignment.Model != "" {
			t.Fatalf("a Phase 16 switch is on by default: %+v %+v %+v", s.Recall, s.Rewrite, s.Assignment)
		}
	}
}

// Every combination of Hyakki, Oni and the switches the new rows read:
// each row is allowed only with Hyakki and Oni on and its own switches, and
// gives the reason every feature gives.
func TestPhase16FeaturesNeedOniAndTheirOwnSwitches(t *testing.T) {
	for bits := range 1 << 9 {
		settings := config.Defaults()
		settings.Enabled = bits&1 != 0
		settings.Oni.Master = bits&2 != 0
		settings.Oni.Inspect = bits&4 != 0
		settings.Oni.Reflex = bits&8 != 0
		settings.Oni.ReflexSettings.Notes = bits&16 != 0
		settings.Oni.Recall = protocol.OniRecallSettings{Enabled: bits&32 != 0, Vault: bits&256 != 0}
		settings.Oni.Rewrite.Enabled = bits&64 != 0
		settings.Oni.Assignment.Enabled = bits&128 != 0
		s := protocol.EffectiveOni(settings)
		on := settings.Enabled && settings.Oni.Master
		want := map[string]bool{
			MemoryForget: settings.Oni.Reflex && settings.Oni.ReflexSettings.Notes,
			Recall:       settings.Oni.Recall.Enabled,
			Rewrite:      settings.Oni.Rewrite.Enabled,
			Assignment:   settings.Oni.Assignment.Enabled && settings.Oni.Inspect,
		}
		for id, own := range want {
			d := Decide(Request{Feature: id}, s)
			reason := ReasonAllowed
			switch {
			case !on:
				reason = ReasonOniOff
			case !own:
				reason = ReasonFeatureOff
			}
			if d.Allow != (on && own) || d.Reason != reason {
				t.Errorf("bits %09b: Decide(%s) = %+v, want reason %q", bits, id, d, reason)
			}
		}
		if Decide(Request{Feature: Notes}, s).Allow != Decide(Request{Feature: MemoryForget}, s).Allow {
			t.Errorf("bits %09b: notes and forgetting by voice do not share one switch", bits)
		}
	}
}

// Forgetting and every guard close ask each time; recall, rewrite and
// assignment ask as the disclosure setting says.
func TestPhase16ConsentAsksAsItsClassSays(t *testing.T) {
	s := everythingOn()
	for _, id := range []string{MemoryForget, FeedGuard} {
		if !MustAsk(id, s, true) {
			t.Errorf("%s acted without asking", id)
		}
	}
	for _, id := range []string{Recall, Rewrite, Assignment} {
		s.Disclosure = protocol.OniDisclosureSession
		if !MustAsk(id, s, false) || MustAsk(id, s, true) {
			t.Errorf("%s does not ask exactly once a session", id)
		}
		s.Disclosure = protocol.OniDisclosureAlways
		if !MustAsk(id, s, true) {
			t.Errorf("%s skipped its card with every-time disclosure", id)
		}
	}
}

func TestPhase16CardsKeepToTheirFeature(t *testing.T) {
	good := []*ConsentFrame{
		ForgetCard("forget-1", "Forget that you live in Lisbon?"),
		GuardCard("guard-1", "Close the Reels window in Instagram?"),
		RecallCard("recall-1", "openrouter/model", "Send 2 passages to openrouter/model?"),
		RewriteCard("rewrite-1", "local", "", "Rewrite 120 characters on this computer?"),
		RewriteCard("rewrite-2", "hosted", "openrouter/model", "Send 120 characters to openrouter/model?"),
		AssignmentCard("assign-1", "hosted", "openrouter/model", "Send this problem to openrouter/model?"),
		AssignmentCard("assign-2", "local", "", "Solve this problem on this computer?"),
	}
	for _, f := range good {
		if _, err := EncodeFrame(f); err != nil {
			t.Errorf("%s: %v", f.ID, err)
		}
		if f.ExpiresMs != ConsentTTL.Milliseconds() {
			t.Errorf("%s lapses after %d ms", f.ID, f.ExpiresMs)
		}
	}
	bad := []*ConsentFrame{
		ForgetCard("guard-1", "Forget it?"),
		ForgetCard("forget-", "Forget it?"),
		ForgetCard("h-1", "Forget it?"),
		GuardCard("forget-1", "Close it?"),
		RecallCard("recall-1", "", "Send it?"),
		RewriteCard("c-1", "local", "", "Rewrite it?"),
		RewriteCard("rewrite-1", "hosted", "", "Rewrite it?"),
		AssignmentCard("rewrite-1", "local", "", "Solve it?"),
		{ID: "recall-1", Feature: Recall, Class: ConsentDisclosure, Destination: "local", Question: "Use it?", ExpiresMs: 1000},
		{ID: "forget-1", Feature: MemoryForget, Class: ConsentPerAction, Destination: "hosted", Route: "openrouter/model", Question: "Forget it?", ExpiresMs: 1000},
		{ID: "guard-1", Feature: FeedGuard, Class: ConsentAnnounced, Destination: "local", Question: "Closing it.", ExpiresMs: 1000},
		{ID: "rewrite-1", Feature: Rewrite, Class: ConsentPerAction, Destination: "local", Question: "Rewrite it?", ExpiresMs: 1000},
		{ID: "forget-1", Feature: MemoryForget, Class: ConsentPerAction, Destination: "local", Question: strings.Repeat("a", 401), ExpiresMs: 1000},
		{ID: "forget-1", Feature: MemoryForget, Class: ConsentPerAction, Destination: "local", Question: "Forget it?", ExpiresMs: ConsentTTL.Milliseconds() + 1},
	}
	for _, f := range bad {
		if out, err := EncodeFrame(f); err == nil {
			t.Errorf("encoded %s", out)
		}
	}
	// a card id names its feature, so no card is taken for another's
	prefixes := []string{ForgetCardPrefix, GuardCardPrefix, RecallCardPrefix, RewriteCardPrefix, AssignmentCardPrefix, HandsCardPrefix, HandsStepCardPrefix}
	for i, a := range prefixes {
		for j, b := range prefixes {
			if i != j && strings.HasPrefix(a, b) && !(a == HandsStepCardPrefix && b == HandsCardPrefix) {
				t.Errorf("card prefix %q starts with %q", a, b)
			}
		}
	}
	// a Phase 16 card is answered yes or no like any other
	if _, err := DecodeCall(protocol.MOniConsentAnswer, json.RawMessage(`{"id":"forget-1","answer":"yes"}`)); err != nil {
		t.Fatal(err)
	}
}

// The focus frame carries a phase, a time and a count: no field could hold
// a feed word, a title or an app.
func TestFocusFramesHaveNoFieldForTitlesOrWords(t *testing.T) {
	typ := reflect.TypeFor[FocusFrame]()
	for i := range typ.NumField() {
		f := typ.Field(i)
		if f.Type.Kind() == reflect.String && f.Name != "Phase" {
			t.Errorf("FocusFrame.%s could carry text", f.Name)
		}
		if f.Type.Kind() == reflect.Slice || f.Type.Kind() == reflect.Map {
			t.Errorf("FocusFrame.%s could carry a list", f.Name)
		}
	}
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	max := sp.Focus.Max().Milliseconds()
	for _, c := range []struct {
		f  FocusFrame
		ok bool
	}{
		{FocusFrame{Phase: FocusOff}, true},
		{FocusFrame{Phase: FocusArmed, LeftMs: 1}, true},
		{FocusFrame{Phase: FocusArmed, LeftMs: max, Closed: 4}, true},
		{FocusFrame{Phase: FocusDone, Closed: 4}, true},
		{FocusFrame{Phase: FocusArmed}, false},
		{FocusFrame{Phase: FocusArmed, LeftMs: max + 1}, false},
		{FocusFrame{Phase: FocusOff, LeftMs: 1}, false},
		{FocusFrame{Phase: FocusOff, Closed: 1}, false},
		{FocusFrame{Phase: FocusDone, LeftMs: 1}, false},
		{FocusFrame{Phase: FocusArmed, LeftMs: 10, Closed: -1}, false},
		{FocusFrame{Phase: FocusArmed, LeftMs: 10, Closed: focusClosedMax + 1}, false},
		{FocusFrame{Phase: ""}, false},
	} {
		if _, err := EncodeFrame(c.f); (err == nil) != c.ok {
			t.Errorf("%+v: encode error %v, want ok %v", c.f, err, c.ok)
		}
	}
}

func TestPhase16CallsDecodeToTheirParams(t *testing.T) {
	v, err := DecodeCall(protocol.MOniRewrite, json.RawMessage(`{"action":"start","style":"custom","instruction":"make it sound like a pirate"}`))
	if p, ok := v.(*protocol.OniRewriteParams); err != nil || !ok || *p != (protocol.OniRewriteParams{Action: protocol.OniRewriteStart, Style: protocol.OniRewriteCustom, Instruction: "make it sound like a pirate"}) {
		t.Fatalf("oni.rewrite decoded as %#v, %v", v, err)
	}
	v, err = DecodeCall(protocol.MOniRecall, json.RawMessage(`{"action":"ask","text":"what did I ask Claude about eigenvalues"}`))
	if p, ok := v.(*protocol.OniRecallParams); err != nil || !ok || p.Action != protocol.OniRecallAsk {
		t.Fatalf("oni.recall decoded as %#v, %v", v, err)
	}
	v, err = DecodeCall(protocol.MOniRecallImport, json.RawMessage(`{"paths":["/home/u/Downloads/export.zip"]}`))
	if p, ok := v.(*protocol.OniRecallImportParams); err != nil || !ok || len(p.Paths) != 1 {
		t.Fatalf("oni.recall.import decoded as %#v, %v", v, err)
	}
	v, err = DecodeCall(protocol.MOniFocus, json.RawMessage(`{"action":"arm","words":["Reels"],"minutes":20}`))
	if p, ok := v.(*protocol.OniFocusParams); err != nil || !ok || p.Minutes != 20 || !slices.Equal(p.Words, []string{"Reels"}) {
		t.Fatalf("oni.focus decoded as %#v, %v", v, err)
	}
	v, err = DecodeCall(protocol.MOniAssignment, json.RawMessage(`{"action":"start","style":"answers"}`))
	if p, ok := v.(*protocol.OniAssignmentParams); err != nil || !ok || *p != (protocol.OniAssignmentParams{Action: protocol.OniAssignmentStart, Style: protocol.OniAssignmentAnswers}) {
		t.Fatalf("oni.assignment decoded as %#v, %v", v, err)
	}
	if _, err := DecodeCall(protocol.MOniAssignmentLog, nil); err != nil {
		t.Fatal(err)
	}
}

func TestPhase16CallParamsDecodeStrictly(t *testing.T) {
	maxMinutes := phase16Spec().Focus.MaxMinutes
	good := [][2]string{
		{"oni.rewrite", `{"action":"start"}`},
		{"oni.rewrite", `{"action":"start","style":"friendly"}`},
		{"oni.rewrite", `{"action":"start","style":"bullets"}`},
		{"oni.rewrite", `{"action":"start","style":"custom","instruction":"shorter and kinder"}`},
		{"oni.rewrite", `{"action":"apply","id":"rewrite-1"}`},
		{"oni.rewrite", `{"action":"undo","id":"rewrite-1"}`},
		{"oni.rewrite", `{"action":"cancel","id":"rewrite-1"}`},
		{"oni.recall", `{"action":"ask","text":"what is in my vault about Limine"}`},
		{"oni.recall", `{"action":"know"}`},
		{"oni.recall", `{"action":"know","text":"my job"}`},
		{"oni.recall.import", `{"paths":["/a/conversations.json","/b/takeout.ZIP"]}`},
		{"oni.focus", `{"action":"status"}`},
		{"oni.focus", `{"action":"stop"}`},
		{"oni.focus", `{"action":"back"}`},
		{"oni.focus", `{"action":"arm","minutes":1}`},
		{"oni.focus", `{"action":"arm","minutes":` + itoa(maxMinutes) + `}`},
		{"oni.focus", `{"action":"arm","untilApp":"Word"}`},
		{"oni.focus", `{"action":"arm","words":["tiktok","reels"],"untilApp":"libreoffice-writer"}`},
		{"oni.assignment", `{"action":"start"}`},
		{"oni.assignment", `{"action":"start","style":"walkthrough"}`},
		{"oni.assignment", `{"action":"next"}`},
		{"oni.assignment", `{"action":"back"}`},
		{"oni.assignment", `{"action":"done"}`},
		{"oni.assignment", `{"action":"stop"}`},
		{"oni.assignment", `{"action":"status"}`},
		{"oni.assignment.log", ``},
		{"oni.assignment.log", `{}`},
	}
	for _, c := range good {
		if _, err := DecodeCall(c[0], json.RawMessage(c[1])); err != nil {
			t.Errorf("%s %s: %v", c[0], c[1], err)
		}
	}
	bad := [][2]string{
		{"oni.rewrite", ``},
		{"oni.rewrite", `{"action":"replace"}`},
		{"oni.rewrite", `{"Action":"start"}`},
		{"oni.rewrite", `{"action":"start","text":"the selection"}`},
		{"oni.rewrite", `{"action":"start","style":"pirate"}`},
		{"oni.rewrite", `{"action":"start","style":"custom"}`},
		{"oni.rewrite", `{"action":"start","style":"custom","instruction":" padded"}`},
		{"oni.rewrite", `{"action":"start","style":"custom","instruction":"two\nlines"}`},
		{"oni.rewrite", `{"action":"start","style":"custom","instruction":"` + strings.Repeat("a", protocol.OniRewriteInstructionMax+1) + `"}`},
		{"oni.rewrite", `{"action":"start","style":"friendly","instruction":"and kind"}`},
		{"oni.rewrite", `{"action":"start","instruction":"kinder"}`},
		{"oni.rewrite", `{"action":"start","id":"rewrite-1"}`},
		{"oni.rewrite", `{"action":"apply"}`},
		{"oni.rewrite", `{"action":"apply","id":"Rewrite 1"}`},
		{"oni.rewrite", `{"action":"undo","id":"rewrite-1","style":"grammar"}`},
		{"oni.rewrite", `{"action":"cancel","id":"` + strings.Repeat("a", protocol.OniCardIDMax+1) + `"}`},
		{"oni.rewrite", `{"action":"apply","id":"rewrite-1","id":"rewrite-2"}`},
		{"oni.recall", `{"action":"ask"}`},
		{"oni.recall", `{"action":"ask","text":"   "}`},
		{"oni.recall", `{"action":"ask","text":"a\u202eb"}`},
		{"oni.recall", `{"action":"ask","text":"` + strings.Repeat("a", protocol.OniRecallTextMax+1) + `"}`},
		{"oni.recall", `{"action":"know","text":"` + strings.Repeat("a", protocol.OniRecallTopicMax+1) + `"}`},
		{"oni.recall", `{"action":"search","text":"x"}`},
		{"oni.recall", `{"action":"ask","text":"x","vault":true}`},
		{"oni.recall.import", `{}`},
		{"oni.recall.import", `{"paths":[]}`},
		{"oni.recall.import", `{"paths":["relative/export.zip"]}`},
		{"oni.recall.import", `{"paths":["/a/../etc/passwd.json"]}`},
		{"oni.recall.import", `{"paths":["/a/export.tar"]}`},
		{"oni.recall.import", `{"paths":["/a/x.json","/a/x.json"]}`},
		{"oni.recall.import", `{"paths":[` + strings.TrimSuffix(strings.Repeat(`"/a/x.json",`, protocol.OniImportPathsMax+1), ",") + `]}`},
		{"oni.recall.import", `{"paths":["/a/x.json"],"watch":true}`},
		{"oni.focus", `{}`},
		{"oni.focus", `{"action":"arm"}`},
		{"oni.focus", `{"action":"arm","minutes":20,"untilApp":"Word"}`},
		{"oni.focus", `{"action":"arm","minutes":` + itoa(maxMinutes+1) + `}`},
		{"oni.focus", `{"action":"arm","minutes":-5}`},
		{"oni.focus", `{"action":"arm","untilApp":" Word"}`},
		{"oni.focus", `{"action":"arm","untilApp":"Word\n"}`},
		{"oni.focus", `{"action":"arm","minutes":20,"words":["reels","Reels"]}`},
		{"oni.focus", `{"action":"arm","minutes":20,"words":[""]}`},
		{"oni.focus", `{"action":"arm","minutes":20,"words":["` + strings.Repeat("a", protocol.OniFeedWordMaxRunes+1) + `"]}`},
		{"oni.focus", `{"action":"status","minutes":20}`},
		{"oni.focus", `{"action":"stop","words":["reels"]}`},
		{"oni.focus", `{"action":"back","untilApp":"Word"}`},
		{"oni.focus", `{"action":"close","minutes":20}`},
		{"oni.assignment", `{}`},
		{"oni.assignment", `{"action":"solve"}`},
		{"oni.assignment", `{"action":"next","style":"answers"}`},
		{"oni.assignment", `{"action":"start","style":"hints"}`},
		{"oni.assignment", `{"action":"start","model":"openrouter/model"}`},
		{"oni.assignment.log", `{"day":"2026-11-23"}`},
		{"oni.assignment.log", `null`},
	}
	for _, c := range bad {
		if v, err := DecodeCall(c[0], json.RawMessage(c[1])); err == nil {
			t.Errorf("%s %s decoded as %#v", c[0], c[1], v)
		}
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
