package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestInitialisms_CorrectsBtw(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Btw, are you ready to go shopping soon?", NewInitialismsLintGroup(), "By the way, are you ready to go shopping soon?")
}

func TestInitialisms_CorrectsStyle(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I love the fit, btw.", NewInitialismsLintGroup(), "I love the fit, by the way.")
}

func TestInitialisms_CorrectsFyi(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Fyi, the meeting is at 3.", NewInitialismsLintGroup(), "For your information, the meeting is at 3.")
}

func TestInitialisms_CorrectsImo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Imo, that is the best option.", NewInitialismsLintGroup(), "In my opinion, that is the best option.")
}

func TestInitialisms_CorrectsOmg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Omg! That's incredible!", NewInitialismsLintGroup(), "Oh my god! That's incredible!")
}

func TestInitialisms_CorrectsBrb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hold on, brb.", NewInitialismsLintGroup(), "Hold on, be right back.")
}

func TestInitialisms_CorrectsTbh(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Tbh, I'm not impressed.", NewInitialismsLintGroup(), "To be honest, I'm not impressed.")
}

func TestInitialisms_CorrectsRly(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Rly excited for this.", NewInitialismsLintGroup(), "Really excited for this.")
}

func TestInitialisms_Issue2181(t *testing.T) {
	linttest.AssertSuggestionResult(t, "AFAIK, we don't currently have an issue for it.", NewInitialismsLintGroup(), "As far as i know, we don't currently have an issue for it.")
}

func TestInitialisms_CorrectsEli5(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Can you eli5 how this works?", NewInitialismsLintGroup(), "Can you explain like i'm five how this works?")
}

func TestInitialisms_CorrectsFwiw(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Fwiw, I think it's a good idea.", NewInitialismsLintGroup(), "For what it's worth, I think it's a good idea.")
}

func TestInitialisms_CorrectsIdk(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Idk if I'll make it to the party.", NewInitialismsLintGroup(), "I don't know if I'll make it to the party.")
}

func TestInitialisms_CorrectsIirc(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Iirc, the event starts at 6 PM.", NewInitialismsLintGroup(), "If i recall correctly, the event starts at 6 PM.")
}

func TestInitialisms_CorrectsIykyk(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Iykyk, this place is amazing.", NewInitialismsLintGroup(), "If you know, you know, this place is amazing.")
}

func TestInitialisms_CorrectsIcymi(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Icymi, the deadline is tomorrow.", NewInitialismsLintGroup(), "In case you missed it, the deadline is tomorrow.")
}

func TestInitialisms_CorrectsIrl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We should meet irl sometime.", NewInitialismsLintGroup(), "We should meet in real life sometime.")
}

func TestInitialisms_CorrectsPtal(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ptal at the document I sent.", NewInitialismsLintGroup(), "Please take a look at the document I sent.")
}

func TestInitialisms_ExpandsImhoBothWays(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Imho, this is a good idea.", NewInitialismsLintGroup(), []string{
		"In my humble opinion, this is a good idea.",
		"In my honest opinion, this is a good idea.",
	}, []string{"In my horrible opinion, this is a good idea."})
}

func TestInitialisms_CorrectsIiuc(t *testing.T) {
	linttest.AssertSuggestionResult(t, "iiuc build caching in hol4 works at the file level", NewInitialismsLintGroup(), "if i understand correctly build caching in hol4 works at the file level")
}

func TestInitialisms_CorrectsLmk(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Lmk if you need any debug logs.", NewInitialismsLintGroup(), "Let me know if you need any debug logs.")
}

func TestInitialisms_CorrectsAfaict(t *testing.T) {
	linttest.AssertSuggestionResult(t, "AFAICT the other drivers of recent progress in LLMs have been: ploughing in lots and lots of specialised training data", NewInitialismsLintGroup(), "As far as i can tell the other drivers of recent progress in LLMs have been: ploughing in lots and lots of specialised training data")
}

func TestInitialisms_CorrectsOtoh(t *testing.T) {
	linttest.AssertSuggestionResult(t, "\"Actual consequences\", OTOH, most directly relates to the camp(s) focused on \"embodiment\"", NewInitialismsLintGroup(), "\"Actual consequences\", On the other hand, most directly relates to the camp(s) focused on \"embodiment\"")
}

func TestInitialisms_CorrectsOotb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Oracle Linux OpenBLAS build isnt detected ootb, and it doesn't perform well compared to x86 for some reason.", NewInitialismsLintGroup(), "The Oracle Linux OpenBLAS build isnt detected ootb, and it doesn't perform well compared to x86 for some reason.")
}

func TestInitialisms_CorrectsAaui(t *testing.T) {
	linttest.AssertSuggestionResult(t, "AIUI, in the US something that's entirely machine-generated is not eligible for copyright protection.", NewInitialismsLintGroup(), "As i understand it, in the US something that's entirely machine-generated is not eligible for copyright protection.")
}

func TestInitialisms_CorrectsTbf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "TBF pretty much no one connects their devices anymore to anything other than charging.", NewInitialismsLintGroup(), "To be fair pretty much no one connects their devices anymore to anything other than charging.")
}
