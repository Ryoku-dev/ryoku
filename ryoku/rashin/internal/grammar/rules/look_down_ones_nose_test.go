package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestLookDownOnesNose_LookHisNose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He seemed to look his nose down upon everything", NewLookDownOnesNose(), "He seemed to look down his nose upon everything")
}

func TestLookDownOnesNose_LookMyNose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm the last one to look my nose down at what people do from a moral perspective.", NewLookDownOnesNose(), "I'm the last one to look down my nose at what people do from a moral perspective.")
}

func TestLookDownOnesNose_LookTheirNose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I hate how humans look their nose down on certain animals", NewLookDownOnesNose(), "I hate how humans look down their nose on certain animals")
}

func TestLookDownOnesNose_LookTheirNoses(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yet many look their noses down at the right as if it were a used Kleenex.", NewLookDownOnesNose(), "Yet many look down their noses at the right as if it were a used Kleenex.")
}

func TestLookDownOnesNose_LookYourNose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You look your nose down on me like I am only a breathing object", NewLookDownOnesNose(), "You look down your nose on me like I am only a breathing object")
}

func TestLookDownOnesNose_LookingHerNose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The long nose definitely helps give the feel of her looking her nose down on others", NewLookDownOnesNose(), "The long nose definitely helps give the feel of her looking down her nose on others")
}

func TestLookDownOnesNose_LookingHisNose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "literally looking his nose down at the commoners", NewLookDownOnesNose(), "literally looking down his nose at the commoners")
}

func TestLookDownOnesNose_LookingTheirNoses(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It makes no sense that the princesses are looking their noses down on her", NewLookDownOnesNose(), "It makes no sense that the princesses are looking down their noses on her")
}

func TestLookDownOnesNose_LookingYourNose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Whatever you do, don’t fall into the trap of looking your nose down at the customer.", NewLookDownOnesNose(), "Whatever you do, don’t fall into the trap of looking down your nose at the customer.")
}

func TestLookDownOnesNose_LooksHerNose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Daenerys looks her nose down at them.", NewLookDownOnesNose(), "Daenerys looks down her nose at them.")
}

func TestLookDownOnesNose_LooksHisNose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He probably looks his nose down on Wraith, who don't eat inbred humans.", NewLookDownOnesNose(), "He probably looks down his nose on Wraith, who don't eat inbred humans.")
}

func TestLookDownOnesNose_LooksTheirNose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Your friend looks their nose down at them and wants to unfriend them based on how little money they have.", NewLookDownOnesNose(), "Your friend looks down their nose at them and wants to unfriend them based on how little money they have.")
}
