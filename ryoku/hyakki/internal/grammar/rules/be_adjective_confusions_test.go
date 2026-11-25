package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestBeAdjectiveConfusions_HeSBias(t *testing.T) {
	linttest.AssertSuggestionResult(t, "he's just on a rush to write a rejection letter probably because he's bias toward the origin or affiliation of the author", NewBeAdjectiveConfusionsLintGroup(), "he's just on a rush to write a rejection letter probably because he's biased toward the origin or affiliation of the author")
}

func TestBeAdjectiveConfusions_IAmBias(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I guess I am bias/prefer udd-style fragments", NewBeAdjectiveConfusionsLintGroup(), "I guess I am biased/prefer udd-style fragments")
}

func TestBeAdjectiveConfusions_IMBias(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm bias towards vunit as it's by far the most user friendly HDL test runner I know of.", NewBeAdjectiveConfusionsLintGroup(), "I'm biased towards vunit as it's by far the most user friendly HDL test runner I know of.")
}

func TestBeAdjectiveConfusions_ImBias(t *testing.T) {
	linttest.AssertSuggestionResult(t, "im bias, cause thats all im here for", NewBeAdjectiveConfusionsLintGroup(), "im biased, cause thats all im here for")
}

func TestBeAdjectiveConfusions_TheyReBias(t *testing.T) {
	linttest.AssertSuggestionResult(t, "See, I told you they’re bias because they flagged my comment!", NewBeAdjectiveConfusionsLintGroup(), "See, I told you they’re biased because they flagged my comment!")
}

func TestBeAdjectiveConfusions_TheyReBiasCouldAlsoBeTheirBias(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As a general rule I try to steer clear of the major news media outlets because of their outright refusal to admit they're bias upfront.", NewBeAdjectiveConfusionsLintGroup(), "As a general rule I try to steer clear of the major news media outlets because of their outright refusal to admit they're biased upfront.")
}

func TestBeAdjectiveConfusions_WereBias(t *testing.T) {
	t.Skip("Upstream ignored: The linter does not yet support the subject being a noun phrase")
	t.Skip("The linter does not yet support the subject being a noun phrase")
	linttest.AssertSuggestionResult(t, "the generated sample were bias in presence of negative cross-section", NewBeAdjectiveConfusionsLintGroup(), "the generated sample were biased in presence of negative cross-section")
}

func TestBeAdjectiveConfusions_DontFlagBiasHyphenCorrected(t *testing.T) {
	linttest.AssertNoLints(t, "I gather they are bias-corrected estimates.", NewBeAdjectiveConfusionsLintGroup())
}

func TestBeAdjectiveConfusions_DontFlagBiasSpaceCorrected(t *testing.T) {
	linttest.AssertNoLints(t, "All the images were bias corrected using N4 algorithm with a threshold value of 0.001.", NewBeAdjectiveConfusionsLintGroup())
}

func TestBeAdjectiveConfusions_DontFlagItsBiasFree(t *testing.T) {
	linttest.AssertNoLints(t, "Both the original model, and its bias-free counterpart, will be loaded.", NewBeAdjectiveConfusionsLintGroup())
}

func TestBeAdjectiveConfusions_DontFlagIfThereWereBias(t *testing.T) {
	linttest.AssertNoLints(t, "which depend on several variables: if there were bias, the average size of the classes, as well as the number of teachers at the school", NewBeAdjectiveConfusionsLintGroup())
}

func TestBeAdjectiveConfusions_HesConcern(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He's concern about my mental health because I'm alone in this city", NewBeAdjectiveConfusionsLintGroup(), "He's concerned about my mental health because I'm alone in this city")
}

func TestBeAdjectiveConfusions_IAmConcern(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am learning to use this, but I am concern on data privacy and sharing issues.", NewBeAdjectiveConfusionsLintGroup(), "I am learning to use this, but I am concerned on data privacy and sharing issues.")
}

func TestBeAdjectiveConfusions_ImConcern(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm concern if there is missing CVEs.", NewBeAdjectiveConfusionsLintGroup(), "I'm concerned if there is missing CVEs.")
}

func TestBeAdjectiveConfusions_ImNoApostropheConcern(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The only thing im concern about is the crash.", NewBeAdjectiveConfusionsLintGroup(), "The only thing im concerned about is the crash.")
}

func TestBeAdjectiveConfusions_ItsNoApostropheConcern(t *testing.T) {
	t.Skip("Upstream ignored: `Its` is currently not flagged because it's not always an error.")
	t.Skip("`Its` is currently not flagged because it's not always an error.")
	linttest.AssertSuggestionResult(t, "yes no one there answered and I thought its concern with this repo too so I clone it here!", NewBeAdjectiveConfusionsLintGroup(), "yes no one there answered and I thought its concerned with this repo too so I cloned it here!")
}

func TestBeAdjectiveConfusions_ShesConcern(t *testing.T) {
	linttest.AssertSuggestionResult(t, "she often courier food to her 40 year old son living in Jeju island because she’s concern about her son’s well being", NewBeAdjectiveConfusionsLintGroup(), "she often courier food to her 40 year old son living in Jeju island because she’s concerned about her son’s well being")
}

