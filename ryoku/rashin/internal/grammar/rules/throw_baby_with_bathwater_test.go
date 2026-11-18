package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestThrowBabyWithBathwater_FixAlmostThrowingTheBabyBathwater(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "trying to reinvent 'safe concurrency' while almost throwing the baby with the bathwater, and making swift even more complex", NewThrowBabyWithBathwater(), []string{
		"trying to reinvent 'safe concurrency' while almost throwing the baby out with the bathwater, and making swift even more complex",
		"trying to reinvent 'safe concurrency' while almost throwing out the baby with the bathwater, and making swift even more complex",
	}, []string{})
}

func TestThrowBabyWithBathwater_FixWithoutThrowingTheBabyWithTheBathwater(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Now, it's not that I can set min_resources to a larger value without throwing the baby with the bathwater.", NewThrowBabyWithBathwater(), []string{
		"Now, it's not that I can set min_resources to a larger value without throwing the baby out with the bathwater.",
		"Now, it's not that I can set min_resources to a larger value without throwing out the baby with the bathwater.",
	}, []string{})
}

func TestThrowBabyWithBathwater_FixSeemsLikeThrowingTheBabyWithTheBathwater(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "that seems like throwing the baby with the bathwater", NewThrowBabyWithBathwater(), []string{
		"that seems like throwing the baby out with the bathwater",
		"that seems like throwing out the baby with the bathwater",
	}, []string{})
}

func TestThrowBabyWithBathwater_FixThrowingAwayTheBabyBathwater(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Instead of gradually improving security, you are basically throwing away the baby with the bathwater.", NewThrowBabyWithBathwater(), "Instead of gradually improving security, you are basically throwing out the baby with the bathwater.")
}

func TestThrowBabyWithBathwater_FixThrowAwayTheBabyWithTheBathWater(t *testing.T) {
	linttest.AssertSuggestionResult(t, "then you don't have to throw away the baby with the bath water: you can apply the simple pattern optimization", NewThrowBabyWithBathwater(), "then you don't have to throw out the baby with the bath water: you can apply the simple pattern optimization")
}

func TestThrowBabyWithBathwater_FixIsThrowingTheBabyWithTheBathwater(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Yeah, this is throwing the baby with the bathwater.", NewThrowBabyWithBathwater(), []string{
		"Yeah, this is throwing the baby out with the bathwater.",
		"Yeah, this is throwing out the baby with the bathwater.",
	}, []string{})
}

func TestThrowBabyWithBathwater_FixYouThrowAwayTheBabyWithTheBathwater(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Changes are that you throw away the baby with the bathwater.", NewThrowBabyWithBathwater(), "Changes are that you throw out the baby with the bathwater.")
}

func TestThrowBabyWithBathwater_FixDidntThrowTheBabyWithTheBathwater(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "tried to remove all unnecessary things, I hope I didn't throw the baby with the bathwater.", NewThrowBabyWithBathwater(), []string{
		"tried to remove all unnecessary things, I hope I didn't throw the baby out with the bathwater.",
		"tried to remove all unnecessary things, I hope I didn't throw out the baby with the bathwater.",
	}, []string{})
}

func TestThrowBabyWithBathwater_FixFeelsLikeThrowingTheBabyWithTheBathwater(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "That feels a bit like throwing the baby with the bathwater.", NewThrowBabyWithBathwater(), []string{
		"That feels a bit like throwing the baby out with the bathwater.",
		"That feels a bit like throwing out the baby with the bathwater.",
	}, []string{})
}

func TestThrowBabyWithBathwater_FixIsntThrowingTheBabyWithTheBathwater(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "OP isn't throwing the baby with the bathwater and he explains it very well in his post", NewThrowBabyWithBathwater(), []string{
		"OP isn't throwing the baby out with the bathwater and he explains it very well in his post",
		"OP isn't throwing out the baby with the bathwater and he explains it very well in his post",
	}, []string{})
}

func TestThrowBabyWithBathwater_FixIThinkWeThrewBabyBathWater(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "But I think we threw the baby with the bath water by getting rid of the stack traces completely.", NewThrowBabyWithBathwater(), []string{
		"But I think we threw the baby out with the bath water by getting rid of the stack traces completely.",
		"But I think we threw out the baby with the bath water by getting rid of the stack traces completely.",
	}, []string{})
}

func TestThrowBabyWithBathwater_FixButWeThrewBabyBathWater(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I understand why the XML world of nonsense had to be stopped, but we threw the baby with the bath water.", NewThrowBabyWithBathwater(), []string{
		"I understand why the XML world of nonsense had to be stopped, but we threw the baby out with the bath water.",
		"I understand why the XML world of nonsense had to be stopped, but we threw out the baby with the bath water.",
	}, []string{})
}

func TestThrowBabyWithBathwater_FixThrown(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "in such a way that they no longer work, we have thrown the baby with the bath water", NewThrowBabyWithBathwater(), []string{
		"in such a way that they no longer work, we have thrown the baby out with the bath water",
		"in such a way that they no longer work, we have thrown out the baby with the bath water",
	}, []string{})
}

func TestThrowBabyWithBathwater_FixThrows(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Throws the baby with the bathwater.", NewThrowBabyWithBathwater(), []string{
		"Throws the baby out with the bathwater.",
		"Throws out the baby with the bathwater.",
	}, []string{})
}

func TestThrowBabyWithBathwater_FixThroughButLeaveMisspelling(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I should not through the baby with the bathwater as they say.", NewThrowBabyWithBathwater(), []string{
		"I should not through the baby out with the bathwater as they say.",
		"I should not through out the baby with the bathwater as they say.",
	}, []string{})
}

func TestThrowBabyWithBathwater_DontFlagTossing(t *testing.T) {
	linttest.AssertNoLints(t, "Give it a try before tossing the baby with the bath water.", NewThrowBabyWithBathwater())
}

func TestThrowBabyWithBathwater_DontFlagDumping(t *testing.T) {
	linttest.AssertNoLints(t, "Jumping from there straight to vintage equipment amounts to dumping the baby with the bathwater, but modern digital cameras can be quite pricey", NewThrowBabyWithBathwater())
}

func TestThrowBabyWithBathwater_DontFixParts(t *testing.T) {
	linttest.AssertNoLints(t, "so I was wondering if I'm throwing parts of the baby with the bath water", NewThrowBabyWithBathwater())
}

func TestThrowBabyWithBathwater_FixMissingThe(t *testing.T) {
	t.Skip("Upstream ignored: Missing `the` not implemented yet")
	t.Skip("Missing `the` not implemented yet")
	linttest.AssertSuggestionResult(t, "That's riding hype machine and throwing baby with bath water.", NewThrowBabyWithBathwater(), "That's riding hype machine and throwing out the baby with bath water.")
}

func TestThrowBabyWithBathwater_FixWaterInsteadOfBathwater(t *testing.T) {
	t.Skip("Upstream ignored: Missing `bath` not implemented yet")
	t.Skip("Missing `bath` not implemented yet")
	linttest.AssertSuggestionResult(t, "My concern is that they have thrown the baby with the water.", NewThrowBabyWithBathwater(), "My concern is that they have thrown out the baby with the water.")
}
