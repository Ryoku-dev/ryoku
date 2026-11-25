package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMassNounsMassPlurals_FixBadAdvices(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You gave me bad advices.", NewMassPlurals(dict.Curated()), "You gave me bad advice.")
}

func TestMassNounsMassPlurals_FlagSourceCodesAndSoftwares(t *testing.T) {
	linttest.AssertLintCount(t, "Do we have the source codes for these softwares?", NewMassPlurals(dict.Curated()), 2)
}

func TestMassNounsMassPlurals_FlagNounEndingInIes(t *testing.T) {
	linttest.AssertLintCount(t, "Celibacies are better than sex.", NewMassPlurals(dict.Curated()), 1)
}

func TestMassNounsMassPlurals_FlagRealEstates(t *testing.T) {
	linttest.AssertLintCount(t, "Instead of giving any of her many luxury real estates or multi-million dollar fortune ...", NewMassPlurals(dict.Curated()), 1)
}

func TestMassNounsMassPlurals_FlagWearAndTears(t *testing.T) {
	linttest.AssertLintCount(t, "Transit costs were high in terms of time, finances, and vehicle wear and tears, which posed significant obstacles to international commerce", NewMassPlurals(dict.Curated()), 1)
}

func TestMassNounsMassPlurals_FixWearAndTears(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Transit costs were high in terms of time, finances, and vehicle wear and tears, which posed significant obstacles to international commerce", NewMassPlurals(dict.Curated()), "Transit costs were high in terms of time, finances, and vehicle wear and tear, which posed significant obstacles to international commerce")
}

func TestMassNounsMassPlurals_FixHorsepowers(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's around about 60 horsepowers. So, you can't go in and say, \"Well, we got more horsepowers than our rivals.\" because it's got the same and through that restriction they've created an absolute banger. I'm so excited about this car.", NewMassPlurals(dict.Curated()), "It's around about 60 horsepower. So, you can't go in and say, \"Well, we got more horsepower than our rivals.\" because it's got the same and through that restriction they've created an absolute banger. I'm so excited about this car.")
}
