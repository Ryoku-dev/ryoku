package rules

import (
	"testing"
)

func TestInformalLaughter_MatchesAlternatingLaughter(t *testing.T) {
	for _, word := range []string{"ha", "hah", "haha", "hahah", "hahaha", "hahahah"} {
		if !informalLaughterIsInformalLaughter([]rune(word)) {
			t.Errorf("%s should match", word)
		}
	}
}

func TestInformalLaughter_MatchesLaughterCaseInsensitively(t *testing.T) {
	for _, word := range []string{"Hah", "Hahahah", "HAHAHA", "HaHaH"} {
		if !informalLaughterIsInformalLaughter([]rune(word)) {
			t.Errorf("%s should match", word)
		}
	}
}

func TestInformalLaughter_DoesNotMatchMalformedHahChains(t *testing.T) {
	for _, word := range []string{"hahhah", "haah", "hahh", "hha"} {
		if informalLaughterIsInformalLaughter([]rune(word)) {
			t.Errorf("%s should not match", word)
		}
	}
}

func TestInformalLaughter_DoesNotMatchOutOfScopeInterjections(t *testing.T) {
	for _, word := range []string{"ah", "Ah", "ahah", "hehe"} {
		if informalLaughterIsInformalLaughter([]rune(word)) {
			t.Errorf("%s should be ignored by this matcher", word)
		}
	}
}
