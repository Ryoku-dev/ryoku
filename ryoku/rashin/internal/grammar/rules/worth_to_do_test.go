package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWorthToDo_WorthToAdd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is it worth to add those files?", NewWorthToDo(dict.Curated()), "Is it worth adding those files?")
}

func TestWorthToDo_WorthToAdjust(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If yes, it would be worth to adjust the description to make it easier to understand", NewWorthToDo(dict.Curated()), "If yes, it would be worth adjusting the description to make it easier to understand")
}

func TestWorthToDo_WorthToAsk(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So it is worth to ask for this there or take a look at their wiki pages. ", NewWorthToDo(dict.Curated()), "So it is worth asking for this there or take a look at their wiki pages. ")
}

func TestWorthToDo_WorthToBuy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and it makes it really worth to buy the software", NewWorthToDo(dict.Curated()), "and it makes it really worth buying the software")
}

func TestWorthToDo_WorthToDeal(t *testing.T) {
	linttest.AssertSuggestionResult(t, "CC2531 is considered as crap in 2024 and its not worth to deal with it.", NewWorthToDo(dict.Curated()), "CC2531 is considered as crap in 2024 and its not worth dealing with it.")
}

func TestWorthToDo_WorthToDo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is it worth to do the credit-card-balance-transfer?", NewWorthToDo(dict.Curated()), "Is it worth doing the credit-card-balance-transfer?")
}

func TestWorthToDo_WorthToExperiment(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hello @tkchia, thanks for the hint, I agree it is worth to experiment.", NewWorthToDo(dict.Curated()), "Hello @tkchia, thanks for the hint, I agree it is worth experimenting.")
}

func TestWorthToDo_WorthToFix(t *testing.T) {
	linttest.AssertSuggestionResult(t, "i dont know if this is worth to fix, i just wanted to point this and start a discussion about this.", NewWorthToDo(dict.Curated()), "i dont know if this is worth fixing, i just wanted to point this and start a discussion about this.")
}

func TestWorthToDo_WorthToGetPublished(t *testing.T) {
	linttest.AssertSuggestionResult(t, "think that would be worth to get published in my Thesis.", NewWorthToDo(dict.Curated()), "think that would be worth getting published in my Thesis.")
}

func TestWorthToDo_WorthToImagine(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Might be worth to imagine how the current Nu style would look like in other programming languages.", NewWorthToDo(dict.Curated()), "Might be worth imagining how the current Nu style would look like in other programming languages.")
}

func TestWorthToDo_WorthToInvest(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It doesn't seem worth to invest much effort in this though...", NewWorthToDo(dict.Curated()), "It doesn't seem worth investing much effort in this though...")
}

func TestWorthToDo_WorthToInvestigate(t *testing.T) {
	linttest.AssertSuggestionResult(t, "to get a feeling how CP-SAT works and what are directions worth to investigate i wanted to ask", NewWorthToDo(dict.Curated()), "to get a feeling how CP-SAT works and what are directions worth investigating i wanted to ask")
}

func TestWorthToDo_WorthToPlay(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is worth to play with thread count if there are no issues?", NewWorthToDo(dict.Curated()), "Is worth playing with thread count if there are no issues?")
}

func TestWorthToDo_WorthToPut(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Do you think it would be worth to put a suggestion to remove the kind network if cluster creation fails", NewWorthToDo(dict.Curated()), "Do you think it would be worth putting a suggestion to remove the kind network if cluster creation fails")
}

func TestWorthToDo_WorthToRead(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Stored books worth to read.", NewWorthToDo(dict.Curated()), "Stored books worth reading.")
}

func TestWorthToDo_WorthToRevisit(t *testing.T) {
	linttest.AssertSuggestionResult(t, "we've had discussions before #260 and it maybe worth to revisit again", NewWorthToDo(dict.Curated()), "we've had discussions before #260 and it maybe worth revisiting again")
}

func TestWorthToDo_WorthToRewrite(t *testing.T) {
	linttest.AssertSuggestionResult(t, "is puppet so bad that it is worth to rewrite everything?", NewWorthToDo(dict.Curated()), "is puppet so bad that it is worth rewriting everything?")
}

func TestWorthToDo_WorthToTry(t *testing.T) {
	linttest.AssertSuggestionResult(t, "is it really worth to try and what are facebook long-term plans about this engine.", NewWorthToDo(dict.Curated()), "is it really worth trying and what are facebook long-term plans about this engine.")
}

func TestWorthToDo_WorthToUpdate(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hi, maybe it's worth to update doc with the script for Bullseye given by @frenchfaso ?", NewWorthToDo(dict.Curated()), "Hi, maybe it's worth updating doc with the script for Bullseye given by @frenchfaso ?")
}

func TestWorthToDo_WorthToUpgrade(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Your PR should've fixed that issue so I think it's worth to upgrade to 10.33 and see if that brings the delta down.", NewWorthToDo(dict.Curated()), "Your PR should've fixed that issue so I think it's worth upgrading to 10.33 and see if that brings the delta down.")
}

func TestWorthToDo_WorthToUseAndDevelop(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I think It worth to use and worth to develop further", NewWorthToDo(dict.Curated()), "I think It worth using and worth developing further")
}

func TestWorthToDo_WorksWithUppercaseGlom(t *testing.T) {
	linttest.AssertSuggestionResult(t, " YES IT IS WORTH TO DO", NewWorthToDo(dict.Curated()), " YES IT IS WORTH DOING")
}

func TestWorthToDo_WorksWithUppercaseFinalE(t *testing.T) {
	linttest.AssertSuggestionResult(t, "THIS LINTER WAS WORTH TO MAKE", NewWorthToDo(dict.Curated()), "THIS LINTER WAS WORTH MAKING")
}

func TestWorthToDo_WorksWithUppercaseDoubleConsonant(t *testing.T) {
	linttest.AssertSuggestionResult(t, "SO YEAH IT WAS WORTH TO GET THIS DONE", NewWorthToDo(dict.Curated()), "SO YEAH IT WAS WORTH GETTING THIS DONE")
}
