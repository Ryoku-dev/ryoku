package linttest

import "testing"

func fixture(errors, fixes []string) *TestLinter {
	return NewTestLinter(TestLinterMap{{Errors: errors, Fixes: fixes}})
}
func TestTest1To1ErrorToFix(t *testing.T) {
	AssertSuggestionResult(t, "bad", fixture([]string{"bad"}, []string{"good"}), "good")
}
func TestTest1To2ErrorToFixes(t *testing.T) {
	l := fixture([]string{"bad"}, []string{"good1", "good2"})
	AssertSuggestionResult(t, "bad", l, "good1")
	AssertSuggestionResult(t, "bad", l, "good2")
}
func TestTest2To1ErrorsToFix(t *testing.T) {
	l := fixture([]string{"bad1", "bad2"}, []string{"good"})
	AssertSuggestionResult(t, "bad1", l, "good")
	AssertSuggestionResult(t, "bad2", l, "good")
}
func TestTest2To2ErrorsToFixes(t *testing.T) {
	l := fixture([]string{"bad1", "bad2"}, []string{"good1", "good2"})
	AssertSuggestionResult(t, "bad1", l, "good1")
	AssertSuggestionResult(t, "bad2", l, "good2")
	AssertSuggestionResult(t, "bad1", l, "good2")
	AssertSuggestionResult(t, "bad2", l, "good1")
}
func TestVerifyNoLints(t *testing.T) { AssertNoLints(t, "hello world", NewTestLinter(nil)) }
func TestVerify1Lint(t *testing.T) {
	AssertLintCount(t, "heloo world", fixture([]string{"heloo"}, []string{"hello"}), 1)
}
func TestVerify2Lints(t *testing.T) {
	AssertLintCount(t, "heloo wolrd", NewTestLinter(TestLinterMap{{[]string{"heloo"}, []string{"hello"}}, {[]string{"wolrd"}, []string{"world"}}}), 2)
}
func TestVerifyNoSuggestions(t *testing.T) {
	AssertSuggestionCount(t, "afjehwkf", NewTestLinter(nil), 0)
}
func TestVerify1Suggestion(t *testing.T) {
	AssertSuggestionCount(t, "dictionery", fixture([]string{"dictionery"}, []string{"dictionary"}), 1)
}
func TestVerifyFixOneLint(t *testing.T) {
	AssertSuggestionResult(t, "find the misstake and fix it", fixture([]string{"misstake"}, []string{"mistake"}), "find the mistake and fix it")
}

type failingAssertion struct{}

func (failingAssertion) Helper()               {}
func (failingAssertion) Fatalf(string, ...any) { panic(failingAssertion{}) }
func mustFail(t *testing.T, f func(TestingT)) {
	t.Helper()
	defer func() {
		v := recover()
		if _, ok := v.(failingAssertion); !ok {
			t.Fatalf("expected failed assertion, got %v", v)
		}
	}()
	f(failingAssertion{})
}
func TestVerifyUnableToFixOneSpanishLint(t *testing.T) {
	mustFail(t, func(recorder TestingT) {
		AssertSuggestionResult(recorder, "Hay una orrrer", NewTestLinter(nil), "Hay una error")
	})
}
func TestVerifyFixTwoLints(t *testing.T) {
	AssertSuggestionResult(t, "find two misstakes and fix theem", NewTestLinter(TestLinterMap{{[]string{"misstakes"}, []string{"mistakes"}}, {[]string{"theem"}, []string{"them"}}}), "find two mistakes and fix them")
}
func TestVerifyFixFiveTypos(t *testing.T) {
	AssertSuggestionResult(t, "Please recieve teh payment untill thier authorization occured", NewTestLinter(TestLinterMap{{[]string{"recieve"}, []string{"receive"}}, {[]string{"teh"}, []string{"the"}}, {[]string{"untill"}, []string{"until"}}, {[]string{"thier"}, []string{"their"}}, {[]string{"occured"}, []string{"occurred"}}}), "Please receive the payment until their authorization occurred")
}
func TestVerifySoleSuggestionIsTheOneWeWanted(t *testing.T) {
	AssertNotInSuggestionResult(t, "Baby cats are called kitens", NewTestLinter(nil), "Baby cats are called puppies")
}
func TestVerifySoleSuggestionNotInResultFails(t *testing.T) {
	mustFail(t, func(recorder TestingT) {
		AssertNotInSuggestionResult(recorder, "heloo", fixture([]string{"heloo"}, []string{"hello"}), "hello")
	})
}
func TestVerifyMutalCorrectionsCauseFailure(t *testing.T) {
	mustFail(t, func(recorder TestingT) {
		AssertSuggestionResult(recorder, "gooder", fixture([]string{"gooder"}, []string{"more good"}), "better")
	})
}
func TestPrimarySuggestionAndGoodBadSemantics(t *testing.T) {
	l := fixture([]string{"bad"}, []string{"first", "second"})
	AssertSuggestionResult(t, "bad", l, "second")
	AssertFirstSuggestionResult(t, "bad", l, "first")
	mustFail(t, func(r TestingT) { AssertFirstSuggestionResult(r, "bad", l, "second") })
	AssertGoodAndBadSuggestions(t, "bad", l, []string{"first", "second"}, []string{"third"})
	mustFail(t, func(r TestingT) { AssertGoodAndBadSuggestions(r, "bad", l, []string{"third"}, nil) })
	mustFail(t, func(r TestingT) { AssertGoodAndBadSuggestions(r, "bad", l, nil, []string{"second"}) })
	AssertLintMessage(t, "bad", l, l.Description())
	mustFail(t, func(r TestingT) { AssertLintMessage(r, "fine", l, l.Description()) })
}
func TestSuggestionDepthLimit(t *testing.T) {
	AssertSuggestionResult(t, "same", NewTestLinter(nil), "same")
	mustFail(t, func(r TestingT) { AssertSuggestionResult(r, "bad", fixture([]string{"bad"}, []string{"bad"}), "other") })
}
