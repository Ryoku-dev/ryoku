package feature

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

var reflexIDs = []string{Routines, Reminders, Notes, Decisions}

// the four reflex rows, denied with reason, as they sit before the guide
// rows
func reflexRows(reason string) string {
	return `{"id":"reflex.routines","stage":"reversible","egress":"local","consent":"per-action","allow":false,"reason":"` + reason + `"},` +
		`{"id":"reflex.reminders","stage":"reversible","egress":"local","consent":"announced","allow":false,"reason":"` + reason + `"},` +
		`{"id":"reflex.notes","stage":"reversible","egress":"local","consent":"announced","allow":false,"reason":"` + reason + `"},` +
		`{"id":"reflex.decisions","stage":"reversible","egress":"disclosed","consent":"disclosure","allow":false,"reason":"` + reason + `"},`
}

func withReflexRows(capabilities, reason string) string {
	return strings.Replace(capabilities, `{"id":"guide"`, reflexRows(reason)+`{"id":"guide"`, 1)
}

// With the reflex switches off the capabilities are Phase 12's byte for
// byte but for the four reflex rows, denied, after feeds.guard, and the
// hands.browser row Phase 14 makes guarded; the reflex row itself is
// unchanged.
func TestCapabilitiesWithReflexOffArePhase12sPlusTheDeniedReflexRows(t *testing.T) {
	inspectOn := config.Defaults()
	inspectOn.Enabled, inspectOn.Oni.Master, inspectOn.Oni.Inspect = true, true, true
	reflexOn := config.Defaults()
	reflexOn.Enabled, reflexOn.Oni.Master, reflexOn.Oni.Reflex = true, true, true
	// Phase 12 with Reflex on and every other switch off: the inspect-on
	// rows with inspect off and reflex allowed
	const (
		inspectRows  = `{"id":"vision.inspect","stage":"read-only","egress":"disclosed","consent":"disclosure","allow":true,"reason":"allowed"},{"id":"voice","stage":"read-only","egress":"disclosed","consent":"user-initiated","allow":false,"reason":"feature-off"},{"id":"voice.dictation","stage":"reversible","egress":"disclosed","consent":"user-initiated","allow":false,"reason":"feature-off"},{"id":"reflex","stage":"reversible","egress":"disclosed","consent":"announced","allow":false,"reason":"feature-off"}`
		reflexRowsOn = `{"id":"vision.inspect","stage":"read-only","egress":"disclosed","consent":"disclosure","allow":false,"reason":"feature-off"},{"id":"voice","stage":"read-only","egress":"disclosed","consent":"user-initiated","allow":false,"reason":"feature-off"},{"id":"voice.dictation","stage":"reversible","egress":"disclosed","consent":"user-initiated","allow":false,"reason":"feature-off"},{"id":"reflex","stage":"reversible","egress":"disclosed","consent":"announced","allow":true,"reason":"allowed"}`
	)
	inspectPhase12 := withGuideRows(phase11InspectOnCapabilities, ReasonFeatureOff)
	if !strings.Contains(inspectPhase12, inspectRows) {
		t.Fatal("the Phase 11 inspect-on fixture does not open with the rows replaced")
	}
	for _, c := range []struct {
		settings protocol.Settings
		phase12  string
		reason   string
	}{
		{config.Defaults(), withGuideRows(phase11DefaultCapabilities, ReasonOniOff), ReasonOniOff},
		{inspectOn, inspectPhase12, ReasonFeatureOff},
		{reflexOn, strings.Replace(inspectPhase12, inspectRows, reflexRowsOn, 1), ReasonFeatureOff},
	} {
		got, err := json.Marshal(Capabilities(protocol.EffectiveOni(c.settings)))
		if err != nil {
			t.Fatal(err)
		}
		if want := withPhase16Rows(withHandsRow(withReflexRows(c.phase12, c.reason), c.reason), c.reason); string(got) != want {
			t.Fatalf("capabilities\n%s\nwant\n%s", got, want)
		}
		if old := withoutHandsRow(strings.Replace(withoutPhase16Rows(string(got), c.reason), reflexRows(c.reason), "", 1), c.reason); old != c.phase12 {
			t.Fatalf("without the reflex rows\n%s\nwant\n%s", old, c.phase12)
		}
	}
	reflex, _ := Lookup(Reflex)
	if want := (Feature{ID: Reflex, Stage: StageReversible, Setting: "oni.reflex", Egress: EgressDisclosed, Consent: ConsentAnnounced}); reflex != want {
		t.Fatalf("the reflex row changed: %+v", reflex)
	}
}

