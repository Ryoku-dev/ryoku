package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDoMistake_DidAMistake(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hi, I did a mistake in my NGINX config file and so once the container is launched, it logs the error...", NewDoMistake(), "Hi, I made a mistake in my NGINX config file and so once the container is launched, it logs the error...")
}

func TestDoMistake_DidMyMistakes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Where i did my mistakes?", NewDoMistake(), "Where i made my mistakes?")
}

func TestDoMistake_DidSeveralMistakes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Maybe I did several mistakes, but I can only find a message about one?", NewDoMistake(), "Maybe I made several mistakes, but I can only find a message about one?")
}

func TestDoMistake_DidSomeMistakes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I made this program to learn goto use. but did some mistakes somewhere", NewDoMistake(), "I made this program to learn goto use. but made some mistakes somewhere")
}

func TestDoMistake_DidThatMistake(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and believe me, I did that mistake too", NewDoMistake(), "and believe me, I made that mistake too")
}

func TestDoMistake_DidTheMistake(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The issue describe is the person who did the mistake in the past & that same person is NOW correcting other people", NewDoMistake(), "The issue describe is the person who made the mistake in the past & that same person is NOW correcting other people")
}

func TestDoMistake_DidThisMistake(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Are there famous mathematicians who did this mistake?", NewDoMistake(), "Are there famous mathematicians who made this mistake?")
}

func TestDoMistake_DoManyMistakes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I observed that my coworkers do many mistakes using the field calculator", NewDoMistake(), "I observed that my coworkers make many mistakes using the field calculator")
}

func TestDoMistake_DoMistake(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If you do a mistake that causes alot of problems, please use the command to redo", NewDoMistake(), "If you make a mistake that causes alot of problems, please use the command to redo")
}

func TestDoMistake_DoSomeMistakes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "so probably if my colleagues do some mistakes I tend to learn them as well", NewDoMistake(), "so probably if my colleagues make some mistakes I tend to learn them as well")
}

func TestDoMistake_DoTheMistake(t *testing.T) {
	linttest.AssertSuggestionResult(t, "do I need to explicitly mention that I did not do the mistake to do not lose the point?", NewDoMistake(), "do I need to explicitly mention that I did not make the mistake to do not lose the point?")
}

func TestDoMistake_DoThisMistake(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I barely remember any frontend developer that wouldn't do this mistake at least once.", NewDoMistake(), "I barely remember any frontend developer that wouldn't make this mistake at least once.")
}

func TestDoMistake_DoThisMistakes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I do this mistakes to check the command detekt with type resolution", NewDoMistake(), "I make this mistakes to check the command detekt with type resolution")
}

func TestDoMistake_DoThoseMistakes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "An experienced developer could do those mistakes as well", NewDoMistake(), "An experienced developer could make those mistakes as well")
}

func TestDoMistake_DoingAMistake(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Here at work, a colleague asked if we were doing a mistake by using the ReactDOM.renderToStaticMarkup on the client side.", NewDoMistake(), "Here at work, a colleague asked if we were making a mistake by using the ReactDOM.renderToStaticMarkup on the client side.")
}

func TestDoMistake_DoingSeveralMistakes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I realized I was doing several mistakes", NewDoMistake(), "I realized I was making several mistakes")
}

func TestDoMistake_DoingTheMistkae(t *testing.T) {
	linttest.AssertSuggestionResult(t, "where am i doing the mistake?", NewDoMistake(), "where am i making the mistake?")
}

func TestDoMistake_DoneSomeMistake(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Might be I have done some mistake, that I do not know.", NewDoMistake(), "Might be I have made some mistake, that I do not know.")
}

func TestDoMistake_DoneThisMistake(t *testing.T) {
	linttest.AssertSuggestionResult(t, "how many more users have done this mistake?", NewDoMistake(), "how many more users have made this mistake?")
}

func TestDoMistake_DontFlagWhenDoesAMistake(t *testing.T) {
	linttest.AssertNoLints(t, "When does a mistake become standard usage? ", NewDoMistake())
}

func TestDoMistake_DontFlagDidThatMistakeVerb(t *testing.T) {
	linttest.AssertNoLints(t, "Did that mistake occurred before or after the day 2 backup?", NewDoMistake())
}

func TestDoMistake_DontFlagDoesThisMistakeVerb(t *testing.T) {
	linttest.AssertNoLints(t, "Does this mistake invalidate your thesis?", NewDoMistake())
}

func TestDoMistake_DontFlagDoesTheMistakeVerb(t *testing.T) {
	linttest.AssertNoLints(t, "Does the mistake change the meaning of the quotation?", NewDoMistake())
}
