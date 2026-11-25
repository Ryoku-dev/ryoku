package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestItLooksLikeThat_DontFlagThatNounIsAlsoVerbPartOfNp(t *testing.T) {
	linttest.AssertNoLints(t, "It looks like that file tree view is just for things that have already been committed?", NewItLooksLikeThat())
}

func TestItLooksLikeThat_DontFlagThatNounIsAlsoAdj(t *testing.T) {
	linttest.AssertNoLints(t, "Yes, unfortunately it looks like that metric kind isn't supported yet.", NewItLooksLikeThat())
}

func TestItLooksLikeThat_CantFlagThatNounIsAlsoVerbFunction(t *testing.T) {
	linttest.AssertNoLints(t, "It looks like that function Config.validate_doc_path is only called in one place", NewItLooksLikeThat())
}

func TestItLooksLikeThat_DontFlagThatNounIsAlsoVerbTest(t *testing.T) {
	linttest.AssertNoLints(t, "It looks like that test runs with -sOFFSCREEN_FRAMEBUFFER", NewItLooksLikeThat())
}

func TestItLooksLikeThat_DontFlagThatOov(t *testing.T) {
	linttest.AssertNoLints(t, "It looks like that nms is not working.", NewItLooksLikeThat())
}

func TestItLooksLikeThat_DontFlagThatNounPad(t *testing.T) {
	linttest.AssertNoLints(t, "It looks like that pad was not covered in solder mask or glue", NewItLooksLikeThat())
}

func TestItLooksLikeThat_DontFlagThatNounPlural(t *testing.T) {
	linttest.AssertNoLints(t, "The issue we're running into is that it looks like that nodes not only want to peer via raft", NewItLooksLikeThat())
}

func TestItLooksLikeThat_FixThatThe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it looks like that the original products should have NULL in the value column", NewItLooksLikeThat(), "it looks like the original products should have NULL in the value column")
}

func TestItLooksLikeThat_FixThatSome(t *testing.T) {
	linttest.AssertSuggestionResult(t, "From first expresion it looks like that some tokkens or what was cached", NewItLooksLikeThat(), "From first expresion it looks like some tokkens or what was cached")
}

func TestItLooksLikeThat_DontFlagThatVerb3pSingPresIs(t *testing.T) {
	linttest.AssertNoLints(t, "Looking at the code it looks like that is not the case", NewItLooksLikeThat())
}

func TestItLooksLikeThat_DontFlagThatVerb3pSingPresComes(t *testing.T) {
	linttest.AssertNoLints(t, "it looks like that comes with additional compile-time dependencies", NewItLooksLikeThat())
}

func TestItLooksLikeThat_FixThatItVerbLemma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It looks like that Renovate decides to not reuse the branch when there are no changes in it", NewItLooksLikeThat(), "It looks like Renovate decides to not reuse the branch when there are no changes in it")
}

func TestItLooksLikeThat_DontFlagThatModalVerbMight(t *testing.T) {
	linttest.AssertNoLints(t, "It looks like that might be exactly what I needed!", NewItLooksLikeThat())
}

func TestItLooksLikeThat_DontFlagThatVerbModalWould(t *testing.T) {
	linttest.AssertNoLints(t, "but it looks like that would require writing the data out to vsimem and reading it back", NewItLooksLikeThat())
}

func TestItLooksLikeThat_FixThatVerbIngHave(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It looks like that having <br> tags inside them breaks the rendering", NewItLooksLikeThat(), "It looks like having <br> tags inside them breaks the rendering")
}

func TestItLooksLikeThat_FixThatVerbIngUsing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it looks like that using TensorFlow in conjunction with packages that use pybind11_abseil will fail", NewItLooksLikeThat(), "it looks like using TensorFlow in conjunction with packages that use pybind11_abseil will fail")
}

func TestItLooksLikeThat_DontFlagThatVerbSimplePast(t *testing.T) {
	linttest.AssertNoLints(t, "but it looks like that got accidentally reverted at some point", NewItLooksLikeThat())
}

func TestItLooksLikeThat_FixThatSubjObjPronounItWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It looks like that it was not improved a lot.", NewItLooksLikeThat(), "It looks like it was not improved a lot.")
}

func TestItLooksLikeThat_FixThatSubjObjPronounItWorks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Thx, it looks like that it works for Inpainting itself", NewItLooksLikeThat(), "Thx, it looks like it works for Inpainting itself")
}

func TestItLooksLikeThat_FixThatSubjObjPronounYou(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It looks like that you can't use the files in combination.", NewItLooksLikeThat(), "It looks like you can't use the files in combination.")
}

func TestItLooksLikeThat_DontFlagThats(t *testing.T) {
	linttest.AssertNoLints(t, "it looks like that's how you access the system changeset functionality", NewItLooksLikeThat())
}

func TestItLooksLikeThat_CantFlagThatIf(t *testing.T) {
	linttest.AssertNoLints(t, "It looks like that if the server goes away in the middle of a request, and a request is cancelled", NewItLooksLikeThat())
}

func TestItLooksLikeThat_CantFlagThatBut(t *testing.T) {
	linttest.AssertNoLints(t, "Yes, it looks like that but it is unreasonable since the shim executable is in the same directory", NewItLooksLikeThat())
}
