package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestNotOnlyInversion_FixNotOnlyHeIs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "not only he is discouraged from look at them but he can't manipulate them", NewNotOnlyInversion(), "not only is he discouraged from look at them but he can't manipulate them")
}

func TestNotOnlyInversion_FixNotOnlyHeWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "not only he was born as a man, he was the Promised Messiah", NewNotOnlyInversion(), "not only was he born as a man, he was the Promised Messiah")
}

func TestNotOnlyInversion_FixNotOnlyIAm(t *testing.T) {
	t.Skip("replace_with_match_case goes by character index matching")
	linttest.AssertSuggestionResult(t, "Not only I am proud of the work we have accomplished together but also I have learned so much from you about statistics, sciences and beyond.", NewNotOnlyInversion(), "Not only am I proud of the work we have accomplished together but also I have learned so much from you about statistics, sciences and beyond.")
}

func TestNotOnlyInversion_FixNotOnlyIWas(t *testing.T) {
	t.Skip("replace_with_match_case goes by character index matching")
	linttest.AssertSuggestionResult(t, "Not only I was wrong in saying the right meaning, I was also wrong in stating the parts of speech", NewNotOnlyInversion(), "Not only was I wrong in saying the right meaning, I was also wrong in stating the parts of speech")
}

func TestNotOnlyInversion_FixNotOnlyItIs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Not only it is not the same problem, #899 is a solution suggested in #969.", NewNotOnlyInversion(), "Not only is it not the same problem, #899 is a solution suggested in #969.")
}

func TestNotOnlyInversion_FixNotOnlyItWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "because not only it was unlikely that I could answer any question I also felt that I cannot even ask any on-topic question", NewNotOnlyInversion(), "because not only was it unlikely that I could answer any question I also felt that I cannot even ask any on-topic question")
}

func TestNotOnlyInversion_FixNotOnlyTheyAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Not only they are written in much cleaner and verbose way, they are also available in 6 languages like russian.", NewNotOnlyInversion(), "Not only are they written in much cleaner and verbose way, they are also available in 6 languages like russian.")
}

func TestNotOnlyInversion_FixNotOnlyTheyWere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Not only they were tall, but also they were strong. ", NewNotOnlyInversion(), "Not only were they tall, but also they were strong. ")
}

func TestNotOnlyInversion_FixNotOnlyWeAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Here not only we are using multiline string to create an HTML output but we also are binding variable using expression language.", NewNotOnlyInversion(), "Here not only are we using multiline string to create an HTML output but we also are binding variable using expression language.")
}

func TestNotOnlyInversion_FixNotOnlyWeWere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Not only we were using an old version of our front end library (React), but we were also locked into a version of our functional programming utility package (Lodash) released more than three years ago.", NewNotOnlyInversion(), "Not only were we using an old version of our front end library (React), but we were also locked into a version of our functional programming utility package (Lodash) released more than three years ago.")
}

func TestNotOnlyInversion_FixNotOnlyYouAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So not only you are a perfect reference, but also a viable candidate for drop-n-use.", NewNotOnlyInversion(), "So not only are you a perfect reference, but also a viable candidate for drop-n-use.")
}

func TestNotOnlyInversion_FixNotOnlyYouWere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "because not only you were able to explain it but you were able to show me how to sort it so it would displayed", NewNotOnlyInversion(), "because not only were you able to explain it but you were able to show me how to sort it so it would displayed")
}
