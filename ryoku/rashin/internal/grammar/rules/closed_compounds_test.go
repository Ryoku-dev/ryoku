package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestClosedCompounds_ItSelf(t *testing.T) {
	test_sentence := "The project, it self, was quite challenging."
	expected := "The project, itself, was quite challenging."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_MySelf(t *testing.T) {
	test_sentence := "He treated my self with respect."
	expected := "He treated myself with respect."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_ThereFore(t *testing.T) {
	test_sentence := "This is the reason; there fore, this is true."
	expected := "This is the reason; therefore, this is true."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_MisUnderstood(t *testing.T) {
	test_sentence := "She miss understood the instructions."
	expected := "She misunderstood the instructions."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_MisUse(t *testing.T) {
	test_sentence := "He tends to miss use the tool."
	expected := "He tends to misuse the tool."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_MisUsed(t *testing.T) {
	test_sentence := "The software was miss used."
	expected := "The software was misused."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_MeanWhile(t *testing.T) {
	test_sentence := "Mean while, the team kept working."
	expected := "Meanwhile, the team kept working."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_WorldWide(t *testing.T) {
	test_sentence := "The world wide impact was significant."
	expected := "The worldwide impact was significant."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_OverAll(t *testing.T) {
	test_sentence := "The over all performance was good."
	expected := "The overall performance was good."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_HowEver(t *testing.T) {
	test_sentence := "This is true, how ever, details matter."
	expected := "This is true, however, details matter."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_WideSpread(t *testing.T) {
	test_sentence := "The news was wide spread throughout the region."
	expected := "The news was widespread throughout the region."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_NotWithStanding(t *testing.T) {
	test_sentence := "They decided to proceed not with standing any further delay."
	expected := "They decided to proceed notwithstanding any further delay."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_AnyHow(t *testing.T) {
	test_sentence := "She solved the problem any how, even under pressure."
	expected := "She solved the problem anyhow, even under pressure."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_NoneTheLess(t *testing.T) {
	test_sentence := "The results were disappointing, none the less, they continued."
	expected := "The results were disappointing, nonetheless, they continued."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_ThereUpon(t *testing.T) {
	test_sentence := "A decision was made there upon reviewing the data."
	expected := "A decision was made thereupon reviewing the data."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_InSoFar(t *testing.T) {
	test_sentence := "This rule applies in so far as it covers all cases."
	expected := "This rule applies insofar as it covers all cases."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_WhereUpon(t *testing.T) {
	test_sentence := "They acted where upon the circumstances allowed."
	expected := "They acted whereupon the circumstances allowed."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_UpWard(t *testing.T) {
	test_sentence := "The temperature moved up ward during the afternoon."
	expected := "The temperature moved upward during the afternoon."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_HenceForth(t *testing.T) {
	test_sentence := "All new policies apply hence forth immediately."
	expected := "All new policies apply henceforth immediately."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_RegardLess(t *testing.T) {
	test_sentence := "The decision was made, regard less of the opposition."
	expected := "The decision was made, regardless of the opposition."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_OverNight(t *testing.T) {
	test_sentence := "They set off on their journey over night."
	expected := "They set off on their journey overnight."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_ByPass(t *testing.T) {
	test_sentence := "Please by pass this check for now."
	expected := "Please bypass this check for now."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_DeadLift(t *testing.T) {
	test_sentence := "I can dead lift 200 kg."
	expected := "I can deadlift 200 kg."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_ChalkBoard(t *testing.T) {
	test_sentence := "The teacher wrote the equation on the chalk board."
	expected := "The teacher wrote the equation on the chalkboard."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_KeyStoke(t *testing.T) {
	test_sentence := "Use this key stoke to open search."
	expected := "Use this keystroke to open search."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_InTact(t *testing.T) {
	test_sentence := "The code remains in tact after the merge."
	expected := "The code remains intact after the merge."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_IntactIsAllowed(t *testing.T) {
	linttest.AssertNoLints(t, "The data set remains intact.", NewClosedCompoundsLintGroup())
}

func TestClosedCompounds_KeyStokes(t *testing.T) {
	test_sentence := "These key stokes are hard to memorize."
	expected := "These keystrokes are hard to memorize."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_KeyStrokes(t *testing.T) {
	test_sentence := "There may have been a missing key stroke."
	expected := "There may have been a missing keystroke."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_WithOut(t *testing.T) {
	test_sentence := "We left with out a map."
	expected := "We left without a map."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_TheRe(t *testing.T) {
	test_sentence := "The re are too many popups on this page."
	expected := "There are too many popups on this page."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_ShortComing(t *testing.T) {
	test_sentence := "That bug is a short coming in the current release."
	expected := "That bug is a shortcoming in the current release."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_ShortComings(t *testing.T) {
	test_sentence := "We listed three short comings in the postmortem."
	expected := "We listed three shortcomings in the postmortem."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_WorthWhile(t *testing.T) {
	test_sentence := "It's worth while documenting all the clientside events that the eventsService emits?"
	expected := "It's worthwhile documenting all the clientside events that the eventsService emits?"
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_WorthHyphenWhile(t *testing.T) {
	test_sentence := "I feel that the special case of looping over sequences that follow a standard iterator protocol (i.e. optionals) is important enough to be worth-while."
	expected := "I feel that the special case of looping over sequences that follow a standard iterator protocol (i.e. optionals) is important enough to be worthwhile."
	linttest.AssertSuggestionResult(t, test_sentence, NewClosedCompoundsLintGroup(), expected)
}

func TestClosedCompounds_Tenfold(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If Andrew Kelly has done something wrong hasn’t Theo done that ten fold", NewClosedCompoundsLintGroup(), "If Andrew Kelly has done something wrong hasn’t Theo done that tenfold")
}

func TestClosedCompounds_Codebase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Having trouble communicating the problems in your code base?", NewClosedCompoundsLintGroup(), "Having trouble communicating the problems in your codebase?")
}

func TestClosedCompounds_Codebases(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Tools to visualize large code bases in different ways.", NewClosedCompoundsLintGroup(), "Tools to visualize large codebases in different ways.")
}

func TestClosedCompounds_Straightforward(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This new vulnerability has a somewhat straight forward path to exploitation and prints funds out of thin air.", NewClosedCompoundsLintGroup(), "This new vulnerability has a somewhat straightforward path to exploitation and prints funds out of thin air.")
}

func TestClosedCompounds_Lifetime(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In a life time it's gone up by more than 10 fold.", NewClosedCompoundsLintGroup(), "In a lifetime it's gone up by more than 10 fold.")
}

func TestClosedCompounds_Tradeoff(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But the trade off was that specific details about the invention, how it worked and how it's made were entered openly for anyone to look up.", NewClosedCompoundsLintGroup(), "But the tradeoff was that specific details about the invention, how it worked and how it's made were entered openly for anyone to look up.")
}

func TestClosedCompounds_Guesswork(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yes this is guess work and we need more evidence of the dangers but at the same time we need evidence of safety.", NewClosedCompoundsLintGroup(), "Yes this is guesswork and we need more evidence of the dangers but at the same time we need evidence of safety.")
}

func TestClosedCompounds_Oftentimes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the rust compiler is fast, but often times it has to perform a lot of unnecessary checks because crates contain more stuff than needed", NewClosedCompoundsLintGroup(), "the rust compiler is fast, but oftentimes it has to perform a lot of unnecessary checks because crates contain more stuff than needed")
}

func TestClosedCompounds_FanSpaceFare(t *testing.T) {
	linttest.AssertSuggestionResult(t, "There was much fan fare over all the technology Bill Gates put into his house.", NewClosedCompoundsLintGroup(), "There was much fanfare over all the technology Bill Gates put into his house.")
}

func TestClosedCompounds_FanHyphenFare(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Indeed, most users will experience major version updates with absolutely zero fan-fare.", NewClosedCompoundsLintGroup(), "Indeed, most users will experience major version updates with absolutely zero fanfare.")
}

func TestClosedCompounds_RoundAbout(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This might be a very round about way to sort right to repair.", NewClosedCompoundsLintGroup(), "This might be a very roundabout way to sort right to repair.")
}

func TestClosedCompounds_WhatSpSoSpEver(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You can literally just apply to be an ATC with zero experience what so ever.", NewClosedCompoundsLintGroup(), "You can literally just apply to be an ATC with zero experience whatsoever.")
}

func TestClosedCompounds_WhatHySoHyEver(t *testing.T) {
	linttest.AssertSuggestionResult(t, "React doesn't have this \"pure function\" feature what-so-ever.", NewClosedCompoundsLintGroup(), "React doesn't have this \"pure function\" feature whatsoever.")
}

func TestClosedCompounds_WhatSpSoever(t *testing.T) {
	linttest.AssertSuggestionResult(t, "No problems what soever in the almost two years, they have been running", NewClosedCompoundsLintGroup(), "No problems whatsoever in the almost two years, they have been running")
}

func TestClosedCompounds_WhatHySoever(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and literally not couching it with any reasoning what-soever when we try to dig in", NewClosedCompoundsLintGroup(), "and literally not couching it with any reasoning whatsoever when we try to dig in")
}

func TestClosedCompounds_WhatsoSpEver(t *testing.T) {
	linttest.AssertSuggestionResult(t, "At one point in time the engine would refuse to create sites, but not give any errors whatso ever.", NewClosedCompoundsLintGroup(), "At one point in time the engine would refuse to create sites, but not give any errors whatsoever.")
}

func TestClosedCompounds_WhatsoHyEver(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Also I get no errors whatso-ever which makes this even more confusing.", NewClosedCompoundsLintGroup(), "Also I get no errors whatsoever which makes this even more confusing.")
}
