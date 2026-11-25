package feature

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/protocol"
)

func everythingOn() protocol.OniSettings {
	s := protocol.DefaultOniSettings()
	s.Master, s.Inspect, s.Voice, s.Reflex, s.Feeds.Guard = true, true, true, true, true
	s.Guide = protocol.OniGuideSettings{Enabled: true, Keep: true, Hints: true}
	s.ReflexSettings = protocol.OniReflexSettings{Routines: true, Reminders: true, Notes: true, Decisions: true}
	s.Recall = protocol.OniRecallSettings{Enabled: true, Vault: true}
	s.Rewrite.Enabled, s.Assignment.Enabled = true, true
	s.VoiceSettings.Dictation.Enabled = true
	s.Disclosure = protocol.OniDisclosureAlways
	return s
}

var handsIDs = []string{HandsBrowser, HandsApp, HandsDesktop}

func TestDefaultsDenyEveryFeatureWithAReason(t *testing.T) {
	defaults := config.Defaults()
	for _, s := range []protocol.OniSettings{defaults.Oni, protocol.EffectiveOni(defaults)} {
		set := Capabilities(s)
		if set.Master {
			t.Fatal("Oni is on by default")
		}
		if len(set.Features) != len(Features()) || len(set.Features) == 0 {
			t.Fatalf("capabilities cover %d of %d features", len(set.Features), len(Features()))
		}
		for _, c := range set.Features {
			if c.Allow || c.Reason != ReasonOniOff {
				t.Errorf("default %s = allow %v reason %q, want denied with %q", c.ID, c.Allow, c.Reason, ReasonOniOff)
			}
			if d := Decide(Request{Feature: c.ID}, s); d.Allow || d.Reason != c.Reason {
				t.Errorf("Decide(%s) = %+v, capabilities say %q", c.ID, d, c.Reason)
			}
		}
	}
}

func TestOniMasterAloneTurnsOnNoFeature(t *testing.T) {
	settings := config.Defaults()
	settings.Enabled = true
	settings.Oni.Master = true
	set := Capabilities(protocol.EffectiveOni(settings))
	if !set.Master {
		t.Fatal("Oni's master switch did not count while Hyakki is on")
	}
	for _, c := range set.Features {
		if c.Allow || c.Reason != ReasonFeatureOff {
			t.Errorf("master alone: %s = allow %v reason %q, want %q", c.ID, c.Allow, c.Reason, ReasonFeatureOff)
		}
	}
}

func TestHyakkiOffTurnsOniOff(t *testing.T) {
	settings := config.Defaults()
	settings.Oni = everythingOn()
	settings.Enabled = false
	for _, c := range Capabilities(protocol.EffectiveOni(settings)).Features {
		if c.Allow {
			t.Errorf("%s allowed while Hyakki is off", c.ID)
		}
		if c.Reason != ReasonOniOff {
			t.Errorf("%s reason %q while Hyakki is off", c.ID, c.Reason)
		}
	}
	settings.Enabled = true
	allowed := 0
	for _, c := range Capabilities(protocol.EffectiveOni(settings)).Features {
		if c.Allow {
			allowed++
		}
	}
	if allowed != len(Features())-len(handsIDs) {
		t.Fatalf("with Hyakki on and every switch on, %d features are allowed", allowed)
	}
}

