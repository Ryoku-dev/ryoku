package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestToTwoToo_FixesToAmbitious(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The project scope is to ambitious", NewToTwoToo(), "The project scope is too ambitious")
}

func TestToTwoToo_FixesEndOfSent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She wants ice cream, to.", NewToTwoToo(), "She wants ice cream, too.")
}

func TestToTwoToo_FlagsToHungry(t *testing.T) {
	linttest.AssertLintCount(t, "I am to hungry.", NewToTwoToo(), 1)
}

func TestToTwoToo_NoLintOnProperToo(t *testing.T) {
	linttest.AssertNoLints(t, "I am too hungry.", NewToTwoToo())
}

func TestToTwoToo_FlagsToWithIrregularWhitespace(t *testing.T) {
	linttest.AssertLintCount(t, "She was to\t   tired.", NewToTwoToo(), 1)
	linttest.AssertLintCount(t, "He felt it was\nto cold.", NewToTwoToo(), 1)
}

func TestToTwoToo_FlagsToWithTrailingPunct(t *testing.T) {
	linttest.AssertLintCount(t, "He spoke to loud!", NewToTwoToo(), 1)
	linttest.AssertLintCount(t, "He spoke to loud?", NewToTwoToo(), 1)
	linttest.AssertLintCount(t, "He spoke to loud.", NewToTwoToo(), 1)
}

func TestToTwoToo_NoLintToEat(t *testing.T) {
	linttest.AssertNoLints(t, "Please remember to eat your vegetables.", NewToTwoToo())
}

func TestToTwoToo_NoLintToNashvilleOrYou(t *testing.T) {
	linttest.AssertNoLints(t, "I’m going to Nashville next week.", NewToTwoToo())
	linttest.AssertNoLints(t, "Talk to you later.", NewToTwoToo())
}

func TestToTwoToo_NoLintDistanceFromCenter(t *testing.T) {
	linttest.AssertNoLints(t, "Distance from the center to any face", NewToTwoToo())
}

func TestToTwoToo_NoLintToAudio(t *testing.T) {
	linttest.AssertNoLints(t, "Convert a book to audio.", NewToTwoToo())
}

func TestToTwoToo_FixesTooGo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I want too go abroad.", NewToTwoToo(), "I want to go abroad.")
}

func TestToTwoToo_FixesTooHim(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Give it too him as a gift", NewToTwoToo(), "Give it to him as a gift")
}

func TestToTwoToo_FixesTooThe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We're going too the conference.", NewToTwoToo(), "We're going to the conference.")
}

func TestToTwoToo_FixesTooA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We're going too a concert.", NewToTwoToo(), "We're going to a concert.")
}

func TestToTwoToo_FixesToHard(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's not to hard, is it?", NewToTwoToo(), "It's not too hard, is it?")
}

func TestToTwoToo_NoLintTooHot(t *testing.T) {
	linttest.AssertNoLints(t, "The coffee is too hot to drink.", NewToTwoToo())
}

func TestToTwoToo_NoLintTooLoud(t *testing.T) {
	linttest.AssertNoLints(t, "The music was too loud, making it hard to hear.", NewToTwoToo())
}

func TestToTwoToo_NoLintTooShy(t *testing.T) {
	linttest.AssertNoLints(t, "He's too shy to speak in public.", NewToTwoToo())
}

func TestToTwoToo_NoLintTooSweet(t *testing.T) {
	linttest.AssertNoLints(t, "The cake is too sweet for my taste.", NewToTwoToo())
}

func TestToTwoToo_NoLintTooExpensive(t *testing.T) {
	linttest.AssertNoLints(t, "It's too expensive for me to buy right now.", NewToTwoToo())
}

func TestToTwoToo_NoLintTooHard(t *testing.T) {
	linttest.AssertNoLints(t, "She worked too hard and ended up getting sick.", NewToTwoToo())
}

func TestToTwoToo_NoLintTooComplicated(t *testing.T) {
	linttest.AssertNoLints(t, "The instructions were too complicated to understand.", NewToTwoToo())
}

