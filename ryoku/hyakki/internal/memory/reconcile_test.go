package memory

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"ryoku-rashin/internal/protocol"
)

// Evidence must say something and share a word with the fact.
func TestEvidenceMustTieToFact(t *testing.T) {
	r := &reconcile{words: "I like the sea and long walks by the river"}
	for _, c := range []struct {
		evidence, fact string
		want           bool
	}{
		{"the", "The user likes the sea", false},
		{"I like the sea", "The user likes the sea", true},
		{"long walks by the river", "The user collects stamps", false},
		{"long walks by the river", "The user enjoys long walks", true},
	} {
		if got := r.grounded(c.evidence, c.fact); got != c.want {
			t.Errorf("grounded(%q, %q) = %v", c.evidence, c.fact, got)
		}
	}
}

// A fact that still says when relative to the day it was said gets that
// day, and the note fits within the cap.
func TestAsOfDatesRelativeFacts(t *testing.T) {
	day := time.Date(2023, 5, 26, 12, 0, 0, 0, time.Local)
	for _, c := range []struct{ in, want string }{
		{"The user got new binoculars three weeks ago", "The user got new binoculars three weeks ago (as of 26 May 2023)"},
		{"The user saw goldfinches return last Tuesday", "The user saw goldfinches return last Tuesday (as of 26 May 2023)"},
		{"The user moved to Lisbon the other day", "The user moved to Lisbon the other day (as of 26 May 2023)"},
		{"The user is saving for a trip to Japan next spring", "The user is saving for a trip to Japan next spring (as of 26 May 2023)"},
		{"The user got new binoculars on 5 May 2023", "The user got new binoculars on 5 May 2023"},
		{"Lastly, the user prefers agony aunts on Thursdays", "Lastly, the user prefers agony aunts on Thursdays"},
	} {
		if got := asOf(c.in, day); got != c.want {
			t.Errorf("asOf(%q) = %q", c.in, got)
		}
	}
	long := strings.Repeat("The user hikes a lot ", 13) + "since last month"
	got := asOf(long, day)
	if utf8.RuneCountInString(got) > protocol.MemoryTextMax || !strings.HasSuffix(got, "(as of 26 May 2023)") {
		t.Fatalf("asOf cut the note or broke the cap: %q (%d)", got, utf8.RuneCountInString(got))
	}
	if asOf("The user ran a race yesterday", time.Time{}) != "The user ran a race yesterday" {
		t.Fatal("dated a fact with no observation date")
	}
}
