package proto

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/oni/hands/consent"
)

var listed = Offer{Controls: []string{"c1", "c2", "c3"}}

func TestParseAcceptsEachTool(t *testing.T) {
	cases := []struct {
		raw  string
		want Response
	}{
		{`{"type":"tool_call","tool":"click","args":{"id":"c1"},"confirm":false,"permission":"normal"}`,
			Response{Type: TypeToolCall, Tool: ToolClick, Args: Args{ID: "c1"}, Permission: "normal"}},
		{`{"type":"tool_call","tool":"fill","args":{"id":"c2","text":"Ada  Lovelace\n"},"confirm":false,"permission":"normal"}`,
			Response{Type: TypeToolCall, Tool: ToolFill, Args: Args{ID: "c2", Text: "Ada  Lovelace\n"}, Permission: "normal"}},
		{`{"type":"tool_call","tool":"fill","args":{"id":"c2","text":""},"confirm":false,"permission":"normal"}`,
			Response{Type: TypeToolCall, Tool: ToolFill, Args: Args{ID: "c2"}, Permission: "normal"}},
		{`{"type":"tool_call","tool":"select","args":{"id":"c3","option":"Japan"},"confirm":true,"permission":"sensitive"}`,
			Response{Type: TypeToolCall, Tool: ToolSelect, Args: Args{ID: "c3", Option: "Japan"}, Confirm: true, Permission: "sensitive"}},
		{`{"type":"tool_call","tool":"scroll","args":{"dir":"down"},"confirm":false,"permission":"normal"}`,
			Response{Type: TypeToolCall, Tool: ToolScroll, Args: Args{Dir: "down"}, Permission: "normal"}},
		{`{"type":"tool_call","tool":"scroll","args":{"dir":"up","id":"c1"},"confirm":false,"permission":"normal"}`,
			Response{Type: TypeToolCall, Tool: ToolScroll, Args: Args{Dir: "up", ID: "c1"}, Permission: "normal"}},
		{`{"type":"tool_call","tool":"wait","args":{"ms":500},"confirm":false,"permission":"normal"}`,
			Response{Type: TypeToolCall, Tool: ToolWait, Args: Args{Ms: 500}, Permission: "normal"}},
		{`{"type":"tool_call","tool":"done","args":{"text":"The form\u200b is\tfilled."},"confirm":false,"permission":"normal"}`,
			Response{Type: TypeToolCall, Tool: ToolDone, Args: Args{Text: "The form is filled."}, Permission: "normal"}},
		{`{"type":"tool_call","tool":"ask_user","args":{"text":"Which address?"},"confirm":true,"permission":"critical"}`,
			Response{Type: TypeToolCall, Tool: ToolAskUser, Args: Args{Text: "Which address?"}, Confirm: true, Permission: "critical"}},
		{`{"type":"response","text":"Working on it."}`,
			Response{Type: TypeResponse, Text: "Working on it.", Permission: "normal"}},
		{"  \n```json\n{\"type\":\"response\",\"text\":\"Hi.\"}\n```\n",
			Response{Type: TypeResponse, Text: "Hi.", Permission: "normal"}},
		{"```\n{\"type\":\"response\",\"text\":\"Hi.\"}\n```",
			Response{Type: TypeResponse, Text: "Hi.", Permission: "normal"}},
	}
	for _, tc := range cases {
		got, err := Parse(tc.raw, listed)
		if err != nil {
			t.Errorf("Parse(%s): %v", tc.raw, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Parse(%s) = %+v, want %+v", tc.raw, got, tc.want)
		}
	}
}

func TestParseRefusals(t *testing.T) {
	ok := `{"type":"tool_call","tool":"click","args":{"id":"c1"},"confirm":false,"permission":"normal"}`
	cases := []struct {
		name string
		raw  string
		want error
	}{
		{"empty", "", ErrJSON},
		{"blank", "  \n ", ErrJSON},
		{"prose", "Sure, I will click Send.", ErrJSON},
		{"prose before", "Here you go: " + ok, ErrJSON},
		{"broken", `{"type":"tool_call","tool":"click"`, ErrJSON},
		{"single quotes", `{'type':'response','text':'hi'}`, ErrJSON},
		{"bad utf8", "{\"type\":\"response\",\"text\":\"\xff\"}", ErrJSON},
		{"legacy tag", "[DO:CLICK c1]", ErrMultiple},
		{"array", "[" + ok + "]", ErrMultiple},
		{"two objects", ok + ok, ErrMultiple},
		{"two objects spaced", ok + "\n\n" + ok, ErrMultiple},
		{"object then tag", ok + " [CLICK:c2]", ErrMultiple},
		{"object then prose", ok + " Done!", ErrMultiple},
		{"key twice", `{"type":"tool_call","tool":"click","tool":"fill","args":{"id":"c1"},"confirm":false,"permission":"normal"}`, ErrMultiple},
		{"arg twice", `{"type":"tool_call","tool":"click","args":{"id":"c1","id":"c2"},"confirm":false,"permission":"normal"}`, ErrMultiple},
		{"invented tool", `{"type":"tool_call","tool":"submit","args":{"id":"c1"},"confirm":false,"permission":"normal"}`, ErrInvented},
		{"invented key tool", `{"type":"tool_call","tool":"press_key","args":{"id":"c1"},"confirm":false,"permission":"normal"}`, ErrInvented},
		{"tool case", `{"type":"tool_call","tool":"Click","args":{"id":"c1"},"confirm":false,"permission":"normal"}`, ErrInvented},
		{"look is not offered", `{"type":"tool_call","tool":"look","args":{},"confirm":false,"permission":"normal"}`, ErrInvented},
		{"handoff is not a tool", `{"type":"tool_call","tool":"handoff","args":{"id":"c1"},"confirm":false,"permission":"normal"}`, ErrInvented},
		{"plan call", `{"type":"plan_call","plan":"checkout","args":{},"confirm":true,"permission":"critical"}`, ErrInvented},
		{"plan on a tool call", `{"type":"tool_call","tool":"click","plan":"x","args":{"id":"c1"},"confirm":false,"permission":"normal"}`, ErrInvented},
		{"invented type", `{"type":"action","tool":"click","args":{"id":"c1"},"confirm":false,"permission":"normal"}`, ErrInvented},
		{"permission word", `{"type":"tool_call","tool":"click","args":{"id":"c1"},"confirm":false,"permission":"low"}`, ErrPermission},
		{"permission case", `{"type":"tool_call","tool":"click","args":{"id":"c1"},"confirm":false,"permission":"Normal"}`, ErrPermission},
		{"permission missing", `{"type":"tool_call","tool":"click","args":{"id":"c1"},"confirm":false}`, ErrPermission},
		{"permission empty", `{"type":"tool_call","tool":"click","args":{"id":"c1"},"confirm":false,"permission":""}`, ErrPermission},
		{"response permission", `{"type":"response","text":"hi","permission":"safe"}`, ErrPermission},
		{"id not listed", `{"type":"tool_call","tool":"click","args":{"id":"c9"},"confirm":false,"permission":"normal"}`, ErrArgs},
		{"id empty", `{"type":"tool_call","tool":"click","args":{"id":""},"confirm":false,"permission":"normal"}`, ErrArgs},
		{"id as selector", `{"type":"tool_call","tool":"click","args":{"id":"#send"},"confirm":false,"permission":"normal"}`, ErrArgs},
		{"click missing id", `{"type":"tool_call","tool":"click","args":{},"confirm":false,"permission":"normal"}`, ErrArgs},
		{"click with text", `{"type":"tool_call","tool":"click","args":{"id":"c1","text":"x"},"confirm":false,"permission":"normal"}`, ErrArgs},
		{"fill missing text", `{"type":"tool_call","tool":"fill","args":{"id":"c1"},"confirm":false,"permission":"normal"}`, ErrArgs},
		{"fill unlisted", `{"type":"tool_call","tool":"fill","args":{"id":"c7","text":"x"},"confirm":false,"permission":"normal"}`, ErrArgs},
		{"select empty option", `{"type":"tool_call","tool":"select","args":{"id":"c3","option":" "},"confirm":false,"permission":"normal"}`, ErrArgs},
		{"scroll sideways", `{"type":"tool_call","tool":"scroll","args":{"dir":"left"},"confirm":false,"permission":"normal"}`, ErrArgs},
		{"scroll unlisted", `{"type":"tool_call","tool":"scroll","args":{"dir":"up","id":"c8"},"confirm":false,"permission":"normal"}`, ErrArgs},
		{"wait zero", `{"type":"tool_call","tool":"wait","args":{"ms":0},"confirm":false,"permission":"normal"}`, ErrArgs},
		{"wait fraction", `{"type":"tool_call","tool":"wait","args":{"ms":1.5},"confirm":false,"permission":"normal"}`, ErrArgs},
		{"done silent", `{"type":"tool_call","tool":"done","args":{"text":" \u200b "},"confirm":false,"permission":"normal"}`, ErrArgs},
		{"unknown arg", `{"type":"tool_call","tool":"click","args":{"id":"c1","selector":"#x"},"confirm":false,"permission":"normal"}`, ErrArgs},
		{"args missing", `{"type":"tool_call","tool":"click","confirm":false,"permission":"normal"}`, ErrArgs},
		{"args null", `{"type":"tool_call","tool":"click","args":null,"confirm":false,"permission":"normal"}`, ErrArgs},
		{"args list", `{"type":"tool_call","tool":"click","args":["c1"],"confirm":false,"permission":"normal"}`, ErrArgs},
		{"no type", `{"tool":"click","args":{"id":"c1"},"confirm":false,"permission":"normal"}`, ErrShape},
		{"no tool", `{"type":"tool_call","args":{"id":"c1"},"confirm":false,"permission":"normal"}`, ErrShape},
		{"no confirm", `{"type":"tool_call","tool":"click","args":{"id":"c1"},"permission":"normal"}`, ErrShape},
		{"confirm word", `{"type":"tool_call","tool":"click","args":{"id":"c1"},"confirm":"yes","permission":"normal"}`, ErrShape},
		{"unknown field", `{"type":"tool_call","tool":"click","args":{"id":"c1"},"confirm":false,"permission":"normal","then":"send"}`, ErrShape},
		{"tool text", `{"type":"tool_call","tool":"click","text":"hi","args":{"id":"c1"},"confirm":false,"permission":"normal"}`, ErrShape},
		{"response with tool", `{"type":"response","text":"hi","tool":"click"}`, ErrShape},
		{"response with args", `{"type":"response","text":"hi","args":{"id":"c1"}}`, ErrShape},
		{"response silent", `{"type":"response","text":""}`, ErrShape},
		{"not an object", `"click c1"`, ErrJSON},
	}
	for _, tc := range cases {
		got, err := Parse(tc.raw, listed)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: Parse = %+v, %v; want %v", tc.name, got, err, tc.want)
			continue
		}
		if got != (Response{}) {
			t.Errorf("%s: a refused reply returned %+v", tc.name, got)
		}
		line := Corrective(err)
		if strings.Contains(line, "\n") || !strings.Contains(line, "exactly one JSON object") {
			t.Errorf("%s: Corrective = %q", tc.name, line)
		}
		if strings.Contains(line, tc.raw) && tc.raw != "" {
			t.Errorf("%s: Corrective repeats the reply", tc.name)
		}
	}
}

