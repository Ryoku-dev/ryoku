package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestThereIsAgreement_StatementPresentPlRegular(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "there is things", NewThereIsAgreement(dict.Curated()), []string{"there are things", "there is a thing"}, []string{})
}

func TestThereIsAgreement_StatementPresentSgIrregular(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "there are person", NewThereIsAgreement(dict.Curated()), []string{"there are people", "there is a person"}, []string{})
}

func TestThereIsAgreement_StatementPresentTheresPl(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "there's secrets", NewThereIsAgreement(dict.Curated()), []string{"there are secrets", "there's a secret"}, []string{})
}

func TestThereIsAgreement_StatementPastPlVowel(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "there was ideas", NewThereIsAgreement(dict.Curated()), []string{"there were ideas", "there was an idea"}, []string{})
}

func TestThereIsAgreement_StatementPastSg(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "there were child", NewThereIsAgreement(dict.Curated()), []string{"there were children", "there was a child"}, []string{})
}

func TestThereIsAgreement_QuestionPresSg(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "are there man", NewThereIsAgreement(dict.Curated()), []string{"are there men", "is there a man"}, []string{})
}

func TestThereIsAgreement_QuestionPresPl(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "is there women", NewThereIsAgreement(dict.Curated()), []string{"are there women", "is there a woman"}, []string{})
}

func TestThereIsAgreement_QuestionPastSg(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "were there cow", NewThereIsAgreement(dict.Curated()), []string{"were there cows", "was there a cow"}, []string{})
}

func TestThereIsAgreement_QuestionPastPl(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "were there elephant", NewThereIsAgreement(dict.Curated()), []string{"were there elephants", "was there an elephant"}, []string{})
}

func TestThereIsAgreement_DontFlagThereAreHyphenatedCompoundStartsSingular(t *testing.T) {
	linttest.AssertNoLints(t, "there are function-like macros", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_DontFlagThereAreOpenCompoundStartsSingular(t *testing.T) {
	linttest.AssertNoLints(t, "there are config errors", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_AOrAnDependsOnDialectHerb(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "there's herbs.", NewThereIsAgreement(dict.Curated()), []string{
		"there are herbs.",

		"there's an herb.",
	}, []string{})
}

func TestThereIsAgreement_AOrAnDependsOnDialectHotel(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "There are hotel.", NewThereIsAgreement(dict.Curated()), []string{
		"There are hotels.",
		"There is a hotel.",
	}, []string{})
}

func TestThereIsAgreement_FixThereIsErrors(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Hi， when I make the code, there is errors", NewThereIsAgreement(dict.Curated()), []string{
		"Hi， when I make the code, there are errors",
		"Hi， when I make the code, there is an error",
	}, []string{"Hi， when I make the code, there is a error"})
}

func TestThereIsAgreement_FixThereIsWarnings(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "There is warnings from kotlin and dart, as reference: Elvis operator (?:) always returns the left operand of non-nullable type String.", NewThereIsAgreement(dict.Curated()), []string{
		"There are warnings from kotlin and dart, as reference: Elvis operator (?:) always returns the left operand of non-nullable type String.",
		"There is a warning from kotlin and dart, as reference: Elvis operator (?:) always returns the left operand of non-nullable type String.",
	}, []string{})
}

func TestThereIsAgreement_FixThereIsProblems(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Problem is that if there is a project that has a csproj file, then there is problems with the history folder.", NewThereIsAgreement(dict.Curated()), []string{
		"Problem is that if there is a project that has a csproj file, then there are problems with the history folder.",
		"Problem is that if there is a project that has a csproj file, then there is a problem with the history folder.",
	}, []string{})
}

func TestThereIsAgreement_FixThereIsCommands(t *testing.T) {
	t.Skip("`replace_with_match_case` matching case char-by-char causes the wrong letters to be uppercased")
	linttest.AssertGoodAndBadSuggestions(t, "Additionally if there is Commands that can be used on multiple Resources at the same time.", NewThereIsAgreement(dict.Curated()), []string{
		"Additionally if there are Commands that can be used on multiple Resources at the same time.",
		"Additionally if there is a Command that can be used on multiple Resources at the same time.",
	}, []string{})
}

