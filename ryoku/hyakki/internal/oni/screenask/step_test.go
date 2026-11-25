package screenask

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/oni/feature"
)

// stepControls adds a field whose control label names a PIN.
var stepControls = slices.Concat(testControls, []Control{
	{ID: "5", Role: "text", Name: "Account PIN", Box: feature.Box{X: 0.3, Y: 0.7, W: 0.3, H: 0.05}, Verified: true},
})

// pictureWords are words read in the picture, one of them a planted
// instruction.
var pictureWords = append(slices.Clone(testWords), Word{Text: injection, Box: feature.Box{X: 0.1, Y: 0.9, W: 0.6, H: 0.03}})

func TestParseStepAccepts(t *testing.T) {
	cases := []struct {
		name, reply string
		kind, say   string
		done        bool
		plan        []PlanItem
	}{
		{name: "valid step with a plan",
			reply: "Open the File menu, top left.\n[POINT:1][CONF:0.9]\n[NEXT:Choose Save (Save) | Name the file (File name)]",
			kind:  KindElement, say: "Open the File menu, top left.",
			plan: []PlanItem{{"Choose Save", "Save"}, {"Name the file", "File name"}}},
		{name: "exactly twelve words",
			reply: "Click Save in the toolbar at the very top of the window.\n[POINT:1][CONF:0.8]",
			kind:  KindElement, say: "Click Save in the toolbar at the very top of the window."},
		{name: "a plan of more than three keeps the first three",
			reply: "Open File.\n[POINT:1][CONF:0.9][NEXT:One (A) | Two (B) | Three (C) | Four (D) | Five (E)]",
			kind:  KindElement, say: "Open File.",
			plan: []PlanItem{{"One", "A"}, {"Two", "B"}, {"Three", "C"}}},
		{name: "plan steps without a control",
			reply: "Open File.\n[POINT:1][CONF:0.9][NEXT:Choose Save |  | Close the dialog]",
			kind:  KindElement, say: "Open File.",
			plan: []PlanItem{{"Choose Save", ""}, {"Close the dialog", ""}}},
		{name: "plan step naming a private field is left out",
			reply: "Open Security settings.\n[POINT:3][CONF:0.9][NEXT:Type the new PIN (Account PIN) | Enter the card number | Press Apply (Apply)]",
			kind:  KindElement, say: "Open Security settings.",
			plan: []PlanItem{{"Press Apply", "Apply"}}},
		{name: "hidden runes leave the sentence and the plan",
			reply: "Open\u202e the \x1bFile\u200b menu.\n[POINT:1][CONF:0.9][NEXT:Ch\u2066oose **Save** (Sa\u200bve\x07)]",
			kind:  KindElement, say: "Open the File menu.",
			plan: []PlanItem{{"Choose Save", "Save"}}},
		{name: "answer",
			reply: "[ANSWER]", kind: KindAnswer},
		{name: "goal already reached",
			reply: "That file is already saved.\n[POINT:none][DONE]",
			kind:  KindNone, say: "That file is already saved.", done: true},
		{name: "planted instruction in the picture does not change a sound step",
			reply: "Export is top right.\n[POINT:0.69,0.09,0.09,0.05:Export][CONF:0.8]",
			kind:  KindBox, say: "Export is top right."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := ParseStep(c.reply, stepControls, pictureWords, testPic)
			if err != nil {
				t.Fatalf("ParseStep: %v", err)
			}
			if p.Kind != c.kind || p.Say != c.say || p.Done != c.done {
				t.Fatalf("got %+v", p)
			}
			if !slices.Equal(p.Plan, c.plan) {
				t.Fatalf("plan %+v, want %+v", p.Plan, c.plan)
			}
			for _, item := range p.Plan {
				for _, s := range []string{item.Text, item.Control} {
					for _, r := range s {
						if hidden(r) || r == '*' {
							t.Fatalf("plan %q kept %U", s, r)
						}
					}
				}
			}
		})
	}
}

func TestParseStepRefuses(t *testing.T) {
	cases := []struct {
		name, reply, code string
	}{
		{"thirteen words", "Click the Save button in the toolbar along the upper edge right here.\n[POINT:1][CONF:0.9]", CodeMalformed},
		{"two sentences past twelve words", "Open the File menu. Then choose Save and name the file before you close.\n[POINT:1][CONF:0.9]", CodeMalformed},
		{"no sentence", "[POINT:1][CONF:0.9]", CodeMalformed},
		{"only hidden runes for a sentence", "\u200b\u202e\n[POINT:1][CONF:0.9]", CodeMalformed},
		{"two plans", "Open File.\n[POINT:1][CONF:0.9][NEXT:Choose Save (Save)][NEXT:Close (Close)]", CodeMalformed},
		{"no point", "Open the File menu.", CodeMalformed},
		{"action tag", "Saving it for you.\n[CLICK:1]\n[POINT:1][CONF:0.9]", CodeAction},
		{"action tag beside a plan", "Open File.\n[POINT:1][CONF:0.9][NEXT:Choose Save (Save)][do:type hunter2]", CodeAction},
		{"action tag inside a plan", "Open File.\n[POINT:1][CONF:0.9][NEXT:Choose Save | [TYPE:hunter2] (File name)]", CodeAction},
		{"planted instruction echoed from the picture", injection + "\n[POINT:1][CONF:0.9]", CodeAction},
		{"control label naming a PIN by id", "Type it here.\n[POINT:5][CONF:0.95]", CodeSensitive},
		{"control label naming a PIN in a box", "Type it here.\n[POINT:0.3,0.7,0.3,0.05:PIN][CONF:0.95]", CodeSensitive},
		{"private field wins over a long sentence", "Type your secret number into this field right here in the middle of the form.\n[POINT:5][CONF:0.95]", CodeSensitive},
		{"low confidence", "Open File.\n[POINT:3][CONF:0.1]", CodeLowConfidence},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := ParseStep(c.reply, stepControls, pictureWords, testPic)
			var r *Refusal
			if !errors.As(err, &r) {
				t.Fatalf("got %+v, %v; want refusal %s", p, err, c.code)
			}
			if r.Code != c.code {
				t.Fatalf("code %s (%s), want %s", r.Code, r.Reason, c.code)
			}
			if r.Line == "" {
				t.Fatal("refusal has no line to say")
			}
			if p.Kind != "" || p.Box != nil || p.Plan != nil || p.Say != "" {
				t.Fatalf("refusal still returned a step: %+v", p)
			}
		})
	}
}

// The step bounds belong to the guide: a plain point keeps reading a long
// sentence and a second plan as it always did.
func TestParsePointKeepsItsOldLeniency(t *testing.T) {
	long := "Click the Save button in the toolbar at the very top of the window.\n[POINT:1][CONF:0.9][NEXT:Choose Save (Save)][NEXT:Close (Close)]"
	p, err := ParsePoint(long, testControls, testWords, testPic)
	if err != nil {
		t.Fatal(err)
	}
	if len(strings.Fields(p.Say)) <= MaxStepWords || !slices.Equal(p.Plan, []PlanItem{{"Choose Save", "Save"}}) {
		t.Fatalf("got %+v", p)
	}
}
