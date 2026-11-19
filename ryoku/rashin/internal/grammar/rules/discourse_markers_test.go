package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDiscourseMarkers_CorrectsFrankly(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Frankly I think he is wrong.", NewDiscourseMarkers(), "Frankly, I think he is wrong.")
}

func TestDiscourseMarkers_CorrectsHowever(t *testing.T) {
	linttest.AssertSuggestionResult(t, "However I disagree with your conclusion.", NewDiscourseMarkers(), "However, I disagree with your conclusion.")
}

func TestDiscourseMarkers_CorrectsTherefore(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Therefore we must act now.", NewDiscourseMarkers(), "Therefore, we must act now.")
}

func TestDiscourseMarkers_CorrectsMeanwhile(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Meanwhile preparations continued in the background.", NewDiscourseMarkers(), "Meanwhile, preparations continued in the background.")
}

func TestDiscourseMarkers_CorrectsFurthermore(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Furthermore this approach reduces complexity.", NewDiscourseMarkers(), "Furthermore, this approach reduces complexity.")
}

func TestDiscourseMarkers_CorrectsNevertheless(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Nevertheless we persevered despite the odds.", NewDiscourseMarkers(), "Nevertheless, we persevered despite the odds.")
}

func TestDiscourseMarkers_CorrectsConsequently(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Consequently the system halted unexpectedly.", NewDiscourseMarkers(), "Consequently, the system halted unexpectedly.")
}

func TestDiscourseMarkers_CorrectsThus(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Thus we arrive at the final verdict.", NewDiscourseMarkers(), "Thus, we arrive at the final verdict.")
}

func TestDiscourseMarkers_AllowsThusFar(t *testing.T) {
	linttest.AssertNoLints(t, "Thus far there have been no problems.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_CorrectsInstead(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Instead he chose a different path.", NewDiscourseMarkers(), "Instead, he chose a different path.")
}

func TestDiscourseMarkers_CorrectsMoreover(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Moreover this solution is more efficient.", NewDiscourseMarkers(), "Moreover, this solution is more efficient.")
}

func TestDiscourseMarkers_CorrectsAlternatively(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Alternatively we could defer the decision.", NewDiscourseMarkers(), "Alternatively, we could defer the decision.")
}

func TestDiscourseMarkers_NoSuggestionIfCommaPresent(t *testing.T) {
	linttest.AssertNoLints(t, "However, I disagree with your point.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_NoLintForMidSentenceMarker(t *testing.T) {
	linttest.AssertNoLints(t, "I said however I would consider it.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_PreservesWhitespace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "However   I disagree.", NewDiscourseMarkers(), "However,   I disagree.")
}

func TestDiscourseMarkers_CorrectsSemicolonCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "However I disagree.", NewDiscourseMarkers(), "However, I disagree.")
}

func TestDiscourseMarkers_CorrectsMultipleSentences(t *testing.T) {
	linttest.AssertSuggestionResult(t, "However I disagree. Therefore I propose an alternative.", NewDiscourseMarkers(), "However, I disagree. Therefore, I propose an alternative.")
}

func TestDiscourseMarkers_AllowsSingleWordSentence(t *testing.T) {
	linttest.AssertNoLints(t, "Thus", NewDiscourseMarkers())
}

func TestDiscourseMarkers_CorrectsForExample(t *testing.T) {
	linttest.AssertSuggestionResult(t, "For example I recommend updating the configuration.", NewDiscourseMarkers(), "For example, I recommend updating the configuration.")
}