func TestThereIsAgreement_FixThereIsValues(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This mean there would not be a single cache, but as many caches as there is values for the second argument.", NewThereIsAgreement(dict.Curated()), "This mean there would not be a single cache, but as many caches as there are values for the second argument.")
}

func TestThereIsAgreement_FixThereIsStrings(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I can image other cases (tools different from SPSS) in which there is strings in both sides of the dictionary", NewThereIsAgreement(dict.Curated()), []string{
		"I can image other cases (tools different from SPSS) in which there are strings in both sides of the dictionary",
		"I can image other cases (tools different from SPSS) in which there is a string in both sides of the dictionary",
	}, []string{})
}

func TestThereIsAgreement_FixThereIsThings(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "even though we can check whether there is things running in Node, we can not do it for Chromium's message loops", NewThereIsAgreement(dict.Curated()), []string{
		"even though we can check whether there are things running in Node, we can not do it for Chromium's message loops",
		"even though we can check whether there is a thing running in Node, we can not do it for Chromium's message loops",
	}, []string{})
}

func TestThereIsAgreement_FixThereIsPeople(t *testing.T) {
	linttest.AssertSuggestionResult(t, "there is people making projects, there is people doing tutorials", NewThereIsAgreement(dict.Curated()), "there are people making projects, there are people doing tutorials")
	linttest.AssertSuggestionResult(t, "there is people making projects, there is people doing tutorials", NewThereIsAgreement(dict.Curated()), "there is a person making projects, there is a person doing tutorials")
}

func TestThereIsAgreement_FixThereIsInstructions(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I am just wondering if there is instructions somewhere for handling deep linking while using Redux.", NewThereIsAgreement(dict.Curated()), []string{
		"I am just wondering if there are instructions somewhere for handling deep linking while using Redux.",
		"I am just wondering if there is an instruction somewhere for handling deep linking while using Redux.",
	}, []string{})
}

func TestThereIsAgreement_FixThereIsPackages(t *testing.T) {
	linttest.AssertSuggestionResult(t, "if there is packages that handle such protocols installed", NewThereIsAgreement(dict.Curated()), "if there are packages that handle such protocols installed")
}

func TestThereIsAgreement_DontFlagThereIsPeopleWhoHaveDoneXyz(t *testing.T) {
	t.Skip("TODO: abort if next word is a relative pronoun like `who`")
	linttest.AssertNoLints(t, "The main expectation there is people who have a deprecated app installed will then get an error if it's disabled", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_DontFlagThereIsPackagesPartOfDir(t *testing.T) {
	t.Skip("TODO: abort if next token is `/`")
	linttest.AssertNoLints(t, "For example there is packages/vite/src/node , but no packages/vite/src/deno", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_FixThereAreBug(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "there are bug in svelte 3.0 that axios from 0.22.0 version undefined", NewThereIsAgreement(dict.Curated()), []string{
		"there is a bug in svelte 3.0 that axios from 0.22.0 version undefined",
		"there are bugs in svelte 3.0 that axios from 0.22.0 version undefined",
	}, []string{})
}

func TestThereIsAgreement_FixThereAreDescription(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "there are description regarding thread safety in zmq document.", NewThereIsAgreement(dict.Curated()), []string{
		"there is a description regarding thread safety in zmq document.",
		"there are descriptions regarding thread safety in zmq document.",
	}, []string{})
}

func TestThereIsAgreement_FixThereAreIssue(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Seems like if there are issue with OpenAI, it is still trying to call chat completion and giving type error.", NewThereIsAgreement(dict.Curated()), []string{
		"Seems like if there are issues with OpenAI, it is still trying to call chat completion and giving type error.",
		"Seems like if there is an issue with OpenAI, it is still trying to call chat completion and giving type error.",
	}, []string{})
}

