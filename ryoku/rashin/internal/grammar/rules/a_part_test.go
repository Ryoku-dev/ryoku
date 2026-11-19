package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAPart_AllowNormalUseOfAPart(t *testing.T) {
	linttest.AssertLintCount(t, "That's not the whole truth, it's just a part.", NewAPart(), 0)
}

func TestAPart_AllowNormalUseOfApart(t *testing.T) {
	linttest.AssertLintCount(t, "You shouldn't have taken it apart.", NewAPart(), 0)
}

func TestAPart_AllowNormalUseOfAPartOf(t *testing.T) {
	linttest.AssertLintCount(t, "The elbow is a part of the arm.", NewAPart(), 0)
}

func TestAPart_AllowNormalUseOfApartFrom(t *testing.T) {
	linttest.AssertLintCount(t, "Apart from one error, the code works.", NewAPart(), 0)
}

func TestAPart_AllowNormalUsOfFallApart(t *testing.T) {
	linttest.AssertLintCount(t, "The roof fell apart.", NewAPart(), 0)
}

func TestAPart_AllowNormalUseOfFarApart(t *testing.T) {
	linttest.AssertLintCount(t, "Okinawa and Hokkaido are far apart.", NewAPart(), 0)
}

func TestAPart_CorrectsAPartFromToApartFromFormat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is it correct that the output file seems to be the same of the input file a part from the format (input: jpg, output: png)?", NewAPart(), "Is it correct that the output file seems to be the same of the input file apart from the format (input: jpg, output: png)?")
}

func TestAPart_CorrectsAPartFromToApartFromEnglish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Do you know there are more languages out there a part from English right?", NewAPart(), "Do you know there are more languages out there apart from English right?")
}

func TestAPart_CorrectsAPartFromToAPartOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "An easy tool to generate backdoor with msfvenom (a part from metasploit framework).", NewAPart(), "An easy tool to generate backdoor with msfvenom (a part of metasploit framework).")
}

func TestAPart_CorrectsApartOfToApartFromCflinuxfs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Doesn't work with any stacks apart of cflinuxfs2 and cflinuxfs3", NewAPart(), "Doesn't work with any stacks apart from cflinuxfs2 and cflinuxfs3")
}

func TestAPart_CorrectsApartOfToApartFromUsing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "apart of using filter, i can't find it in the documentation", NewAPart(), "apart from using filter, i can't find it in the documentation")
}

func TestAPart_CorrectsApartOfToAPartOfOpenai(t *testing.T) {
	linttest.AssertSuggestionResult(t, "export 'Usage' class as apart of openai.types", NewAPart(), "export 'Usage' class as a part of openai.types")
}

func TestAPart_CorrectsApartOfToAPartOfFormly(t *testing.T) {
	linttest.AssertSuggestionResult(t, "FormlyDatepickerTypeComponent is not listed as apart of the Formly Public API", NewAPart(), "FormlyDatepickerTypeComponent is not listed as a part of the Formly Public API")
}

func TestAPart_CorrectsFarAPart(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That leaves you only the other hand on the keyboard and you don't want the keys to be that far a part.", NewAPart(), "That leaves you only the other hand on the keyboard and you don't want the keys to be that far apart.")
}

func TestAPart_CorrectsSoFarAPartFromBeingTaken(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I can't see in the code what is done really with this session_timeout so far a part from being taken from the conf if defined there or setup ...", NewAPart(), "I can't see in the code what is done really with this session_timeout so far apart from being taken from the conf if defined there or setup ...")
}

func TestAPart_CorrectsSoFarAPartFromVersionUpgrade(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Any workaround so far a part from the version upgrade?", NewAPart(), "Any workaround so far apart from the version upgrade?")
}

func TestAPart_CorrectsFallAPart(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When I set up a script I set up card priority based on my frontline but sometimes a servant dies which sometimes causes things to fall a part.", NewAPart(), "When I set up a script I set up card priority based on my frontline but sometimes a servant dies which sometimes causes things to fall apart.")
}