func TestEachSwitchTurnsOnOnlyItsFeature(t *testing.T) {
	// a guide feature's switch counts only with the ones it builds on, so
	// those are on too and their features are allowed with it
	switches := map[string]struct {
		on    func(*protocol.OniSettings)
		allow []string
	}{
		Inspect:        {func(s *protocol.OniSettings) { s.Inspect = true }, nil},
		Voice:          {func(s *protocol.OniSettings) { s.Voice = true }, nil},
		VoiceDictation: {func(s *protocol.OniSettings) { s.VoiceSettings.Dictation.Enabled = true }, nil},
		Reflex:         {func(s *protocol.OniSettings) { s.Reflex = true }, nil},
		FeedGuard:      {func(s *protocol.OniSettings) { s.Feeds.Guard = true }, nil},
		Guide:          {func(s *protocol.OniSettings) { s.Inspect, s.Guide.Enabled = true, true }, []string{Inspect}},
		GuideKeep:      {func(s *protocol.OniSettings) { s.Inspect, s.Guide.Enabled, s.Guide.Keep = true, true, true }, []string{Inspect, Guide}},
		GuideHints: {func(s *protocol.OniSettings) {
			s.Inspect, s.Guide.Enabled, s.Guide.Keep, s.Guide.Hints = true, true, true, true
		}, []string{Inspect, Guide, GuideKeep}},
		Routines:  {func(s *protocol.OniSettings) { s.Reflex, s.ReflexSettings.Routines = true, true }, []string{Reflex}},
		Reminders: {func(s *protocol.OniSettings) { s.Reflex, s.ReflexSettings.Reminders = true, true }, []string{Reflex}},
		// forgetting by voice shares the notes switch
		Notes:     {func(s *protocol.OniSettings) { s.Reflex, s.ReflexSettings.Notes = true, true }, []string{Reflex, MemoryForget}},
		Decisions: {func(s *protocol.OniSettings) { s.Reflex, s.ReflexSettings.Decisions = true, true }, []string{Reflex}},
		// every Hands stage looks at what it works in, so looking at
		// windows is on too; each stage turns on its own row alone
		HandsBrowser: {func(s *protocol.OniSettings) { s.Inspect, s.Hands = true, protocol.OniHandsBrowser }, []string{Inspect}},
		HandsApp:     {func(s *protocol.OniSettings) { s.Inspect, s.Hands = true, protocol.OniHandsApp }, []string{Inspect}},
		HandsDesktop: {func(s *protocol.OniSettings) { s.Inspect, s.Hands = true, protocol.OniHandsDesktop }, []string{Inspect}},
		MemoryForget: {func(s *protocol.OniSettings) { s.Reflex, s.ReflexSettings.Notes = true, true }, []string{Reflex, Notes}},
		Recall:       {func(s *protocol.OniSettings) { s.Recall.Enabled = true }, nil},
		Rewrite:      {func(s *protocol.OniSettings) { s.Rewrite.Enabled = true }, nil},
		// assignment mode reads the window, so looking at windows is on too
		Assignment: {func(s *protocol.OniSettings) { s.Inspect, s.Assignment.Enabled = true, true }, []string{Inspect}},
	}
	for _, f := range Features() {
		if f.Stage == StageUnavailable {
			continue
		}
		sw, ok := switches[f.ID]
		if !ok {
			t.Errorf("feature %s has no switch in this test", f.ID)
			continue
		}
		s := protocol.DefaultOniSettings()
		s.Master = true
		sw.on(&s)
		allowed := append([]string{f.ID}, sw.allow...)
		for _, c := range Capabilities(s).Features {
			if c.Allow != slices.Contains(allowed, c.ID) {
				t.Errorf("turning on %s left %s allow=%v (%s)", f.ID, c.ID, c.Allow, c.Reason)
			}
		}
		s.Master = false
		if Decide(Request{Feature: f.ID}, s).Allow {
			t.Errorf("%s allowed with Oni's master off", f.ID)
		}
	}
}

// The stages are exclusive. Under every setting the app row is allowed only
// with Hyakki, Oni, looking at windows and the app stage on, the desktop row
// only with the desktop stage, never both, and each always asks; a stage
// is stored only exactly spelled.
func TestHandsStagesAreExclusiveAcrossEverySetting(t *testing.T) {
	stages := append(slices.Clone(protocol.OniHandsStages), "", "everything", "App")
	disclosures := []string{protocol.OniDisclosureSession, protocol.OniDisclosureAlways, ""}
	accepted, combos := 0, 0
	for bits := range 1 << 6 {
		for _, stage := range stages {
			for _, disclosure := range disclosures {
				combos++
				settings := config.Defaults()
				settings.Enabled = bits&1 != 0
				settings.Oni.Master = bits&2 != 0
				settings.Oni.Inspect = bits&4 != 0
				settings.Oni.Voice = bits&8 != 0
				settings.Oni.Reflex = bits&16 != 0
				settings.Oni.Feeds.Guard = bits&32 != 0
				settings.Oni.Disclosure = disclosure
				settings.Oni.Hands = stage
				eff := protocol.EffectiveOni(settings)
				on := settings.Enabled && settings.Oni.Master && settings.Oni.Inspect
				want := map[string]bool{
					HandsBrowser: on && stage == protocol.OniHandsBrowser,
					HandsApp:     on && stage == protocol.OniHandsApp,
					HandsDesktop: on && stage == protocol.OniHandsDesktop,
				}
				allowed := 0
				for _, id := range handsIDs {
					if !MustAsk(id, eff, true) {
						t.Fatalf("%s would act without asking under %+v", id, eff)
					}
					d := Decide(Request{Feature: id}, eff)
					if d.Allow != want[id] {
						t.Fatalf("Decide(%s) under %+v = %+v, want allow %v", id, eff, d, want[id])
					}
					if d.Allow {
						allowed++
					}
				}
				if allowed > 1 {
					t.Fatalf("%d Hands stages allowed at once under %+v", allowed, eff)
				}
				patched, err := config.Merge(config.Defaults(), json.RawMessage(`{"oni":{"master":true,"inspect":true,"hands":"`+stage+`"}}`))
				if err != nil {
					if slices.Contains(protocol.OniHandsStages, stage) {
						t.Fatalf("settings.set refused stage %q: %v", stage, err)
					}
					continue
				}
				accepted++
				if patched.Oni.Hands != stage {
					t.Fatalf("settings.set stored hands %q for %q", patched.Oni.Hands, stage)
				}
			}
		}
	}
	if combos != 64*len(stages)*len(disclosures) || accepted == 0 {
		t.Fatalf("sweep ran %d combinations, settings.set accepted %d", combos, accepted)
	}
}