func TestThereIsAgreement_FixThereAreProblem(t *testing.T) {
	t.Skip("TODO: 'problem' is being rejected as an adjective as well as a singular noun")
	linttest.AssertGoodAndBadSuggestions(t, "There are problem with official serving docker image gpu version", NewThereIsAgreement(dict.Curated()), []string{
		"There is a problem with official serving docker image gpu version",
		"There are problems with official serving docker image gpu version",
	}, []string{})
}

func TestThereIsAgreement_DontFlagThereAreDepConflicts(t *testing.T) {
	linttest.AssertNoLints(t, "Wrong advice when there are dependency conflicts", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_DontFlagThereAreDepErrs(t *testing.T) {
	linttest.AssertNoLints(t, "If there are dependency errors they will be immediately logged out to you.", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_DontFlagThereAreDefSyntaxErrs(t *testing.T) {
	t.Skip("'definition syntax' looks singular but the full np is the plural 'definition syntax errors'")
	linttest.AssertNoLints(t, "New Campaign Properties dialog loses changes if there are definition syntax errors", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_DontFlagThereAreDescriptionAndInstructionKeys(t *testing.T) {
	linttest.AssertNoLints(t, "There are description and instruction keys for the classes.", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_DontFlagThereAreFunctionLike(t *testing.T) {
	linttest.AssertNoLints(t, "clang-format indents class member functions oddly if there are function-like macro invocations", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_FixThereWasConfigs(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I did see that there was configs for it before but it isn't in the configs anymore.", NewThereIsAgreement(dict.Curated()), []string{
		"I did see that there were configs for it before but it isn't in the configs anymore.",
		"I did see that there was a config for it before but it isn't in the configs anymore.",
	}, []string{})
}

func TestThereIsAgreement_FixThereWasExamples(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "It would be awesome if there was examples on how to include inline citations", NewThereIsAgreement(dict.Curated()), []string{
		"It would be awesome if there were examples on how to include inline citations",
		"It would be awesome if there was an example on how to include inline citations",
	}, []string{})
}

func TestThereIsAgreement_FixThereWasFunctions(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I noticed in the AXP2101_Class for the unified library that there was functions like \"isPekeyShortPressIrq()\"", NewThereIsAgreement(dict.Curated()), []string{
		"I noticed in the AXP2101_Class for the unified library that there were functions like \"isPekeyShortPressIrq()\"",
		"I noticed in the AXP2101_Class for the unified library that there was a function like \"isPekeyShortPressIrq()\"",
	}, []string{})
}

func TestThereIsAgreement_FixThereWasIssues(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Restored to a Snapshot, but there was issues with Hyprland.", NewThereIsAgreement(dict.Curated()), []string{
		"Restored to a Snapshot, but there were issues with Hyprland.",
		"Restored to a Snapshot, but there was an issue with Hyprland.",
	}, []string{})
}

func TestThereIsAgreement_FixThereWasSettings(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I also tried creating a fresh page on the same site, and there was settings but only 2 pages of settings", NewThereIsAgreement(dict.Curated()), []string{
		"I also tried creating a fresh page on the same site, and there were settings but only 2 pages of settings",
		"I also tried creating a fresh page on the same site, and there was a setting but only 2 pages of settings",
	}, []string{})
}

func TestThereIsAgreement_FixThereWereFunction(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Instead, it would be helpful if there were function in the autoloader capable of taking the a list of names of packages to enable.", NewThereIsAgreement(dict.Curated()), []string{
		"Instead, it would be helpful if there were functions in the autoloader capable of taking the a list of names of packages to enable.",
		"Instead, it would be helpful if there was a function in the autoloader capable of taking the a list of names of packages to enable.",
	}, []string{})
}

func TestThereIsAgreement_FixThereWereHint(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "there were hint that this could break the build", NewThereIsAgreement(dict.Curated()), []string{
		"there were hints that this could break the build",
		"there was a hint that this could break the build",
	}, []string{})
}