// Each reflex feature needs Hyakki, Oni's switch, Reflex and its own switch;
// no other switch counts, and each own switch turns on only its feature.
// The reasons are the ones every feature gives.
func TestReflexFeaturesNeedOniReflexAndTheirOwnSwitch(t *testing.T) {
	for bits := range 1 << 7 {
		settings := config.Defaults()
		settings.Enabled = bits&1 != 0
		settings.Oni.Master = bits&2 != 0
		settings.Oni.Reflex = bits&4 != 0
		settings.Oni.ReflexSettings = protocol.OniReflexSettings{Routines: bits&8 != 0, Reminders: bits&16 != 0, Notes: bits&32 != 0, Decisions: bits&64 != 0}
		// switches the reflex family does not build on change nothing
		settings.Oni.Inspect, settings.Oni.Voice, settings.Oni.Guide.Enabled = true, true, true
		s := protocol.EffectiveOni(settings)
		on := settings.Enabled && settings.Oni.Master
		own := map[string]bool{
			Routines:  settings.Oni.ReflexSettings.Routines,
			Reminders: settings.Oni.ReflexSettings.Reminders,
			Notes:     settings.Oni.ReflexSettings.Notes,
			Decisions: settings.Oni.ReflexSettings.Decisions,
		}
		for id, mine := range own {
			allowed := settings.Oni.Reflex && mine
			d := Decide(Request{Feature: id}, s)
			reason := ReasonAllowed
			switch {
			case !on:
				reason = ReasonOniOff
			case !allowed:
				reason = ReasonFeatureOff
			}
			if d.Allow != (on && allowed) || d.Reason != reason {
				t.Errorf("bits %07b: Decide(%s) = %+v, want reason %q", bits, id, d, reason)
			}
		}
		if Decide(Request{Feature: Reflex}, s).Allow != (on && settings.Oni.Reflex) {
			t.Errorf("bits %07b: a reflex switch changed the reflex row", bits)
		}
	}
	// a reflex switch alone, with Reflex off, allows nothing
	s := protocol.DefaultOniSettings()
	s.Master = true
	s.ReflexSettings = protocol.OniReflexSettings{Routines: true, Reminders: true, Notes: true, Decisions: true}
	for _, c := range Capabilities(s).Features {
		if c.Allow {
			t.Errorf("%s allowed with Reflex off", c.ID)
		}
	}
}

func TestReflexFamilyConsentAsksAsItsClassSays(t *testing.T) {
	s := everythingOn()
	for _, disclosure := range []string{protocol.OniDisclosureSession, protocol.OniDisclosureAlways} {
		s.Disclosure = disclosure
		if !MustAsk(Routines, s, true) {
			t.Errorf("%s: keeping a routine did not ask each time", disclosure)
		}
		if MustAsk(Reminders, s, false) || MustAsk(Notes, s, false) {
			t.Errorf("%s: a reminder or a note asks instead of announcing itself", disclosure)
		}
		if !MustAsk(Decisions, s, false) {
			t.Errorf("%s: a hosted decision went without its disclosure card", disclosure)
		}
	}
	s.Disclosure = protocol.OniDisclosureAlways
	if !MustAsk(Decisions, s, true) {
		t.Error("a hosted decision skipped its card with disclosure always")
	}
	for _, id := range reflexIDs {
		f, ok := Lookup(id)
		if !ok || f.Stage != StageReversible || f.Setting != "oni.reflexSettings."+strings.TrimPrefix(id, "reflex.") {
			t.Errorf("%s is %+v", id, f)
		}
		if f.Egress != EgressLocal && id != Decisions {
			t.Errorf("%s may leave the computer", id)
		}
	}
}

