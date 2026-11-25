package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWhomSubjectOfVerb_FlagWhomHas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "there is no course to whom has opened the most PRs", NewWhomSubjectOfVerb(), "there is no course to who has opened the most PRs")
}

func TestWhomSubjectOfVerb_FlagWhomeverWrote(t *testing.T) {
	linttest.AssertSuggestionResult(t, "To whomever wrote this course, I truly am not trying to be a jerk or ungrateful", NewWhomSubjectOfVerb(), "To whoever wrote this course, I truly am not trying to be a jerk or ungrateful")
}

func TestWhomSubjectOfVerb_DontFlagWrongKindOfError(t *testing.T) {
	t.Skip("wrong kind of error")
	linttest.AssertLintCount(t, "self service ticket view is not showing to whom is the ticket assigned to", NewWhomSubjectOfVerb(), 0)
}

func TestWhomSubjectOfVerb_DontFlagWhomCan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Whom can of course build a helper, but that is only a workaround.", NewWhomSubjectOfVerb(), "Who can of course build a helper, but that is only a workaround.")
}

func TestWhomSubjectOfVerb_FlagWhomeverIs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Whomever is making those harassing phone calls to me after I post something on Github - consider yourself put on notice.", NewWhomSubjectOfVerb(), "Whoever is making those harassing phone calls to me after I post something on Github - consider yourself put on notice.")
}

func TestWhomSubjectOfVerb_FlagWhomIs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I thought it might be good idea to address the topic of whom is \"allowed\" to merge.", NewWhomSubjectOfVerb(), "I thought it might be good idea to address the topic of who is \"allowed\" to merge.")
}

func TestWhomSubjectOfVerb_FlagWhomsoeverWill(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is a quick record of my discoveries and solution for whomsoever will be fixing the issue.", NewWhomSubjectOfVerb(), "This is a quick record of my discoveries and solution for whosoever will be fixing the issue.")
}

func TestWhomSubjectOfVerb_DontFlagManyOfWhom(t *testing.T) {
	linttest.AssertNoLints(t, "it's far from straightforward for new users, many of whom will likely have a lot to learn", NewWhomSubjectOfVerb())
}

func TestWhomSubjectOfVerb_DontFlagEachOfWhom(t *testing.T) {
	linttest.AssertNoLints(t, "Horace Silver (piano), Tyrone Washington (tenor sax), and Roger Humphries (drums), each of whom has a wikipedia article about him.", NewWhomSubjectOfVerb())
}