func TestThereIsAgreement_FixThereWereIssue(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "there were issue with using venv and it was pulled", NewThereIsAgreement(dict.Curated()), []string{
		"there were issues with using venv and it was pulled",
		"there was an issue with using venv and it was pulled",
	}, []string{})
}

func TestThereIsAgreement_FixThereWereProblemAbout(t *testing.T) {
	t.Skip("TODO: 'problem' is being rejected as an adjective as well as a singular noun")
	linttest.AssertGoodAndBadSuggestions(t, "there were problem about version but this error is solved by correcting version in these files", NewThereIsAgreement(dict.Curated()), []string{
		"there were problems about version but this error is solved by correcting version in these files",
		"there was a problem about version but this error is solved by correcting version in these files",
	}, []string{})
}

func TestThereIsAgreement_FixThereWereProblemWith(t *testing.T) {
	t.Skip("TODO: 'problem' is being rejected as an adjective as well as a singular noun")
	linttest.AssertGoodAndBadSuggestions(t, "there were problem with page alignment crossing", NewThereIsAgreement(dict.Curated()), []string{
		"there were problems with page alignment crossing",
		"there was a problem with page alignment crossing",
	}, []string{})
}

func TestThereIsAgreement_DontFlagAliceThereWereTwo(t *testing.T) {
	linttest.AssertNoLints(t, "This time there were two little shrieks, and more sounds of broken glass.", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_DontFlagGatsbyThereWereTwinkleBells(t *testing.T) {
	linttest.AssertNoLints(t, "When he realized what I was talking about, that there were twinkle-bells of sunshine in the room, he smiled like a weather man", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_DontFlagGatsbyThereWereAAndB(t *testing.T) {
	t.Skip("We can't detect first name + surname, especially for rare surnames like 'Waize'")
	linttest.AssertNoLints(t, "Of theatrical people there were Gus Waize and Horace O’Donavan and Lester Myer and George Duckweed and Francis", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_FixIsThereApps(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Ok, but is there apps that actually do that?", NewThereIsAgreement(dict.Curated()), []string{
		"Ok, but are there apps that actually do that?",
		"Ok, but is there an app that actually do that?",
	}, []string{})
}

func TestThereIsAgreement_FixIsThereIdeas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is there ideas to make regular page to listen for api (POST/PUT/.. etc requests)", NewThereIsAgreement(dict.Curated()), "Are there ideas to make regular page to listen for api (POST/PUT/.. etc requests)")
}

func TestThereIsAgreement_FixIsTherePeople(t *testing.T) {
	linttest.AssertSuggestionResult(t, "please guys tell me, is there people really making money with bot?", NewThereIsAgreement(dict.Curated()), "please guys tell me, are there people really making money with bot?")
}

func TestThereIsAgreement_FixIsThereSolutions(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Run-as binary without the suid bit set, is there solutions?", NewThereIsAgreement(dict.Curated()), []string{
		"Run-as binary without the suid bit set, are there solutions?",
		"Run-as binary without the suid bit set, is there a solution?",
	}, []string{})
}

func TestThereIsAgreement_FixIsThereThings(t *testing.T) {
	linttest.AssertSuggestionResult(t, "is there things you could change to make it a more general product", NewThereIsAgreement(dict.Curated()), "are there things you could change to make it a more general product")
}

func TestThereIsAgreement_FixIsThereTools(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Is there tools or documentation how to recover / rebuild /run fsck on the failed replicas.", NewThereIsAgreement(dict.Curated()), []string{
		"Are there tools or documentation how to recover / rebuild /run fsck on the failed replicas.",
		"Is there a tool or documentation how to recover / rebuild /run fsck on the failed replicas.",
	}, []string{})
}

func TestThereIsAgreement_FixAreThereProblem(t *testing.T) {
	t.Skip("TODO: 'problem' is being rejected as an adjective as well as a singular noun")
	linttest.AssertGoodAndBadSuggestions(t, "Is it just the namespace or are there problem in the use statements as well?", NewThereIsAgreement(dict.Curated()), []string{
		"Is it just the namespace or are there problems in the use statements as well?",
		"Is it just the namespace or is there a problem in the use statements as well?",
	}, []string{})
}

