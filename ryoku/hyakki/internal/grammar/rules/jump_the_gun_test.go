package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestJumpTheGun_DontFlagJumpTheGun(t *testing.T) {
	linttest.AssertNoLints(t, "jump the gun", NewJumpTheGun())
}

func TestJumpTheGun_FixJumpAGun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But in Race Max Pro, it's possible to get a happy finger case and jump a gun.", NewJumpTheGun(), "But in Race Max Pro, it's possible to get a happy finger case and jump the gun.")
}

func TestJumpTheGun_FixJumpedAGun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Well, I may have jumped a gun here creating this issue.", NewJumpTheGun(), "Well, I may have jumped the gun here creating this issue.")
}

func TestJumpTheGun_FixJumpedTheGuns(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Lexxy being a js library I jumped the guns and felt removing the button completely on initialization was safer.", NewJumpTheGun(), "Lexxy being a js library I jumped the gun and felt removing the button completely on initialization was safer.")
}

func TestJumpTheGun_FixJumpingTheGuns(t *testing.T) {
	linttest.AssertSuggestionResult(t, "To make sure I'm not jumping the guns, @hkuadithya how did you change the JDK version exactly?", NewJumpTheGun(), "To make sure I'm not jumping the gun, @hkuadithya how did you change the JDK version exactly?")
}
