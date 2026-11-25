package feature

import (
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

// the capabilities Phase 11 shipped, before the guide rows
const (
	phase11DefaultCapabilities = `{"master":false,"features":[` +
		`{"id":"vision.inspect","stage":"read-only","egress":"disclosed","consent":"disclosure","allow":false,"reason":"oni-off"},` +
		`{"id":"voice","stage":"read-only","egress":"disclosed","consent":"user-initiated","allow":false,"reason":"oni-off"},` +
		`{"id":"voice.dictation","stage":"reversible","egress":"disclosed","consent":"user-initiated","allow":false,"reason":"oni-off"},` +
		`{"id":"reflex","stage":"reversible","egress":"disclosed","consent":"announced","allow":false,"reason":"oni-off"},` +
		`{"id":"feeds.guard","stage":"reversible","egress":"local","consent":"per-action","allow":false,"reason":"oni-off"},` +
		`{"id":"hands.browser","stage":"unavailable","egress":"disclosed","consent":"firm","allow":false,"reason":"unavailable"},` +
		`{"id":"hands.app","stage":"unavailable","egress":"disclosed","consent":"firm","allow":false,"reason":"unavailable"},` +
		`{"id":"hands.desktop","stage":"unavailable","egress":"disclosed","consent":"firm","allow":false,"reason":"unavailable"}]}`
	phase11InspectOnCapabilities = `{"master":true,"features":[` +
		`{"id":"vision.inspect","stage":"read-only","egress":"disclosed","consent":"disclosure","allow":true,"reason":"allowed"},` +
		`{"id":"voice","stage":"read-only","egress":"disclosed","consent":"user-initiated","allow":false,"reason":"feature-off"},` +
		`{"id":"voice.dictation","stage":"reversible","egress":"disclosed","consent":"user-initiated","allow":false,"reason":"feature-off"},` +
		`{"id":"reflex","stage":"reversible","egress":"disclosed","consent":"announced","allow":false,"reason":"feature-off"},` +
		`{"id":"feeds.guard","stage":"reversible","egress":"local","consent":"per-action","allow":false,"reason":"feature-off"},` +
		`{"id":"hands.browser","stage":"unavailable","egress":"disclosed","consent":"firm","allow":false,"reason":"unavailable"},` +
		`{"id":"hands.app","stage":"unavailable","egress":"disclosed","consent":"firm","allow":false,"reason":"unavailable"},` +
		`{"id":"hands.desktop","stage":"unavailable","egress":"disclosed","consent":"firm","allow":false,"reason":"unavailable"}]}`
)

// the three guide rows, denied with reason, as they sit before the hands
// rows
func guideRows(reason string) string {
	return `{"id":"guide","stage":"read-only","egress":"disclosed","consent":"disclosure","allow":false,"reason":"` + reason + `"},` +
		`{"id":"guide.keep","stage":"read-only","egress":"local","consent":"per-action","allow":false,"reason":"` + reason + `"},` +
		`{"id":"guide.hints","stage":"read-only","egress":"local","consent":"announced","allow":false,"reason":"` + reason + `"},`
}

func withGuideRows(capabilities, reason string) string {
	return strings.Replace(capabilities, `{"id":"hands.browser"`, guideRows(reason)+`{"id":"hands.browser"`, 1)
}

func TestGuidePhasesAreTheSpecsPhases(t *testing.T) {
	if !slices.Equal(GuidePhases, spec.GuidePhaseIDs) {
		t.Fatalf("frame phases %v, spec phases %v", GuidePhases, spec.GuidePhaseIDs)
	}
	for phase := range guidePhaseActions {
		if !slices.Contains(GuidePhases, phase) || phase == GuideIdle {
			t.Errorf("actions listed for phase %q", phase)
		}
	}
	if MaxGuideSteps() != 12 {
		t.Errorf("a guide takes %d steps, the spec says 12", MaxGuideSteps())
	}
}

// Each guide feature needs Oni's switch, looking at windows and its own
// switch, and Keep and hints the switches they build on; the reasons are
// the ones every feature gives.
func TestGuideFeaturesNeedOniInspectAndTheirOwnSwitch(t *testing.T) {
	for bits := range 1 << 6 {
		settings := config.Defaults()
		settings.Enabled = bits&1 != 0
		settings.Oni.Master = bits&2 != 0
		settings.Oni.Inspect = bits&4 != 0
		settings.Oni.Guide = protocol.OniGuideSettings{Enabled: bits&8 != 0, Keep: bits&16 != 0, Hints: bits&32 != 0}
		s := protocol.EffectiveOni(settings)
		on := settings.Enabled && settings.Oni.Master
		guide := settings.Oni.Inspect && settings.Oni.Guide.Enabled
		want := map[string]bool{
			Guide:      guide,
			GuideKeep:  guide && settings.Oni.Guide.Keep,
			GuideHints: guide && settings.Oni.Guide.Keep && settings.Oni.Guide.Hints,
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
				t.Errorf("bits %06b: Decide(%s) = %+v, want reason %q", bits, id, d, reason)
			}
		}
		if Decide(Request{Feature: Inspect}, s).Allow != (on && settings.Oni.Inspect) {
			t.Errorf("bits %06b: a guide switch changed vision.inspect", bits)
		}
	}
}

