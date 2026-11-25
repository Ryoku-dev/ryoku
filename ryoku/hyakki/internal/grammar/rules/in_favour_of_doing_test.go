package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestInFavourOfDoing_FixFavourDoing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I don't think anyone is in favour doing nothing, but I wouldn't know which of the first two is best.", NewInFavourOfDoing(), "I don't think anyone is in favour of doing nothing, but I wouldn't know which of the first two is best.")
}

func TestInFavourOfDoing_FixFavorGetting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've definitely been in favor getting Observable as a language primitive since the beginning.", NewInFavourOfDoing(), "I've definitely been in favor of getting Observable as a language primitive since the beginning.")
}

func TestInFavourOfDoing_FixFavorHaving(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm in favor having all-in-one @std/std as convenient entrypoint for all modules.", NewInFavourOfDoing(), "I'm in favor of having all-in-one @std/std as convenient entrypoint for all modules.")
}

func TestInFavourOfDoing_FixFavorUsing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm closing this in favor using the official guide.", NewInFavourOfDoing(), "I'm closing this in favor of using the official guide.")
}

func TestInFavourOfDoing_FixFavourHaving(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Would you be in favour having a feature to suppress local configuration files via an env var e.g. DISABLE_LOCAL_CONFIG or similar?", NewInFavourOfDoing(), "Would you be in favour of having a feature to suppress local configuration files via an env var e.g. DISABLE_LOCAL_CONFIG or similar?")
}

func TestInFavourOfDoing_FixFavourPutting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've abandoned just optimising the cpu renderers (which could then just be compiled on linux as well) in favour putting the work onto gpu.", NewInFavourOfDoing(), "I've abandoned just optimising the cpu renderers (which could then just be compiled on linux as well) in favour of putting the work onto gpu.")
}

func TestInFavourOfDoing_FixFavourUsing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is also the reason I am personally not in favour using AI for this task because of the latter.", NewInFavourOfDoing(), "This is also the reason I am personally not in favour of using AI for this task because of the latter.")
}
