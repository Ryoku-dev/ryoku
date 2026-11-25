package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCloudOnesJudgement_ClownedYourJudgement(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I think the dark side of the force has clowned your judgement young Padawan.", NewCloudOnesJudgement(), "I think the dark side of the force has clouded your judgement young Padawan.")
}

func TestCloudOnesJudgement_ClownHisJudgement(t *testing.T) {
	linttest.AssertSuggestionResult(t, "...because he knows that the sarcophagus will only clown his judgement and he can actually get ahead...", NewCloudOnesJudgement(), "...because he knows that the sarcophagus will only cloud his judgement and he can actually get ahead...")
}

func TestCloudOnesJudgement_ClownYourJudgement(t *testing.T) {
	linttest.AssertSuggestionResult(t, "stop letting infatuation clown your judgement.", NewCloudOnesJudgement(), "stop letting infatuation cloud your judgement.")
}

func TestCloudOnesJudgement_CloningMyJudgement(t *testing.T) {
	linttest.AssertSuggestionResult(t, "...cloning my judgement knew the genius would rise...", NewCloudOnesJudgement(), "...cloning my judgement knew the genius would rise...")
}

func TestCloudOnesJudgement_ClownYourJudgements(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You Should Never Allow Your Emotions Clown Your Judgements Over Somebody", NewCloudOnesJudgement(), "You Should Never Allow Your Emotions Cloud Your Judgements Over Somebody")
}

func TestCloudOnesJudgement_ClownOurJudgment(t *testing.T) {
	linttest.AssertSuggestionResult(t, "know-it-all attitude is what can clown our judgment when we become more experienced", NewCloudOnesJudgement(), "know-it-all attitude is what can cloud our judgment when we become more experienced")
}

func TestCloudOnesJudgement_ClownsHisJudgment(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yoda accuses him of hatred and clowns his judgment, questioning why he came", NewCloudOnesJudgement(), "Yoda accuses him of hatred and clouds his judgment, questioning why he came")
}

func TestCloudOnesJudgement_ClowningMyJudgment(t *testing.T) {
	linttest.AssertSuggestionResult(t, "trying to make a public POTD was somehow clowning my judgment", NewCloudOnesJudgement(), "trying to make a public POTD was somehow clouding my judgment")
}

func TestCloudOnesJudgement_DontFlagCoincidence(t *testing.T) {
	linttest.AssertNoLints(t, "Same old from this clown. His judgment of talent the last few years is less than sub par.", NewCloudOnesJudgement())
}
