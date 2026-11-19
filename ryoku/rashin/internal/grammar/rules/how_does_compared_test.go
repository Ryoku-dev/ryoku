package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestHowDoesCompared_FixDoesItCompared(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Oo how does it compared to matrix chatGPT bot", NewHowDoesCompared(), "Oo how does it compare to matrix chatGPT bot")
}

func TestHowDoesCompared_FixDoesItCompares(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How does it compares to openspec or other spec driven frameworks?", NewHowDoesCompared(), "How does it compare to openspec or other spec driven frameworks?")
}

func TestHowDoesCompared_FixDoesThatCompared(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Also I wonder if your touchscreen works with VoodooRMI, or how does that compared to VoodooI2CHID?", NewHowDoesCompared(), "Also I wonder if your touchscreen works with VoodooRMI, or how does that compare to VoodooI2CHID?")
}

func TestHowDoesCompared_FixDoesThatCompares(t *testing.T) {
	linttest.AssertSuggestionResult(t, "lets take a look at where we are as per the latest SBP payment system report and how does that compares with the historical trend for these figures", NewHowDoesCompared(), "lets take a look at where we are as per the latest SBP payment system report and how does that compare with the historical trend for these figures")
}

func TestHowDoesCompared_FixDoesThisCompared(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How does this compared to other softwares like 3d-agent , threedle or SAM 3d from meta?", NewHowDoesCompared(), "How does this compare to other softwares like 3d-agent , threedle or SAM 3d from meta?")
}

func TestHowDoesCompared_FixDoesThisCompares(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How does this compares to the RetinaFace?", NewHowDoesCompared(), "How does this compare to the RetinaFace?")
}
