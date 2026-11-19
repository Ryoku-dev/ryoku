package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestInflectedVerbAfterTo_DontFlagToCheckBothVerbAndNoun(t *testing.T) {
	linttest.AssertLintCount(t, "to check", NewInflectedVerbAfterTo(dict.Curated()), 0)
}

func TestInflectedVerbAfterTo_DontFlagToChecksBothVerbAndNoun(t *testing.T) {
	linttest.AssertLintCount(t, "to checks", NewInflectedVerbAfterTo(dict.Curated()), 0)
}

func TestInflectedVerbAfterTo_DontFlagToChequesNotAVerb(t *testing.T) {
	linttest.AssertLintCount(t, "to cheques", NewInflectedVerbAfterTo(dict.Curated()), 0)
}

func TestInflectedVerbAfterTo_FlagToChecking(t *testing.T) {
	t.Skip("-ing forms can act as nouns, current heuristics cannot distinguish")
	linttest.AssertLintCount(t, "to checking", NewInflectedVerbAfterTo(dict.Curated()), 1)
}

func TestInflectedVerbAfterTo_DontFlagCheckEd(t *testing.T) {
	linttest.AssertLintCount(t, "to checked", NewInflectedVerbAfterTo(dict.Curated()), 0)
}

func TestInflectedVerbAfterTo_DontFlagNounBeliefS(t *testing.T) {
	linttest.AssertLintCount(t, "to beliefs", NewInflectedVerbAfterTo(dict.Curated()), 0)
}

func TestInflectedVerbAfterTo_DontFlagNounMeatS(t *testing.T) {
	linttest.AssertLintCount(t, "to meats", NewInflectedVerbAfterTo(dict.Curated()), 0)
}

func TestInflectedVerbAfterTo_Check993Suggestions(t *testing.T) {
	t.Skip("can't check yet. 'capture' is noun as well as verb. \"to nouns\" is good English. we can't disambiguate verbs from nouns.")
	linttest.AssertSuggestionResult(t, "A location-agnostic structure that attempts to captures the context and content that a Lint occurred.", NewInflectedVerbAfterTo(dict.Curated()), "A location-agnostic structure that attempts to capture the context and content that a Lint occurred.")
}

func TestInflectedVerbAfterTo_DontFlagEmbarrassNotInDictionary(t *testing.T) {
	linttest.AssertLintCount(t, "Second I'm going to embarrass you for a.", NewInflectedVerbAfterTo(dict.Curated()), 0)
}

func TestInflectedVerbAfterTo_CorrectsExistS(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A valid solution is expected to exists.", NewInflectedVerbAfterTo(dict.Curated()), "A valid solution is expected to exist.")
}

func TestInflectedVerbAfterTo_CorrectsEsEnding(t *testing.T) {
	t.Skip("can't check yet. 'catch' is noun as well as verb. 'to nouns' is good English. we can't disambiguate verbs from nouns.")
	linttest.AssertSuggestionResult(t, "I need it to catches every exception.", NewInflectedVerbAfterTo(dict.Curated()), "I need it to catch every exception.")
}

func TestInflectedVerbAfterTo_CorrectsEdEnding(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I had to expanded my horizon.", NewInflectedVerbAfterTo(dict.Curated()), "I had to expand my horizon.")
}

func TestInflectedVerbAfterTo_FlagsExpireD(t *testing.T) {
	linttest.AssertLintCount(t, "I didn't know it was going to expired.", NewInflectedVerbAfterTo(dict.Curated()), 1)
}

func TestInflectedVerbAfterTo_CorrectsExplainEd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "To explained the rules to the team.", NewInflectedVerbAfterTo(dict.Curated()), "To explain the rules to the team.")
}

func TestInflectedVerbAfterTo_CorrectsExplorEd(t *testing.T) {
	t.Skip("can't check yet. surprisingly, 'explore' is noun as well as verb. 'to nouns' is good English. we can't disambiguate verbs from nouns.")
	linttest.AssertSuggestionResult(t, "I went to explored distant galaxies.", NewInflectedVerbAfterTo(dict.Curated()), "I went to explore distant galaxies.")
}

func TestInflectedVerbAfterTo_CantFlagExpressEdAlsoNoun(t *testing.T) {
	linttest.AssertLintCount(t, "I failed to clearly expressed my point.", NewInflectedVerbAfterTo(dict.Curated()), 0)
}

func TestInflectedVerbAfterTo_CorrectFeignEd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I was able to feigned ignorance.", NewInflectedVerbAfterTo(dict.Curated()), "I was able to feign ignorance.")
}

func TestInflectedVerbAfterTo_Issue241(t *testing.T) {
	linttest.AssertLintCount(t, "Comparison to Expected Results", NewInflectedVerbAfterTo(dict.Curated()), 0)
}
