package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOffLimits_FixNoOffLimitContacts(t *testing.T) {
	linttest.AssertSuggestionResult(t, "no task duration bonus (no successful insertion), no force penalty, and no off-limit contacts", NewOffLimits(), "no task duration bonus (no successful insertion), no force penalty, and no off-limits contacts")
}

func TestOffLimits_FixAnyOffLimitValues(t *testing.T) {
	linttest.AssertSuggestionResult(t, "create html-pages, and optionally warn contacts about any off-limit values", NewOffLimits(), "create html-pages, and optionally warn contacts about any off-limits values")
}

func TestOffLimits_FixInTwoPlaces(t *testing.T) {
	linttest.AssertSuggestionResult(t, "\"off-limit airspace\": no-fly zones. a target is off-limit if it belongs to one of these classes.", NewOffLimits(), "\"off-limits airspace\": no-fly zones. a target is off-limits if it belongs to one of these classes.")
}

func TestOffLimits_FixIsOffLimitSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "one can have this issue if one of the joint position is off limit", NewOffLimits(), "one can have this issue if one of the joint position is off limits")
}

func TestOffLimits_DontFlagBackOff(t *testing.T) {
	linttest.AssertNoLints(t, "In fact, setting it to Never is the only way right now to have a back off limit be applied and effective.", NewOffLimits())
}

func TestOffLimits_DontFlagTryBackingOff(t *testing.T) {
	linttest.AssertNoLints(t, "try backing off limit screw another quarter turn", NewOffLimits())
}

func TestOffLimits_DontFlagBacksOff(t *testing.T) {
	linttest.AssertNoLints(t, "Backs off limit when system is stressed, increases when healthy.", NewOffLimits())
}

func TestOffLimits_DontFlagTurnOff(t *testing.T) {
	linttest.AssertNoLints(t, "Turn off LIMIT and OFFSET?", NewOffLimits())
}

func TestOffLimits_DontFlagTurnedOff(t *testing.T) {
	linttest.AssertNoLints(t, "I have turned off limit child processes in developer settings.", NewOffLimits())
}

func TestOffLimits_DontFlagTurningOff(t *testing.T) {
	linttest.AssertNoLints(t, "should address turning off limit scaling for pods, which would address your immediate problem.", NewOffLimits())
}

func TestOffLimits_DontFlagTurnsOffLimit(t *testing.T) {
	linttest.AssertNoLints(t, "LIMIT=-1 turns off limit checking.", NewOffLimits())
}

func TestOffLimits_DontFlagSwitchOff(t *testing.T) {
	linttest.AssertNoLints(t, "Switch off limit exceeded Info(REAL)", NewOffLimits())
}

func TestOffLimits_DontFlagSwitchedOff(t *testing.T) {
	linttest.AssertNoLints(t, "Switched off Limit IP Address Tracking.", NewOffLimits())
}

func TestOffLimits_DontFlagSwitchingOff(t *testing.T) {
	linttest.AssertNoLints(t, "You can do that by switching off Limit IP Address tracking on that specific WiFi network.", NewOffLimits())
}
