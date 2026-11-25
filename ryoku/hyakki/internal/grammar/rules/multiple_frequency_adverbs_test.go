package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMultipleFrequencyAdverbs_OftenNeverWithoutComma(t *testing.T) {
	linttest.AssertLintCount(t, "People have often never even heard of nutrinos, but yeah, about 100 billion solar nutrinos are passing through your thumbnail every second.\n", NewMultipleFrequencyAdverbs(), 1)
}

func TestMultipleFrequencyAdverbs_OftenNeverWithComma(t *testing.T) {
	linttest.AssertLintCount(t, "often, never", NewMultipleFrequencyAdverbs(), 1)
}

func TestMultipleFrequencyAdverbs_SometimesNever(t *testing.T) {
	linttest.AssertLintCount(t, "Using @ directive in comments renders modal/portal that is sometimes never destroyed until app is closed.", NewMultipleFrequencyAdverbs(), 1)
}

func TestMultipleFrequencyAdverbs_UsuallyAlways(t *testing.T) {
	linttest.AssertLintCount(t, "Unfortunately, I can't switch to Pip with Mamba (to avoid conda), which I usually always do.", NewMultipleFrequencyAdverbs(), 1)
}

func TestMultipleFrequencyAdverbs_SometimesUsually(t *testing.T) {
	linttest.AssertLintCount(t, "I do my best to fix stuff when some issues pop up, but it sometimes usually doesn't work out.", NewMultipleFrequencyAdverbs(), 1)
}
