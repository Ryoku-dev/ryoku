package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestExpandFavourite_FixLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's my fav", NewExpandFavourite(dict.American), "It's my favorite")
}

func TestExpandFavourite_FixAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "IT'S MY FAV", NewExpandFavourite(dict.American), "IT'S MY FAVORITE")
}

func TestExpandFavourite_FixTitleCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's My Fav", NewExpandFavourite(dict.American), "It's My Favorite")
}

func TestExpandFavourite_DontFlagPascalCase(t *testing.T) {
	linttest.AssertNoLints(t, "It's a project called FaVe", NewExpandFavourite(dict.American))
}

func TestExpandFavourite_FixFavsLowerUs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Each single favorite group - no matter if it contains 1 or 1000 favs - in here is displayed as 11MB", NewExpandFavourite(dict.American), "Each single favorite group - no matter if it contains 1 or 1000 favorites - in here is displayed as 11MB")
}

func TestExpandFavourite_FixFavsTitleAu(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Lost all my Favs again on reinstalling mac - got angry - then used claude.ai to help make a tool", NewExpandFavourite(dict.Australian), "Lost all my Favourites again on reinstalling mac - got angry - then used claude.ai to help make a tool")
}

func TestExpandFavourite_DontFlagFaveMixedCaseUk(t *testing.T) {
	linttest.AssertNoLints(t, "FaVe employs end-to-end encryption and leverages the inherent security features of FairOS and Swarm to ensure data integrity and confidentiality.", NewExpandFavourite(dict.British))
}

func TestExpandFavourite_FixFaveAndFavesCa(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Once faves are created, a user's fave posts can be queried", NewExpandFavourite(dict.Canadian), "Once favourites are created, a user's favourite posts can be queried")
}

func TestExpandFavourite_FixFavTitleIn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Save your Data ,by not Streaming your Fav Songs Online again & again(Just Download Them!)", NewExpandFavourite(dict.Indian), "Save your Data ,by not Streaming your Favourite Songs Online again & again(Just Download Them!)")
}
