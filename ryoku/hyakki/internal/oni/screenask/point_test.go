package screenask

import (
	"errors"
	"math"
	"strings"
	"testing"

	"ryoku-rashin/internal/oni/feature"
)

var (
	testPic      = Size{W: 1568, H: 980}
	testControls = []Control{
		{ID: "1", Role: "button", Name: "Save", Box: feature.Box{X: 0.02, Y: 0.02, W: 0.04, H: 0.03}, Verified: true},
		{ID: "2", Role: "password text", Name: "Password", Box: feature.Box{X: 0.3, Y: 0.4, W: 0.3, H: 0.05}},
		{ID: "3", Role: "text", Name: "Search", Box: feature.Box{X: 0.3, Y: 0.1, W: 0.3, H: 0.05}},
		{ID: "4", Role: "text", Name: "", Box: feature.Box{X: 0.3, Y: 0.6, W: 0.3, H: 0.05}},
	}
	testWords = []Word{
		{Text: "Export", Box: feature.Box{X: 0.70, Y: 0.10, W: 0.06, H: 0.03}},
		{Text: "number", Box: feature.Box{X: 0.215, Y: 0.611, W: 0.06, H: 0.03}},
		{Text: "Card", Box: feature.Box{X: 0.15, Y: 0.612, W: 0.05, H: 0.03}},
	}
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-3 }

