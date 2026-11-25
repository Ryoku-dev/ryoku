package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWishCould_FlagWishWeCan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "i wish we can spend more time together", NewWishCould(), "i wish we could spend more time together")
}

func TestWishCould_FlagWishICan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I wish I can finally forgive myself for all the things I am not.", NewWishCould(), "I wish I could finally forgive myself for all the things I am not.")
}

func TestWishCould_FlagWishYouCan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I wish you can find your true love.", NewWishCould(), "I wish you could find your true love.")
}

func TestWishCould_FlagWishesTheyCan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What your Therapist wishes they can tell you.", NewWishCould(), "What your Therapist wishes they could tell you.")
}

func TestWishCould_FlagWishingSomeoneCan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Forever wishing someone can point me in the right direction", NewWishCould(), "Forever wishing someone could point me in the right direction")
}

func TestWishCould_FlagWishTheyCan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I wish they can plant more trees on this road.", NewWishCould(), "I wish they could plant more trees on this road.")
}

func TestWishCould_FlagWishedHeCan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I just wished he can talk and tell me how he feels", NewWishCould(), "I just wished he could talk and tell me how he feels")
}

func TestWishCould_WishThisCan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but I wish this can be fixed by Electron team", NewWishCould(), "but I wish this could be fixed by Electron team")
}

func TestWishCould_WishItCan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Wish it can be supported.", NewWishCould(), "Wish it could be supported.")
}

func TestWishCould_WishSomebodyCan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I wish somebody can fix this issue.", NewWishCould(), "I wish somebody could fix this issue.")
}
