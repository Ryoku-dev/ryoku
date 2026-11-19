package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestLessWorse_CorrectLeastWorse(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Maybe downstream packaging folks could advice what would be least worse option.", NewLessWorse(), []string{
		"Maybe downstream packaging folks could advice what would be least worst option.",
		"Maybe downstream packaging folks could advice what would be least bad option.",
		"Maybe downstream packaging folks could advice what would be less bad option.",
	}, []string{})
}

func TestLessWorse_CorrectLeastWorstHyphen(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "async-dropper is probably the least-worst ad-hoc AsyncDrop implementation you've seen.", NewLessWorse(), []string{
		"async-dropper is probably the least worst ad-hoc AsyncDrop implementation you've seen.",
		"async-dropper is probably the least bad ad-hoc AsyncDrop implementation you've seen.",
	}, []string{})
}

func TestLessWorse_CorrectLessWorse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Professionally I've convinced the team at @Roave to pay me for making their PHP code marginally less worse.", NewLessWorse(), "Professionally I've convinced the team at @Roave to pay me for making their PHP code marginally less bad.")
}

func TestLessWorse_CorrectLessWorst(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "May be the less worst choice for some little playlists.", NewLessWorse(), []string{
		"May be the less bad choice for some little playlists.",
		"May be the least bad choice for some little playlists.",
	}, []string{})
}
