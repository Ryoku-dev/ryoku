package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMassNounsNounCountability_CorrectsA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If the unit turns out to be noisy, can I expect a firmware with phase ...", NewNounCountability(), "If the unit turns out to be noisy, can I expect some firmware with phase ...")
}

func TestMassNounsNounCountability_CorrectsATitleCase(t *testing.T) {
	t.Skip("replace_with_match_case matches by index, not by lower vs title vs upper")
	linttest.AssertSuggestionResult(t, "Simple POC of a Ransomware.", NewNounCountability(), "Simple POC of a piece of Ransomware.")
}

func TestMassNounsNounCountability_CorrectsAn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The PlaySEM platform provides an infrastructure for playing and rendering sensory effects in multimedia applications.", NewNounCountability(), "The PlaySEM platform provides infrastructure for playing and rendering sensory effects in multimedia applications.")
}

func TestMassNounsNounCountability_CorrectsAnTitleCase(t *testing.T) {
	t.Skip("replace_with_match_case matches by index, not by lower vs title vs upper")
	linttest.AssertSuggestionResult(t, "An Infrastructure for Integrated EDA.", NewNounCountability(), "Infrastructure for Integrated EDA.")
}

func TestMassNounsNounCountability_CorrectsAnother(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Another ransomware made by me for fun.", NewNounCountability(), "Another piece of ransomware made by me for fun.")
}

func TestMassNounsNounCountability_CorrectsBoth(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Make a terminal show both information of your CPU and GPU!", NewNounCountability(), "Make a terminal show both pieces of information of your CPU and GPU!")
}

func TestMassNounsNounCountability_CanCorrectEachWithTraffic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Beside each traffic there is also a pedestrian traffic light.", NewNounCountability(), "Beside each traffic there is also a pedestrian traffic light.")
}

func TestMassNounsNounCountability_CorrectsEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Capacitor plugin to get access to every info about the device software and hardware.", NewNounCountability(), "Capacitor plugin to get access to every piece of info about the device software and hardware.")
}

func TestMassNounsNounCountability_CorrectsFew(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Displays a few information to help you rotating through your spells.", NewNounCountability(), "Displays a few pieces of information to help you rotating through your spells.")
}

func TestMassNounsNounCountability_CorrectsMany(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It shows clearly how many information about objects you can get with old search ...", NewNounCountability(), "It shows clearly how much information about objects you can get with old search ...")
}

func TestMassNounsNounCountability_CorrectsOne(t *testing.T) {
	linttest.AssertSuggestionResult(t, "For example, it only makes sense to compare global protein q-value filtering in one software with that in another.", NewNounCountability(), "For example, it only makes sense to compare global protein q-value filtering in one application with that in another.")
}

func TestMassNounsNounCountability_CorrectsSeveral(t *testing.T) {
	t.Skip("'in' = noun because conflated with 'IN' (Indiana)")
	linttest.AssertSuggestionResult(t, "The program takes in input a single XML file and outputs several info in different files.", NewNounCountability(), "The program takes in input a single XML file and outputs several pieces of info in different files.")
}

func TestMassNounsNounCountability_DontCorrectManyCompound(t *testing.T) {
	linttest.AssertLintCount(t, "Additionally, many software development platforms also provide access to a community of developers.", NewNounCountability(), 0)
}

func TestMassNounsNounCountability_DontFlagIngModifierBeforeAdjectiveNoun(t *testing.T) {
	linttest.AssertNoLints(t, "They succeeded by receiving a working direct link in return.", NewNounCountability())
}

func TestMassNounsNounCountability_DontFlagSlashSeparatedIngModifiersBeforeNoun(t *testing.T) {
	linttest.AssertNoLints(t, "MockAPI is meant to be a prototyping/testing/learning tool.", NewNounCountability())
}

func TestMassNounsNounCountability_DontFlagCoordinatePredicateIngAdjective(t *testing.T) {
	linttest.AssertNoLints(t, "They're both confusing and tedious to use.", NewNounCountability())
}

