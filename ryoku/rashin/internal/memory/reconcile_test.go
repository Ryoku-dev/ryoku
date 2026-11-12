package memory

import "testing"

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
