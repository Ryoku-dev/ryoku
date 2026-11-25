package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestBehindTheScenes_PluralizeWorkBts(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How does this tool work behind the scene.", NewBehindTheScenes(), "How does this tool work behind the scenes.")
}

func TestBehindTheScenes_PluralizeAndHyphenate(t *testing.T) {
	t.Skip("Upstream ignored: Correcting hyphenation is not yet implemented.")
	linttest.AssertSuggestionResult(t, "So, to open the 'real' behind the scene menu i need to do these steps:", NewBehindTheScenes(), "So, to open the 'real' behind-the-scenes menu i need to do these steps:")
}

func TestBehindTheScenes_DontFlagWhenHyphenatedToPreviousWord(t *testing.T) {
	linttest.AssertNoLints(t, "Contribute to techking11/react-behind-the-scene development by creating an account on GitHub.", NewBehindTheScenes())
}

func TestBehindTheScenes_PluralizeBtsProcessing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Behind-the-scene processing details are printed in the Log window.", NewBehindTheScenes(), "Behind-the-scenes processing details are printed in the Log window.")
}
