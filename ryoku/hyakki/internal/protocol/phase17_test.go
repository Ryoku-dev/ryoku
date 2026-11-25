package protocol

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// The bridge's tool ids, program names, component ids, activity stores and
// pill lines take what they say and refuse the rest.
func TestPhase17ValuesValidate(t *testing.T) {
	if len(OniMCPTools) != 7 || slices.Contains(OniMCPTools, "tasks_list") || slices.Contains(OniMCPTools, "habits_list") {
		t.Fatalf("the bridge's tools are %q", OniMCPTools)
	}
	for _, name := range []string{"Claude Code", "zed", "codex-cli 1.2", strings.Repeat("a", OniMCPClientNameMax)} {
		if !ValidOniMCPClientName(name) {
			t.Errorf("%q refused", name)
		}
	}
	for _, name := range []string{"", " Claude", "Claude ", "two\nlines", "tab\there", strings.Repeat("a", OniMCPClientNameMax+1)} {
		if ValidOniMCPClientName(name) {
			t.Errorf("%q passed", name)
		}
	}
	if got := CleanOniMCPTools(map[string]bool{OniMCPNotify: true, OniMCPScreenAsk: false, "tasks_add": true}); !reflect.DeepEqual(got, map[string]bool{OniMCPNotify: true}) {
		t.Errorf("cleaned tools %v", got)
	}
	if got := CleanOniMCPTools(map[string]bool{OniMCPNotify: false}); got != nil {
		t.Errorf("no tool on cleaned to %v", got)
	}
	if got, ok := CleanOniMCPClients([]string{"Zed", "Claude Code", "Zed"}); !ok || !slices.Equal(got, []string{"Zed", "Claude Code"}) {
		t.Errorf("cleaned clients %q %v", got, ok)
	}
	if _, ok := CleanOniMCPClients([]string{"Zed", " bad"}); ok {
		t.Error("an invalid program name passed")
	}
	many := make([]string, OniMCPClientsMax+1)
	for i := range many {
		many[i] = strings.Repeat("p", i+1)
	}
	if _, ok := CleanOniMCPClients(many); ok {
		t.Error("too many programs passed")
	}
	for _, id := range []string{"stt:base.en", "tts:kokoro", "hands:profile", "program:wl-paste", "recall:chats", "model:ggml-base_en:q5"} {
		if !ValidComponentID(id) {
			t.Errorf("%q refused", id)
		}
	}
	for _, id := range []string{"", "stt", "stt:", ":x", "STT:x", "stt:Base", "stt:../x", "stt:-x", "stt:.x", "stt:a/b", "stt:" + strings.Repeat("a", ComponentIDMax)} {
		if ValidComponentID(id) {
			t.Errorf("%q passed", id)
		}
	}
	for _, stores := range [][]string{{OniActivityAll}, {OniActivityReflexLog}, OniActivityStores} {
		if !ValidOniActivityStores(stores) {
			t.Errorf("%q refused", stores)
		}
	}
	for _, stores := range [][]string{nil, {}, {"memory"}, {OniActivityAll, OniActivityRoutines}, {OniActivityRoutines, OniActivityRoutines}, append(slices.Clone(OniActivityStores), OniActivityAll)} {
		if ValidOniActivityStores(stores) {
			t.Errorf("%q passed", stores)
		}
	}
	if !ValidOniNotifyText("Build finished") || ValidOniNotifyText("") || ValidOniNotifyText("a\nb") || ValidOniNotifyText(strings.Repeat("a", OniNotifyMax+1)) {
		t.Error("pill lines validate wrongly")
	}
}

// A machine that never changed a setting has the bridge off with no tool
// and no program, and the tour unseen; the wire spells them so.
func TestPhase17SettingsAreOffByDefaultOnTheWire(t *testing.T) {
	var s Settings
	s.Oni = DefaultOniSettings()
	b, err := json.Marshal(struct {
		M OniMCPSettings     `json:"mcp"`
		O OnboardingSettings `json:"onboarding"`
	}{s.Oni.MCP, s.Onboarding})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"mcp":{"enabled":false},"onboarding":{"seen":false,"done":false}}`; string(b) != want {
		t.Fatalf("the blocks are %s", b)
	}
	whole, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(whole), `"mcp":{"enabled":false}`) || !strings.HasSuffix(string(whole), `,"onboarding":{"seen":false,"done":false}}`) {
		t.Fatalf("settings on the wire: %s", whole)
	}
	// a reminder that is no add carries no due time on the wire
	if b, _ := json.Marshal(OniRemindersParams{Action: OniRemindersList}); string(b) != `{"action":"list"}` {
		t.Fatalf("a list is %s", b)
	}
}

// Every Phase 17 call is registered with its params and result.
func TestPhase17CallsAreRegistered(t *testing.T) {
	for method, types := range map[string][2]any{
		MComponents:       {Empty{}, ComponentsResult{}},
		MComponentRemove:  {ComponentRemoveParams{}, ComponentsResult{}},
		MOniActivity:      {Empty{}, OniActivityResult{}},
		MOniActivityClear: {OniActivityClearParams{}, OniActivityResult{}},
		MOniNotify:        {OniNotifyParams{}, Empty{}},
		MOniMCP:           {Empty{}, OniMCPStatus{}},
	} {
		m, ok := Lookup(method)
		if !ok || m.Params != reflect.TypeOf(types[0]) || m.Result != reflect.TypeOf(types[1]) || m.Doc == "" {
			t.Errorf("%s is registered as %+v %v", method, m, ok)
		}
	}
}

func TestOniActivityPartialCountsAndErrorsOnTheWire(t *testing.T) {
	partial := OniActivityResult{
		Counts: map[string]int{OniActivityReminders: 0, OniActivityRoutines: 2},
		Errors: map[string]string{OniActivityWalkthroughs: "Records could not be read."},
		Rows:   []OniActivityRow{},
	}
	wire, err := json.Marshal(partial)
	if err != nil {
		t.Fatal(err)
	}
	var got OniActivityResult
	if err := json.Unmarshal(wire, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, partial) {
		t.Fatalf("partial activity result did not round trip: %s", wire)
	}
	if _, fabricatedZero := got.Counts[OniActivityWalkthroughs]; fabricatedZero {
		t.Fatal("an unreadable store acquired a zero count")
	}
	whole, err := json.Marshal(OniActivityResult{Counts: map[string]int{OniActivityReminders: 0}, Rows: []OniActivityRow{}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(whole), `"errors"`) || !strings.Contains(string(whole), `"reminders":0`) {
		t.Fatalf("healthy zero/optional errors changed meaning: %s", whole)
	}
}
