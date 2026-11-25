package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRiseTheRanks_FixRiseTheRanks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Rise the ranks from Division 8 to the Champion!", NewRiseTheRanks(), "Rise through the ranks from Division 8 to the Champion!")
}

func TestRiseTheRanks_FixRisedTheRanks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "while there I've rised the ranks from L1 Tech > L2 Tech > Team lead and now Manager", NewRiseTheRanks(), "while there I've rised through the ranks from L1 Tech > L2 Tech > Team lead and now Manager")
}

func TestRiseTheRanks_FixRisenTheRanks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "AngularJS has risen the ranks of popularity a lot over the last couple of years.", NewRiseTheRanks(), "AngularJS has risen through the ranks of popularity a lot over the last couple of years.")
}

func TestRiseTheRanks_FixRisingTheRanks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Rust is new coding language rising the ranks, aiming at safe concurrency, data safe programming at zero abstraction cost.", NewRiseTheRanks(), "Rust is new coding language rising through the ranks, aiming at safe concurrency, data safe programming at zero abstraction cost.")
}

func TestRiseTheRanks_FixRoseTheRanks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "we follow these women as they quickly rose the ranks of NASA alongside many of history's greatest minds", NewRiseTheRanks(), "we follow these women as they quickly rose through the ranks of NASA alongside many of history's greatest minds")
}

func TestRiseTheRanks_DontFlagRiseThroughTheRanks(t *testing.T) {
	linttest.AssertNoLints(t, "I hadn't noticed any particular suspicious behavior during my slow but steady rise through the ranks.", NewRiseTheRanks())
}

func TestRiseTheRanks_DontFlagRisesThroughTheRanks(t *testing.T) {
	linttest.AssertNoLints(t, "Vito Scaletta rises through the ranks of the mafia, becoming a powerful don.", NewRiseTheRanks())
}

func TestRiseTheRanks_DontFlagRisingFromTheRanks(t *testing.T) {
	linttest.AssertNoLints(t, "Rising from the ranks of a sailor, he seems to have embodied the American Dream among a navy full of elites.", NewRiseTheRanks())
}

func TestRiseTheRanks_DontFlagRisingThroughTheRanks(t *testing.T) {
	linttest.AssertNoLints(t, "From a tranquil town, he discovered his passion in online gaming tournaments, rising through the ranks with unwavering dedication.", NewRiseTheRanks())
}