func TestToTwoToo_NoLintTooToo(t *testing.T) {
	linttest.AssertNoLints(t, "I like apples, and my brother does too.", NewToTwoToo())
}

func TestToTwoToo_NoLintTooToo2(t *testing.T) {
	linttest.AssertNoLints(t, "She's coming to the party, and he is too.", NewToTwoToo())
}

func TestToTwoToo_NoLintTooToo3(t *testing.T) {
	linttest.AssertNoLints(t, "I want to go to the beach, and you do too?", NewToTwoToo())
}

func TestToTwoToo_NoLintTooToo4(t *testing.T) {
	linttest.AssertNoLints(t, "He's a talented musician, and a great friend too.", NewToTwoToo())
}

func TestToTwoToo_NoLintTooToo5(t *testing.T) {
	linttest.AssertNoLints(t, "The movie was good, and the popcorn was delicious too.", NewToTwoToo())
}

func TestToTwoToo_NoLintTooDifficultTooClose(t *testing.T) {
	linttest.AssertNoLints(t, "The problem is too difficult, and the deadline is too close.", NewToTwoToo())
}

func TestToTwoToo_NoLintTooGoodTooNice(t *testing.T) {
	linttest.AssertNoLints(t, "He's too good at the game, and he's too nice to win.", NewToTwoToo())
}

func TestToTwoToo_AllowYoungMusicians(t *testing.T) {
	linttest.AssertNoLints(t, "Bringing Hope and Opportunity to Young Musicians", NewToTwoToo())
}

func TestToTwoToo_AllowSemicolon(t *testing.T) {
	linttest.AssertNoLints(t, "Attendees can look forward to:", NewToTwoToo())
}

func TestToTwoToo_AllowBuildBrighter(t *testing.T) {
	linttest.AssertNoLints(t, "We're empowering them to build brighter futures.", NewToTwoToo())
}

func TestToTwoToo_AllowDelegate(t *testing.T) {
	linttest.AssertNoLints(t, "I’d like you to consciously delegate one task", NewToTwoToo())
}

func TestToTwoToo_NoLintSoundscapes(t *testing.T) {
	linttest.AssertNoLints(t, "Soundscapes are not merely environmental features; they are integral to human identity and cultural expression.", NewToTwoToo())
}

func TestToTwoToo_NoLintSpeedFlexibility(t *testing.T) {
	linttest.AssertNoLints(t, "Its speed, flexibility, and seamless integration with FZF make it a compelling alternative to traditional fuzzy finding solutions.", NewToTwoToo())
}

func TestToTwoToo_NoLintExplicitlyCast(t *testing.T) {
	linttest.AssertNoLints(t, "Attempted to explicitly cast the result back to a string", NewToTwoToo())
}

func TestToTwoToo_NoLintBuriedUnderData(t *testing.T) {
	linttest.AssertNoLints(t, "They felt buried under the data, unable to proactively address emerging threats.", NewToTwoToo())
}

func TestToTwoToo_NoLintFamiliarize(t *testing.T) {
	linttest.AssertNoLints(t, "Familiarize yourself with these resources to learn how to effectively utilize the plugin’s features.", NewToTwoToo())
}

func TestToTwoToo_NoLintGreatDealOfEnergy(t *testing.T) {
	linttest.AssertNoLints(t, "It takes a great deal of energy to consistently operate under that kind of pressure.", NewToTwoToo())
}

func TestToTwoToo_NoLintOccasionallyTroubleshoot(t *testing.T) {
	linttest.AssertNoLints(t, "Just be prepared to occasionally troubleshoot the debugger itself.", NewToTwoToo())
}

func TestToTwoToo_CcoveilleSuggestion(t *testing.T) {
	linttest.AssertNoLints(t, "He goes too far with bets.", NewToTwoToo())
}

func TestToTwoToo_NoLintAutoDetectDebuggers(t *testing.T) {
	linttest.AssertNoLints(t, "Daprio attempts to auto-detect debugger servers and configurations, which can save significant time, especially for common languages.", NewToTwoToo())
}

