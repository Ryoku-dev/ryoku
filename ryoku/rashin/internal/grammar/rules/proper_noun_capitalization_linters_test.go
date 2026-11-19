package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestProperNounCapitalizationLinters_NameWithAmericanSpellingLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "pearl harbor", NewProperNounCapitalizationLintersLintGroup(), "Pearl Harbor")
}

func TestProperNounCapitalizationLinters_WorldHealthOrganizationLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "world health organization", NewProperNounCapitalizationLintersLintGroup(), "World Health Organization")
}

func TestProperNounCapitalizationLinters_NameWithBritishSpellingLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "ministry of defence", NewProperNounCapitalizationLintersLintGroup(), "Ministry of Defence")
}

func TestProperNounCapitalizationLinters_AmericasLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "south america", NewProperNounCapitalizationLintersLintGroup(), "South America")
	linttest.AssertSuggestionResult(t, "north america", NewProperNounCapitalizationLintersLintGroup(), "North America")
}

func TestProperNounCapitalizationLinters_AmericasUppercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "SOUTH AMERICA", NewProperNounCapitalizationLintersLintGroup(), "South America")
	linttest.AssertSuggestionResult(t, "NORTH AMERICA", NewProperNounCapitalizationLintersLintGroup(), "North America")
}

func TestProperNounCapitalizationLinters_AmericasAllowCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "South America", NewProperNounCapitalizationLintersLintGroup(), 0)
	linttest.AssertLintCount(t, "North America", NewProperNounCapitalizationLintersLintGroup(), 0)
}

func TestProperNounCapitalizationLinters_Issue798(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The United states is a big country.", NewProperNounCapitalizationLintersLintGroup(), "The United States is a big country.")
}

func TestProperNounCapitalizationLinters_UnitedNationsUppercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "UNITED NATIONS", NewProperNounCapitalizationLintersLintGroup(), "United Nations")
}

func TestProperNounCapitalizationLinters_UnitedArabEmiratesLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "UNITED ARAB EMIRATES", NewProperNounCapitalizationLintersLintGroup(), "United Arab Emirates")
}

func TestProperNounCapitalizationLinters_UnitedNationsAllowCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "United Nations", NewProperNounCapitalizationLintersLintGroup(), 0)
}

func TestProperNounCapitalizationLinters_MetaAllowCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "Meta Quest", NewProperNounCapitalizationLintersLintGroup(), 0)
}

func TestProperNounCapitalizationLinters_MicrosoftLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "microsoft visual studio", NewProperNounCapitalizationLintersLintGroup(), "Microsoft Visual Studio")
}

func TestProperNounCapitalizationLinters_MicrosoftFirstWordIsCorrect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Microsoft visual studio", NewProperNounCapitalizationLintersLintGroup(), "Microsoft Visual Studio")
}

func TestProperNounCapitalizationLinters_AppleIpodLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "apple ipod", NewProperNounCapitalizationLintersLintGroup(), "Apple iPod")
}

func TestProperNounCapitalizationLinters_AppleIpodAllowCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "Apple iPod", NewProperNounCapitalizationLintersLintGroup(), 0)
}

func TestProperNounCapitalizationLinters_AppleMacbookFamilyLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "macbook pro", NewProperNounCapitalizationLintersLintGroup(), "MacBook Pro")
	linttest.AssertSuggestionResult(t, "macbook air", NewProperNounCapitalizationLintersLintGroup(), "MacBook Air")
}

func TestProperNounCapitalizationLinters_AppleMacbookFamilyAllowCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "MacBook Pro", NewProperNounCapitalizationLintersLintGroup(), 0)
	linttest.AssertLintCount(t, "MacBook Air", NewProperNounCapitalizationLintersLintGroup(), 0)
}

func TestProperNounCapitalizationLinters_AppleMacFamilyLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "mac pro", NewProperNounCapitalizationLintersLintGroup(), "Mac Pro")
	linttest.AssertSuggestionResult(t, "mac mini", NewProperNounCapitalizationLintersLintGroup(), "Mac Mini")
}