// Hands in a browser is guarded: it is allowed only with Hyakki, Oni,
// looking at windows and the browser stage all on, and otherwise denied
// with the reason the order names first. No other switch counts, and it
// always asks.
func TestBrowserHandsNeedOniInspectAndTheBrowserStage(t *testing.T) {
	f, ok := Lookup(HandsBrowser)
	if want := (Feature{ID: HandsBrowser, Stage: StageGuarded, Setting: "oni.hands", Egress: EgressDisclosed, Consent: ConsentFirm}); !ok || f != want {
		t.Fatalf("the browser row is %+v", f)
	}
	stages := append(slices.Clone(protocol.OniHandsStages), "", "everything", "BROWSER")
	allowed := 0
	for bits := range 1 << 6 {
		for _, stage := range stages {
			settings := config.Defaults()
			settings.Enabled = bits&1 != 0
			settings.Oni.Master = bits&2 != 0
			settings.Oni.Inspect = bits&4 != 0
			settings.Oni.Voice = bits&8 != 0
			settings.Oni.Reflex = bits&16 != 0
			settings.Oni.Feeds.Guard = bits&32 != 0
			settings.Oni.Hands = stage
			for _, s := range []protocol.OniSettings{settings.Oni, protocol.EffectiveOni(settings)} {
				want := Decision{Allow: true, Reason: ReasonAllowed}
				switch {
				case !s.Master:
					want = Decision{Reason: ReasonOniOff}
				case !s.Inspect || stage != protocol.OniHandsBrowser:
					want = Decision{Reason: ReasonFeatureOff}
				}
				if d := Decide(Request{Feature: HandsBrowser}, s); d != want {
					t.Fatalf("Decide(hands.browser) under %+v = %+v, want %+v", s, d, want)
				}
				if want.Allow {
					allowed++
				}
				if !MustAsk(HandsBrowser, s, true) {
					t.Fatalf("hands.browser would act without asking under %+v", s)
				}
			}
		}
	}
	// the browser stage with Master and Inspect on and the four other
	// switches either way: allowed raw all 16 times, and effective only in
	// the 8 where Hyakki is on too
	if allowed != 16+8 {
		t.Fatalf("hands.browser allowed %d times", allowed)
	}
}

func TestUnknownFeaturesAreDeniedAndAsk(t *testing.T) {
	s := everythingOn()
	for _, id := range []string{"", "hands", "oni.settings.patch", "VISION.INSPECT", Inspect + " ", "vision"} {
		if d := Decide(Request{Feature: id}, s); d.Allow || d.Reason != ReasonUnknown {
			t.Errorf("Decide(%q) = %+v", id, d)
		}
		if !MustAsk(id, s, true) {
			t.Errorf("unknown feature %q would act without asking", id)
		}
	}
}

func TestChangingTheReturnedContractChangesNoDecision(t *testing.T) {
	list := Features()
	for i := range list {
		list[i].Stage = StageReadOnly
		list[i].Consent = ConsentUserInitiated
	}
	s := everythingOn()
	s.Hands = protocol.OniHandsDesktop
	for _, id := range handsIDs {
		if d := Decide(Request{Feature: id}, s); d.Allow != (id == HandsDesktop) {
			t.Fatalf("%s decided %+v after the caller edited its copy of the contract", id, d)
		}
		if !MustAsk(id, s, true) {
			t.Fatalf("%s stopped asking after the caller edited its copy", id)
		}
	}
}

