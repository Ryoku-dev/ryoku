package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestLetToDo_LetMeTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Let me to decide show player controls or not", NewLetToDo(), "Let me decide show player controls or not")
}

func TestLetToDo_LetYouTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Azure provider does not let you to ask for approval", NewLetToDo(), "Azure provider does not let you ask for approval")
}

func TestLetToDo_LetUsTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Currently JavaScript syntax does not let us to pick one or more properties from an object", NewLetToDo(), "Currently JavaScript syntax does not let us pick one or more properties from an object")
}

func TestLetToDo_LetItTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I modified the load_checkpoints and let it to only load the autoencoder's checkpoint.", NewLetToDo(), "I modified the load_checkpoints and let it only load the autoencoder's checkpoint.")
}

func TestLetToDo_LetHimTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "let him to delete the information he wrote", NewLetToDo(), "let him delete the information he wrote")
}

func TestLetToDo_LetAnybodyTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but at least will not let anybody to sub from tls as far as i understood this properly", NewLetToDo(), "but at least will not let anybody sub from tls as far as i understood this properly")
}

func TestLetToDo_LetAnyoneTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How would you let anyone to help you?", NewLetToDo(), "How would you let anyone help you?")
}

func TestLetToDo_LetAnyOneTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "set up a mcp server to let any one to query how to use the api in the repo", NewLetToDo(), "set up a mcp server to let any one query how to use the api in the repo")
}

func TestLetToDo_LetEverybodyTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "on a project that let everybody to create things", NewLetToDo(), "on a project that let everybody create things")
}

func TestLetToDo_LetEveryoneTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We want to let everyone to be able to select between the servers we have right now", NewLetToDo(), "We want to let everyone be able to select between the servers we have right now")
}

func TestLetToDo_LetEveryOneTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "you SHOULDN'T DO THIS because you let every one to upload an executable", NewLetToDo(), "you SHOULDN'T DO THIS because you let every one upload an executable")
}

func TestLetToDo_LetNobodyTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "i wouldn't let nobody to snoop on user's data", NewLetToDo(), "i wouldn't let nobody snoop on user's data")
}

func TestLetToDo_LetNoOneTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "hide video download link in wordpress - let no one to see video download link", NewLetToDo(), "hide video download link in wordpress - let no one see video download link")
}

func TestLetToDo_LetSomebodyTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So it should be the same let somebody to make required for reviewers.", NewLetToDo(), "So it should be the same let somebody make required for reviewers.")
}

func TestLetToDo_LetSomeOneTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "let some one to help me who can do it", NewLetToDo(), "let some one help me who can do it")
}

func TestLetToDo_LetsMeTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Also, clicking on the gear lets me to put login credentials", NewLetToDo(), "Also, clicking on the gear lets me put login credentials")
}

func TestLetToDo_LetsYouTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A chrome extension which lets you to create your own customised text snippets and use in your browser.", NewLetToDo(), "A chrome extension which lets you create your own customised text snippets and use in your browser.")
}

func TestLetToDo_LetsUsTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A menu which lets us to get money", NewLetToDo(), "A menu which lets us get money")
}

func TestLetToDo_LetsAnyoneTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "fake ssh server that lets anyone to connect and monitor their activty", NewLetToDo(), "fake ssh server that lets anyone connect and monitor their activty")
}

func TestLetToDo_LetsEverybodyTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Use set.seed function which lets everybody to check your result on their computers.", NewLetToDo(), "Use set.seed function which lets everybody check your result on their computers.")
}

func TestLetToDo_LetsEveryoneTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "what lets everyone to know what to expect", NewLetToDo(), "what lets everyone know what to expect")
}

func TestLetToDo_LetsSomeoneTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it works correctly and lets someone to connect using telnet", NewLetToDo(), "it works correctly and lets someone connect using telnet")
}

func TestLetToDo_LetsMeToApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I need be able to clone receiver, which crossbeam let's me to do.", NewLetToDo(), "I need be able to clone receiver, which crossbeam let's me do.")
}

func TestLetToDo_LetsUsToApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "this option let's us to cut the whole project in smaller pieces, like time based sprites.", NewLetToDo(), "this option let's us cut the whole project in smaller pieces, like time based sprites.")
}

func TestLetToDo_LetsYouToApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Let's you to migrate/transfer save files from older outward versions to outward definitive edition.", NewLetToDo(), "Let's you migrate/transfer save files from older outward versions to outward definitive edition.")
}

func TestLetToDo_LetsHimToApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A good woman gives wings to a man and let's him to take on the whole world", NewLetToDo(), "A good woman gives wings to a man and let's him take on the whole world")
}

func TestLetToDo_LetsHerToApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "if Igor let's her to be with Wonder she would crush everyone", NewLetToDo(), "if Igor let's her be with Wonder she would crush everyone")
}

func TestLetToDo_LetsItToApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's as accurate as your GPS let's it to be.", NewLetToDo(), "It's as accurate as your GPS let's it be.")
}

func TestLetToDo_LetsThemToApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it does not cut the dataset into 2 separate subsets in each step but let's them to get predictions from both branches", NewLetToDo(), "it does not cut the dataset into 2 separate subsets in each step but let's them get predictions from both branches")
}

func TestLetToDo_LetsAnyoneToApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It let's anyone to discover, take, or even teach a class.", NewLetToDo(), "It let's anyone discover, take, or even teach a class.")
}

func TestLetToDo_LetsAnyOneToApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Whyyyy hashmeet let's any one to create their own tinder like swipe clubs or groups?", NewLetToDo(), "Whyyyy hashmeet let's any one create their own tinder like swipe clubs or groups?")
}

func TestLetToDo_LetsEveryoneToApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How do you feel that America let's everyone to have guns", NewLetToDo(), "How do you feel that America let's everyone have guns")
}

func TestLetToDo_LetsSomeoneToApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Commands and attributes let's someone to compare it with other TRV", NewLetToDo(), "Commands and attributes let's someone compare it with other TRV")
}