func TestProperNounCapitalizationLinters_AppleMacFamilyAllowCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "Mac Pro", NewProperNounCapitalizationLintersLintGroup(), 0)
	linttest.AssertLintCount(t, "Mac Mini", NewProperNounCapitalizationLintersLintGroup(), 0)
}

func TestProperNounCapitalizationLinters_AppleAirpodsFamilyLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "airpods pro", NewProperNounCapitalizationLintersLintGroup(), "AirPods Pro")
	linttest.AssertSuggestionResult(t, "airpods max", NewProperNounCapitalizationLintersLintGroup(), "AirPods Max")
}

func TestProperNounCapitalizationLinters_AppleAirpodsFamilyAllowCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "AirPods Pro", NewProperNounCapitalizationLintersLintGroup(), 0)
	linttest.AssertLintCount(t, "AirPods Max", NewProperNounCapitalizationLintersLintGroup(), 0)
}

func TestProperNounCapitalizationLinters_TestAtlanticOceanLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "atlantic ocean", NewProperNounCapitalizationLintersLintGroup(), "Atlantic Ocean")
}

func TestProperNounCapitalizationLinters_TestPacificOceanLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "pacific ocean", NewProperNounCapitalizationLintersLintGroup(), "Pacific Ocean")
}

func TestProperNounCapitalizationLinters_TestIndianOceanLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "indian ocean", NewProperNounCapitalizationLintersLintGroup(), "Indian Ocean")
}

func TestProperNounCapitalizationLinters_TestSouthernOceanLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "southern ocean", NewProperNounCapitalizationLintersLintGroup(), "Southern Ocean")
}

func TestProperNounCapitalizationLinters_TestArcticOceanLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "arctic ocean", NewProperNounCapitalizationLintersLintGroup(), "Arctic Ocean")
}

func TestProperNounCapitalizationLinters_TestMediterraneanSeaLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "mediterranean sea", NewProperNounCapitalizationLintersLintGroup(), "Mediterranean Sea")
}

func TestProperNounCapitalizationLinters_TestCaribbeanSeaLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "caribbean sea", NewProperNounCapitalizationLintersLintGroup(), "Caribbean Sea")
}

func TestProperNounCapitalizationLinters_TestSouthChinaSeaLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "south china sea", NewProperNounCapitalizationLintersLintGroup(), "South China Sea")
}

func TestProperNounCapitalizationLinters_TestAtlanticOceanCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "Atlantic Ocean", NewProperNounCapitalizationLintersLintGroup(), 0)
}

func TestProperNounCapitalizationLinters_TestPacificOceanCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "Pacific Ocean", NewProperNounCapitalizationLintersLintGroup(), 0)
}

func TestProperNounCapitalizationLinters_TestIndianOceanCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "Indian Ocean", NewProperNounCapitalizationLintersLintGroup(), 0)
}

func TestProperNounCapitalizationLinters_TestMediterraneanSeaCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "Mediterranean Sea", NewProperNounCapitalizationLintersLintGroup(), 0)
}

func TestProperNounCapitalizationLinters_TestSouthChinaSeaCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "South China Sea", NewProperNounCapitalizationLintersLintGroup(), 0)
}

func TestProperNounCapitalizationLinters_DayOneInSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I love day one. It is the best journaling app.", NewProperNounCapitalizationLintersLintGroup(), "I love Day One. It is the best journaling app.")
}

func TestProperNounCapitalizationLinters_GildedAgeInSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Mani-Chess Destiny is a JavaScript based computer game built off of chess, but in the style of the gilded age.", NewProperNounCapitalizationLintersLintGroup(), "Mani-Chess Destiny is a JavaScript based computer game built off of chess, but in the style of the Gilded Age.")
}

