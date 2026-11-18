package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestUseTitleCase_DoesntLowercaseThisInGithubTemplateTitle(t *testing.T) {
	linttest.AssertNoLints(t, "# How Has This Been Tested?", NewUseTitleCase(dict.Curated()))
}

func TestUseTitleCase_IgnoresLeadingNumberListMarkerInHeading(t *testing.T) {
	linttest.AssertNoLints(t, "### 1. To Do a Thing", NewUseTitleCase(dict.Curated()))
}

func TestUseTitleCase_DoesNotCapitalizeAmInTimeExpression(t *testing.T) {
	linttest.AssertNoLints(t, "# Meeting at 9:05am", NewUseTitleCase(dict.Curated()))
}

func TestUseTitleCase_DoesNotCapitalizePmInTimeExpression(t *testing.T) {
	linttest.AssertNoLints(t, "# Dinner at 7pm", NewUseTitleCase(dict.Curated()))
}

func TestUseTitleCase_DoesNotCapitalizeStandaloneXBetweenWords(t *testing.T) {
	linttest.AssertNoLints(t, "# Project 1 x Project 2", NewUseTitleCase(dict.Curated()))
}

func TestUseTitleCase_CapitalizesStandaloneXWhenLastWord(t *testing.T) {
	linttest.AssertNoLints(t, "# Project X", NewUseTitleCase(dict.Curated()))
}