func TestBeAdjectiveConfusions_WereConcern(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As far as we're concern, this is a very simple try catch on your end", NewBeAdjectiveConfusionsLintGroup(), "As far as we're concerned, this is a very simple try catch on your end")
}

func TestBeAdjectiveConfusions_WeAreConcern(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We are concern and prioritize users' feedback and maintainability.", NewBeAdjectiveConfusionsLintGroup(), "We are concerned and prioritize users' feedback and maintainability.")
}

func TestBeAdjectiveConfusions_YouAreConcern(t *testing.T) {
	linttest.AssertSuggestionResult(t, "you are concern only in logging a more appropriate message error, but continue to throw the opaque assertion error.", NewBeAdjectiveConfusionsLintGroup(), "you are concerned only in logging a more appropriate message error, but continue to throw the opaque assertion error.")
}

func TestBeAdjectiveConfusions_DontFlagNounConcern(t *testing.T) {
	linttest.AssertNoLints(t, "My main concern is the data that could be lost using this deletion.", NewBeAdjectiveConfusionsLintGroup())
}

func TestBeAdjectiveConfusions_DontFlagVerbConcern(t *testing.T) {
	linttest.AssertNoLints(t, "This doesn't concern me at all.", NewBeAdjectiveConfusionsLintGroup())
}

func TestBeAdjectiveConfusions_DontFlagConcernFree(t *testing.T) {
	linttest.AssertNoLints(t, "They don't pretend they're concern-free.", NewBeAdjectiveConfusionsLintGroup())
}

func TestBeAdjectiveConfusions_DontFlagLegitItsConcern(t *testing.T) {
	linttest.AssertNoLints(t, "The ByteBlockPool loses its reference to the buffers it's just zeroed, so it seems outside its concern to zero the buffers.", NewBeAdjectiveConfusionsLintGroup())
}

func TestBeAdjectiveConfusions_DontFlagYoureYourError(t *testing.T) {
	t.Skip("Upstream ignored: This is a different error that should be caught by `ShouldContract`")
	t.Skip("This is a different error that should be caught by `ShouldContract`")
	linttest.AssertNoLints(t, "You're concern then is literally the \"-like\" in the message?", NewBeAdjectiveConfusionsLintGroup())
}

func TestBeAdjectiveConfusions_IWasPrejudice(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I didn't know I was prejudice against some of my own people until I went looking for web framework documentation and was met with a BLM banner.", NewBeAdjectiveConfusionsLintGroup(), "I didn't know I was prejudiced against some of my own people until I went looking for web framework documentation and was met with a BLM banner.")
}

func TestBeAdjectiveConfusions_ShesPrejudice(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She's prejudice against muslims.", NewBeAdjectiveConfusionsLintGroup(), "She's prejudiced against muslims.")
}

func TestBeAdjectiveConfusions_TheyrePrejudice(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The people who manage the law are all about lying and tricking people, theyre prejudice and trying to make people look stupid", NewBeAdjectiveConfusionsLintGroup(), "The people who manage the law are all about lying and tricking people, theyre prejudiced and trying to make people look stupid")
}

func TestBeAdjectiveConfusions_WeWereShock(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We were shock because its working fine on 1st of April just after 3 days its not working with the same settings, that is our major concern.", NewBeAdjectiveConfusionsLintGroup(), "We were shocked because its working fine on 1st of April just after 3 days its not working with the same settings, that is our major concern.")
}

func TestBeAdjectiveConfusions_DontFlagShockJocks(t *testing.T) {
	linttest.AssertNoLints(t, "They're not commentators/anaylsts, they're shock jocks.", NewBeAdjectiveConfusionsLintGroup())
}

func TestBeAdjectiveConfusions_DontFlagWereShock(t *testing.T) {
	t.Skip("Upstream ignored: Legitimate use of noun that the linter cannot yet distinguish.")
	t.Skip("Legitimate use of noun that the linter cannot yet distinguish.")
	linttest.AssertNoLints(t, "My first reactions when the topic of cryonics came up (early in our relationship) were shock, a bit of revulsion, and a lot of confusion.", NewBeAdjectiveConfusionsLintGroup())
}

func TestBeAdjectiveConfusions_HeIsWorry(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I guess he is worry about \" * user * \" tag.", NewBeAdjectiveConfusionsLintGroup(), "I guess he is worried about \" * user * \" tag.")
}

func TestBeAdjectiveConfusions_HeWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So he was worry about her. Especially, when he got no response by calling her on her phone nor ranging her doorbell.", NewBeAdjectiveConfusionsLintGroup(), "So he was worried about her. Especially, when he got no response by calling her on her phone nor ranging her doorbell.")
}

func TestBeAdjectiveConfusions_IAmWorry(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I didn't see any section dedicated to this so I am worry about:", NewBeAdjectiveConfusionsLintGroup(), "I didn't see any section dedicated to this so I am worried about:")
}