func TestToTwoToo_NoLintCommitmentOpenSource(t *testing.T) {
	linttest.AssertNoLints(t, "I believe a commitment to open-source solutions and internal skill development would ultimately yield a more sustainable and ethical approach.", NewToTwoToo())
}

func TestToTwoToo_NoLintFeelingConfidentDominate(t *testing.T) {
	linttest.AssertNoLints(t, "I'm feeling confident, and I suspect you all should be too – because I’m about to dominate.", NewToTwoToo())
}

func TestToTwoToo_NoLintEgyptianSmilingFacesCommentary(t *testing.T) {
	linttest.AssertNoLints(t, "Today I learned that the ubiquitous, seemingly cheerful faces carved into ancient Egyptian relief sculptures – often referred to as “smiling faces” – weren’t simply a stylistic choice reflecting happiness. Recent scholarship suggests they functioned as a subtle, often satirical, form of social commentary, particularly targeting individuals who were arrogant, boastful, or otherwise deserving of ridicule.", NewToTwoToo())
}

func TestToTwoToo_NoLintIntendedToLeaveItTo(t *testing.T) {
	linttest.AssertNoLints(t, "Beatrice never explicitly said who she intended to leave it to.", NewToTwoToo())
}

func TestToTwoToo_NoLintTimeForGoodGirlToBed(t *testing.T) {
	linttest.AssertNoLints(t, "Time for this good girl to go to bed.", NewToTwoToo())
}

func TestToTwoToo_NoLintConnectedToManyOtherFields(t *testing.T) {
	linttest.AssertNoLints(t, "The study is connected to many other fields.", NewToTwoToo())
}

func TestToTwoToo_NoLintTillSheTooBeganDreaming(t *testing.T) {
	linttest.AssertNoLints(t, "till she too began dreaming after a fashion", NewToTwoToo())
}

func TestToTwoToo_NoLintToQuicklyFindAFactory(t *testing.T) {
	linttest.AssertNoLints(t, "To quickly find a factory, look for a map.", NewToTwoToo())
}

func TestToTwoToo_NoLintAskingToSimplyInScareQuotes(t *testing.T) {
	linttest.AssertNoLints(t, "He was asking to simply \"show up\" for the meeting.", NewToTwoToo())
}

func TestToTwoToo_NoLintLlmAsJudgeToAutomaticallyScore(t *testing.T) {
	linttest.AssertNoLints(t, "We used an LLM-as-judge to automatically score agent trajectories.", NewToTwoToo())
}

func TestToTwoToo_NoLintAllTheWayToAdvancedUsage(t *testing.T) {
	linttest.AssertNoLints(t, "All the way to advanced usage, like an expert.", NewToTwoToo())
}

func TestToTwoToo_NoLintAccessToOver400Integrations(t *testing.T) {
	linttest.AssertNoLints(t, "You'll have access to over 400 integrations.", NewToTwoToo())
}

func TestToTwoToo_NoLintAccustomedToPrecision(t *testing.T) {
	linttest.AssertNoLints(t, "I’m rather accustomed to precision.", NewToTwoToo())
}

func TestToTwoToo_NoLintProneToMelancholy(t *testing.T) {
	linttest.AssertNoLints(t, "He wasn’t a man prone to melancholy.", NewToTwoToo())
}

func TestToTwoToo_NoLintSuperlativeRange(t *testing.T) {
	linttest.AssertNoLints(t, "Sort speeds from slowest to fastest.", NewToTwoToo())
}

func TestToTwoToo_NoLintComparativeRange(t *testing.T) {
	linttest.AssertNoLints(t, "We rank tasks from harder to easier.", NewToTwoToo())
}

func TestToTwoToo_StillLintsPositiveAdjectiveEnd(t *testing.T) {
	linttest.AssertLintCount(t, "The room felt to cold.", NewToTwoToo(), 1)
}

func TestToTwoToo_NoLintToOnly(t *testing.T) {
	linttest.AssertNoLints(t, "The dog was restricted to only one can of food.", NewToTwoToo())
}

func TestToTwoToo_NoLintToOnlyAtEnd(t *testing.T) {
	linttest.AssertNoLints(t, "limited to only", NewToTwoToo())
}
