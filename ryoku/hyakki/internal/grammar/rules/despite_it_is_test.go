package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDespiteItIs_DespiteIAm(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Cronicle shuts down randomly despite I am running simple Python scripts via \"Test Plugin\"", NewDespiteItIs(), []string{
		"Cronicle shuts down randomly despite me running simple Python scripts via \"Test Plugin\"",
		"Cronicle shuts down randomly despite my running simple Python scripts via \"Test Plugin\"",
	}, []string{})
}

func TestDespiteItIs_DespiteItIsAvailable(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Actual behavior Extension not installed despite it is available in PECL", NewDespiteItIs(), []string{
		"Actual behavior Extension not installed despite being available in PECL",
		"Actual behavior Extension not installed despite it being available in PECL",
		"Actual behavior Extension not installed despite its being available in PECL",
	}, []string{})
}

func TestDespiteItIs_DespiteItIsDetected(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "FP2 not detected despite it is detected - split brain?", NewDespiteItIs(), []string{
		"FP2 not detected despite being detected - split brain?",
		"FP2 not detected despite it being detected - split brain?",
		"FP2 not detected despite its being detected - split brain?",
	}, []string{})
}

func TestDespiteItIs_DespiteIAmIn(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "My application was rejected due to location basis despite I am in the same city as my campus.", NewDespiteItIs(), []string{
		"My application was rejected due to location basis despite me being in the same city as my campus.",
		"My application was rejected due to location basis despite my being in the same city as my campus.",
	}, []string{})
}

func TestDespiteItIs_DespiteItWasNot(t *testing.T) {
	t.Skip("Upstream ignored: negatives are not handled yet")
	t.Skip("negatives are not handled yet")
	linttest.AssertGoodAndBadSuggestions(t, "despite it was not able to fulfill desired ordering with these modules", NewDespiteItIs(), []string{
		"despite it not being able to fulfill desired ordering with these modules",
		"despite its not being able to fulfill desired ordering with these modules",
		"despite not being able to fulfill desired ordering with these modules",
	}, []string{})
}

func TestDespiteItIs_DespiteWeAreUsing(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "However, GFW still can decode the content despite we are using overlapped ip fragmentation.", NewDespiteItIs(), []string{
		"However, GFW still can decode the content despite us using overlapped ip fragmentation.",
		"However, GFW still can decode the content despite our using overlapped ip fragmentation.",
	}, []string{})
}

func TestDespiteItIs_DespiteTheyAreAlready(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "v5.7.2 keeps adding temperature commands on start_gcode despite they are already present", NewDespiteItIs(), []string{
		"v5.7.2 keeps adding temperature commands on start_gcode despite them being already present",
		"v5.7.2 keeps adding temperature commands on start_gcode despite their being already present",
	}, []string{})
}

func TestDespiteItIs_DespiteItWasRemoved(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Freshwater Research Station is selectable as starting location despite it was removed by Dark Days of the Dead mod", NewDespiteItIs(), []string{
		"Freshwater Research Station is selectable as starting location despite being removed by Dark Days of the Dead mod",

		"Freshwater Research Station is selectable as starting location despite it being removed by Dark Days of the Dead mod",

		"Freshwater Research Station is selectable as starting location despite its being removed by Dark Days of the Dead mod",
	}, []string{})
}

func TestDespiteItIs_IgnoreDespiteTheyShouldnt(t *testing.T) {
	linttest.AssertNoLints(t, "Some tools and gears have attack damage values despite they shouldn't", NewDespiteItIs())
}

func TestDespiteItIs_IgnoreDespiteIWasPlaying(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "it showed me Maria despite I was playing someone else", NewDespiteItIs(), []string{
		"it showed me Maria despite me playing someone else",
		"it showed me Maria despite my playing someone else",
	}, []string{})
}

func TestDespiteItIs_IgnoreDespiteTheyWereValid(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "You'll get pages that becomes invalid with time despite they were valid before", NewDespiteItIs(), []string{
		"You'll get pages that becomes invalid with time despite being valid before",
		"You'll get pages that becomes invalid with time despite them being valid before",
		"You'll get pages that becomes invalid with time despite their being valid before",
	}, []string{})
}
