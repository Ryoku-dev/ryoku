package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestFootInchMinuteSecondSymbols_WithoutSpacesAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "i'm 5'7\", 140 pounds and the small was a perfect fit for me.", NewFootInchMinuteSecondSymbols(), "i'm 5′7″, 140 pounds and the small was a perfect fit for me.")
}

func TestFootInchMinuteSecondSymbols_WithSpacesAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "i'm a tall gal, 5' 11\" with broad shoulders and i weigh about 165", NewFootInchMinuteSecondSymbols(), "i'm a tall gal, 5′ 11″ with broad shoulders and i weigh about 165")
}

func TestFootInchMinuteSecondSymbols_WithSpacesBefore(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm about 6 ' 1 \" tall.", NewFootInchMinuteSecondSymbols(), "I'm about 6 ′ 1 ″ tall.")
}

func TestFootInchMinuteSecondSymbols_WithOpenSmartFeet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "LeBron James at approximately 6‘7\"", NewFootInchMinuteSecondSymbols(), "LeBron James at approximately 6′7″")
}

func TestFootInchMinuteSecondSymbols_WithCloseSmartFeet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Michael Jordan: 6’6\"", NewFootInchMinuteSecondSymbols(), "Michael Jordan: 6′6″")
}

func TestFootInchMinuteSecondSymbols_WithOpenSmartInches(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Shaquille O'Neal: 7' 1“", NewFootInchMinuteSecondSymbols(), "Shaquille O'Neal: 7′ 1″")
}

func TestFootInchMinuteSecondSymbols_WithCloseSmartInches(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Magic Johnson: 6' 9”", NewFootInchMinuteSecondSymbols(), "Magic Johnson: 6′ 9″")
}

func TestFootInchMinuteSecondSymbols_DegreesAndMinutesNoSpacesAsciiApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Eiffel Tower: 48°51'N, 2°17'E", NewFootInchMinuteSecondSymbols(), "Eiffel Tower: 48°51′N, 2°17′E")
}

func TestFootInchMinuteSecondSymbols_DegreesAndMinutesWithSpacesSmartApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Statue of Liberty: 40° 41’ N, 74° 02’ W", NewFootInchMinuteSecondSymbols(), "Statue of Liberty: 40° 41′ N, 74° 02′ W")
}

func TestFootInchMinuteSecondSymbols_FixPrimeFeetAsciiQuote(t *testing.T) {
	linttest.AssertSuggestionResult(t, "6′7\"", NewFootInchMinuteSecondSymbols(), "6′7″")
}

func TestFootInchMinuteSecondSymbols_FixAsciiFeetPrimeSeconds(t *testing.T) {
	linttest.AssertSuggestionResult(t, "6'7″", NewFootInchMinuteSecondSymbols(), "6′7″")
}

func TestFootInchMinuteSecondSymbols_FixPrimeMinuteAsciiQuote(t *testing.T) {
	linttest.AssertSuggestionResult(t, "30′15\"", NewFootInchMinuteSecondSymbols(), "30′15″")
}

func TestFootInchMinuteSecondSymbols_FixAsciiMinutePrimeSecond(t *testing.T) {
	linttest.AssertSuggestionResult(t, "30'15″", NewFootInchMinuteSecondSymbols(), "30′15″")
}

func TestFootInchMinuteSecondSymbols_FixDegreesApostropheQuote(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Sydney Opera House: 33°51'25\"S, 151° 12' 55\" East", NewFootInchMinuteSecondSymbols(), "Sydney Opera House: 33°51′25″S, 151° 12′ 55″ East")
}

func TestFootInchMinuteSecondSymbols_FixDegreesPrimeQuote(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Taj Mahal: 27°10′30\"N & 78° 02’ 32” E", NewFootInchMinuteSecondSymbols(), "The Taj Mahal: 27°10′30″N & 78° 02′ 32″ E")
}

func TestFootInchMinuteSecondSymbols_FixDegreesApostrophePrime(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Colosseum: 41°53'25″North; 12°29'32″East", NewFootInchMinuteSecondSymbols(), "Colosseum: 41°53′25″North; 12°29′32″East")
}

func TestFootInchMinuteSecondSymbols_DontFlagDegreesPrimeDoublePrime(t *testing.T) {
	linttest.AssertNoLints(t, "There are pyramids at 29°58′45″ north and 31°08′03″ east", NewFootInchMinuteSecondSymbols())
}
