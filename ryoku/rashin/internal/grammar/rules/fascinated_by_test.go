package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestFascinatedBy_FixAmiga(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Now, one aspect of the Amiga that I've always been fascinated about is making my own games for the Amiga.", NewFascinatedBy(), []string{
		"Now, one aspect of the Amiga that I've always been fascinated by is making my own games for the Amiga.",
		"Now, one aspect of the Amiga that I've always been fascinated with is making my own games for the Amiga.",
	}, []string{})
}

func TestFascinatedBy_FixMicrobit(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "also why I am very fascinated about the micro:bit itself", NewFascinatedBy(), []string{
		"also why I am very fascinated by the micro:bit itself",
		"also why I am very fascinated with the micro:bit itself",
	}, []string{})
}

func TestFascinatedBy_FixSoftwareDevelopment(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Self-learner, fascinated about software development, especially computer graphics and web - marcus-phi.", NewFascinatedBy(), []string{
		"Self-learner, fascinated by software development, especially computer graphics and web - marcus-phi.",
		"Self-learner, fascinated with software development, especially computer graphics and web - marcus-phi.",
	}, []string{})
}

func TestFascinatedBy_FixComputerScience(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Fascinated about Computer Science, Finance and Statistics.", NewFascinatedBy(), []string{
		"Fascinated by Computer Science, Finance and Statistics.",
		"Fascinated with Computer Science, Finance and Statistics.",
	}, []string{})
}

func TestFascinatedBy_FixPossibilities(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "m relatively new to deCONZ and Conbee2 but already very fascinated about the possibilities compared to Philips and Ikea's", NewFascinatedBy(), []string{
		"m relatively new to deCONZ and Conbee2 but already very fascinated by the possibilities compared to Philips and Ikea's",
		"m relatively new to deCONZ and Conbee2 but already very fascinated with the possibilities compared to Philips and Ikea's",
	}, []string{})
}

func TestFascinatedBy_FixProject(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I have been using browser use in local mode for a while and i am pretty fascinated about the project.", NewFascinatedBy(), []string{
		"I have been using browser use in local mode for a while and i am pretty fascinated by the project.",
		"I have been using browser use in local mode for a while and i am pretty fascinated with the project.",
	}, []string{})
}

func TestFascinatedBy_FixWork(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Hey guys, I am really fascinated about your work and I tried to build Magisk so I will be able to contribute for the project.", NewFascinatedBy(), []string{
		"Hey guys, I am really fascinated by your work and I tried to build Magisk so I will be able to contribute for the project.",
		"Hey guys, I am really fascinated with your work and I tried to build Magisk so I will be able to contribute for the project.",
	}, []string{})
}

func TestFascinatedBy_FixAis(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I am a retired Dutch telecom engineer and fascinated about AIS applications.", NewFascinatedBy(), []string{
		"I am a retired Dutch telecom engineer and fascinated by AIS applications.",
		"I am a retired Dutch telecom engineer and fascinated with AIS applications.",
	}, []string{})
}

func TestFascinatedBy_FixInnovativeIdeas(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Software Developer fascinated about innovative ideas, love to learn and share new technologies and ideas.", NewFascinatedBy(), []string{
		"Software Developer fascinated by innovative ideas, love to learn and share new technologies and ideas.",
		"Software Developer fascinated with innovative ideas, love to learn and share new technologies and ideas.",
	}, []string{})
}

func TestFascinatedBy_FixCoding(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "m fascinated about coding and and sharing my code to the world.", NewFascinatedBy(), []string{
		"m fascinated by coding and and sharing my code to the world.",
		"m fascinated with coding and and sharing my code to the world.",
	}, []string{})
}