// The guide adds three rows before the hands rows; the rows Phase 11
// shipped keep their bytes and their order but for hands.browser, which
// Phase 14 makes guarded. The reflex rows Phase 13 adds sit before the
// guide's, and the Phase 16 rows after them.
func TestCapabilitiesWithTheGuideOffAreTheOldRowsInTheOldOrder(t *testing.T) {
	inspectOn := config.Defaults()
	inspectOn.Enabled, inspectOn.Oni.Master, inspectOn.Oni.Inspect = true, true, true
	for _, c := range []struct {
		settings protocol.Settings
		before   string
		reason   string
	}{
		{config.Defaults(), phase11DefaultCapabilities, ReasonOniOff},
		{inspectOn, phase11InspectOnCapabilities, ReasonFeatureOff},
	} {
		got, err := json.Marshal(Capabilities(protocol.EffectiveOni(c.settings)))
		if err != nil {
			t.Fatal(err)
		}
		if want := withPhase16Rows(withHandsRow(withReflexRows(withGuideRows(c.before, c.reason), c.reason), c.reason), c.reason); string(got) != want {
			t.Fatalf("capabilities\n%s\nwant\n%s", got, want)
		}
		old := strings.Replace(withoutPhase16Rows(string(got), c.reason), reflexRows(c.reason), "", 1)
		if old = withoutHandsRow(strings.Replace(old, guideRows(c.reason), "", 1), c.reason); old != c.before {
			t.Fatalf("without the guide and reflex rows\n%s\nwant\n%s", old, c.before)
		}
	}
}

func TestGuideConsentAsksAsItsClassSays(t *testing.T) {
	s := everythingOn()
	s.Disclosure = protocol.OniDisclosureSession
	if !MustAsk(Guide, s, false) || MustAsk(Guide, s, true) {
		t.Error("a guide's picture does not follow once-a-session disclosure")
	}
	s.Disclosure = protocol.OniDisclosureAlways
	if !MustAsk(Guide, s, true) {
		t.Error("a guide skipped asking with disclosure always")
	}
	if !MustAsk(GuideKeep, s, true) {
		t.Error("keeping a guide did not ask each time")
	}
	if MustAsk(GuideHints, s, false) {
		t.Error("a hint asks instead of announcing itself")
	}
}