// A routine offer is per-action, stays on this computer and lapses with the
// spec's reflex.routines.chipS; oni.keep or oni.skip answers it.
func TestRoutineOfferIsLocalAndLapsesWithTheChip(t *testing.T) {
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Duration(sp.Reflex.Routines.ChipS * float64(time.Second)); RoutineOfferTTL() != want || want <= 0 || want > ConsentTTL {
		t.Fatalf("routine offer TTL %v, spec %v, consent TTL %v", RoutineOfferTTL(), want, ConsentTTL)
	}
	offer := RoutineOffer("r-1", "Keep these three as a routine?")
	out, err := EncodeFrame(offer)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"kind":"consent","data":{"id":"r-1","feature":"reflex.routines","class":"per-action","destination":"local","question":"Keep these three as a routine?","expiresMs":12000}}`; string(out) != want {
		t.Fatalf("routine offer %s\nwant %s", out, want)
	}
	if p, err := DecodeFrame(out); err != nil || !reflect.DeepEqual(p, offer) {
		t.Fatalf("routine offer decoded as %#v, %v", p, err)
	}
	for name, edit := range map[string]func(*ConsentFrame){
		"hosted":        func(f *ConsentFrame) { f.Destination, f.Route = "hosted", "openrouter/x" },
		"local route":   func(f *ConsentFrame) { f.Route = "rashin-models/gemma" },
		"past the chip": func(f *ConsentFrame) { f.ExpiresMs = RoutineOfferTTL().Milliseconds() + 1 },
		"announced":     func(f *ConsentFrame) { f.Class = ConsentAnnounced },
		"disclosure":    func(f *ConsentFrame) { f.Class = ConsentDisclosure },
		"no question":   func(f *ConsentFrame) { f.Question = "" },
	} {
		f := RoutineOffer("r-1", "Keep these three as a routine?")
		edit(f)
		if out, err := EncodeFrame(f); err == nil {
			t.Errorf("%s: encoded %s", name, out)
		}
	}
}

// The hosted decision card discloses where the decision goes and names its
// route; a local decision needs none, and the card takes no other class.
func TestDecisionCardDisclosesAHostedRoute(t *testing.T) {
	card := DecisionCard("c-7", "ember-decisions", "Send this sentence to Ryoku Ember as the signed-in account?")
	out, err := EncodeFrame(card)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"kind":"consent","data":{"id":"c-7","feature":"reflex.decisions","class":"disclosure","destination":"hosted","route":"ember-decisions","question":"Send this sentence to Ryoku Ember as the signed-in account?","expiresMs":120000}}`; string(out) != want {
		t.Fatalf("decision card %s\nwant %s", out, want)
	}
	for name, edit := range map[string]func(*ConsentFrame){
		"local":          func(f *ConsentFrame) { f.Destination, f.Route = "local", "" },
		"no route":       func(f *ConsentFrame) { f.Route = "" },
		"per-action":     func(f *ConsentFrame) { f.Class = ConsentPerAction },
		"announced":      func(f *ConsentFrame) { f.Class = ConsentAnnounced },
		"user-initiated": func(f *ConsentFrame) { f.Class = ConsentUserInitiated },
		"too long":       func(f *ConsentFrame) { f.ExpiresMs = ConsentTTL.Milliseconds() + 1 },
	} {
		f := DecisionCard("c-7", "ember-decisions", "Send it?")
		edit(f)
		if out, err := EncodeFrame(f); err == nil {
			t.Errorf("%s: encoded %s", name, out)
		}
	}
}

// Reflex, reminders and notes announce what they do; no consent frame asks
// for them, whatever its class.
func TestAnnouncedReflexesNeverAsk(t *testing.T) {
	for _, id := range []string{Reflex, Reminders, Notes} {
		for _, class := range []string{ConsentAnnounced, ConsentPerAction, ConsentUserInitiated, ConsentDisclosure, ConsentFirm} {
			f := &ConsentFrame{ID: "c-1", Feature: id, Class: class, Destination: "local", Question: "Set it?", ExpiresMs: 1000}
			if out, err := EncodeFrame(f); err == nil {
				t.Errorf("%s %s: encoded %s", id, class, out)
			}
		}
	}
	for _, raw := range []string{
		`{"kind":"consent","data":{"id":"r-1","feature":"reflex.reminders","class":"announced","destination":"local","question":"Remind you?","expiresMs":1000}}`,
		`{"kind":"consent","data":{"id":"r-1","feature":"reflex.notes","class":"announced","destination":"local","question":"Keep it?","expiresMs":1000}}`,
		`{"kind":"consent","data":{"id":"r-1","feature":"reflex.routines","class":"announced","destination":"local","question":"Keep it?","expiresMs":1000}}`,
		`{"kind":"consent","data":{"id":"r-1","feature":"reflex.decisions","class":"per-action","destination":"hosted","route":"ember-decisions","question":"Send?","expiresMs":1000}}`,
	} {
		if p, err := DecodeFrame([]byte(raw)); err == nil {
			t.Errorf("%s decoded as %#v", raw, p)
		}
	}
}

