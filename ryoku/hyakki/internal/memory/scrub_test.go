package memory

import (
	"strings"
	"testing"
)

func TestScrubOrder(t *testing.T) {
	user := "my token=ghp_abcdefghijklmnopqrstuvwxyz0123 and <private>my diary</private> here"
	reply := "I stored ghp_abcdefghijklmnopqrstuvwxyz0123 for you"
	scrubTexts([]*string{&user, &reply})
	if strings.Contains(user, "diary") || strings.Contains(user, "ghp_") || strings.Contains(reply, "ghp_") {
		t.Fatalf("user = %q, reply = %q", user, reply)
	}
}

func TestLearnedTokenMasksBareRepeat(t *testing.T) {
	first := "export DEPLOY_SECRET=Zq9xLmN3pR7tVw2YkB5cHd8F"
	second := "the value Zq9xLmN3pR7tVw2YkB5cHd8F works"
	scrubTexts([]*string{&first, &second})
	if strings.Contains(second, "Zq9x") {
		t.Fatalf("a token caught once was not masked where it was repeated: %q", second)
	}
}

func TestStripThink(t *testing.T) {
	for in, want := range map[string]string{
		"<think>plan</think>Answer":          "Answer",
		"<thinking>a</thinking> b":           "b",
		"<think>never closed":                "",
		"<|channel|>final text":              "final text",
		"text that ends in <thi":             "text that ends in",
		"talks about <think> tags mid text.": "talks about <think> tags mid text.",
	} {
		if got := stripThink(in); got != want {
			t.Errorf("stripThink(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTrivialAndRecallIntent(t *testing.T) {
	for _, text := range []string{"thanks!", "ok.", "LGTM", "/compact", "", "yes :)", "go ahead…"} {
		if !trivial(text) {
			t.Errorf("trivial(%q) = false", text)
		}
	}
	for _, text := range []string{"k8s", "note this down", "thanks, and also fix the build"} {
		if trivial(text) {
			t.Errorf("trivial(%q) = true", text)
		}
	}
	for _, text := range []string{"what did we decide about the cache?", "do you remember my cat's name", "which library did you suggest", "lo que hablamos la última vez"} {
		if !recallIntent(text) {
			t.Errorf("recallIntent(%q) = false", text)
		}
	}
	if recallIntent("what we did you see") || recallIntent("write a haiku about rain") {
		t.Error("recall intent found where none is")
	}
}

func TestParseLast(t *testing.T) {
	for in, hours := range map[string]float64{"6h": 6, "7d": 168, "2w": 336, "1m": 720} {
		got, err := parseLast(in)
		if err != nil || got.Hours() != hours {
			t.Errorf("parseLast(%q) = %v, %v", in, got, err)
		}
	}
	if _, err := parseLast("3y"); err == nil {
		t.Error("3y accepted")
	}
}

func TestExtractJSONTolerance(t *testing.T) {
	var out learnReply
	if err := parseReply("Here you go:\n```json\n{\"summary\":\"s\",\"ops\":[{\"op\":\"add\",\"scope\":\"personal\",\"text\":\"t\",\"supersedes\":[2]}]}\n```", &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Ops) != 1 || len(out.Ops[0].Supersedes) != 1 || out.Ops[0].Supersedes[0] != "2" {
		t.Fatalf("ops = %+v", out.Ops)
	}
	if err := parseReply(`{"summary":"s","ops":[],"extra":1}`, &out); err == nil {
		t.Fatal("an unknown field was accepted")
	}
}

func TestRecallIntentFirstPerson(t *testing.T) {
	for _, text := range []string{
		"Did I ever mention my sister's name?",
		"when did I start running?",
		"How many times did I go to the gym in March?",
		"how many books have I read this year",
		"What did I tell you about my landlord?",
		"what did i ask about visas",
		"The last time I flew, which airline was it?",
		"remind me when the dentist appointment is",
	} {
		if !recallIntent(text) {
			t.Errorf("recallIntent(%q) = false", text)
		}
	}
	for _, text := range []string{"I did it myself", "how do I install go", "what should I eat tonight", "time I went to bed early"} {
		if recallIntent(text) {
			t.Errorf("recallIntent(%q) = true", text)
		}
	}
}
