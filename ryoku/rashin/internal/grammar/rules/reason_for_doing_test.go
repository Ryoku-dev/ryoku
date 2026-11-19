package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestReasonForDoing_FixSgOfDoing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "However i could not find any valid reason of doing this in one project, it's still possible.", NewReasonForDoing(), "However i could not find any valid reason for doing this in one project, it's still possible.")
}

func TestReasonForDoing_FixPlOfDoing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It actually helped me a lot understanding what is your recommended way of implementing safe-listing and your reasons of doing it this way.", NewReasonForDoing(), "It actually helped me a lot understanding what is your recommended way of implementing safe-listing and your reasons for doing it this way.")
}

func TestReasonForDoing_FixSgOfHaving(t *testing.T) {
	linttest.AssertSuggestionResult(t, "what's the reason of having USE_INTERPOLATION_TABLES in UserParams then?", NewReasonForDoing(), "what's the reason for having USE_INTERPOLATION_TABLES in UserParams then?")
}

func TestReasonForDoing_FixPlOfHaving(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Any reasons of having other implementation than specified in docs?", NewReasonForDoing(), "Any reasons for having other implementation than specified in docs?")
}

func TestReasonForDoing_IgnoreForReasonsOfLogging(t *testing.T) {
	linttest.AssertNoLints(t, "whether for reasons of logging, monitoring, etc.", NewReasonForDoing())
}

func TestReasonForDoing_FixSgOfMaking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The fact is that I am seeing where is it being used and I cannot understand the reason of making it boolean.", NewReasonForDoing(), "The fact is that I am seeing where is it being used and I cannot understand the reason for making it boolean.")
}

func TestReasonForDoing_FixPlOfMaking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So for the reasons of making it self describable as much as possible, I think it is important to express version there.", NewReasonForDoing(), "So for the reasons for making it self describable as much as possible, I think it is important to express version there.")
}

func TestReasonForDoing_AllowForReasonsOfMaking(t *testing.T) {
	linttest.AssertNoLints(t, "That said, and just for reasons of making the simple-openai code more robust, one could verify that it is not null and throw the appropriate exceptions", NewReasonForDoing())
}

func TestReasonForDoing_FixForReasonsOfDoingThis(t *testing.T) {
	t.Skip("Upstream ignored: Known false negative where we should't allow 'reasons of doing' just because it follows 'for'")
	linttest.AssertSuggestionResult(t, "For reasons of doing this, it is because I have some ops that need to carefully sort their execution orders in not only forward pass", NewReasonForDoing(), "For reasons for doing this, it is because I have some ops that need to carefully sort their execution orders in not only forward pass")
}

func TestReasonForDoing_FixSgOfNeeding(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I just came to the same conclusion for the same reason of needing a scoped dependency in my DbContext to support global query filtering.", NewReasonForDoing(), "I just came to the same conclusion for the same reason for needing a scoped dependency in my DbContext to support global query filtering.")
}

func TestReasonForDoing_FixSgOfOpening(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hi, What is reason of opening DB exception \"UnknownError: Internal error opening backing store for indexedDB\"?", NewReasonForDoing(), "Hi, What is reason for opening DB exception \"UnknownError: Internal error opening backing store for indexedDB\"?")
}

func TestReasonForDoing_FixSgOfSaving(t *testing.T) {
	linttest.AssertSuggestionResult(t, "be notified about document save events with source/reason of saving", NewReasonForDoing(), "be notified about document save events with source/reason for saving")
}

func TestReasonForDoing_FixSgOfWanting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How do you use Severity and what is the impact/reason of wanting to having different values to you (i.e. what difference does it make to you)?", NewReasonForDoing(), "How do you use Severity and what is the impact/reason for wanting to having different values to you (i.e. what difference does it make to you)?")
}

func TestReasonForDoing_AllowForReasonsOfWanting(t *testing.T) {
	t.Skip("Upstream ignored: We don't yet handle words between 'for' and 'reasons of'")
	linttest.AssertNoLints(t, "Ideally we don't want to reduce the idleTimeout (currently set to 120s) for the standard reasons of wanting to be able to handle bursty traffic.", NewReasonForDoing())
}