func TestThereIsAgreement_FixAreThereSolution(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Are there solution for making lsws using h2 protocol in client browsers?", NewThereIsAgreement(dict.Curated()), []string{
		"Are there solutions for making lsws using h2 protocol in client browsers?",
		"Is there a solution for making lsws using h2 protocol in client browsers?",
	}, []string{})
}

func TestThereIsAgreement_FixAreThereSolutionSlashWorkaround(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "are there solution/workaround ?", NewThereIsAgreement(dict.Curated()), []string{

		"are there solutions/workaround ?",
		"is there a solution/workaround ?",
	}, []string{})
}

func TestThereIsAgreement_FixAreThereConcept(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "So, what is a external interface in C++? are there concept of interface in C++?", NewThereIsAgreement(dict.Curated()), []string{
		"So, what is a external interface in C++? are there concepts of interface in C++?",
		"So, what is a external interface in C++? is there a concept of interface in C++?",
	}, []string{})
}

func TestThereIsAgreement_FixAreThereObject(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "He check are there object with same id, if there is no object with same id he creates new and add to array", NewThereIsAgreement(dict.Curated()), []string{
		"He check are there objects with same id, if there is no object with same id he creates new and add to array",
		"He check is there an object with same id, if there is no object with same id he creates new and add to array",
	}, []string{})
}

func TestThereIsAgreement_FixAreThereVariable(t *testing.T) {
	t.Skip("variable is being rejected as an adjective as well as a singular noun")
	linttest.AssertGoodAndBadSuggestions(t, "Are there variable in side it or is it some sort of dataset?", NewThereIsAgreement(dict.Curated()), []string{
		"Are there variables in side it or is it some sort of dataset?",
		"Is there a variable in side it or is it some sort of dataset?",
	}, []string{})
}