func TestMassNounsNounCountability_DontCorrectFirstDoCorrectSecond(t *testing.T) {
	t.Skip("Ignored upstream.")
	linttest.AssertSuggestionResult(t, "A advice description is required for each advice.", NewNounCountability(), "A advice description is required for each piece of advice.")
}

func TestMassNounsNounCountability_CorrectsAnAdvice(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Origin will not always provide the right method when an advice is applied to a bridged method.", NewNounCountability(), "Origin will not always provide the right method when an tip is applied to a bridged method.")
}

func TestMassNounsNounCountability_CorrectsOneAdvice(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is it possible to use more than one advice on the same method?", NewNounCountability(), "Is it possible to use more than one tip on the same method?")
}

func TestMassNounsNounCountability_CorrectsEveryAdvice(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ideally every advice would have a unique identifier.", NewNounCountability(), "Ideally every tip would have a unique identifier.")
}

func TestMassNounsNounCountability_CorrectsAAdvice(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hello! I need a advice.", NewNounCountability(), "Hello! I need a tip.")
}

func TestMassNounsNounCountability_CorrectsASoftware(t *testing.T) {
	linttest.AssertSuggestionResult(t, "HGroup-DIA, a software for analyzing multiple DIA data files.", NewNounCountability(), "HGroup-DIA, a software package for analyzing multiple DIA data files.")
}

func TestMassNounsNounCountability_CorrectsALuggage(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A luggage with a little engine, sensors (gps, ultrasounds, etc...) and bluetooth connection that will follow you everywhere.", NewNounCountability(), "A suitcase with a little engine, sensors (gps, ultrasounds, etc...) and bluetooth connection that will follow you everywhere.")
}

func TestMassNounsNounCountability_CorrectsMultipleAdvice(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Update Advice API doc for event and data params, multiple advice.", NewNounCountability(), "Update Advice API doc for event and data params, multiple suggestions.")
}

func TestMassNounsNounCountability_CorrectsEverySoftware(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Rewrite every software known to man in Rust.", NewNounCountability(), "Rewrite every application known to man in Rust.")
}

func TestMassNounsNounCountability_CorrectsEachFurniture(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the position (x, y) and size (height, width, length) of each furniture", NewNounCountability(), "the position (x, y) and size (height, width, length) of each piece of furniture")
}

func TestMassNounsNounCountability_CorrectsOneClothing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Each list element represents one clothing based on weather conditions.", NewNounCountability(), "Each list element represents one garment based on weather conditions.")
}

func TestMassNounsNounCountability_DontFlagCompoundNouns(t *testing.T) {
	linttest.AssertLintCount(t, "Fill in the blanks following the creation of each Furniture class instance.", NewNounCountability(), 0)
	linttest.AssertLintCount(t, "This project is a clothing shop that let users buy and pay for they purchases.", NewNounCountability(), 0)
	linttest.AssertLintCount(t, "Yet another software router.", NewNounCountability(), 0)
	linttest.AssertLintCount(t, "Calculate a rate for every software component.", NewNounCountability(), 0)
}

func TestMassNounsNounCountability_CorrectsFewer(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Why do my packages have fewer information?", NewNounCountability(), "Why do my packages have less information?")
}

func TestMassNounsNounCountability_DontFlagFewerInCompoundNoun(t *testing.T) {
	linttest.AssertLintCount(t, "Additionally, less traffic leads to fewer traffic jams, resulting in a more fluent, thus more efficient, trip.", NewNounCountability(), 0)
}

func TestMassNounsNounCountability_DontFlagMassNounPartOfHyphenatedCompound(t *testing.T) {
	linttest.AssertLintCount(t, "Internally, we have a hardware-in-the-loop Jenkins test suite that builds and unit tests the various processes.", NewNounCountability(), 0)
}

func TestMassNounsNounCountability_CorrectsPunctuation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Not in this form because it currently works with one punctuation with one letter either side.", NewNounCountability(), "Not in this form because it currently works with one punctuation mark with one letter either side.")
}

func TestMassNounsNounCountability_DontFlagASoftwareRenderedGame(t *testing.T) {
	linttest.AssertNoLints(t, "There was a software rendered game in vein of Tomb Raider", NewNounCountability())
}