func TestKeepOfferIsLocalAndLapsesWithTheChip(t *testing.T) {
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Duration(sp.Guide.Keep.ChipS * float64(time.Second)); KeepOfferTTL() != want {
		t.Fatalf("keep offer TTL %v, spec %v", KeepOfferTTL(), want)
	}
	if KeepOfferTTL() <= 0 || KeepOfferTTL() > ConsentTTL {
		t.Fatalf("keep offer TTL %v is not within the consent TTL %v", KeepOfferTTL(), ConsentTTL)
	}
	offer := KeepOffer("keep-1", "Keep these steps for another time?")
	out, err := EncodeFrame(offer)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"kind":"consent","data":{"id":"keep-1","feature":"guide.keep","class":"per-action","destination":"local","question":"Keep these steps for another time?","expiresMs":12000}}`; string(out) != want {
		t.Fatalf("keep offer %s\nwant %s", out, want)
	}
	if p, err := DecodeFrame(out); err != nil || !reflect.DeepEqual(p, offer) {
		t.Fatalf("keep offer decoded as %#v, %v", p, err)
	}
	for name, edit := range map[string]func(*ConsentFrame){
		"hosted":        func(f *ConsentFrame) { f.Destination, f.Route = "hosted", "openrouter/x" },
		"local route":   func(f *ConsentFrame) { f.Route = "rashin-models/gemma" },
		"past the chip": func(f *ConsentFrame) { f.ExpiresMs = KeepOfferTTL().Milliseconds() + 1 },
		"disclosure":    func(f *ConsentFrame) { f.Class = ConsentDisclosure },
		"no question":   func(f *ConsentFrame) { f.Question = "" },
	} {
		f := KeepOffer("keep-1", "Keep these steps for another time?")
		edit(f)
		if out, err := EncodeFrame(f); err == nil {
			t.Errorf("%s: encoded %s", name, out)
		}
	}
}

// A hint rides on an announced guide.hints consent frame whose id oni.skip
// answers; it stays local and retracts with the spec's guide.hints.retractS.
func TestHintOfferIsLocalAndRetractsOnTime(t *testing.T) {
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Duration(sp.Guide.Hints.RetractS * float64(time.Second)); HintOfferTTL() != want || want > ConsentTTL {
		t.Fatalf("hint TTL %v, spec %v, consent TTL %v", HintOfferTTL(), want, ConsentTTL)
	}
	offer := HintOffer("hint-1", "Save this file the way you did before?")
	out, err := EncodeFrame(offer)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"kind":"consent","data":{"id":"hint-1","feature":"guide.hints","class":"announced","destination":"local","question":"Save this file the way you did before?","expiresMs":45000}}`; string(out) != want {
		t.Fatalf("hint offer %s\nwant %s", out, want)
	}
	if MustAsk(GuideHints, everythingOn(), false) {
		t.Fatal("a hint asks instead of announcing itself")
	}
	for name, edit := range map[string]func(*ConsentFrame){
		"hosted":          func(f *ConsentFrame) { f.Destination, f.Route = "hosted", "openrouter/x" },
		"local route":     func(f *ConsentFrame) { f.Route = "rashin-models/gemma" },
		"past retracting": func(f *ConsentFrame) { f.ExpiresMs = HintOfferTTL().Milliseconds() + 1 },
		"per-action":      func(f *ConsentFrame) { f.Class = ConsentPerAction },
	} {
		f := HintOffer("hint-1", "Again?")
		edit(f)
		if out, err := EncodeFrame(f); err == nil {
			t.Errorf("%s: encoded %s", name, out)
		}
	}
}

