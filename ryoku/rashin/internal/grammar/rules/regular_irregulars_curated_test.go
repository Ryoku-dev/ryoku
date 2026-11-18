package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRegularIrregulars_FixFeeded(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Rune is a TUI editor, so I feeded the same terminal sequences to the old and new apps.", NewCurated(dict.Curated(), dict.American), "Rune is a TUI editor, so I fed the same terminal sequences to the old and new apps.")
}
func TestRegularIrregulars_FixGatekepped(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Foundational principles between languages are the same, I don’t understand why positions are gatekeeped based on implementation trivia.", NewCurated(dict.Curated(), dict.American), "Foundational principles between languages are the same, I don’t understand why positions are gatekept based on implementation trivia.")
}
