package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOnceOrTwice_CorrectsOnceATwice(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He wants to do it once a twice a month.", NewOnceOrTwice(), "He wants to do it once or twice a month.")
}

func TestOnceOrTwice_AllowsOnceOrTwice(t *testing.T) {
	linttest.AssertNoLints(t, "He wants to do it once or twice a month.", NewOnceOrTwice())
}

func TestOnceOrTwice_CorrectsOnceATwiceSentenceStart(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Once a twice, we gathered for coffee.", NewOnceOrTwice(), "Once or twice, we gathered for coffee.")
}

func TestOnceOrTwice_CorrectsOnceATwiceUppercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "ONCE A TWICE WE MET.", NewOnceOrTwice(), "ONCE OR TWICE WE MET.")
}

func TestOnceOrTwice_CorrectsOnceATwiceMixedCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "once a Twice sounds odd.", NewOnceOrTwice(), "once or Twice sounds odd.")
}

func TestOnceOrTwice_CorrectsOnceATwiceWithExclamation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Let's do it once a twice!", NewOnceOrTwice(), "Let's do it once or twice!")
}

func TestOnceOrTwice_CorrectsOnceATwiceWithQuestionMark(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You really tried once a twice?", NewOnceOrTwice(), "You really tried once or twice?")
}

func TestOnceOrTwice_CorrectsOnceATwiceInsideQuotes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He said, \"once a twice\" without thinking.", NewOnceOrTwice(), "He said, \"once or twice\" without thinking.")
}

func TestOnceOrTwice_CorrectsOnceATwiceWithComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We planned it once a twice, but never finished.", NewOnceOrTwice(), "We planned it once or twice, but never finished.")
}

func TestOnceOrTwice_CorrectsOnceATwiceWithParentheses(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Try it (just once a twice) before judging.", NewOnceOrTwice(), "Try it (just once or twice) before judging.")
}

func TestOnceOrTwice_CorrectsOnceATwiceAfterColon(t *testing.T) {
	linttest.AssertSuggestionResult(t, "My answer is simple: once a twice is too many.", NewOnceOrTwice(), "My answer is simple: once or twice is too many.")
}

func TestOnceOrTwice_CorrectsOnceATwiceWithDoubleSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We tested once a twice  before launch.", NewOnceOrTwice(), "We tested once or twice  before launch.")
}

func TestOnceOrTwice_CorrectsOnceATwiceBeforeSemicolon(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They tried once a twice; it still failed.", NewOnceOrTwice(), "They tried once or twice; it still failed.")
}

func TestOnceOrTwice_CorrectsOnceATwiceNewlineSplit(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We met once a twice\nwhen the cafe was quiet.", NewOnceOrTwice(), "We met once or twice\nwhen the cafe was quiet.")
}

func TestOnceOrTwice_CorrectsOnceATwiceWithTab(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Schedule it once a twice\tfor testing.", NewOnceOrTwice(), "Schedule it once or twice\tfor testing.")
}

func TestOnceOrTwice_CorrectsOnceATwiceMultipleSentences(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Do it once a twice. Then rest.", NewOnceOrTwice(), "Do it once or twice. Then rest.")
}

func TestOnceOrTwice_CorrectsOnceATwiceBeforePeriod(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He rehearsed once a twice.", NewOnceOrTwice(), "He rehearsed once or twice.")
}

func TestOnceOrTwice_CorrectsOnceATwiceWithTrailingSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Practice once a twice .", NewOnceOrTwice(), "Practice once or twice .")
}

func TestOnceOrTwice_CorrectsOnceATwiceBeforeDash(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He called once a twice—no response.", NewOnceOrTwice(), "He called once or twice—no response.")
}

func TestOnceOrTwice_CorrectsOnceATwiceAroundEmDash(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She visits once a twice—maybe thrice.", NewOnceOrTwice(), "She visits once or twice—maybe thrice.")
}

func TestOnceOrTwice_CorrectsOnceATwiceBeforeQuote(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We heard once a twice, \"she's late.\"", NewOnceOrTwice(), "We heard once or twice, \"she's late.\"")
}

func TestOnceOrTwice_CorrectsOnceATwiceAllCapsSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "DO IT ONCE A TWICE RIGHT NOW!", NewOnceOrTwice(), "DO IT ONCE OR TWICE RIGHT NOW!")
}

func TestOnceOrTwice_AllowsOnceATimeStory(t *testing.T) {
	linttest.AssertNoLints(t, "Once a time, in a distant land...", NewOnceOrTwice())
}

func TestOnceOrTwice_AllowsOnceAWeekRoutine(t *testing.T) {
	linttest.AssertNoLints(t, "We meet once a week to sync up.", NewOnceOrTwice())
}

func TestOnceOrTwice_AllowsOnceAWhilePhrase(t *testing.T) {
	linttest.AssertNoLints(t, "Check in every once a while to stay updated.", NewOnceOrTwice())
}

func TestOnceOrTwice_AllowsOnceOrTwiceUppercase(t *testing.T) {
	linttest.AssertNoLints(t, "ONCE OR TWICE, WE MADE IT WORK.", NewOnceOrTwice())
}

func TestOnceOrTwice_AllowsTwiceWithoutOnce(t *testing.T) {
	linttest.AssertNoLints(t, "We only managed it twice this year.", NewOnceOrTwice())
}

func TestOnceOrTwice_AllowsOnceAndTwiceSeparated(t *testing.T) {
	linttest.AssertNoLints(t, "Once I tried; twice I failed.", NewOnceOrTwice())
}

func TestOnceOrTwice_AllowsOncemisatypo(t *testing.T) {
	linttest.AssertNoLints(t, "oncemisatypo appears once a line.", NewOnceOrTwice())
}

func TestOnceOrTwice_AllowsSpacedWords(t *testing.T) {
	linttest.AssertNoLints(t, "We say once at twice distance to be safe.", NewOnceOrTwice())
}
