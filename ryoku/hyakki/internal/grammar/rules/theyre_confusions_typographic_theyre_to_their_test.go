package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTheyreConfusionsTypographicTheyreToTheir_CorrectsSmartApostrophePossessive(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I think they’re house is the blue one.", NewTypographicTheyreToTheir(), "I think their house is the blue one.")
}

func TestTheyreConfusionsTypographicTheyreToTheir_IgnoresAsciiFormForExistingRule(t *testing.T) {
	linttest.AssertLintCount(t, "I think they're house is the blue one.", NewTypographicTheyreToTheir(), 0)
}

func TestTheyreConfusionsTypographicTheyreToTheir_IgnoresNonPossessiveUsage(t *testing.T) {
	linttest.AssertLintCount(t, "I think they’re coming tonight.", NewTypographicTheyreToTheir(), 0)
}

func TestTheyreConfusionsTypographicTheyreToTheir_IgnoresContractionExamplesFromBooks(t *testing.T) {
	linttest.AssertLintCount(t, "No, they’re not.", NewTypographicTheyreToTheir(), 0)
	linttest.AssertLintCount(t, "I don’t know what they’re like.", NewTypographicTheyreToTheir(), 0)
	linttest.AssertLintCount(t, "They’re all over crumbs.", NewTypographicTheyreToTheir(), 0)
	linttest.AssertLintCount(t, "They’re done with blacking, I believe.", NewTypographicTheyreToTheir(), 0)
}