func TestBeAdjectiveConfusions_ImWorry(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but I'm worry about memory leak caused by that long delay", NewBeAdjectiveConfusionsLintGroup(), "but I'm worried about memory leak caused by that long delay")
}

func TestBeAdjectiveConfusions_ImNoApostropheWorry(t *testing.T) {
	linttest.AssertSuggestionResult(t, "im worry now that if this inbound was dangerous and maybe it does something to my pc", NewBeAdjectiveConfusionsLintGroup(), "im worried now that if this inbound was dangerous and maybe it does something to my pc")
}

func TestBeAdjectiveConfusions_IWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So that's why I was worry.", NewBeAdjectiveConfusionsLintGroup(), "So that's why I was worried.")
}

func TestBeAdjectiveConfusions_IWere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The only things that I were worry about is the data that could be lost using this deletion.", NewBeAdjectiveConfusionsLintGroup(), "The only things that I were worried about is the data that could be lost using this deletion.")
}

func TestBeAdjectiveConfusions_TheyAreWorry(t *testing.T) {
	linttest.AssertSuggestionResult(t, "at the same time they are worry about the price for the upgrade each 3 years", NewBeAdjectiveConfusionsLintGroup(), "at the same time they are worried about the price for the upgrade each 3 years")
}

func TestBeAdjectiveConfusions_TheyreWorry(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Because they're worry this link is spam or they scare have to pay more money.", NewBeAdjectiveConfusionsLintGroup(), "Because they're worried this link is spam or they scare have to pay more money.")
}

func TestBeAdjectiveConfusions_WeAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We are analised this and we are worry because when our platform go to market", NewBeAdjectiveConfusionsLintGroup(), "We are analised this and we are worried because when our platform go to market")
}

func TestBeAdjectiveConfusions_Were(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We're worry about all kinds of minority representation in TV.", NewBeAdjectiveConfusionsLintGroup(), "We're worried about all kinds of minority representation in TV.")
}

func TestBeAdjectiveConfusions_YouAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You are worry because we are not annotating view interface itself, right?", NewBeAdjectiveConfusionsLintGroup(), "You are worried because we are not annotating view interface itself, right?")
}

func TestBeAdjectiveConfusions_Youre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You're worry about memory usage and wanna be sure that a Sequence-class won't hold your activity against GC — declare this class as static", NewBeAdjectiveConfusionsLintGroup(), "You're worried about memory usage and wanna be sure that a Sequence-class won't hold your activity against GC — declare this class as static")
}

func TestBeAdjectiveConfusions_DontFlagItIs(t *testing.T) {
	linttest.AssertNoLints(t, "Part of it is worry that my bosses will get angry and fire me.", NewBeAdjectiveConfusionsLintGroup())
}

func TestBeAdjectiveConfusions_DontFlagItWas(t *testing.T) {
	linttest.AssertNoLints(t, "Because what followed wasn't indifference, it was worry.", NewBeAdjectiveConfusionsLintGroup())
}

func TestBeAdjectiveConfusions_DontFlagSheWasWorryFree(t *testing.T) {
	linttest.AssertNoLints(t, "Finally, she was worry-free.", NewBeAdjectiveConfusionsLintGroup())
}

func TestBeAdjectiveConfusions_DontFlagTheyreWorryFree(t *testing.T) {
	linttest.AssertNoLints(t, "They don't pretend they're worry-free.", NewBeAdjectiveConfusionsLintGroup())
}

func TestBeAdjectiveConfusions_DontFlagWorryWarts(t *testing.T) {
	linttest.AssertNoLints(t, "They’re worry warts", NewBeAdjectiveConfusionsLintGroup())
}

func TestBeAdjectiveConfusions_DontFlagWereWorrySpaceFree(t *testing.T) {
	linttest.AssertNoLints(t, "Thanks to jQuery, we're worry free from browser compatibility.", NewBeAdjectiveConfusionsLintGroup())
}

func TestBeAdjectiveConfusions_DifferentCaseNotHandled(t *testing.T) {
	t.Skip("Upstream ignored: different error case not handled by this linter")
	t.Skip("different error case not handled by this linter")
	linttest.AssertGoodAndBadSuggestions(t, "Myself along with others are using it on an iPad successfully, so it is worry to hear that is broken for you.", NewBeAdjectiveConfusionsLintGroup(), []string{
		"Myself along with others are using it on an iPad successfully, so it is worrying to hear that is broken for you.",
		"Myself along with others are using it on an iPad successfully, so it is a worry to hear that is broken for you.",
	}, []string{})
}

func TestBeAdjectiveConfusions_DontFlagYoureYoureError(t *testing.T) {
	t.Skip("Upstream ignored: This is a different error that should be caught by `ShouldContract`")
	t.Skip("This is a different error that should be caught by `ShouldContract`")
	linttest.AssertNoLints(t, "I think you're worry is not really an issue.", NewBeAdjectiveConfusionsLintGroup())
}
