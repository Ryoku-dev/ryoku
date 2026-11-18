package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestFishNorFowl_FixBird(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But there's one bus in Australia that actually tries to be that little bit train and is neither fish nor bird doing so.", NewFishNorFowl(), "But there's one bus in Australia that actually tries to be that little bit train and is neither fish nor fowl doing so.")
}

func TestFishNorFowl_FixFoul(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But I do doubt chromebooks will be big hit, they're neither fish nor foul, since tablets stole netbooks' market.", NewFishNorFowl(), "But I do doubt chromebooks will be big hit, they're neither fish nor fowl, since tablets stole netbooks' market.")
}
