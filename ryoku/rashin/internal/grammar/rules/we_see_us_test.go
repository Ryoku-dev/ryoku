package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWeSeeUs_See(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In the example below we see us using a schema to validate a configuration and setup defaults.", NewWeSeeUs(), "In the example below we see each other using a schema to validate a configuration and setup defaults.")
}

func TestWeSeeUs_Saw(t *testing.T) {
	linttest.AssertSuggestionResult(t, "From time to time we saw us in front of meaningless stack traces.", NewWeSeeUs(), "From time to time we saw each other in front of meaningless stack traces.")
}

func TestWeSeeUs_CanSee(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We can see us chucking the nodes or in this case, we most likely have to give it back to them", NewWeSeeUs(), "We can see each other chucking the nodes or in this case, we most likely have to give it back to them")
}

func TestWeSeeUs_CantSee(t *testing.T) {
	linttest.AssertSuggestionResult(t, "On the list endpoints, we can't see us ever leaving the older (current) endpoints behind", NewWeSeeUs(), "On the list endpoints, we can't see each other ever leaving the older (current) endpoints behind")
}

func TestWeSeeUs_ShouldSee(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If we can see shadows on the ground, we should see us making shadows.", NewWeSeeUs(), "If we can see shadows on the ground, we should see each other making shadows.")
}

func TestWeSeeUs_WillSee(t *testing.T) {
	linttest.AssertSuggestionResult(t, "we will see us through others and we will connect our experiences with testimonies of people", NewWeSeeUs(), "we will see each other through others and we will connect our experiences with testimonies of people")
}

func TestWeSeeUs_HaveSeen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We have seen us almost every Day for the last 5 years.", NewWeSeeUs(), "We have seen each other almost every Day for the last 5 years.")
}

func TestWeSeeUs_HadSeen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We are very lucky about the fact that we had seen us in 2009, while we were spending our vacation with you.", NewWeSeeUs(), "We are very lucky about the fact that we had seen each other in 2009, while we were spending our vacation with you.")
}

func TestWeSeeUs_AreSeeing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We are seeing us through their eyes.", NewWeSeeUs(), "We are seeing each other through their eyes.")
}

func TestWeSeeUs_DontFlagSeeUsCaps(t *testing.T) {
	linttest.AssertNoLints(t, "From the array, above we see US customers are listed as \"USA\"", NewWeSeeUs())
}

func TestWeSeeUs_DontFlagCanSeeUsCaps(t *testing.T) {
	linttest.AssertNoLints(t, "We can see US is the largest contributor of netflix", NewWeSeeUs())
}