// A guide frame has no field a picture, screen text, a title or the user's
// words could ride on.
func TestGuideFramesHaveNoFieldForPicturesTitlesOrWords(t *testing.T) {
	names := func(v any) []string {
		var out []string
		rt := reflect.TypeOf(v)
		for i := range rt.NumField() {
			name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
			out = append(out, name)
		}
		return out
	}
	if got, want := names(GuideFrame{}), []string{"seq", "phase", "step", "kicker", "text", "plan", "actions", "replay", "target"}; !slices.Equal(got, want) {
		t.Fatalf("guide frame fields %q, want %q", got, want)
	}
	if got, want := names(GuidePlan{}), []string{"done", "current", "next"}; !slices.Equal(got, want) {
		t.Fatalf("guide plan fields %q, want %q", got, want)
	}
	for _, raw := range []string{
		`{"kind":"guide","data":{"seq":1,"phase":"pointing","step":1,"text":"Open File.","target":2,"title":"Notes"}}`,
		`{"kind":"guide","data":{"seq":1,"phase":"reading","step":1,"picture":"AAAA"}}`,
		`{"kind":"guide","data":{"seq":1,"phase":"reading","step":1,"goal":"save this file"}}`,
		`{"kind":"guide","data":{"seq":1,"phase":"pointing","step":1,"text":"Open File.","target":2,"words":["Save"]}}`,
	} {
		if p, err := DecodeFrame([]byte(raw)); err == nil {
			t.Errorf("%s decoded as %#v", raw, p)
		}
	}
}