func TestReflexCallsDecodeToTheirParams(t *testing.T) {
	v, err := DecodeCall(protocol.MOniReminders, json.RawMessage(`{"action":"cancel","id":"0123456789abcdef"}`))
	if p, ok := v.(*protocol.OniRemindersParams); err != nil || !ok || *p != (protocol.OniRemindersParams{Action: protocol.OniRemindersCancel, ID: "0123456789abcdef"}) {
		t.Fatalf("oni.reminders decoded as %#v, %v", v, err)
	}
	v, err = DecodeCall(protocol.MOniRoutines, json.RawMessage(`{"action":"run","name":"Morning"}`))
	if p, ok := v.(*protocol.OniRoutinesParams); err != nil || !ok || *p != (protocol.OniRoutinesParams{Action: protocol.OniRoutinesRun, Name: "Morning"}) {
		t.Fatalf("oni.routines decoded as %#v, %v", v, err)
	}
	if _, err := DecodeCall(protocol.MOniReflexLog, nil); err != nil {
		t.Fatalf("oni.reflex.log: %v", err)
	}
	// the actions are the protocol's lists, each decoding
	for _, action := range protocol.OniRemindersActions {
		extra := ""
		switch action {
		case protocol.OniRemindersCancel:
			extra = `,"id":"ffffffffffffffff"`
		case protocol.OniRemindersAdd:
			extra = `,"kind":"reminder","due":"2026-11-23T18:00:00Z","text":"Call the dentist"`
		}
		if _, err := DecodeCall(protocol.MOniReminders, json.RawMessage(`{"action":"`+action+`"`+extra+`}`)); err != nil {
			t.Errorf("oni.reminders %s: %v", action, err)
		}
	}
	// an add carries its kind, due time and words, and nothing else does
	v, err = DecodeCall(protocol.MOniReminders, json.RawMessage(`{"action":"add","kind":"timer","due":"2026-11-23T18:00:00Z"}`))
	if p, ok := v.(*protocol.OniRemindersParams); err != nil || !ok || p.Kind != protocol.OniReminderKindTimer || p.Due.IsZero() || p.Text != "" {
		t.Fatalf("a timer add decoded as %#v, %v", v, err)
	}
	for _, bad := range []string{
		`{"action":"add","kind":"reminder","due":"2026-11-23T18:00:00Z"}`,
		`{"action":"add","kind":"alarm","due":"2026-11-23T18:00:00Z","text":"x"}`,
		`{"action":"add","kind":"reminder","text":"Call the dentist"}`,
		`{"action":"add","kind":"reminder","due":"2026-11-23T18:00:00Z","text":"two\nlines"}`,
		`{"action":"add","kind":"reminder","due":"2026-11-23T18:00:00Z","text":"` + strings.Repeat("a", protocol.OniReminderTextMax+1) + `"}`,
		`{"action":"add","id":"ffffffffffffffff","kind":"timer","due":"2026-11-23T18:00:00Z"}`,
		`{"action":"list","text":"Call the dentist"}`,
		`{"action":"cancel","id":"ffffffffffffffff","kind":"timer"}`,
		`{"action":"cancel_all","due":"2026-11-23T18:00:00Z"}`,
	} {
		if _, err := DecodeCall(protocol.MOniReminders, json.RawMessage(bad)); err == nil {
			t.Errorf("%s decoded", bad)
		}
	}
	for _, action := range protocol.OniRoutinesActions {
		name := ""
		if action == protocol.OniRoutinesForget {
			name = `,"name":"Routine 1"`
		}
		if _, err := DecodeCall(protocol.MOniRoutines, json.RawMessage(`{"action":"`+action+`"`+name+`}`)); err != nil {
			t.Errorf("oni.routines %s: %v", action, err)
		}
	}
	// no reflex call is reserved: the daemon serves each once registered
	for _, method := range []string{protocol.MOniReminders, protocol.MOniRoutines, protocol.MOniReflexLog} {
		if slices.Contains(ReservedCalls(), method) {
			t.Errorf("%s is reserved", method)
		}
	}
}

