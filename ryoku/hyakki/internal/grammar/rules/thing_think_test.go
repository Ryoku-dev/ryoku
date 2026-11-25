package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestThingThink_FixYouThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Whad do you thing about tinygo?", NewThingThink(), "Whad do you think about tinygo?")
}

func TestThingThink_FixIThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "bcz i thing hugging face embeddings and models are very complex", NewThingThink(), "bcz i think hugging face embeddings and models are very complex")
}

func TestThingThink_FixWeThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "which information we thing to be missing", NewThingThink(), "which information we think to be missing")
}

func TestThingThink_FixTheyThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "they thing something is a good idea", NewThingThink(), "they think something is a good idea")
}

func TestThingThink_FixEveryoneThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What does everyone thing here?", NewThingThink(), "What does everyone think here?")
}

func TestThingThink_FixAnyoneThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Can anyone thing of a (reasonable) way to align them such that the 'a's in all 4 words will be in (more or less) the same vertical position?", NewThingThink(), "Can anyone think of a (reasonable) way to align them such that the 'a's in all 4 words will be in (more or less) the same vertical position?")
}

func TestThingThink_FixAnybodyThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If anybody thing there is an issue in Karma, please re-open.", NewThingThink(), "If anybody think there is an issue in Karma, please re-open.")
}

func TestThingThink_FixEveryBodyThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What does every body thing I should do with my Randy Johnson rookie card.", NewThingThink(), "What does every body think I should do with my Randy Johnson rookie card.")
}

func TestThingThink_FixHaveToThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I always have to thing what button does what action.", NewThingThink(), "I always have to think what button does what action.")
}

func TestThingThink_FixNeedToThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "No need to thing about the REGEX.", NewThingThink(), "No need to think about the REGEX.")
}

func TestThingThink_FixWantToThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "maybe you want to thing of this also as a feature enhancement.", NewThingThink(), "maybe you want to think of this also as a feature enhancement.")
}

func TestThingThink_FixHavingToThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it has saved me personally hours in combined time not having to thing about whether something is in seconds or milliseconds", NewThingThink(), "it has saved me personally hours in combined time not having to think about whether something is in seconds or milliseconds")
}

func TestThingThink_FixNeedsTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When implementing any functionality once needs to thing aboiut how it is going to be used.", NewThingThink(), "When implementing any functionality once needs to think aboiut how it is going to be used.")
}

func TestThingThink_FixNeededTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Even in that case we needed to thing about the syntax so that we wouldn't need to change existing syntax", NewThingThink(), "Even in that case we needed to think about the syntax so that we wouldn't need to change existing syntax")
}

func TestThingThink_FixHadTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I had to thing in ways of making people more interested in it", NewThingThink(), "I had to think in ways of making people more interested in it")
}

func TestThingThink_FixTryingToThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Here I'm trying to thing about the following questions:", NewThingThink(), "Here I'm trying to think about the following questions:")
}

func TestThingThink_FixCanThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The exe file dosen't work allways, because antivirus can thing it is a virus.", NewThingThink(), "The exe file dosen't work allways, because antivirus can think it is a virus.")
}

func TestThingThink_FixCouldThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "\"doesNotReturnSameInstanceWhenCalledMultipleTimes\" is a terrible name, but the only one i could thing of immediately.", NewThingThink(), "\"doesNotReturnSameInstanceWhenCalledMultipleTimes\" is a terrible name, but the only one i could think of immediately.")
}

func TestThingThink_FixMightThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Consider what a reader might thing when reading a switch", NewThingThink(), "Consider what a reader might think when reading a switch")
}

func TestThingThink_FixShouldThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And we should thing to add a flag so the user could decide if internal top level extension functions are ok or not.", NewThingThink(), "And we should think to add a flag so the user could decide if internal top level extension functions are ok or not.")
}

func TestThingThink_FixMayThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It is easier than you may thing to run both bands with hostapd.", NewThingThink(), "It is easier than you may think to run both bands with hostapd.")
}

func TestThingThink_FixCannotThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I cannot thing of a simple way to implement compensation of a change in Fnco.", NewThingThink(), "I cannot think of a simple way to implement compensation of a change in Fnco.")
}

func TestThingThink_FixWillThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So user will thing that delete operation is fine but its not this code deletes the wrong page and make one extra page which wrong.", NewThingThink(), "So user will think that delete operation is fine but its not this code deletes the wrong page and make one extra page which wrong.")
}

func TestThingThink_FixCantThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "can't thing of another place, which could have such effect", NewThingThink(), "can't think of another place, which could have such effect")
}

func TestThingThink_FixCouldntThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I couldn't thing about a better title, but I run into problems since the new dplyr release.", NewThingThink(), "I couldn't think about a better title, but I run into problems since the new dplyr release.")
}

func TestThingThink_FixShouldntThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When dealing with a multi-tenanted system, users shouldn't thing about 'Databases', they should think about Tenants.", NewThingThink(), "When dealing with a multi-tenanted system, users shouldn't think about 'Databases', they should think about Tenants.")
}

func TestThingThink_FixWontThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I think you need to use an io.Pipe so the Go HTTP Request won't thing the buf has been fulling read.", NewThingThink(), "I think you need to use an io.Pipe so the Go HTTP Request won't think the buf has been fulling read.")
}

func TestThingThink_FixAlwaysThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "one should always thing of whether the efforts are better targeted to the improvement", NewThingThink(), "one should always think of whether the efforts are better targeted to the improvement")
}

func TestThingThink_FixSometimesThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One thing that I sometimes thing would be nice is if I could make different instances", NewThingThink(), "One thing that I sometimes think would be nice is if I could make different instances")
}

func TestThingThink_FixOftenThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When working with workflows on many forms I often thing I need to do the same over and over", NewThingThink(), "When working with workflows on many forms I often think I need to do the same over and over")
}

func TestThingThink_FixNeverThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "just use UUIDv7 and never thing about those details again", NewThingThink(), "just use UUIDv7 and never think about those details again")
}

func TestThingThink_FixUsuallyThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And the order of that relationship might be reversed from what one might usually thing.", NewThingThink(), "And the order of that relationship might be reversed from what one might usually think.")
}
