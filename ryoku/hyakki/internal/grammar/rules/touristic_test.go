package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTouristic_FixesTouristicAlone(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "touristic", NewTouristic(), []string{"tourist", "tourism", "touristy"}, []string{})
}

func TestTouristic_FixesVeryT(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "very touristic", NewTouristic(), []string{"very touristy"}, []string{"very tourist", "very tourism"})
}

func TestTouristic_FixesTLocationGoodAndBad(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "touristic location", NewTouristic(), []string{"tourist location", "tourism location", "touristy location"}, []string{})
}

func TestTouristic_FixesIsT(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "That place is touristic", NewTouristic(), []string{"That place is touristy"}, []string{"That place is tourist", "That place is tourism"})
}

func TestTouristic_FixesTInformation(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "The AI Touristic Information Tool for Liquid Galaxy is a Flutter-based Android tablet application that simplifies and enhances travel planning.", NewTouristic(), []string{
		"The AI Tourist Information Tool for Liquid Galaxy is a Flutter-based Android tablet application that simplifies and enhances travel planning.",
		"The AI Tourism Information Tool for Liquid Galaxy is a Flutter-based Android tablet application that simplifies and enhances travel planning.",
	}, []string{
		"The AI Touristy Information Tool for Liquid Galaxy is a Flutter-based Android tablet application that simplifies and enhances travel planning.",
	})
}

func TestTouristic_FixesTData(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Official API to access Apidae touristic data.", NewTouristic(), []string{
		"Official API to access Apidae tourist data.",
		"Official API to access Apidae tourism data.",
	}, []string{"Official API to access Apidae touristy data."})
}

func TestTouristic_CorrectsTInformation2(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Oppidums is open source app that provide cultural, historical and touristic information on different cities.", NewTouristic(), []string{
		"Oppidums is open source app that provide cultural, historical and tourist information on different cities.",
		"Oppidums is open source app that provide cultural, historical and tourism information on different cities.",
	}, []string{
		"Oppidums is open source app that provide cultural, historical and touristy information on different cities.",
	})
}

func TestTouristic_CorrectsVeryTSpot(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "The destination is a very touristic spot, many people visit this place at the weekend.", NewTouristic(), []string{
		"The destination is a very touristy spot, many people visit this place at the weekend.",
	}, []string{
		"The destination is a very tourist spot, many people visit this place at the weekend.",
		"The destination is a very tourism spot, many people visit this place at the weekend.",
	})
}

func TestTouristic_FixesTPlatform(t *testing.T) {
	t.Skip("Checks previous word but results depend on the next word")
	linttest.AssertGoodAndBadSuggestions(t, "Incuti is touristic platform for African destinations.", NewTouristic(), []string{
		"Incuti is tourist platform for African destinations.",
		"Incuti is tourism platform for African destinations.",
	}, []string{"Incuti is touristy platform for African destinations."})
}

func TestTouristic_FixesTServiceProviders(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Onlim API is a tool that touristic service providers utilize to generate social media posts by injecting data about their offers into some templates.", NewTouristic(), []string{
		"Onlim API is a tool that tourist service providers utilize to generate social media posts by injecting data about their offers into some templates.",
		"Onlim API is a tool that tourism service providers utilize to generate social media posts by injecting data about their offers into some templates.",
	}, []string{
		"Onlim API is a tool that touristy service providers utilize to generate social media posts by injecting data about their offers into some templates.",
	})
}

func TestTouristic_FixesAreTAreas(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "We can determine that most of the busier areas are touristic areas, which in return helps with the high demand for the shared bikes.", NewTouristic(), []string{
		"We can determine that most of the busier areas are tourist areas, which in return helps with the high demand for the shared bikes.",
		"We can determine that most of the busier areas are tourism areas, which in return helps with the high demand for the shared bikes.",
		"We can determine that most of the busier areas are touristy areas, which in return helps with the high demand for the shared bikes.",
	}, []string{})
}

func TestTouristic_FixesVeryTArea(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "This is Manhattan, a very popular, very touristic area of New York.", NewTouristic(), []string{"This is Manhattan, a very popular, very touristy area of New York."}, []string{
		"This is Manhattan, a very popular, very tourist area of New York.",
		"This is Manhattan, a very popular, very tourism area of New York.",
	})
}

func TestTouristic_FixesForTPhotographic(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Python implementation of my clustering-based recommendation system for touristic photographic spots.", NewTouristic(), []string{
		"Python implementation of my clustering-based recommendation system for touristy photographic spots.",
	}, []string{
		"Python implementation of my clustering-based recommendation system for tourist photographic spots.",
		"Python implementation of my clustering-based recommendation system for tourism photographic spots.",
	})
}

func TestTouristic_FixesCzechTRoutes(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Management and Control Application for Czech Touristic Routes in OSM.", NewTouristic(), []string{
		"Management and Control Application for Czech Tourist Routes in OSM.",
		"Management and Control Application for Czech Tourism Routes in OSM.",
		"Management and Control Application for Czech Touristy Routes in OSM.",
	}, []string{})
}

func TestTouristic_FixesPromoteTActivities(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Application to promote touristic activities in Valencia.", NewTouristic(), []string{
		"Application to promote tourist activities in Valencia.",
		"Application to promote tourism activities in Valencia.",
		"Application to promote touristy activities in Valencia.",
	}, []string{})
}

func TestTouristic_FixesATFlutter(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "A Touristic Flutter App.", NewTouristic(), []string{"A Tourist Flutter App.", "A Tourism Flutter App."}, []string{})
}

func TestTouristic_FixesOfTInterest(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "ARCHEO: a python lib for sound event detection in areas of touristic Interest.", NewTouristic(), []string{
		"ARCHEO: a python lib for sound event detection in areas of tourist Interest.",
		"ARCHEO: a python lib for sound event detection in areas of tourism Interest.",
	}, []string{"ARCHEO: a python lib for sound event detection in areas of touristy Interest."})
}

func TestTouristic_FixesTEstablishments(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Touristic establishments by EUROSTAT NUTS regions.", NewTouristic(), []string{
		"Tourist establishments by EUROSTAT NUTS regions.",
		"Tourism establishments by EUROSTAT NUTS regions.",
	}, []string{"Touristy establishments by EUROSTAT NUTS regions."})
}
