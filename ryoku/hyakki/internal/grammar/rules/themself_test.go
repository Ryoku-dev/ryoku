package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestThemself_FixRespectTheirSelf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You can't respect someone who doesn't respect their self.", NewThemself(), "You can't respect someone who doesn't respect themself.")
}

func TestThemself_DontFlagSelfHyphen(t *testing.T) {
	linttest.AssertNoLints(t, "For most renaissance artists, their self-portraits are far from photorealistic.", NewThemself())
}

func TestThemself_FixTheirSelves(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Constructedness: see \"people as forming and reforming their selves within each relationship\".", NewThemself(), "Constructedness: see \"people as forming and reforming themselves within each relationship\".")
}

func TestThemself_DontFlagSelfHosted(t *testing.T) {
	linttest.AssertNoLints(t, "ZeroTier is certainly another viable alternative, but their self hosted option still relies on the ZeroTier root servers", NewThemself())
}

func TestThemself_DontFlagSelfInterest(t *testing.T) {
	linttest.AssertNoLints(t, "this state that everyone is acting in their self interest is not something we should promote", NewThemself())
}

func TestThemself_DontFlagSelfProclaimed(t *testing.T) {
	linttest.AssertNoLints(t, "Which is why they feel threatened by anything that challenges their self proclaimed uniquely human trait.", NewThemself())
}
