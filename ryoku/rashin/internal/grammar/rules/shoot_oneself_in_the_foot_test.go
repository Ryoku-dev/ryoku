package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestShootOneselfInTheFoot_IgnoreCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "Don't shoot yourself in the foot.", NewShootOneselfInTheFoot(), 0)
}

func TestShootOneselfInTheFoot_IgnoreTitleCase(t *testing.T) {
	linttest.AssertLintCount(t, "Don't Shoot Yourself In The Foot.", NewShootOneselfInTheFoot(), 0)
}

func TestShootOneselfInTheFoot_IgnoreAllCaps(t *testing.T) {
	linttest.AssertLintCount(t, "DON'T SHOOT YOURSELF IN THE FOOT.", NewShootOneselfInTheFoot(), 0)
}

func TestShootOneselfInTheFoot_FixShootLeg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I managed to shoot myself in the leg when using CF Workers deployment", NewShootOneselfInTheFoot(), "I managed to shoot myself in the foot when using CF Workers deployment")
}

func TestShootOneselfInTheFoot_FixShootIntoFoot(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Or should we keep them to prevent users from shooting themselves into the foot?", NewShootOneselfInTheFoot(), "Or should we keep them to prevent users from shooting themselves in the foot?")
}

func TestShootOneselfInTheFoot_FixShootIntoFeet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "(to prevent you from shooting yourself into the feet)", NewShootOneselfInTheFoot(), "(to prevent you from shooting yourself in the foot)")
}

func TestShootOneselfInTheFoot_IgnoreThemselvesFoot(t *testing.T) {
	linttest.AssertLintCount(t, "Thou shalt not make a rule that prevents C++ programmers from shooting themselves in the foot.", NewShootOneselfInTheFoot(), 0)
}

func TestShootOneselfInTheFoot_IgnoreOurselvesFeet(t *testing.T) {
	linttest.AssertLintCount(t, "It will help avoiding shooting ourselves in the feet.", NewShootOneselfInTheFoot(), 0)
}

func TestShootOneselfInTheFoot_FixAFoot(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Shot ourselves in a foot, \"Wrong X-Request-Key\" error #589.", NewShootOneselfInTheFoot(), "Shot ourselves in the foot, \"Wrong X-Request-Key\" error #589.")
}

func TestShootOneselfInTheFoot_IgnoreShootsHimself(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the administrator shoots himself in the foot and then hops around", NewShootOneselfInTheFoot(), "the administrator shoots himself in the foot and then hops around")
}

func TestShootOneselfInTheFoot_IgnoreShootingOneselfInTheFoot(t *testing.T) {
	linttest.AssertLintCount(t, "A historical document of shooting oneself in the foot, if you will.", NewShootOneselfInTheFoot(), 0)
}

func TestShootOneselfInTheFoot_FixOneselfInAFoot(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Forgetting to declare some variable local withing a function definition is a common way to shoot oneself in a foot", NewShootOneselfInTheFoot(), "Forgetting to declare some variable local withing a function definition is a common way to shoot oneself in the foot")
}

func TestShootOneselfInTheFoot_FixOneselfInTheFeet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Forgetting to declare some variable local withing a function definition is a common way to shoot oneself in the feet", NewShootOneselfInTheFoot(), "Forgetting to declare some variable local withing a function definition is a common way to shoot oneself in the foot")
}

func TestShootOneselfInTheFoot_FixOneselfIntoTheLeg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Forgetting to declare some variable local withing a function definition is a common way to shoot oneself into the foot", NewShootOneselfInTheFoot(), "Forgetting to declare some variable local withing a function definition is a common way to shoot oneself in the foot")
}

func TestShootOneselfInTheFoot_IgnoreOneselfInTheToes(t *testing.T) {
	linttest.AssertLintCount(t, "Forgetting to declare some variable local withing a function definition is a common way to shoot oneself in the toes", NewShootOneselfInTheFoot(), 0)
}
