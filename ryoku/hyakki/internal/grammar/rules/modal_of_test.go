package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestModalOf_TestLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "could of", NewModalOf(), "could have")
}

func TestModalOf_TestNegative(t *testing.T) {
	linttest.AssertSuggestionResult(t, "mightn't of", NewModalOf(), "mightn't have")
}

func TestModalOf_TestUppercaseNegative(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Mustn't of", NewModalOf(), "Mustn't have")
}

func TestModalOf_TestFalsePositiveOfCourse(t *testing.T) {
	linttest.AssertLintCount(t, "should of course", NewModalOf(), 0)
}

func TestModalOf_TestFalsePositiveTheMightOf(t *testing.T) {
	linttest.AssertLintCount(t, "the might of", NewModalOf(), 0)
}

func TestModalOf_TestFalsePositiveGreatMightOf(t *testing.T) {
	linttest.AssertLintCount(t, "great might of", NewModalOf(), 0)
}

func TestModalOf_TestFalsePositiveCapitalNegative(t *testing.T) {
	linttest.AssertLintCount(t, "Wouldn't of course", NewModalOf(), 0)
}

func TestModalOf_TestBuggyImplementation(t *testing.T) {
	linttest.AssertLintCount(t, "... could of just been a buggy implementation", NewModalOf(), 1)
}

func TestModalOf_TestMissedOne(t *testing.T) {
	linttest.AssertLintCount(t, "We already have a function ... that nedb can understand so we might of missed one.", NewModalOf(), 1)
}

func TestModalOf_TestUserOption(t *testing.T) {
	linttest.AssertLintCount(t, "im more likely to believe you might of left in the 'user' option", NewModalOf(), 1)
}

func TestModalOf_CatchesMustOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ah I must of missed that part.", NewModalOf(), "Ah I must have missed that part.")
}

func TestModalOf_CatchesShouldOf(t *testing.T) {
	linttest.AssertLintCount(t, "Yeah I should of just mentioned it should of been a for of.", NewModalOf(), 2)
}

func TestModalOf_CatchesWouldOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "now this issue would of caused hundreds of thousands of extra lines", NewModalOf(), "now this issue would have caused hundreds of thousands of extra lines")
}

func TestModalOf_DoesntCatchYouCouldOfCourse(t *testing.T) {
	linttest.AssertLintCount(t, "You could of course explicit the else with each possibility", NewModalOf(), 0)
}

func TestModalOf_DoesntCatchCompilerCouldOfCourse(t *testing.T) {
	linttest.AssertLintCount(t, "The compiler could of course detect this too", NewModalOf(), 0)
}

func TestModalOf_DoesntCatchMightOfCourseBe(t *testing.T) {
	linttest.AssertLintCount(t, "There might of course be other places where not implementing the IMemberSource might break ...", NewModalOf(), 0)
}

func TestModalOf_DoesntCatchNotAMustOfCourse(t *testing.T) {
	linttest.AssertLintCount(t, "Not a must of course if the convention should be .ts", NewModalOf(), 0)
}

func TestModalOf_DoesntCatchMustOfCourseAlso(t *testing.T) {
	linttest.AssertLintCount(t, "the schedular must of course also have run through", NewModalOf(), 0)
}

func TestModalOf_DoesntCatchShouldOfCourseNot(t *testing.T) {
	linttest.AssertLintCount(t, "not being local should of course not be supported", NewModalOf(), 0)
}

func TestModalOf_DoesntCatchWouldOfCourseJust(t *testing.T) {
	linttest.AssertLintCount(t, "I would of course just test this by compiling with MATX_MULTI_GPU=ON", NewModalOf(), 0)
}

func TestModalOf_DoesntCatchToTakeOnTheFullMightOfNato(t *testing.T) {
	linttest.AssertLintCount(t, "To take on the full might of NATO.", NewModalOf(), 0)
}

func TestModalOf_DoesntCatchMixedCaseOfCourse(t *testing.T) {
	linttest.AssertLintCount(t, "... for now you could of Course put ...", NewModalOf(), 0)
}

func TestModalOf_CatchesMixedCaseCouldOfPut(t *testing.T) {
	linttest.AssertLintCount(t, "... for now you could of Put ...", NewModalOf(), 1)
}

func TestModalOf_DoesntCatchNounWillOf(t *testing.T) {
	linttest.AssertLintCount(t, "the will of the many", NewModalOf(), 0)
}

func TestModalOf_DoesntCatchNounWillOfEdgecase(t *testing.T) {
	linttest.AssertLintCount(t, "he sent us a will of his", NewModalOf(), 0)
}

func TestModalOf_CatchModalWillOf(t *testing.T) {
	linttest.AssertLintCount(t, "that will of an impact", NewModalOf(), 1)
}

func TestModalOf_CatchMayOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I may of made a mistake", NewModalOf(), "I may have made a mistake")
}

func TestModalOf_DontFlagInMayOfLastYearBug2786(t *testing.T) {
	linttest.AssertNoLints(t, "This happened in May of last year.", NewModalOf())
}

func TestModalOf_DontFlagCanOfRedBull2807(t *testing.T) {
	linttest.AssertNoLints(t, "I drank a can of Red Bull.", NewModalOf())
}
