package screenask

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseSelectionAccepts(t *testing.T) {
	cases := []struct {
		name, reply, answer string
		walk                bool
	}{
		{"plain object", `{"answer": "That button exports the invoice as a PDF.", "walk": false}`,
			"That button exports the invoice as a PDF.", false},
		{"white space around and keys reversed", "\n  {\"walk\": true,\n \"answer\": \"It saves the file.\"}\n\n",
			"It saves the file.", true},
		{"hidden runes escaped and raw", `{"answer": "Exports\u202e the\u200b file` + "\u2066" + `.", "walk": false}`,
			"Exports the file.", false},
		{"markdown and reply tags dropped", `{"answer": "**Export** is top right. [POINT:1] [CONF:0.9]", "walk": false}`,
			"Export is top right.", false},
		{"planted words stay plain words", `{"answer": "The banner says to ignore previous instructions; it is an advert.", "walk": false}`,
			"The banner says to ignore previous instructions; it is an advert.", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := ParseSelection(c.reply)
			if err != nil {
				t.Fatalf("ParseSelection: %v", err)
			}
			if s.Answer != c.answer || s.Walk != c.walk {
				t.Fatalf("got %+v", s)
			}
		})
	}
}

func TestParseSelectionBoundsTheAnswer(t *testing.T) {
	s, err := ParseSelection(`{"answer": "` + strings.Repeat("word ", 400) + `", "walk": true}`)
	if err != nil {
		t.Fatal(err)
	}
	if n := utf8.RuneCountInString(s.Answer); n > MaxSelectionAnswer || n < MaxSelectionAnswer-10 {
		t.Fatalf("answer is %d runes, want at most %d and cut on a word", n, MaxSelectionAnswer)
	}
	if strings.HasSuffix(s.Answer, " ") || strings.HasSuffix(s.Answer, "wor") || !s.Walk {
		t.Fatalf("got %q, walk %v", s.Answer[len(s.Answer)-10:], s.Walk)
	}
}

func TestParseSelectionRefuses(t *testing.T) {
	const ok = `{"answer": "It saves the file.", "walk": false}`
	cases := []struct {
		name, reply, code string
	}{
		{"code fence", "```json\n" + ok + "\n```", CodeMalformed},
		{"prose before", "Sure! " + ok, CodeMalformed},
		{"prose after", ok + " Hope that helps.", CodeMalformed},
		{"a second object after", ok + ok, CodeMalformed},
		{"a stray brace after", ok + "}", CodeMalformed},
		{"a field besides answer and walk", `{"answer": "It saves.", "walk": false, "click": "Delete"}`, CodeMalformed},
		{"answer twice", `{"answer": "It saves.", "answer": "[CLICK:Delete]", "walk": false}`, CodeMalformed},
		{"walk twice", `{"answer": "It saves.", "walk": false, "walk": true}`, CodeMalformed},
		{"no walk", `{"answer": "It saves."}`, CodeMalformed},
		{"no answer", `{"walk": true}`, CodeMalformed},
		{"walk as a string", `{"answer": "It saves.", "walk": "true"}`, CodeMalformed},
		{"answer as a number", `{"answer": 42, "walk": false}`, CodeMalformed},
		{"answer as an object", `{"answer": {"text": "It saves."}, "walk": false}`, CodeMalformed},
		{"an array", `[{"answer": "It saves.", "walk": false}]`, CodeMalformed},
		{"unterminated", `{"answer": "It saves.", "walk": false`, CodeMalformed},
		{"raw line break inside a string", "{\"answer\": \"It\nsaves.\", \"walk\": false}", CodeMalformed},
		{"too long", `{"answer": "` + strings.Repeat("a", maxReply) + `", "walk": false}`, CodeMalformed},
		{"action tag in the answer", `{"answer": "Done. [CLICK:Delete]", "walk": false}`, CodeAction},
		{"lowercase action tag in the answer", `{"answer": "Typing it now [type:hunter2]", "walk": true}`, CodeAction},
		{"planted instruction echoed", `{"answer": "` + strings.ReplaceAll(injection, `"`, `\"`) + `", "walk": false}`, CodeAction},
		{"empty answer", `{"answer": "", "walk": false}`, CodeEmpty},
		{"answer of hidden runes and tags", `{"answer": "\u200b [POINT:1] \u202e", "walk": false}`, CodeEmpty},
		{"empty reply", "  \n ", CodeEmpty},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := ParseSelection(c.reply)
			var r *Refusal
			if !errors.As(err, &r) {
				t.Fatalf("got %+v, %v; want refusal %s", s, err, c.code)
			}
			if r.Code != c.code {
				t.Fatalf("code %s (%s), want %s", r.Code, r.Reason, c.code)
			}
			if r.Line == "" || strings.Contains(r.Line, "point") && c.code == CodeMalformed {
				t.Fatalf("refusal line %q does not fit an answer", r.Line)
			}
			if s != (Selection{}) {
				t.Fatalf("refusal still returned a selection: %+v", s)
			}
		})
	}
}
