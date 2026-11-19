package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestInOnTheCards_CorrectAreOnForAmerican(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Both these features are on the cards, but for now we want to let users know if they have requested an invalid example.", NewInOnTheCards(dict.American), "Both these features are in the cards, but for now we want to let users know if they have requested an invalid example.")
}

func TestInOnTheCards_DontCorrectIsOnForBritish(t *testing.T) {
	linttest.AssertLintCount(t, "Yes, I think this is on the cards.", NewInOnTheCards(dict.British), 0)
}

func TestInOnTheCards_CorrectNotOnForAmerican(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If a permanent unique identifier is not on the cards any time soon for WebHID, we should consider a WebUSB alternative.", NewInOnTheCards(dict.American), "If a permanent unique identifier is not in the cards any time soon for WebHID, we should consider a WebUSB alternative.")
}

func TestInOnTheCards_CorrectBeOnForAmerican(t *testing.T) {
	linttest.AssertSuggestionResult(t, "a full breach of genomics (patient?) data can be on the cards since S3 AWS bucket credentials can be slurped from the process's memory", NewInOnTheCards(dict.American), "a full breach of genomics (patient?) data can be in the cards since S3 AWS bucket credentials can be slurped from the process's memory")
}

func TestInOnTheCards_CorrectWasOnForAmerican(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Virtualising the message summaries ObservableCollection was on the cards so I also take note of your last point.", NewInOnTheCards(dict.American), "Virtualising the message summaries ObservableCollection was in the cards so I also take note of your last point.")
}

func TestInOnTheCards_CorrectIsntOnNoApostropheForAmerican(t *testing.T) {
	linttest.AssertSuggestionResult(t, "parallelising that part isnt on the cards since there would be no noticeable ...", NewInOnTheCards(dict.American), "parallelising that part isnt in the cards since there would be no noticeable ...")
}

func TestInOnTheCards_CorrectItsOnForAmerican(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Regarding extensive documentation, as mentioned, its on the cards, project being sponsored by the aforementioned organisations.", NewInOnTheCards(dict.American), "Regarding extensive documentation, as mentioned, its in the cards, project being sponsored by the aforementioned organisations.")
}

func TestInOnTheCards_CorrectWereOnForAmerican(t *testing.T) {
	linttest.AssertSuggestionResult(t, "lots of high altitudes were on the cards again", NewInOnTheCards(dict.American), "lots of high altitudes were in the cards again")
}

func TestInOnTheCards_CorrectIsntOnForAmerican(t *testing.T) {
	linttest.AssertSuggestionResult(t, "downgrading to an end-of-life operating system isn't on the cards", NewInOnTheCards(dict.American), "downgrading to an end-of-life operating system isn't in the cards")
}

func TestInOnTheCards_CorrectWasntOnForAmerican(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it's only a middleground for an org because passwordless wasn't on the cards previously", NewInOnTheCards(dict.American), "it's only a middleground for an org because passwordless wasn't in the cards previously")
}

func TestInOnTheCards_CorrectWasInForBritish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Just wondering if it was in the cards or not for something like the Quest3 to get support in the future.", NewInOnTheCards(dict.British), "Just wondering if it was on the cards or not for something like the Quest3 to get support in the future.")
}

func TestInOnTheCards_DontCorrectIsInForAmerican(t *testing.T) {
	linttest.AssertLintCount(t, "Not sure if such a project is in the cards", NewInOnTheCards(dict.American), 0)
}

func TestInOnTheCards_CorrectNotInForBritish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is that just not in the cards for WASM at this time?", NewInOnTheCards(dict.British), "Is that just not on the cards for WASM at this time?")
}

func TestInOnTheCards_CorrectBeInForBritish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Would this be in the cards?", NewInOnTheCards(dict.British), "Would this be on the cards?")
}

func TestInOnTheCards_CorrectAreInForBritish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Manifest files are in the cards but haven't been implemented yet.", NewInOnTheCards(dict.British), "Manifest files are on the cards but haven't been implemented yet.")
}

func TestInOnTheCards_CorrectItsInForBritish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As far as an error, that probably would be helpful but doesn't sound like its in the cards.", NewInOnTheCards(dict.British), "As far as an error, that probably would be helpful but doesn't sound like its on the cards.")
}

func TestInOnTheCards_CorrectWereInForBritish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "a year or two given the major overhauls that were in the cards at the time", NewInOnTheCards(dict.British), "a year or two given the major overhauls that were on the cards at the time")
}

func TestInOnTheCards_CorrectIsntInForBritish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm going to close this as opting out of the installation framework that Electron gives us isn't in the cards for the project at this time.", NewInOnTheCards(dict.British), "I'm going to close this as opting out of the installation framework that Electron gives us isn't on the cards for the project at this time.")
}

func TestInOnTheCards_CorrectWasntInForBritish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "doing something better than just swapping our internal log package for glog wasn’t in the cards back then", NewInOnTheCards(dict.British), "doing something better than just swapping our internal log package for glog wasn’t on the cards back then")
}

func TestInOnTheCards_CorrectWerentInForBritish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I had thought stacked borrows was mostly in a final tweaking phase and major changes weren't in the cards.", NewInOnTheCards(dict.British), "I had thought stacked borrows was mostly in a final tweaking phase and major changes weren't on the cards.")
}
