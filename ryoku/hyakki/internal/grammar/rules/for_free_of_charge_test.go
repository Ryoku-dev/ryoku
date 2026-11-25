package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestForFreeOfCharge_FixForFocQuestionMark(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Where to check my paper grammars for free of charge?", NewForFreeOfCharge(), []string{
		"Where to check my paper grammars for free?",
		"Where to check my paper grammars free of charge?",
	}, []string{})
}

func TestForFreeOfCharge_FixForFocFor(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "In Hungary, restaurants are required by law to provide tap water for free of charge for any customers upon request.", NewForFreeOfCharge(), []string{
		"In Hungary, restaurants are required by law to provide tap water for free for any customers upon request.",
		"In Hungary, restaurants are required by law to provide tap water free of charge for any customers upon request.",
	}, []string{})
}

func TestForFreeOfCharge_FixForFocCommaNot(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I provide this software in its entirety for free of charge, and support via GitHub is also free.", NewForFreeOfCharge(), []string{
		"I provide this software in its entirety for free, and support via GitHub is also free.",
		"I provide this software in its entirety free of charge, and support via GitHub is also free.",
	}, []string{})
}

func TestForFreeOfCharge_FixForFocQuote(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Copyright protects copying, not \"giving things away for free of charge\" or anything like that.", NewForFreeOfCharge(), []string{
		"Copyright protects copying, not \"giving things away for free\" or anything like that.",
		"Copyright protects copying, not \"giving things away free of charge\" or anything like that.",
	}, []string{})
}

func TestForFreeOfCharge_FixForFocOtherwise(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "For Wizzair, if you have purchased a separate ticket for your infant, then only car seat is allowed for free of charge otherwise its not.", NewForFreeOfCharge(), []string{
		"For Wizzair, if you have purchased a separate ticket for your infant, then only car seat is allowed for free otherwise its not.",
		"For Wizzair, if you have purchased a separate ticket for your infant, then only car seat is allowed free of charge otherwise its not.",
	}, []string{})
}

func TestForFreeOfCharge_FixForFocCommaBut(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "user may use the Software for free of charge, but the Software is basically paid software", NewForFreeOfCharge(), []string{
		"user may use the Software for free, but the Software is basically paid software",
		"user may use the Software free of charge, but the Software is basically paid software",
	}, []string{})
}

func TestForFreeOfCharge_FixForFocPeriod(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "...giving them away for free of charge. :-) SCNR", NewForFreeOfCharge(), []string{
		"...giving them away for free. :-) SCNR",
		"...giving them away free of charge. :-) SCNR",
	}, []string{})
}

func TestForFreeOfCharge_FixForFocAre(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Also what they offer for free of charge are very very powerful machines for unlimited amount of time.", NewForFreeOfCharge(), []string{
		"Also what they offer for free are very very powerful machines for unlimited amount of time.",
		"Also what they offer free of charge are very very powerful machines for unlimited amount of time.",
	}, []string{})
}

func TestForFreeOfCharge_AllowsFocAsAdjTier(t *testing.T) {
	linttest.AssertNoLints(t, "We prioritize interactive notebook compute for free of charge tier users.", NewForFreeOfCharge())
}

func TestForFreeOfCharge_AllowsFocAsAdjProduct(t *testing.T) {
	linttest.AssertNoLints(t, "This is a cost for free of charge product.", NewForFreeOfCharge())
}

func TestForFreeOfCharge_AllowsFocAsAdjFeedback(t *testing.T) {
	linttest.AssertNoLints(t, "... but if you are not a student in a university I don't know where you should turn for free of charge feedback", NewForFreeOfCharge())
}

func TestForFreeOfCharge_AllowsFocAsAdjPlace(t *testing.T) {
	linttest.AssertNoLints(t, "you see how many free places are in a particular garage, so you decide either to occupy it or search for free of charge place somewhere else", NewForFreeOfCharge())
}

func TestForFreeOfCharge_AllowsFocAdjEvents(t *testing.T) {
	linttest.AssertNoLints(t, "Create reservation for free-of-charge events should not require any approval of the reservation", NewForFreeOfCharge())
}

func TestForFreeOfCharge_AllowsWorkingForFreeOfCharge(t *testing.T) {
	linttest.AssertNoLints(t, "I have a client who I’m working for free of charge", NewForFreeOfCharge())
}