func TestGuideFramesFollowTheirPhaseRules(t *testing.T) {
	long := strings.Repeat("x", MaxGuideText+1)
	plan := func(done, next int) *GuidePlan {
		p := &GuidePlan{Current: "Click Save."}
		for range done {
			p.Done = append(p.Done, "Open File.")
		}
		for range next {
			p.Next = append(p.Next, "Name the file.")
		}
		return p
	}
	good := []GuideFrame{
		{Seq: 1, Phase: GuideIdle},
		{Seq: 2, Phase: GuideReading, Step: 1, Kicker: "Step 1", Actions: []string{GuideActionStop}},
		{Seq: 3, Phase: GuidePointing, Step: 1, Kicker: "Step 1", Text: "Open the File menu.", Target: 4, Actions: []string{GuideActionNext, GuideActionStop, GuideActionCircle}},
		{Seq: 4, Phase: GuideWaiting, Step: 2, Text: "Click Save.", Target: 5, Plan: plan(6, 3)},
		{Seq: 5, Phase: GuideExplaining, Step: 2, Kicker: "Step 2 · again", Text: "That did not change the window; try the menu bar.", Actions: []string{GuideActionStop}},
		{Seq: 6, Phase: GuideKeeping, Step: 2, Actions: []string{GuideActionKeep, GuideActionSkip}},
		{Seq: 7, Phase: GuideDone, Step: 2, Text: "All done, just as you kept it.", Replay: true},
		{Seq: 8, Phase: GuideDone, Text: "That is already done here."},
		{Seq: 9, Phase: GuideWinding, Step: 1},
		{Seq: 10, Phase: GuideNudging, Kicker: "Kept from before", Text: "Save this file the way you did before?", Actions: []string{GuideActionNext, GuideActionSkip}},
		{Seq: 11, Phase: GuidePointing, Step: MaxGuideSteps(), Text: "Press Export.", Target: 9},
		{Seq: 12, Phase: GuideExplaining, Step: 3, Text: "Look under Tools.", Target: 9, Actions: []string{GuideActionCircle}},
	}
	for _, f := range good {
		out, err := EncodeFrame(&f)
		if err != nil {
			t.Errorf("%+v: %v", f, err)
			continue
		}
		if p, err := DecodeFrame(out); err != nil || !reflect.DeepEqual(p, &f) {
			t.Errorf("%s decoded as %#v, %v", out, p, err)
		}
	}
	bad := map[string]GuideFrame{
		"seq zero":                   {Phase: GuideIdle},
		"unknown phase":              {Seq: 1, Phase: "clicking", Step: 1},
		"idle with a step":           {Seq: 1, Phase: GuideIdle, Step: 1},
		"idle with a kicker":         {Seq: 1, Phase: GuideIdle, Kicker: "Step 1"},
		"idle with text":             {Seq: 1, Phase: GuideIdle, Text: "Open File."},
		"idle with a plan":           {Seq: 1, Phase: GuideIdle, Plan: plan(1, 0)},
		"idle with actions":          {Seq: 1, Phase: GuideIdle, Actions: []string{GuideActionStop}},
		"idle replaying":             {Seq: 1, Phase: GuideIdle, Replay: true},
		"idle with a target":         {Seq: 1, Phase: GuideIdle, Target: 3},
		"reading with text":          {Seq: 1, Phase: GuideReading, Step: 1, Text: "Save notes"},
		"keeping with text":          {Seq: 1, Phase: GuideKeeping, Step: 2, Text: "Keep it?"},
		"winding with text":          {Seq: 1, Phase: GuideWinding, Step: 1, Text: "Stopped."},
		"pointing without text":      {Seq: 1, Phase: GuidePointing, Step: 1, Target: 2},
		"pointing without target":    {Seq: 1, Phase: GuidePointing, Step: 1, Text: "Open File."},
		"waiting without target":     {Seq: 1, Phase: GuideWaiting, Step: 1, Text: "Open File."},
		"waiting without text":       {Seq: 1, Phase: GuideWaiting, Step: 1, Target: 2},
		"done without text":          {Seq: 1, Phase: GuideDone, Step: 2},
		"nudging without text":       {Seq: 1, Phase: GuideNudging},
		"reading with a target":      {Seq: 1, Phase: GuideReading, Step: 1, Target: 2},
		"done with a target":         {Seq: 1, Phase: GuideDone, Step: 1, Text: "Done.", Target: 2},
		"reading step zero":          {Seq: 1, Phase: GuideReading},
		"pointing step zero":         {Seq: 1, Phase: GuidePointing, Text: "Open File.", Target: 2},
		"past the step ceiling":      {Seq: 1, Phase: GuidePointing, Step: MaxGuideSteps() + 1, Text: "Open File.", Target: 2},
		"negative step":              {Seq: 1, Phase: GuideDone, Step: -1, Text: "Done."},
		"a hint with a step":         {Seq: 1, Phase: GuideNudging, Step: 1, Text: "Again?"},
		"text too long":              {Seq: 1, Phase: GuidePointing, Step: 1, Text: long, Target: 2},
		"text on two lines":          {Seq: 1, Phase: GuidePointing, Step: 1, Text: "Open File.\nThen Save.", Target: 2},
		"blank text":                 {Seq: 1, Phase: GuidePointing, Step: 1, Text: "   ", Target: 2},
		"hidden runes":               {Seq: 1, Phase: GuidePointing, Step: 1, Text: "Open\u200b File.", Target: 2},
		"kicker too long":            {Seq: 1, Phase: GuideReading, Step: 1, Kicker: strings.Repeat("k", MaxGuideKicker+1)},
		"too many done steps":        {Seq: 1, Phase: GuideWaiting, Step: 1, Text: "Click Save.", Target: 2, Plan: plan(7, 0)},
		"too many next steps":        {Seq: 1, Phase: GuideWaiting, Step: 1, Text: "Click Save.", Target: 2, Plan: plan(0, 4)},
		"empty plan":                 {Seq: 1, Phase: GuideWaiting, Step: 1, Text: "Click Save.", Target: 2, Plan: &GuidePlan{}},
		"empty done list":            {Seq: 1, Phase: GuideWaiting, Step: 1, Text: "Click Save.", Target: 2, Plan: &GuidePlan{Done: []string{}, Current: "Click Save."}},
		"plan step too long":         {Seq: 1, Phase: GuideWaiting, Step: 1, Text: "Click Save.", Target: 2, Plan: &GuidePlan{Next: []string{long}}},
		"plan step blank":            {Seq: 1, Phase: GuideWaiting, Step: 1, Text: "Click Save.", Target: 2, Plan: &GuidePlan{Done: []string{""}}},
		"winding with a plan":        {Seq: 1, Phase: GuideWinding, Step: 1, Plan: plan(1, 0)},
		"a hint with a plan":         {Seq: 1, Phase: GuideNudging, Text: "Again?", Plan: plan(1, 0)},
		"unknown action":             {Seq: 1, Phase: GuidePointing, Step: 1, Text: "Open File.", Target: 2, Actions: []string{"click"}},
		"keep while pointing":        {Seq: 1, Phase: GuidePointing, Step: 1, Text: "Open File.", Target: 2, Actions: []string{GuideActionKeep}},
		"next while reading":         {Seq: 1, Phase: GuideReading, Step: 1, Actions: []string{GuideActionNext}},
		"actions when done":          {Seq: 1, Phase: GuideDone, Step: 1, Text: "Done.", Actions: []string{GuideActionStop}},
		"actions while winding":      {Seq: 1, Phase: GuideWinding, Step: 1, Actions: []string{GuideActionStop}},
		"an action twice":            {Seq: 1, Phase: GuidePointing, Step: 1, Text: "Open File.", Target: 2, Actions: []string{GuideActionNext, GuideActionNext}},
		"empty action list":          {Seq: 1, Phase: GuidePointing, Step: 1, Text: "Open File.", Target: 2, Actions: []string{}},
		"circling with no target":    {Seq: 1, Phase: GuideExplaining, Step: 1, Text: "Look under Tools.", Actions: []string{GuideActionCircle}},
		"keeping offers next":        {Seq: 1, Phase: GuideKeeping, Step: 1, Actions: []string{GuideActionNext}},
		"a hint offers keep":         {Seq: 1, Phase: GuideNudging, Text: "Again?", Actions: []string{GuideActionKeep}},
		"keeping without a step":     {Seq: 1, Phase: GuideKeeping},
		"explaining without a step":  {Seq: 1, Phase: GuideExplaining, Text: "Look under Tools."},
		"a hint pointing somewhere":  {Seq: 1, Phase: GuideNudging, Text: "Again?", Target: 2},
		"winding pointing somewhere": {Seq: 1, Phase: GuideWinding, Step: 1, Target: 2},
	}
	for name, f := range bad {
		if out, err := EncodeFrame(&f); err == nil {
			t.Errorf("%s: encoded %s", name, out)
		}
	}
}