func TestParsePointAccepts(t *testing.T) {
	cases := []struct {
		name, reply string
		kind, id    string
		box         *feature.Box
		conf        float64
		sure, done  bool
		verified    bool
		say, note   string
	}{
		{name: "element", reply: "Save is the first icon, top left.\n[POINT:1]\n[CONF:0.82]",
			kind: KindElement, id: "1", box: &testControls[0].Box, conf: 0.82, sure: true, verified: true, say: "Save is the first icon, top left."},
		{name: "tags after prose on one line", reply: "Use Save, top left. [POINT:1][CONF:.7][DONE]",
			kind: KindElement, id: "1", box: &testControls[0].Box, conf: 0.7, sure: true, done: true, verified: true, say: "Use Save, top left."},
		{name: "element at its floor is a best guess", reply: "Try Search.\n[POINT:3] [CONF:0.3]",
			kind: KindElement, id: "3", box: &testControls[2].Box, conf: 0.3, say: "Try Search."},
		{name: "lowercase tags", reply: "Save.\n[point:1] [conf:0.9]", kind: KindElement, id: "1", box: &testControls[0].Box, conf: 0.9, sure: true, verified: true, say: "Save."},
		{name: "percent confidence", reply: "Save.\n[POINT:1][CONF:85%]", kind: KindElement, id: "1", box: &testControls[0].Box, conf: 0.85, sure: true, verified: true, say: "Save."},
		{name: "box read text agrees at the verified floor", reply: "Export is top right.\n[POINT:0.69,0.09,0.09,0.05:Export][CONF:0.4]",
			kind: KindBox, box: &feature.Box{X: 0.69, Y: 0.09, W: 0.09, H: 0.05}, conf: 0.4, verified: true, say: "Export is top right."},
		{name: "unverified box at its floor", reply: "Share is top right.\n[POINT:0.85,0.02,0.05,0.04:Share][CONF:0.5]",
			kind: KindBox, box: &feature.Box{X: 0.85, Y: 0.02, W: 0.05, H: 0.04}, conf: 0.5, say: "Share is top right."},
		{name: "box in pixels", reply: "Export.\n[POINT:1082,88,141,49:Export][CONF:0.9]",
			kind: KindBox, box: &feature.Box{X: 1082.0 / 1568, Y: 88.0 / 980, W: 141.0 / 1568, H: 49.0 / 980}, conf: 0.9, sure: true, verified: true, say: "Export."},
		{name: "box with a little slack is held inside", reply: "Close.\n[POINT:0.96,-0.01,0.05,0.04:Close][CONF:0.7]",
			kind: KindBox, box: &feature.Box{X: 0.96, Y: 0, W: 0.04, H: 0.03}, conf: 0.7, sure: true, say: "Close."},
		{name: "area exactly a quarter", reply: "That panel.\n[POINT:0.1,0.1,0.5,0.5:Preview][CONF:0.9]",
			kind: KindBox, box: &feature.Box{X: 0.1, Y: 0.1, W: 0.5, H: 0.5}, conf: 0.9, sure: true, say: "That panel."},
		{name: "none with a note", reply: "Nothing here does that.\n[POINT:none]\n[NOTE:Open the Settings app instead.]",
			kind: KindNone, say: "Nothing here does that.", note: "Open the Settings app instead."},
		{name: "answer", reply: "[ANSWER]", kind: KindAnswer},
		{name: "answer with stray prose", reply: "This is a question.\n[ANSWER]", kind: KindAnswer},
		{name: "plan tag leaves the point alone", reply: "Save.\n[POINT:1][CONF:.9][NEXT:Export (Export) | Close (Close)]",
			kind: KindElement, id: "1", box: &testControls[0].Box, conf: 0.9, sure: true, verified: true, say: "Save."},
		{name: "code fence around the reply", reply: "```\nSave, top left.\n[POINT:1][CONF:0.9]\n```",
			kind: KindElement, id: "1", box: &testControls[0].Box, conf: 0.9, sure: true, verified: true, say: "Save, top left."},
		{name: "bracketed prose stays prose", reply: "Press [Save] at the top.\n[POINT:1][CONF:0.8]",
			kind: KindElement, id: "1", box: &testControls[0].Box, conf: 0.8, sure: true, verified: true, say: "Press [Save] at the top."},
		{name: "hidden runes and markdown leave the sentence", reply: "**Save**\u202e is \x1bhere\u200b.\n[POINT:1][CONF:0.8]",
			kind: KindElement, id: "1", box: &testControls[0].Box, conf: 0.8, sure: true, verified: true, say: "Save is here."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := ParsePoint(c.reply, testControls, testWords, testPic)
			if err != nil {
				t.Fatalf("ParsePoint: %v", err)
			}
			if p.Kind != c.kind || p.ID != c.id || p.Sure != c.sure || p.Done != c.done || p.Verified != c.verified || p.Say != c.say || p.Note != c.note {
				t.Fatalf("got %+v", p)
			}
			if !near(p.Conf, c.conf) {
				t.Fatalf("conf %v, want %v", p.Conf, c.conf)
			}
			if (p.Box == nil) != (c.box == nil) {
				t.Fatalf("box %v, want %v", p.Box, c.box)
			}
			if c.box != nil && !(near(p.Box.X, c.box.X) && near(p.Box.Y, c.box.Y) && near(p.Box.W, c.box.W) && near(p.Box.H, c.box.H)) {
				t.Fatalf("box %+v, want %+v", *p.Box, *c.box)
			}
		})
	}
}

