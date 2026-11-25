package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMeansALotTo_FixMeanALotFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It would mean a lot for me and save me a lot of time and effort", NewMeansALotTo(), "It would mean a lot to me and save me a lot of time and effort")
}

func TestMeansALotTo_FixMeanAlotFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Appreciate it! Would mean alot for me!", NewMeansALotTo(), "Appreciate it! Would mean a lot to me!")
}

func TestMeansALotTo_FixMeanLotFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It would be very grateful to achieve number of sponsors for me and it mean lot for me.", NewMeansALotTo(), "It would be very grateful to achieve number of sponsors for me and it mean a lot to me.")
}

func TestMeansALotTo_FixMeansALotFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Your star means a lot for us to develop this project!", NewMeansALotTo(), "Your star means a lot to us to develop this project!")
}

func TestMeansALotTo_FixMeansAlotFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Even a single sponsor means alot for me.", NewMeansALotTo(), "Even a single sponsor means a lot to me.")
}

func TestMeansALotTo_FixMeansLotFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Any help means lot for me.", NewMeansALotTo(), "Any help means a lot to me.")
}

func TestMeansALotTo_FixMeantALotFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It meant a lot for me to improve this library further.", NewMeansALotTo(), "It meant a lot to me to improve this library further.")
}

func TestMeansALotTo_FixMeantAlotFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Thanks a lot by the way. this meant alot for me.", NewMeansALotTo(), "Thanks a lot by the way. this meant a lot to me.")
}