func TestGuideCallsDecodeStrictly(t *testing.T) {
	good := []struct {
		method, raw string
		want        any
	}{
		{"oni.guide", `{"action":"start","goal":"save this file"}`, &protocol.OniGuideParams{Action: "start", Goal: "save this file"}},
		{"oni.guide", `{"action":"next"}`, &protocol.OniGuideParams{Action: "next"}},
		{"oni.guide", `{"action":"stop"}`, &protocol.OniGuideParams{Action: "stop"}},
		{"oni.guide", `{"action":"circle"}`, &protocol.OniGuideParams{Action: "circle"}},
		{"oni.guide", `{"action":"start","goal":"` + strings.Repeat("g", protocol.OniGuideGoalMax) + `"}`, &protocol.OniGuideParams{Action: "start", Goal: strings.Repeat("g", protocol.OniGuideGoalMax)}},
		{"oni.keep", `{"id":"keep-1"}`, &protocol.OniKeepParams{ID: "keep-1"}},
		{"oni.skip", `{"id":"oni-consent-7"}`, &protocol.OniKeepParams{ID: "oni-consent-7"}},
		{"oni.pick", `{"action":"arm"}`, &protocol.OniPickParams{Action: "arm"}},
		// an empty question is no question, as absent
		{"oni.pick", `{"action":"arm","text":""}`, &protocol.OniPickParams{Action: "arm"}},
		{"oni.pick", `{"action":"arm","text":"What does this do?"}`, &protocol.OniPickParams{Action: "arm", Text: "What does this do?"}},
		{"oni.pick", `{"action":"point","output":"DP-1","x":812.5,"y":40}`, &protocol.OniPickParams{Action: "point", Output: "DP-1", X: 812.5, Y: 40}},
		{"oni.pick", `{"action":"point","output":"HEADLESS-2"}`, &protocol.OniPickParams{Action: "point", Output: "HEADLESS-2"}},
		{"oni.pick", `{"action":"cancel"}`, &protocol.OniPickParams{Action: "cancel"}},
	}
	for _, c := range good {
		v, err := DecodeCall(c.method, json.RawMessage(c.raw))
		if err != nil || !reflect.DeepEqual(v, c.want) {
			t.Errorf("%s %s decoded as %#v, %v", c.method, c.raw, v, err)
		}
	}
	bad := [][2]string{
		{"oni.guide", ``},
		{"oni.guide", `{}`},
		{"oni.guide", `{"action":"start"}`},
		{"oni.guide", `{"action":"start","goal":""}`},
		{"oni.guide", `{"action":"start","goal":"   "}`},
		{"oni.guide", `{"action":"start","goal":"save\nthis"}`},
		{"oni.guide", `{"action":"start","goal":"save\u202ethis"}`},
		{"oni.guide", `{"action":"start","goal":"` + strings.Repeat("g", protocol.OniGuideGoalMax+1) + `"}`},
		{"oni.guide", `{"action":"next","goal":"save this file"}`},
		{"oni.guide", `{"action":"click"}`},
		{"oni.guide", `{"action":"keep"}`},
		{"oni.guide", `{"Action":"next"}`},
		{"oni.guide", `{"action":"next","action":"stop"}`},
		{"oni.guide", `{"action":"next","step":2}`},
		{"oni.guide", `{"action":null}`},
		{"oni.guide", `{"action":"next"} {}`},
		{"oni.keep", `{}`},
		{"oni.keep", ``},
		{"oni.keep", `{"id":""}`},
		{"oni.keep", `{"id":"Keep 1"}`},
		{"oni.keep", `{"id":"keep-1","answer":"yes"}`},
		{"oni.keep", `{"id":"` + strings.Repeat("k", 65) + `"}`},
		{"oni.skip", `{"id":"../keeps"}`},
		{"oni.skip", `{"id":1}`},
		{"oni.pick", ``},
		{"oni.pick", `{}`},
		{"oni.pick", `{"action":"click"}`},
		{"oni.pick", `{"action":"arm","output":"DP-1"}`},
		{"oni.pick", `{"action":"arm","x":4}`},
		{"oni.pick", `{"action":"arm","text":"  "}`},
		{"oni.pick", `{"action":"arm","text":"what\u200b"}`},
		{"oni.pick", `{"action":"arm","text":"` + strings.Repeat("q", protocol.OniPickTextMax+1) + `"}`},
		{"oni.pick", `{"action":"point"}`},
		{"oni.pick", `{"action":"point","x":4,"y":4}`},
		{"oni.pick", `{"action":"point","output":"DP-1","x":-1,"y":4}`},
		{"oni.pick", `{"action":"point","output":"DP-1","x":4,"y":70000}`},
		{"oni.pick", `{"action":"point","output":"DP-1\u200b","x":4,"y":4}`},
		{"oni.pick", `{"action":"point","output":"DP-1","x":4,"y":4,"text":"what is this?"}`},
		{"oni.pick", `{"action":"point","output":"DP-1","x":"4","y":4}`},
		{"oni.pick", `{"action":"point","output":"DP-1","window":"21"}`},
		{"oni.pick", `{"action":"cancel","output":"DP-1"}`},
		{"oni.pick", `{"action":"cancel","text":"never mind"}`},
		{"oni.pick", `{"action":"cancel","x":1}`},
	}
	for _, c := range bad {
		if v, err := DecodeCall(c[0], json.RawMessage(c[1])); err == nil {
			t.Errorf("%s %s decoded as %#v", c[0], c[1], v)
		}
	}
	// the decoder rejects what JSON cannot carry, and validate rejects the
	// rest of the non-finite clicks
	if err := (pickCall{Action: protocol.OniPickPoint, Output: "DP-1", X: math.Inf(1)}).validate(); err == nil {
		t.Error("a click at infinity passed")
	}
	if err := (pickCall{Action: protocol.OniPickPoint, Output: "DP-1", Y: math.NaN()}).validate(); err == nil {
		t.Error("a click at NaN passed")
	}
}