func TestParsePointRefuses(t *testing.T) {
	cases := []struct {
		name, reply string
		pic         Size
		code        string
	}{
		{"element under its floor", "Search.\n[POINT:3][CONF:0.29]", testPic, CodeLowConfidence},
		{"verified box under its floor", "Export.\n[POINT:0.69,0.09,0.09,0.05:Export][CONF:0.39]", testPic, CodeLowConfidence},
		{"unverified box under its floor", "Share.\n[POINT:0.85,0.02,0.05,0.04:Share][CONF:0.45]", testPic, CodeLowConfidence},
		{"no confidence", "Save.\n[POINT:1]", testPic, CodeLowConfidence},
		{"area over a quarter", "Here.\n[POINT:0,0,0.6,0.6:Panel][CONF:0.9]", testPic, CodeTooLarge},
		{"area over a quarter in pixels", "Here.\n[POINT:0,0,1000,500:Panel][CONF:0.9]", testPic, CodeTooLarge},
		{"box outside the picture", "Here.\n[POINT:0.9,0.9,0.3,0.3:Edge][CONF:0.9]", testPic, CodeOutside},
		{"pixels without a picture size", "Export.\n[POINT:1082,88,141,49:Export][CONF:0.9]", Size{}, CodeMalformed},
		{"unknown control", "That one.\n[POINT:99][CONF:0.9]", testPic, CodeUnknownControl},
		{"no point", "Click Save.", testPic, CodeMalformed},
		{"empty point", "Save.\n[POINT:][CONF:0.9]", testPic, CodeMalformed},
		{"two points", "Save.\n[POINT:1][POINT:3][CONF:0.9]", testPic, CodeMalformed},
		{"two confidences", "Save.\n[POINT:1][CONF:0.9][CONF:0.2]", testPic, CodeMalformed},
		{"answer and point", "[ANSWER][POINT:1][CONF:0.9]", testPic, CodeMalformed},
		{"confidence over one", "Save.\n[POINT:1][CONF:1.7]", testPic, CodeMalformed},
		{"confidence NaN", "Save.\n[POINT:1][CONF:NaN]", testPic, CodeMalformed},
		{"confidence Inf", "Save.\n[POINT:1][CONF:+Inf]", testPic, CodeMalformed},
		{"box NaN", "Here.\n[POINT:NaN,0.1,0.1,0.1:X][CONF:0.9]", testPic, CodeMalformed},
		{"box negative size", "Here.\n[POINT:0.5,0.5,-0.1,0.1:X][CONF:0.9]", testPic, CodeMalformed},
		{"box three numbers", "Here.\n[POINT:0.5,0.5,0.1:X][CONF:0.9]", testPic, CodeMalformed},
		{"action tag", "Saving it now.\n[DO:click {\"id\": 1}]\n[POINT:1][CONF:0.9]", testPic, CodeAction},
		{"action tag in lowercase", "On it. [type:hunter2] [POINT:none]", testPic, CodeAction},
		{"click tag", "[CLICK:1]", testPic, CodeAction},
		{"reply too long", "Save. " + strings.Repeat("a", maxReply) + "\n[POINT:1][CONF:0.9]", testPic, CodeMalformed},
		{"private field by role", "Type it here.\n[POINT:2][CONF:0.95]", testPic, CodeSensitive},
		{"private field by the text beside it", "This field.\n[POINT:4][CONF:0.95]", testPic, CodeSensitive},
		{"private field by box label", "Here.\n[POINT:0.3,0.4,0.3,0.05:Enter your PIN][CONF:0.95]", testPic, CodeSensitive},
		{"private field hidden in a nested tag", "Here.\n[POINT:[POINT:2]][CONF:0.95]", testPic, CodeSensitive},
		{"private field below its floor still refused as private", "Here.\n[POINT:2][CONF:0.1]", testPic, CodeSensitive},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := ParsePoint(c.reply, testControls, testWords, c.pic)
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
			if p.Box != nil || p.Kind != "" {
				t.Fatalf("refusal still returned a point: %+v", p)
			}
		})
	}
}

func TestPrivateFieldRefusalLine(t *testing.T) {
	_, err := ParsePoint("Here.\n[POINT:2][CONF:0.9]", testControls, nil, testPic)
	var r *Refusal
	if !errors.As(err, &r) || r.Code != CodeSensitive {
		t.Fatalf("got %v", err)
	}
	if r.Line != RefusalLine {
		t.Fatalf("line %q", r.Line)
	}
}

func TestPointingNeverCarriesHiddenRunes(t *testing.T) {
	reply := "Save\u202e\u2066\x00\x07 now.\n[POINT:0.85,0.02,0.05,0.04:Sh\u200bare\x1b][CONF:0.9]\n[NOTE:it\u202d cannot be undone]"
	p, err := ParsePoint(reply, nil, nil, testPic)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{p.Say, p.Label, p.Note} {
		for _, r := range s {
			if hidden(r) || r == '\n' {
				t.Fatalf("%q kept hidden rune %U", s, r)
			}
		}
	}
	if p.Label != "Share" || p.Say != "Save now." || p.Note != "it cannot be undone" {
		t.Fatalf("got %+v", p)
	}
}
