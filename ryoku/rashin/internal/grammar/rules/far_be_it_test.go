package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestFarBeIt_FarBeItForMeCapitalized(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Far be it for me to suggestion that additional cardinality be added to the already TOO MUCH CARDINALITY metric space.", NewFarBeIt(), "Far be it from me to suggestion that additional cardinality be added to the already TOO MUCH CARDINALITY metric space.")
}

func TestFarBeIt_FarBeItForMeLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Far be it for me to tell people what to do so I'm not earnestly proposing to take away the ability to add literals to lazyframes.", NewFarBeIt(), "Far be it from me to tell people what to do so I'm not earnestly proposing to take away the ability to add literals to lazyframes.")
}

func TestFarBeIt_FarBeItThat(t *testing.T) {
	linttest.AssertSuggestionCount(t, "Far be it that I get in the middle of this thread (and the complexity WebAuthn has spawned)", NewFarBeIt(), 0)
}

func TestFarBeIt_FarBeItForTheSoftware(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Far be it for the software to give any indication of that fact.", NewFarBeIt(), "Far be it from the software to give any indication of that fact.")
}

func TestFarBeIt_MissingPunctuationFalsePositive(t *testing.T) {
	t.Skip("No punctuation between '... so far' and 'be it ...'")
	linttest.AssertNoLints(t, "but it is failing for master and all the 11.x branches i have tried so far be it 11.0.0, 11.0.1 ...", NewFarBeIt())
}

func TestFarBeIt_FarBeItTo(t *testing.T) {
	linttest.AssertSuggestionCount(t, "I'm not a marketing guy, so far be it to second guess that.", NewFarBeIt(), 0)
}