func TestReflexCallParamsDecodeStrictly(t *testing.T) {
	good := [][2]string{
		{"oni.reminders", `{"action":"list"}`},
		{"oni.reminders", `{"action":"cancel","id":"00ff00ff00ff00ff"}`},
		{"oni.reminders", `{"action":"cancel_all"}`},
		{"oni.routines", `{"action":"list"}`},
		{"oni.routines", `{"action":"run"}`},
		{"oni.routines", `{"action":"run","name":"Routine 2"}`},
		{"oni.routines", `{"action":"forget","name":"Wind down"}`},
		{"oni.routines", `{"action":"forget_all"}`},
		{"oni.reflex.log", ``},
		{"oni.reflex.log", `{}`},
	}
	for _, c := range good {
		if _, err := DecodeCall(c[0], json.RawMessage(c[1])); err != nil {
			t.Errorf("%s %s: %v", c[0], c[1], err)
		}
	}
	bad := [][2]string{
		{"oni.reminders", ``},
		{"oni.reminders", `{}`},
		{"oni.reminders", `null`},
		{"oni.reminders", `{"action":"List"}`},
		{"oni.reminders", `{"Action":"list"}`},
		{"oni.reminders", `{"action":"list","id":"00ff00ff00ff00ff"}`},
		{"oni.reminders", `{"action":"list","action":"cancel_all"}`},
		{"oni.reminders", `{"action":"cancel"}`},
		{"oni.reminders", `{"action":"cancel","id":"00FF00FF00FF00FF"}`},
		{"oni.reminders", `{"action":"cancel","id":"00ff00ff00ff00f"}`},
		{"oni.reminders", `{"action":"cancel","id":"00ff00ff00ff00ff0"}`},
		{"oni.reminders", `{"action":"cancel","id":"../oni-reminders"}`},
		{"oni.reminders", `{"action":"cancel_all","id":"00ff00ff00ff00ff"}`},
		{"oni.reminders", `{"action":"add","text":"call the bank"}`},
		{"oni.reminders", `{"action":"list","text":"call the bank"}`},
		{"oni.reminders", `{"action":"cancel","id":5}`},
		{"oni.routines", ``},
		{"oni.routines", `{}`},
		{"oni.routines", `{"action":"save","name":"Morning"}`},
		{"oni.routines", `{"action":"list","name":"Morning"}`},
		{"oni.routines", `{"action":"forget"}`},
		{"oni.routines", `{"action":"forget","name":" Morning"}`},
		{"oni.routines", `{"action":"forget","name":"Morning\n"}`},
		{"oni.routines", `{"action":"forget","name":"Morn\u200bing"}`},
		{"oni.routines", `{"action":"run","name":"` + strings.Repeat("a", protocol.OniRoutineNameMax+1) + `"}`},
		{"oni.routines", `{"action":"run","name":"   "}`},
		{"oni.routines", `{"action":"forget_all","name":"Morning"}`},
		{"oni.routines", `{"action":"run","steps":[]}`},
		{"oni.routines", `{"Name":"Morning","action":"forget"}`},
		{"oni.reflex.log", `null`},
		{"oni.reflex.log", `{"rows":10}`},
		{"oni.reflex.log", `{"words":true}`},
	}
	for _, c := range bad {
		if v, err := DecodeCall(c[0], json.RawMessage(c[1])); err == nil {
			t.Errorf("%s %s decoded as %#v", c[0], c[1], v)
		}
	}
}

// No package under internal/oni may reach the reflex executors under
// internal/reflex, even through another package; the checker sees it.
func TestNoOniPackageReachesTheReflexExecutors(t *testing.T) {
	i := slices.IndexFunc(boundaries, func(b struct {
		from      string
		forbidden []string
	}) bool {
		return b.from == "internal/oni/..."
	})
	if i < 0 || !slices.Contains(boundaries[i].forbidden, "internal/reflex") {
		t.Fatal("no boundary keeps the Oni packages from the reflex executors")
	}
	root := t.TempDir()
	const module = "example.test/app"
	for name, body := range map[string]string{
		"internal/oni/reflex/r.go":  "package reflex\n",
		"internal/oni/routine/r.go": "package routine\n\nimport _ \"" + module + "/internal/relay\"\n",
		"internal/relay/r.go":       "package relay\n\nimport _ \"" + module + "/internal/reflex/act\"\n",
		"internal/reflex/act/a.go":  "package act\n\nimport _ \"" + module + "/internal/oni/reflex\"\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var bad []string
	for _, from := range expand(t, root, boundaries[i].from) {
		got, _, err := violations(root, module, module+"/"+from, boundaries[i].forbidden)
		if err != nil {
			t.Fatal(err)
		}
		bad = append(bad, got...)
	}
	want := module + "/internal/oni/routine -> " + module + "/internal/relay -> " + module + "/internal/reflex/act"
	if len(bad) != 1 || bad[0] != want {
		t.Fatalf("violations %q, want [%q]", bad, want)
	}
}
