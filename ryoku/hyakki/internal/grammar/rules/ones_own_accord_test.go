package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOnesOwnAccord_Her(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Monika going to bed on her own accord?", NewOnesOwnAccord(), "Monika going to bed of her own accord?")
}

func TestOnesOwnAccord_His(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The shot went down without a thought and the man soon left on his own accord.", NewOnesOwnAccord(), "The shot went down without a thought and the man soon left of his own accord.")
}

func TestOnesOwnAccord_Its(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It appears typescript is on its own accord looking at every dependency in my package.json", NewOnesOwnAccord(), "It appears typescript is of its own accord looking at every dependency in my package.json")
}

func TestOnesOwnAccord_ItsApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "will return the memory to the operating system on it's own accord to prevent linked freelists", NewOnesOwnAccord(), "will return the memory to the operating system of it's own accord to prevent linked freelists")
}

func TestOnesOwnAccord_My(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I would love to be able to re-order the icons on my own accord.", NewOnesOwnAccord(), "I would love to be able to re-order the icons of my own accord.")
}

func TestOnesOwnAccord_Our(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We did not include polyfill-php80 on our own accord, Google Ads package did.", NewOnesOwnAccord(), "We did not include polyfill-php80 of our own accord, Google Ads package did.")
}

func TestOnesOwnAccord_Their(t *testing.T) {
	linttest.AssertSuggestionResult(t, "they can also post messages to rooms on their own accord", NewOnesOwnAccord(), "they can also post messages to rooms of their own accord")
}

func TestOnesOwnAccord_TheirPlural(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Both frontend and backend can be launched on their own accords from the relevant Dockerfile's in each directory.", NewOnesOwnAccord(), "Both frontend and backend can be launched of their own accords from the relevant Dockerfile's in each directory.")
}

func TestOnesOwnAccord_ThereSpello(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Let them fail on there own accord because that's where they're heading.", NewOnesOwnAccord(), "Let them fail of there own accord because that's where they're heading.")
}

func TestOnesOwnAccord_Your(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If you want to do this you are free to on your own accord.", NewOnesOwnAccord(), "If you want to do this you are free to of your own accord.")
}

func TestOnesOwnAccord_YourPlural(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Name things on your own accords.", NewOnesOwnAccord(), "Name things of your own accords.")
}
