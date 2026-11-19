package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTherein_FixThereInStandsSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "WELL THERE IN STANDS THE ISSUE RIGHT THERE BRIGHT SPARK.", NewTherein(), "WELL THEREIN STANDS THE ISSUE RIGHT THERE BRIGHT SPARK.")
}

func TestTherein_ContainedThereHyphenIn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "contained there-in", NewTherein(), "contained therein")
}

func TestTherein_FixThereInIsHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "name and password contained there-in is only for accessing the PostgreSQL instance inside the created ephemeral container.", NewTherein(), "name and password contained therein is only for accessing the PostgreSQL instance inside the created ephemeral container.")
}

func TestTherein_DontFlagThereInA(t *testing.T) {
	linttest.AssertNoLints(t, "It might have started happening after I tried to bump the ancient node version used there in a specific PR, but affecting unrelated ones.", NewTherein())
}

func TestTherein_DontFlagOutThereInAn(t *testing.T) {
	linttest.AssertNoLints(t, "But if it actually became a language out there in an ecosystem, I worry that you might have people start abusing that.", NewTherein())
}

func TestTherein_DontFlagThereInIts(t *testing.T) {
	linttest.AssertNoLints(t, "It was a rich cream color, bright with nickel, swollen here and there in its monstrous length with triumphant hat-boxes", NewTherein())
}

func TestTherein_DontFlagThereInThat(t *testing.T) {
	linttest.AssertNoLints(t, "It's over there in that box.", NewTherein())
}

func TestTherein_DontFlagThereInThe(t *testing.T) {
	linttest.AssertNoLints(t, "What was it up there in the song that seemed to be calling her back inside?", NewTherein())
}

func TestTherein_DontFlagThereInYour(t *testing.T) {
	linttest.AssertNoLints(t, "If you want to get the latest changes there in your external save button, then you could use the html.get method.", NewTherein())
}

func TestTherein_DontFlagThereInYear(t *testing.T) {
	linttest.AssertNoLints(t, "... but I think by the time I got moved there in 2001, Casey had moved on.", NewTherein())
}

func TestTherein_DontFlagPointItThereInSettings(t *testing.T) {
	linttest.AssertNoLints(t, "JetBrains Junie can read `AGENTS.md` once you point it there in Settings → Tools → Junie → Project Settings → Guidelines Path (it is not automatic yet).", NewTherein())
}

func TestTherein_DontFlagGetsYouThereInTwoCommands(t *testing.T) {
	linttest.AssertNoLints(t, "Quick Start below gets you there in two commands:", NewTherein())
}
