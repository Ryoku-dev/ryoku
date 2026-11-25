package hygiene

import (
	"strconv"
	"strings"
	"testing"
)

func TestCollapse(t *testing.T) {
	loop43 := strings.Repeat("Open the window. ", 43)
	cases := []struct {
		name, in, want string
	}{
		{"43 repeats collapse to one phrase", loop43, "Open the window."},
		{"loop after real words", "close the tab " + strings.Repeat("thank you ", 12) + "bye", "close the tab thank you bye"},
		{"three in a row is a loop", "go go go home", "go home"},
		{"two in a row is speech", "no no that one", "no no that one"},
		{"case and punctuation ignored", "Thank you. thank you, THANK YOU!", "Thank you."},
		{"smallest repeating phrase wins", "la la la la la la", "la"},
		{"loops on both sides", "a b a b a b then c c c", "a b then c"},
		{"non-adjacent repeats stay", "set a timer and set a timer later and set a timer", "set a timer and set a timer later and set a timer"},
		{"punctuation-only tokens never loop", "- - - ok", "- - - ok"},
		{"whitespace normalised", "  hello \n world ", "hello world"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Collapse(c.in); got != c.want {
				t.Fatalf("Collapse(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestCollapseLeavesLongPhrasesToSentenceHygiene(t *testing.T) {
	var words []string
	for i := range 26 {
		words = append(words, "w"+strconv.Itoa(i))
	}
	phrase := strings.Join(words, " ")
	long := phrase + " " + phrase + " " + phrase
	if got := Collapse(long); got != long {
		t.Fatalf("a 26-word phrase past the loop limit was collapsed to %q", got)
	}
}

func TestPlausible(t *testing.T) {
	cases := []struct {
		words int
		secs  float64
		want  bool
	}{
		{8, 1, true},   // ceil(5)+3
		{9, 1, false},  // one past
		{4, 0.1, true}, // ceil(0.5)+3
		{5, 0.1, false},
		{3, 0, true},
		{4, 0, false},
		{13, 2, true},
		{14, 2, false},
		{0, 0, true},
		{4, -1, false},
	}
	for _, c := range cases {
		text := strings.TrimSpace(strings.Repeat("word ", c.words))
		if got := Plausible(text, c.secs); got != c.want {
			t.Errorf("Plausible(%d words, %gs) = %v, want %v", c.words, c.secs, got, c.want)
		}
	}
}

func TestDropRepeatedSentences(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"adjacent repeat", "Turn it off. Turn it off.", "Turn it off."},
		{"later repeat", "Turn it off. Then sleep. turn it OFF!", "Turn it off. Then sleep."},
		{"short sentences stay", "No. No. Stop now.", "No. No. Stop now."},
		{"mark runs end one sentence", "Really?! Really?! yes", "Really?! Really?! yes"},
		{"two-word repeat goes", "Thank you. Thank you!", "Thank you."},
		{"abbreviation-free text untouched", "Open notes and write it down.", "Open notes and write it down."},
		{"marks inside a word do not split", "Go to v1.2 now. Go to v1.2 now.", "Go to v1.2 now."},
		{"tail without a mark", "Play music. play music", "Play music."},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DropRepeatedSentences(c.in); got != c.want {
				t.Fatalf("DropRepeatedSentences(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestTidy(t *testing.T) {
	loop43 := strings.TrimSpace(strings.Repeat("open the window ", 43))
	if Plausible(loop43, 3) {
		t.Fatal("fixture: the raw loop should be implausible for 3s")
	}
	cases := []struct {
		name string
		in   string
		secs float64
		want string
		ok   bool
	}{
		{"43-repeat loop kept as one phrase", loop43, 3, "open the window", true},
		{"implausible transcript dropped", "the quick brown fox jumps over the lazy dog while seven bright planets turn", 1, "", false},
		{"plausible transcript kept", "set a timer for ten minutes", 2, "set a timer for ten minutes", true},
		{"repeated sentences then plausible", "Call mum. Text dad. call mum!", 1.5, "Call mum. Text dad.", true},
		{"nothing heard", "   ", 2, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := Tidy(c.in, c.secs)
			if got != c.want || ok != c.ok {
				t.Fatalf("Tidy(%q, %g) = %q, %v; want %q, %v", c.in, c.secs, got, ok, c.want, c.ok)
			}
		})
	}
}