func TestConsentAsksAsEachClassSays(t *testing.T) {
	s := everythingOn()
	s.Disclosure = protocol.OniDisclosureSession
	if !MustAsk(Inspect, s, false) || MustAsk(Inspect, s, true) {
		t.Error("once-a-session disclosure does not ask exactly once")
	}
	s.Disclosure = protocol.OniDisclosureAlways
	if !MustAsk(Inspect, s, true) {
		t.Error("every-time disclosure skipped a capture")
	}
	s.Disclosure = "never"
	if !MustAsk(Inspect, s, true) {
		t.Error("an unknown disclosure mode skipped asking")
	}
	if MustAsk(Voice, s, false) {
		t.Error("hold-to-talk asks on top of the user holding the chord")
	}
	if !MustAsk(FeedGuard, s, true) {
		t.Error("the feed guard closed something without asking")
	}
}

func TestFrameFixturesDecodeStrictlyAndReencodeExactly(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "frames", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	bad := 0
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(path)
		p, err := DecodeFrame(raw)
		if strings.HasPrefix(name, "bad-") {
			bad++
			if err == nil {
				t.Errorf("%s decoded as %#v", name, p)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		out, err := EncodeFrame(p)
		if err != nil {
			t.Errorf("%s: re-encode: %v", name, err)
			continue
		}
		if want := bytes.TrimSuffix(raw, []byte("\n")); !bytes.Equal(out, want) {
			t.Errorf("%s re-encoded as\n%s\nwant\n%s", name, out, want)
		}
		seen[p.Kind()] = true
	}
	for _, kind := range Kinds() {
		if !seen[kind] {
			t.Errorf("no valid fixture for frame kind %s", kind)
		}
	}
	if bad == 0 {
		t.Fatal("no malformed fixtures ran")
	}
}

func TestEncodeRefusesFramesTheContractForbids(t *testing.T) {
	for _, p := range []Payload{
		&HandsFrame{Stage: protocol.OniHandsBrowser, Reason: ReasonUnavailable},
		&HandsFrame{Stage: protocol.OniHandsOff, Active: true, Reason: ReasonUnavailable},
		&ConsentFrame{ID: "c-1", Feature: HandsApp, Class: ConsentFirm, Destination: "local", Question: "Type here?", ExpiresMs: 1000},
		&StrandTargetFrame{Seq: 1, X: math.NaN(), W: 0.1, H: 0.1},
		&StrandTargetFrame{Seq: 1, W: 0.1, H: 0.1, Mode: StrandModeRegion},
		&StrandTargetFrame{Seq: 1, W: 0.1, H: 0.1, Mode: StrandModeRegion, Region: &Box{W: math.Inf(1), H: 1}},
		&PerchFrame{Seq: 1, Attached: true, Window: "12", Corner: "top-left", Output: "DP-1"},
		&StrandArriveFrame{Seq: 1, Skipped: "asleep"},
		&StrandRetractFrame{},
		&StateFrame{State: "idle", Reason: "Oni Off"},
		&CharacterFrame{ID: protocol.OniDefaultCharacter, Name: "Oni\u200b"},
	} {
		if out, err := EncodeFrame(p); err == nil {
			t.Errorf("encoded %s", out)
		}
	}
}

func TestReservedCallsAreNotServed(t *testing.T) {
	for _, method := range ReservedCalls() {
		if _, ok := protocol.Lookup(method); ok {
			t.Errorf("%s is registered before any phase serves it", method)
		}
	}
	for _, method := range []string{"oni.settings.patch", "oni.settings.set"} {
		if _, ok := protocol.Lookup(method); ok || slices.Contains(ReservedCalls(), method) {
			t.Errorf("%s exists; Oni settings change through settings.set", method)
		}
		if _, err := DecodeCall(method, json.RawMessage(`{}`)); err == nil {
			t.Errorf("%s decoded as an Oni call", method)
		}
	}
}

func TestOniCallsAreServedAndDecodeToTheirParams(t *testing.T) {
	served := []string{
		protocol.MOniOpen, protocol.MOniClose, protocol.MOniPlace, protocol.MOniInteract, protocol.MOniCharacterAssets, protocol.MOniStrandArrived,
		protocol.MOniPerch, protocol.MOniDetach, protocol.MOniInspect, protocol.MOniAsk, protocol.MOniConsentAnswer, protocol.MOniOverlay,
		protocol.MOniTalk, protocol.MOniSpeak, protocol.MOniDictate, protocol.MOniVoiceTest,
		protocol.MOniGuide, protocol.MOniKeep, protocol.MOniSkip, protocol.MOniPick,
		protocol.MOniRoutines,
		protocol.MOniReflexLog,
		protocol.MOniReminders,
		protocol.MOniHands, protocol.MOniHandsLog,
		protocol.MOniFocus, protocol.MOniRewrite,
		protocol.MOniRecall, protocol.MOniRecallImport, protocol.MOniAssignment, protocol.MOniAssignmentLog,
	}
	if got := ReservedCalls(); got == nil || len(got) != 0 {
		t.Errorf("reserved calls are %#v", got)
	}
	for _, method := range served {
		if _, ok := protocol.Lookup(method); !ok {
			t.Errorf("%s is not registered", method)
		}
		if slices.Contains(ReservedCalls(), method) {
			t.Errorf("%s is served and still reserved", method)
		}
	}
	v, err := DecodeCall(protocol.MOniPlace, json.RawMessage(`{"output":"DP-1","fx":0.25,"fy":1,"corner":""}`))
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := v.(*protocol.OniPlaceParams); !ok || *p != (protocol.OniPlaceParams{Output: "DP-1", FX: 0.25, FY: 1}) {
		t.Fatalf("oni.place decoded as %#v", v)
	}
	v, err = DecodeCall(protocol.MOniOpen, json.RawMessage(`{"renderer":"observer"}`))
	if p, ok := v.(*protocol.OniOpenParams); err != nil || !ok || p.Renderer != protocol.OniRendererObserver {
		t.Fatalf("oni.open decoded as %#v, %v", v, err)
	}
	v, err = DecodeCall(protocol.MOniStrandArrived, json.RawMessage(`{"seq":4,"skipped":"off-output"}`))
	if p, ok := v.(*protocol.OniStrandArrivedParams); err != nil || !ok || *p != (protocol.OniStrandArrivedParams{Seq: 4, Skipped: SkipOffOutput}) {
		t.Fatalf("oni.strandArrived decoded as %#v, %v", v, err)
	}
	v, err = DecodeCall(protocol.MOniPerch, json.RawMessage(`{"output":"DP-1","x":12.5,"y":40,"hover":true}`))
	if p, ok := v.(*protocol.OniPerchParams); err != nil || !ok || *p != (protocol.OniPerchParams{Output: "DP-1", X: 12.5, Y: 40, Hover: true}) {
		t.Fatalf("oni.perch decoded as %#v, %v", v, err)
	}
	v, err = DecodeCall(protocol.MOniDetach, nil)
	if p, ok := v.(*protocol.OniDetachParams); err != nil || !ok || p.Reason != "" {
		t.Fatalf("oni.detach decoded as %#v, %v", v, err)
	}
	v, err = DecodeCall(protocol.MOniOverlay, json.RawMessage(`{"output":"DP-1","rects":[{"x":1,"y":2,"w":3,"h":4}]}`))
	if p, ok := v.(*protocol.OniOverlayParams); err != nil || !ok || len(p.Rects) != 1 || p.Rects[0] != (protocol.OniRect{X: 1, Y: 2, W: 3, H: 4}) {
		t.Fatalf("oni.overlay decoded as %#v, %v", v, err)
	}
	v, err = DecodeCall(protocol.MOniAsk, json.RawMessage(`{"text":"Where is Save?"}`))
	if p, ok := v.(*protocol.OniAskParams); err != nil || !ok || p.Text != "Where is Save?" {
		t.Fatalf("oni.ask decoded as %#v, %v", v, err)
	}
	v, err = DecodeCall(protocol.MOniConsentAnswer, json.RawMessage(`{"id":"c-1","answer":"yes"}`))
	if p, ok := v.(*protocol.OniConsentAnswerParams); err != nil || !ok || *p != (protocol.OniConsentAnswerParams{ID: "c-1", Answer: "yes"}) {
		t.Fatalf("oni.consent.answer decoded as %#v, %v", v, err)
	}
	v, err = DecodeCall(protocol.MOniTalk, json.RawMessage(`{"action":"start"}`))
	if p, ok := v.(*protocol.OniTalkParams); err != nil || !ok || p.Action != protocol.OniVoiceStart {
		t.Fatalf("oni.talk decoded as %#v, %v", v, err)
	}
	v, err = DecodeCall(protocol.MOniSpeak, json.RawMessage(`{"action":"stop"}`))
	if p, ok := v.(*protocol.OniSpeakParams); err != nil || !ok || p.Action != protocol.OniVoiceStop {
		t.Fatalf("oni.speak decoded as %#v, %v", v, err)
	}
	v, err = DecodeCall(protocol.MOniDictate, json.RawMessage(`{"action":"start","mode":"polished"}`))
	if p, ok := v.(*protocol.OniDictateParams); err != nil || !ok || *p != (protocol.OniDictateParams{Action: protocol.OniVoiceStart, Mode: protocol.OniDictatePolished}) {
		t.Fatalf("oni.dictate decoded as %#v, %v", v, err)
	}
	v, err = DecodeCall(protocol.MOniVoiceTest, nil)
	if p, ok := v.(*protocol.OniVoiceTestParams); err != nil || !ok || *p != (protocol.OniVoiceTestParams{}) {
		t.Fatalf("oni.voice.test decoded as %#v, %v", v, err)
	}
}

func TestPerchReasonsAreTheClosedSet(t *testing.T) {
	want := []string{"closed", "user", "dragged-off", "output-gone", "hidden", "no-geometry", "oni-off", "presenter-lost"}
	if !slices.Equal(PerchReasons, want) {
		t.Fatalf("perch reasons = %v", PerchReasons)
	}
	for _, reason := range PerchReasons {
		if !validCode(reason, 32) {
			t.Errorf("reason %q is not a short code", reason)
		}
	}
}

func TestCallParamsDecodeStrictly(t *testing.T) {
	good := [][2]string{
		{"oni.open", `{"renderer":"native"}`},
		{"oni.open", `{"renderer":"ryoku"}`},
		{"oni.close", `{}`},
		{"oni.close", ``},
		{"oni.place", `{"output":"","fx":0,"fy":0,"corner":"tr"}`},
		{"oni.place", `{"output":"HDMI-A-1","fx":1,"fy":0,"corner":""}`},
		{"oni.interact", `{"kind":"wake"}`},
		{"oni.interact", `{"kind":"drop"}`},
		{"oni.character.assets", `{"id":"testmoth"}`},
		{"oni.perch", `{"window":"0x55d1c2a0"}`},
		{"oni.ask", `{"text":"What does this button do?"}`},
		{"oni.consent.answer", `{"id":"c-1","answer":"no"}`},
		{"oni.strandArrived", `{"seq":3}`},
		{"oni.strandArrived", `{"seq":3,"skipped":"covers-screen"}`},
		{"oni.strandArrived", `{"seq":3,"skipped":"hidden"}`},
		{"oni.perch", `{"output":"DP-1","x":10,"y":20}`},
		{"oni.perch", `{"output":"DP-1","hover":true}`},
		{"oni.detach", ``},
		{"oni.detach", `{"reason":"dragged-off"}`},
		{"oni.detach", `{"reason":"hidden"}`},
		{"oni.inspect", `{}`},
		{"oni.overlay", `{"output":"DP-1","rects":[]}`},
		{"oni.overlay", `{"output":"DP-1","rects":[{"x":-4,"y":0,"w":96,"h":96}]}`},
		{"oni.talk", `{"action":"start"}`},
		{"oni.talk", `{"action":"stop"}`},
		{"oni.talk", `{"action":"cancel"}`},
		{"oni.speak", `{"action":"stop"}`},
		{"oni.dictate", `{"action":"start"}`},
		{"oni.dictate", `{"action":"start","mode":"exact"}`},
		{"oni.dictate", `{"action":"stop"}`},
		{"oni.dictate", `{"action":"cancel"}`},
		{"oni.voice.test", ``},
		{"oni.voice.test", `{"mic":"alsa_input.pci-0000_00_1f.3.analog-stereo","ms":100}`},
		{"oni.voice.test", `{"mic":"oni_lab_mic","ms":5000}`},
	}
	for _, c := range good {
		if _, err := DecodeCall(c[0], json.RawMessage(c[1])); err != nil {
			t.Errorf("%s %s: %v", c[0], c[1], err)
		}
	}
	bad := [][2]string{
		{"oni.open", ``},
		{"oni.open", `{"renderer":"native","force":true}`},
		{"oni.open", `{"renderer":"gio"}`},
		{"oni.open", `{"Renderer":"native"}`},
		{"oni.close", `{"now":true}`},
		{"oni.place", `{"output":"","fx":0,"fy":0,"corner":""}`},
		{"oni.place", `{"output":"DP-1","fx":1.01,"fy":0,"corner":""}`},
		{"oni.place", `{"output":"DP-1","fx":0.5,"fy":-0.1,"corner":""}`},
		{"oni.place", `{"output":"DP-1","fx":0.5,"fy":0.5,"corner":"top-left"}`},
		{"oni.place", `{"output":"DP-1","fx":0.5,"fy":0.5}`},
		{"oni.place", `{"output":"DP-1\u200b","fx":0.5,"fy":0.5,"corner":""}`},
		{"oni.place", `{"output":"DP-1","fx":0.5,"fx":0.4,"fy":0.5,"corner":""}`},
		{"oni.interact", `{"kind":"tickle"}`},
		{"oni.interact", `{}`},
		{"oni.character.assets", `{"id":"../oni"}`},
		{"oni.character.assets", `{"id":null}`},
		{"oni.inspect", `null`},
		{"oni.perch", `{}`},
		{"oni.perch", `{"Window":"1"}`},
		{"oni.perch", `{"window":"1","window":"2"}`},
		{"oni.perch", `{"window":"1 && rm"}`},
		{"oni.ask", `{"text":""}`},
		{"oni.strandArrived", `{"seq":0}`},
		{"oni.strandArrived", `{}`},
		{"oni.strandArrived", `{"seq":3,"skipped":"asleep"}`},
		{"oni.strandArrived", `{"seq":3,"advanced":true}`},
		{"oni.strandArrived", `{"seq":-1}`},
		{"oni.consent.answer", `{"id":"c-1","answer":"yes","remember":true}`},
		{"oni.consent.answer", `{"id":"c-1"}`},
		{"oni.consent.answer", `{"id":"C 1","answer":"yes"}`},
		{"oni.perch", `{"window":"1","output":"DP-1"}`},
		{"oni.perch", `{"window":"1","hover":true}`},
		{"oni.perch", `{"window":"1","x":3}`},
		{"oni.perch", `{"output":"DP-1","x":-1,"y":0}`},
		{"oni.perch", `{"hover":true}`},
		{"oni.detach", `{"reason":"closed"}`},
		{"oni.detach", `{"reason":"oni-off"}`},
		{"oni.inspect", `{"text":"look"}`},
		{"oni.overlay", `{"output":"DP-1"}`},
		{"oni.overlay", `{"rects":[]}`},
		{"oni.overlay", `{"output":"DP-1","rects":[{"x":0,"y":0,"w":0,"h":4}]}`},
		{"oni.overlay", `{"output":"DP-1","rects":[{"x":0,"y":0,"w":4}]}`},
		{"oni.overlay", `{"output":"DP-1","rects":[` + strings.Repeat(`{"x":0,"y":0,"w":1,"h":1},`, MaxOverlayRects) + `{"x":0,"y":0,"w":1,"h":1}]}`},
		{"oni.talk", ``},
		{"oni.talk", `{"action":"hold"}`},
		{"oni.talk", `{"action":"start","source":"wake"}`},
		{"oni.talk", `{"Action":"start"}`},
		{"oni.speak", `{"action":"start"}`},
		{"oni.speak", `{}`},
		{"oni.dictate", `{"action":"start","mode":"loud"}`},
		{"oni.dictate", `{"action":"stop","mode":"clean"}`},
		{"oni.dictate", `{"action":"start","text":"hello"}`},
		{"oni.voice.test", `{"mic":"/dev/snd/pcmC0D0c"}`},
		{"oni.voice.test", `{"mic":"-h"}`},
		{"oni.voice.test", `{"mic":"a b"}`},
		{"oni.voice.test", `{"ms":99}`},
		{"oni.voice.test", `{"ms":5001}`},
		{"oni.voice.test", `{"ms":-1}`},
		{"oni.voice.test", `{"record":true}`},
	}
	for _, c := range bad {
		if v, err := DecodeCall(c[0], json.RawMessage(c[1])); err == nil {
			t.Errorf("%s %s decoded as %#v", c[0], c[1], v)
		}
	}
}