func TestNoControlListMeansNoTarget(t *testing.T) {
	for _, raw := range []string{
		`{"type":"tool_call","tool":"click","args":{"id":"c1"},"confirm":false,"permission":"normal"}`,
		`{"type":"tool_call","tool":"fill","args":{"id":"c1","text":"x"},"confirm":false,"permission":"normal"}`,
	} {
		if _, err := Parse(raw, Offer{}); !errors.Is(err, ErrArgs) {
			t.Errorf("Parse with no controls = %v, want ErrArgs", err)
		}
	}
}

func TestToolSetIsFixed(t *testing.T) {
	want := []string{"click", "fill", "select", "scroll", "wait", "done", "ask_user"}
	if !slices.Equal(Tools, want) {
		t.Fatalf("Tools = %v, want %v", Tools, want)
	}
	for _, v := range []string{consent.VerbClick, consent.VerbFill, consent.VerbSelect, consent.VerbScroll, consent.VerbWait} {
		if !slices.Contains(Tools, v) {
			t.Errorf("page verb %q is not a tool", v)
		}
	}
}

func TestCorrectiveNamesEachFault(t *testing.T) {
	seen := map[string]error{}
	for _, err := range []error{ErrJSON, ErrMultiple, ErrInvented, ErrPermission, ErrArgs, ErrShape} {
		line := Corrective(err)
		if prev, dup := seen[line]; dup {
			t.Errorf("Corrective(%v) == Corrective(%v)", err, prev)
		}
		seen[line] = err
	}
}

// The model's permission and confirm never make a risky step safe: consent
// judges the control, and the claim only raises.
func TestParsedClaimNeverLowersConsent(t *testing.T) {
	r, err := Parse(`{"type":"tool_call","tool":"click","args":{"id":"c1"},"confirm":false,"permission":"normal"}`, listed)
	if err != nil {
		t.Fatal(err)
	}
	send := consent.Subject{Role: "button", Name: "Send"}
	if c, _ := consent.Classify(r.Tool, send, r.Permission); c != consent.Handoff {
		t.Fatalf("a normal, unconfirmed click on Send = %v, want handoff", c)
	}
}