func TestProperNounCapitalizationLinters_ChromeExtensionLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "chrome extension", NewProperNounCapitalizationLintersLintGroup(), "Chrome Extension")
}

func TestProperNounCapitalizationLinters_ChromeExtensionUppercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "CHROME EXTENSION", NewProperNounCapitalizationLintersLintGroup(), "Chrome Extension")
}

func TestProperNounCapitalizationLinters_ChromeExtensionMixedCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "cHrOmE eXtEnSiOn", NewProperNounCapitalizationLintersLintGroup(), "Chrome Extension")
}

func TestProperNounCapitalizationLinters_ChromeExtensionSecondWordLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Chrome extension", NewProperNounCapitalizationLintersLintGroup(), "Chrome Extension")
}

func TestProperNounCapitalizationLinters_ChromeExtensionFirstWordLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "chrome Extension", NewProperNounCapitalizationLintersLintGroup(), "Chrome Extension")
}

func TestProperNounCapitalizationLinters_ChromeExtensionInSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Install the chrome extension from the store.", NewProperNounCapitalizationLintersLintGroup(), "Install the Chrome Extension from the store.")
}

func TestProperNounCapitalizationLinters_ChromeExtensionWithLeadingArticle(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The chrome extension is ready.", NewProperNounCapitalizationLintersLintGroup(), "The Chrome Extension is ready.")
}

func TestProperNounCapitalizationLinters_ChromeExtensionWithTrailingPeriod(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We shipped the chrome extension.", NewProperNounCapitalizationLintersLintGroup(), "We shipped the Chrome Extension.")
}

func TestProperNounCapitalizationLinters_ChromeExtensionWithTrailingComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The chrome extension, not the app, needs review.", NewProperNounCapitalizationLintersLintGroup(), "The Chrome Extension, not the app, needs review.")
}

func TestProperNounCapitalizationLinters_ChromeExtensionWithTrailingColon(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Preferred install target: chrome extension", NewProperNounCapitalizationLintersLintGroup(), "Preferred install target: Chrome Extension")
}

func TestProperNounCapitalizationLinters_ChromeExtensionInsideQuotes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They called it the \"chrome extension\" build.", NewProperNounCapitalizationLintersLintGroup(), "They called it the \"Chrome Extension\" build.")
}

func TestProperNounCapitalizationLinters_ChromeExtensionAcrossSentenceBoundaryNotPresent(t *testing.T) {
	linttest.AssertLintCount(t, "Chrome. Extension", NewProperNounCapitalizationLintersLintGroup(), 0)
}

func TestProperNounCapitalizationLinters_ChromeExtensionAllowsCorrectCase(t *testing.T) {
	linttest.AssertLintCount(t, "Chrome Extension", NewProperNounCapitalizationLintersLintGroup(), 0)
}

func TestProperNounCapitalizationLinters_ChromeExtensionAllowsCorrectCaseInSentence(t *testing.T) {
	linttest.AssertLintCount(t, "The Chrome Extension is ready.", NewProperNounCapitalizationLintersLintGroup(), 0)
}

func TestProperNounCapitalizationLinters_BrowserExtensionNotFlagged(t *testing.T) {
	linttest.AssertLintCount(t, "browser extension", NewProperNounCapitalizationLintersLintGroup(), 0)
}

func TestProperNounCapitalizationLinters_FixBatonRogue(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The programs know you dont care about the bronx or baton rouge, you just want a spot", NewProperNounCapitalizationLintersLintGroup(), "The programs know you dont care about the bronx or Baton Rouge, you just want a spot")
}

func TestProperNounCapitalizationLinters_FixKhmerRouge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I read that the khmer rouge still existed as a group after vietnam invaded cambodia and they still had a seat in the UN until 1993.", NewProperNounCapitalizationLintersLintGroup(), "I read that the Khmer Rouge still existed as a group after vietnam invaded cambodia and they still had a seat in the UN until 1993.")
}