func TestThereIsAgreement_IgnoreAreThereAnswerGenerationErrors(t *testing.T) {
	linttest.AssertNoLints(t, "Are there Answer Generation Errors?", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_IgnoreAreThereApplicationObjects(t *testing.T) {
	linttest.AssertNoLints(t, "Are there application objects assigned to non-existent sites", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_IgnoreAreThereCodeFiles(t *testing.T) {
	linttest.AssertNoLints(t, "Why are there code files more than 10k lines long?", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_IgnoreAreThereErrorLogs(t *testing.T) {
	linttest.AssertNoLints(t, "Are there error logs in your worker when flows fail?", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_IgnoreAreThereToolSpecific(t *testing.T) {
	linttest.AssertNoLints(t, "Are there tool specific constraints for RM tool exchange for EA?", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_FixWasThereBugs(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Was there bugs, and goofed-up quests?", NewThereIsAgreement(dict.Curated()), []string{
		"Were there bugs, and goofed-up quests?",
		"Was there a bug, and goofed-up quests?",
	}, []string{})
}

func TestThereIsAgreement_FixWasThereHints(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Was there hints in the game that points you to it?", NewThereIsAgreement(dict.Curated()), []string{
		"Were there hints in the game that points you to it?",
		"Was there a hint in the game that points you to it?",
	}, []string{})
}

func TestThereIsAgreement_FixWasThereIssues(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I saw you closed that PR, was there issues getting it merged upstream?", NewThereIsAgreement(dict.Curated()), []string{
		"I saw you closed that PR, were there issues getting it merged upstream?",
		"I saw you closed that PR, was there an issue getting it merged upstream?",
	}, []string{})
}

func TestThereIsAgreement_FixWasThereProblems(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Was there problems with other files that you had written to the SD card (PRG/CRT)?", NewThereIsAgreement(dict.Curated()), []string{
		"Were there problems with other files that you had written to the SD card (PRG/CRT)?",
		"Was there a problem with other files that you had written to the SD card (PRG/CRT)?",
	}, []string{})
}

func TestThereIsAgreement_DontFlagLastTimeIWasThere(t *testing.T) {
	t.Skip("Removing the noun flag from 'were' stopped this test passing")
	linttest.AssertNoLints(t, "Last time I was there flags were on almost every house or fence.", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_DontFlagTheGrateWasThere(t *testing.T) {
	t.Skip("Harper cannot detect this construction")
	linttest.AssertNoLints(t, "Saying the grate was there helps him avoid a lawsuit.", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_DontFlagWhenHeWasThere(t *testing.T) {
	t.Skip("Removing the noun flag from 'were' stopped this test passing")
	linttest.AssertNoLints(t, "When he was there tips were going way, way up.", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_WereThereDescription(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Were there pictures of the Primarchs around before then? Were there description?", NewThereIsAgreement(dict.Curated()), []string{
		"Were there pictures of the Primarchs around before then? Were there descriptions?",
		"Were there pictures of the Primarchs around before then? Was there a description?",
	}, []string{})
}

func TestThereIsAgreement_FontFlagWereThereBombDrills(t *testing.T) {
	linttest.AssertNoLints(t, "Were there bomb drills in school?", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_DontFlagWereThereSetRules(t *testing.T) {
	linttest.AssertNoLints(t, "For instance, were there set rules as to the funds that candidates would have to have acquired", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_DontFlagWereThereTourBuses(t *testing.T) {
	linttest.AssertNoLints(t, "Were there tour buses and different hotels and parties each night? Yep.", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_FixTheresChildren(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "now, check whether there's children", NewThereIsAgreement(dict.Curated()), []string{
		"now, check whether there's a child",
		"now, check whether there are children",
	}, []string{})
}

func TestThereIsAgreement_FixTheresIdeas(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "there's ideas how it should behave for some media etc.", NewThereIsAgreement(dict.Curated()), []string{
		"there's an idea how it should behave for some media etc.",
		"there are ideas how it should behave for some media etc.",
	}, []string{})
}

func TestThereIsAgreement_FixTheresPeople(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Currently there's people helping out, and it's fairly easy to find someone if you need something.", NewThereIsAgreement(dict.Curated()), []string{
		"Currently there are people helping out, and it's fairly easy to find someone if you need something.",
		"Currently there's a person helping out, and it's fairly easy to find someone if you need something.",
	}, []string{})
}

func TestThereIsAgreement_FixTheresProblems(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "there's problems when using WSL::Ubuntu", NewThereIsAgreement(dict.Curated()), []string{
		"there are problems when using WSL::Ubuntu",
		"there's a problem when using WSL::Ubuntu",
	}, []string{})
}

func TestThereIsAgreement_FixTheresThings(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "If there's things you love/hate about Vidstack please let us know", NewThereIsAgreement(dict.Curated()), []string{
		"If there are things you love/hate about Vidstack please let us know",
		"If there's a thing you love/hate about Vidstack please let us know",
	}, []string{})
}

func TestThereIsAgreement_FixTheresUrls(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "so you're not suprised if there's urls missing or your data isn't being refreshed daily", NewThereIsAgreement(dict.Curated()), []string{
		"so you're not suprised if there are urls missing or your data isn't being refreshed daily",
		"so you're not suprised if there's a url missing or your data isn't being refreshed daily",
	}, []string{})
}

func TestThereIsAgreement_FixBrowserConsole3066(t *testing.T) {
	linttest.AssertNoLints(t, "Finally, I’ve checked the browser’s developer console for any JavaScript errors, but there are none.", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_FixThereWasNone3066(t *testing.T) {
	linttest.AssertNoLints(t, "There were none.", NewThereIsAgreement(dict.Curated()))
}

func TestThereIsAgreement_FixNoneRelatedTo3066(t *testing.T) {
	linttest.AssertNoLints(t, "I’ve inspected the browser console for JavaScript errors, and there are none related to WooCommerce or the product variation functionality.", NewThereIsAgreement(dict.Curated()))
}