func TestDiscourseMarkers_NoSuggestionIfCommaAfterForExample(t *testing.T) {
	linttest.AssertNoLints(t, "For example, I recommend updating the configuration.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_PreservesWhitespaceForExample(t *testing.T) {
	linttest.AssertSuggestionResult(t, "For example   the outcome was unexpected.", NewDiscourseMarkers(), "For example,   the outcome was unexpected.")
}

func TestDiscourseMarkers_CorrectsOnTheOtherHand(t *testing.T) {
	linttest.AssertSuggestionResult(t, "On the other hand we could delay the deployment.", NewDiscourseMarkers(), "On the other hand, we could delay the deployment.")
}

func TestDiscourseMarkers_NoLintForMidSentenceOnTheOtherHand(t *testing.T) {
	linttest.AssertNoLints(t, "We might postpone, on the other hand this introduces risk.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_Check2966IsAvoided(t *testing.T) {
	linttest.AssertNoLints(t, "Honestly and graciously convince someone of something.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_CorrectsNowadays(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Nowadays E-Commerce is one of the hot topics.", NewDiscourseMarkers(), "Nowadays, E-Commerce is one of the hot topics.")
}

func TestDiscourseMarkers_NoSuggestionIfNowadaysNotAtStart(t *testing.T) {
	linttest.AssertNoLints(t, "You are the reason the tech state is awful nowadays", NewDiscourseMarkers())
}

func TestDiscourseMarkers_CorrectsNowADays(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Now a days Automation plays a crucial role in mankind.", NewDiscourseMarkers(), "Now a days, Automation plays a crucial role in mankind.")
}

func TestDiscourseMarkers_NoSuggestionIfCommaAfterNowadays(t *testing.T) {
	linttest.AssertNoLints(t, "Nowadays, data has become the most valuable part of any individual or organization.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_NoSuggestionIfCommaAfterNowADays(t *testing.T) {
	linttest.AssertNoLints(t, "Now a days, movie recommendation systems are well developed and are user focused.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_CorrectsAlso(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Also I think we should go.", NewDiscourseMarkers(), "Also, I think we should go.")
}

func TestDiscourseMarkers_NoSuggestionIfCommaAfterAlso(t *testing.T) {
	linttest.AssertNoLints(t, "Also, I think we should go.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_NoLintForMidSentenceAlso(t *testing.T) {
	linttest.AssertNoLints(t, "She also said hi.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_AllowsAlsoKnownAs(t *testing.T) {
	linttest.AssertNoLints(t, "Also known as the big one.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_AllowsAlsoCalled(t *testing.T) {
	linttest.AssertNoLints(t, "Also called the big one.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_AllowsAlsoSee(t *testing.T) {
	linttest.AssertNoLints(t, "Also see chapter two.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_AllowsAlsoAvailable(t *testing.T) {
	linttest.AssertNoLints(t, "Also available in blue.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_CorrectsActually(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Actually I don't know.", NewDiscourseMarkers(), "Actually, I don't know.")
}

func TestDiscourseMarkers_NoSuggestionIfCommaAfterActually(t *testing.T) {
	linttest.AssertNoLints(t, "Actually, I don't know.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_NoLintForMidSentenceActually(t *testing.T) {
	linttest.AssertNoLints(t, "She is actually right.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_AllowsActuallyExisting(t *testing.T) {
	linttest.AssertNoLints(t, "Actually existing socialism was a term.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_CorrectsAlsoAndActuallyInOneDocument(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Actually I don't know. Also I forgot.", NewDiscourseMarkers(), "Actually, I don't know. Also, I forgot.")
}

func TestDiscourseMarkers_CorrectsInAddition(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In addition we need to test this.", NewDiscourseMarkers(), "In addition, we need to test this.")
}

func TestDiscourseMarkers_AllowsInAdditionTo(t *testing.T) {
	linttest.AssertNoLints(t, "In addition to that we need to test this.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_CorrectsEvenSo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Even so the plan failed.", NewDiscourseMarkers(), "Even so, the plan failed.")
}

func TestDiscourseMarkers_AllowsEvenSoMuchAs(t *testing.T) {
	linttest.AssertNoLints(t, "Even so much as a whisper would wake him.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_CorrectsThatSaid(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That said we should wait.", NewDiscourseMarkers(), "That said, we should wait.")
}

func TestDiscourseMarkers_CorrectsInOtherWords(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In other words we should go.", NewDiscourseMarkers(), "In other words, we should go.")
}

func TestDiscourseMarkers_AllowsInOtherWordsAndPhrases(t *testing.T) {
	linttest.AssertNoLints(t, "In other words and phrases, keep it short.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_CorrectsByTheWay(t *testing.T) {
	linttest.AssertSuggestionResult(t, "By the way I saw him yesterday.", NewDiscourseMarkers(), "By the way, I saw him yesterday.")
}

func TestDiscourseMarkers_AllowsByWayOf(t *testing.T) {
	linttest.AssertNoLints(t, "By way of introduction he waved.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_CorrectsBesides(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Besides we have no time.", NewDiscourseMarkers(), "Besides, we have no time.")
}

func TestDiscourseMarkers_CorrectsBesidesBeforeList(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Besides we need milk, eggs, and bread.", NewDiscourseMarkers(), "Besides, we need milk, eggs, and bread.")
}

func TestDiscourseMarkers_AllowsBesidesAsPreposition(t *testing.T) {
	linttest.AssertNoLints(t, "Besides me, nobody knew the answer.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_CorrectsNotwithstanding(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Notwithstanding they still went out.", NewDiscourseMarkers(), "Notwithstanding, they still went out.")
}

func TestDiscourseMarkers_AllowsNotwithstandingAsPreposition(t *testing.T) {
	linttest.AssertNoLints(t, "Notwithstanding the rain, we went out.", NewDiscourseMarkers())
}

func TestDiscourseMarkers_CorrectsOtherMarkersBeforeDeterminer(t *testing.T) {
	linttest.AssertSuggestionResult(t, "However the rain held off.", NewDiscourseMarkers(), "However, the rain held off.")
}
