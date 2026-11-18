package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPiqueInterest_CorrectsPeakInterest(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The story managed to peak his interest.", NewPiqueInterest(), "The story managed to pique his interest.")
}

func TestPiqueInterest_CorrectsPeekedInterestAtStart(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Peeked his interest, did she?", NewPiqueInterest(), "Piqued his interest, did she?")
}

func TestPiqueInterest_CorrectsPeakInterestInMiddle(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She tried to peak his interest during the lecture.", NewPiqueInterest(), "She tried to pique his interest during the lecture.")
}

func TestPiqueInterest_CorrectsPeakedInterestAtEnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "All along, she hoped she peaked his interest.", NewPiqueInterest(), "All along, she hoped she piqued his interest.")
}

func TestPiqueInterest_DoesNotCorrectUnrelatedPeak(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He reached the peak of the mountain.", NewPiqueInterest(), "He reached the peak of the mountain.")
}

func TestPiqueInterest_CorrectsPeakingInterest(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She was peaking his interest with her stories.", NewPiqueInterest(), "She was piquing his interest with her stories.")
}

func TestPiqueInterest_CorrectsPeakedMyInterest(t *testing.T) {
	linttest.AssertSuggestionResult(t, "you've peaked my interest.", NewPiqueInterest(), "you've piqued my interest.")
}
