package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSentenceCapitalization_CatchesBasic(t *testing.T) {
	linttest.AssertLintCount(t, "there is no way she is not guilty.", NewSentenceCapitalization(dict.Curated()), 1)
}

func TestSentenceCapitalization_NoPeriod(t *testing.T) {
	linttest.AssertLintCount(t, "there is no way she is not guilty", NewSentenceCapitalization(dict.Curated()), 1)
}

func TestSentenceCapitalization_TwoSentence(t *testing.T) {
	linttest.AssertLintCount(t, "i have complete conviction in this. she is absolutely guilty", NewSentenceCapitalization(dict.Curated()), 2)
}

func TestSentenceCapitalization_StartWithNumber(t *testing.T) {
	linttest.AssertLintCount(t, "53 is the length of the longest word.", NewSentenceCapitalization(dict.Curated()), 0)
}

func TestSentenceCapitalization_IgnoresUnlintable(t *testing.T) {
	linttest.AssertLintCount(t, "[`misspelled_word`] is assumed to be quite small (n < 100). ", NewSentenceCapitalization(dict.Curated()), 0)
}

func TestSentenceCapitalization_UnfazedUnlintable(t *testing.T) {
	linttest.AssertLintCount(t, "the linter should not be affected by `this` unlintable.", NewSentenceCapitalization(dict.Curated()), 1)
}

func TestSentenceCapitalization_UnfazedEllipsis(t *testing.T) {
	linttest.AssertLintCount(t, "the linter should not be affected by... that ellipsis.", NewSentenceCapitalization(dict.Curated()), 1)
}

func TestSentenceCapitalization_UnfazedComma(t *testing.T) {
	linttest.AssertLintCount(t, "the linter should not be affected by, that comma.", NewSentenceCapitalization(dict.Curated()), 1)
}

func TestSentenceCapitalization_Issue228AllowsLabels(t *testing.T) {
	linttest.AssertLintCount(t, "python lsp (fork of pyright)", NewSentenceCapitalization(dict.Curated()), 0)
}

func TestSentenceCapitalization_AllowCamelCaseTrademarks(t *testing.T) {
	linttest.AssertLintCount(t, "macOS 16 could be called something like Redwood or Shasta", NewSentenceCapitalization(dict.Curated()), 0)
}

func TestSentenceCapitalization_UppercaseUnamericanAtStart(t *testing.T) {
	t.Skip("This can't work because currently hyphens are not included in tokenized words\nalthough they are now permitted in `dictionary.dict`")
	linttest.AssertLintCount(t, "un-American starts with a lowercase letter and contains an uppercase letter, but is not a proper noun or trademark.", NewSentenceCapitalization(dict.Curated()), 1)
}

func TestSentenceCapitalization_AllowLowercaseProperNouns(t *testing.T) {
	linttest.AssertLintCount(t, "npm is the world's largest software registry. Open source developers from every "+
		"continent use npm to share and borrow packages, and many organizations use npm to "+
		"manage private development as well.", NewSentenceCapitalization(dict.Curated()), 0)
}

func TestSentenceCapitalization_DoesntFlagAfterEspIssue2753(t *testing.T) {
	linttest.AssertLintCount(t, "I'll go, esp. if it's a free event.", NewSentenceCapitalization(dict.Curated()), 0)
}

func TestSentenceCapitalization_AllowLowerCamelCaseNonProperNouns(t *testing.T) {
	linttest.AssertLintCount(t, "mRNA is synthesized from the coding sequence of a gene during the transcriptional process.", NewSentenceCapitalization(dict.Curated()), 0)
}
