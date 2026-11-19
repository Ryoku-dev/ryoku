package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDayAndAge_AllowInThisDayAndAge(t *testing.T) {
	linttest.AssertNoLints(t, "I do belive in this day and age with the amount of printer on the market ", NewDayAndAge())
}

func TestDayAndAge_ForThisDayAndAgeSeemsToBeAcceptable(t *testing.T) {
	linttest.AssertNoLints(t, "As for my specs, I understand that my PC is quite underpowered for this day and age, but I'd say it's still within the hardware combos that ...", NewDayAndAge())
}

func TestDayAndAge_AtThisDayAndAge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How would one add diagnostics to the compiler at this day and age?", NewDayAndAge(), "How would one add diagnostics to the compiler in this day and age?")
}

func TestDayAndAge_ByThisDayInAge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Don't most people by this day in age, just have a spare laptop in their kitchens", NewDayAndAge(), "Don't most people in this day and age, just have a spare laptop in their kitchens")
}

func TestDayAndAge_InTheseDayAndAge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "still don't come with load sharing components built in these day and age.", NewDayAndAge(), "still don't come with load sharing components built in this day and age.")
}

func TestDayAndAge_InTheseDaysAndAge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But in these days and age floppies are replaced by USB flash drives.", NewDayAndAge(), "But in this day and age floppies are replaced by USB flash drives.")
}

func TestDayAndAge_InTheseDaysInAge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In these days in age, this is considered as 'heresy'.", NewDayAndAge(), "In this day and age, this is considered as 'heresy'.")
}

func TestDayAndAge_InThisDayAnAge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but in this day an age things progressed a tad so might it be the time for increasing it?", NewDayAndAge(), "but in this day and age things progressed a tad so might it be the time for increasing it?")
}

func TestDayAndAge_InThisDayAndAges(t *testing.T) {
	linttest.AssertSuggestionResult(t, "or at least it should be in this day and ages", NewDayAndAge(), "or at least it should be in this day and age")
}

func TestDayAndAge_InThisDayInAge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "or anything else that in this day in age is useful to have a reminder about", NewDayAndAge(), "or anything else that in this day and age is useful to have a reminder about")
}

func TestDayAndAge_InThisDaysAndAge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We as a whole realize that in this days and age being on social networking has got a sort of ...", NewDayAndAge(), "We as a whole realize that in this day and age being on social networking has got a sort of ...")
}

func TestDayAndAge_IsThisDayAndAgeTypo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Agreed, dark mode is a necessity is this day and age.", NewDayAndAge(), "Agreed, dark mode is a necessity in this day and age.")
}

func TestDayAndAge_ItThisDayAndAgeTypo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And it this day and age you really shouldn't but asking people to download random files", NewDayAndAge(), "And in this day and age you really shouldn't but asking people to download random files")
}

func TestDayAndAge_OfThisDayAndAge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it is completely incompatible with Juice Shop of this day and age", NewDayAndAge(), "it is completely incompatible with Juice Shop in this day and age")
}

func TestDayAndAge_ToThisDayAndAge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Still can't believe this has to be done in Safari to this day and age with responsive images", NewDayAndAge(), "Still can't believe this has to be done in Safari in this day and age with responsive images")
}

func TestDayAndAge_NoPrepThisDayInAge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "if that is how gpu programming is still done this day in age then id have a very hard time seeing valhalla ever run on a gpu", NewDayAndAge(), "if that is how gpu programming is still done in this day and age then id have a very hard time seeing valhalla ever run on a gpu")
}

func TestDayAndAge_NoPrepTheseDaysAndAges(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Btw I think you should write React the React Hooks way these days and ages, where you'll never see this keyword` again", NewDayAndAge(), "Btw I think you should write React the React Hooks way in this day and age, where you'll never see this keyword` again")
}

func TestDayAndAge_SinceIsAPrepositionButAlsoAConjunction(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and since these days and age storage is usually not a problem, I usually play it safe and just don't bother", NewDayAndAge(), "and since in this day and age storage is usually not a problem, I usually play it safe and just don't bother")
}

func TestDayAndAge_FixDayOfAge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If you want in this day of age an AI agent can probably implement what you are looking for.", NewDayAndAge(), "If you want in this day and age an AI agent can probably implement what you are looking for.")
}
