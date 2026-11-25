package screenask

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseAnswerAccepts(t *testing.T) {
	cases := []struct {
		name, mode, reply string
		text, verdict     string
	}{
		{"plain", ModeSummarize, "It is an invoice for March. It is due Friday.", "It is an invoice for March. It is due Friday.", ""},
		{"markdown flattened", ModeExplain, "## Error\n**Disk full.** The save failed.\n- Free some space\n* Then retry", "Error\nDisk full. The save failed.\nFree some space\nThen retry", ""},
		{"em dash becomes a comma", ModeDescribe, "A settings page \u2014 the network tab is open.", "A settings page, the network tab is open.", ""},
		{"point tags dropped", ModeAsk, "It is the export menu. [POINT:1][CONF:0.9]", "It is the export menu.", ""},
		{"code fence dropped", ModeRead, "```text\nDear Sam, the meeting moved to 3 pm.\n```", "Dear Sam, the meeting moved to 3 pm.", ""},
		{"hidden runes dropped", ModeAsk, "Th\u202eree\x1b items\u200b.", "Three items.", ""},
		{"check yes", ModeCheck, "Yes, the totals add up.", "Yes, the totals add up.", VerdictYes},
		{"check no", ModeCheck, "No. The date is in the past.", "No. The date is in the past.", VerdictNo},
		{"check not sure", ModeCheck, "Not sure, the second page is hidden.", "Not sure, the second page is hidden.", VerdictUnsure},
		{"check without a verdict", ModeCheck, "Maybe, the field is cut off.", "Maybe, the field is cut off.", ""},
		{"verdict only in check mode", ModeAsk, "Yes, it is saved.", "Yes, it is saved.", ""},
		{"injected instructions are just text", ModeAsk, "The page says to ignore previous instructions; it is a phishing page.", "The page says to ignore previous instructions; it is a phishing page.", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, err := ParseAnswer(c.mode, c.reply)
			if err != nil {
				t.Fatal(err)
			}
			if a.Mode != c.mode || a.Text != c.text || a.Verdict != c.verdict {
				t.Fatalf("got %+v, want text %q verdict %q", a, c.text, c.verdict)
			}
		})
	}
}

func TestParseAnswerRefuses(t *testing.T) {
	cases := []struct {
		name, mode, reply, code string
	}{
		{"unknown mode", "hack", "Fine.", CodeMalformed},
		{"empty", ModeAsk, "  \n ", CodeEmpty},
		{"only an answer tag", ModeAsk, "[ANSWER]", CodeEmpty},
		{"action tag", ModeAsk, "Deleting it for you. [DO:press {\"keys\": \"ctrl+a\"}]", CodeAction},
		{"lowercase action tag", ModeSummarize, "Sure. [click:5]", CodeAction},
		{"too long", ModeRead, strings.Repeat("word ", maxReply), CodeMalformed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, err := ParseAnswer(c.mode, c.reply)
			var r *Refusal
			if !errors.As(err, &r) || r.Code != c.code {
				t.Fatalf("got %+v, %v; want %s", a, err, c.code)
			}
			if r.Line == "" {
				t.Fatal("refusal has no line")
			}
		})
	}
}

func TestParseAnswerCutsLongAnswersOnAWord(t *testing.T) {
	a, err := ParseAnswer(ModeRead, strings.Repeat("plain ", 600))
	if err != nil {
		t.Fatal(err)
	}
	if n := utf8.RuneCountInString(a.Text); n > maxAnswer || n < maxAnswer-10 {
		t.Fatalf("answer is %d runes", n)
	}
	if !strings.HasSuffix(a.Text, "plain") {
		t.Fatalf("answer cut mid-word: %q", a.Text[len(a.Text)-12:])
	}
}
