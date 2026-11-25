package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSpellCheck_AmericaCapitalized(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The word america should be capitalized.", NewSpellCheck(dict.Curated(), dict.American), "The word America should be capitalized.")
}

func TestSpellCheck_HarperAutomatticCapitalized(t *testing.T) {
	linttest.AssertLintCount(t, "So should harper and automattic.", NewSpellCheck(dict.Curated(), dict.American), 2)
}

func TestSpellCheck_AmericanColorInBritishDialect(t *testing.T) {
	linttest.AssertLintCount(t, "Do you like the color?", NewSpellCheck(dict.Curated(), dict.British), 1)
}

func TestSpellCheck_LowercaseUkSuggestsAllCaps(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "We flew to the uk.", NewSpellCheck(dict.Curated(), dict.British), []string{"We flew to the UK."}, []string{})
}

func TestSpellCheck_AmericanSpellingInNameInBritishDialect(t *testing.T) {
	linttest.AssertLintCount(t, "The attack on Pearl Harbor came before Labor Day.", NewSpellCheck(dict.Curated(), dict.British), 0)
}

func TestSpellCheck_TitleCaseNzSuggestsAllCaps(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "We flew to Nz.", NewSpellCheck(dict.Curated(), dict.British), []string{"We flew to NZ."}, []string{})
}

func TestSpellCheck_BritishSpellingInNameInAmericanDialect(t *testing.T) {
	linttest.AssertLintCount(t, "The Ministry of Defence met the Labour Party.", NewSpellCheck(dict.Curated(), dict.American), 0)
}

func TestSpellCheck_AmericanSpellingOutsideNameInBritishDialect(t *testing.T) {
	linttest.AssertLintCount(t, "The ship left the harbor near Pearl Harbor.", NewSpellCheck(dict.Curated(), dict.British), 1)
}

func TestSpellCheck_BritishIseWordsInBritishDialect(t *testing.T) {
	linttest.AssertLintCount(t, "We anonymise it with the customisable tokeniser, then we synthesised the results.", NewSpellCheck(dict.Curated(), dict.British), 0)
}

func TestSpellCheck_BritishIseWordsInAmericanDialect(t *testing.T) {
	linttest.AssertLintCount(t, "We anonymise it with the customisable tokeniser, then we synthesised the results.", NewSpellCheck(dict.Curated(), dict.American), 4)
}

func TestSpellCheck_CanadianWordsInAustralianDialect(t *testing.T) {
	linttest.AssertLintCount(t, "Does your mom like yogourt?", NewSpellCheck(dict.Curated(), dict.Australian), 2)
}

func TestSpellCheck_AustralianWordsInCanadianDialect(t *testing.T) {
	linttest.AssertLintCount(t, "We mine bauxite to make aluminium.", NewSpellCheck(dict.Curated(), dict.Canadian), 1)
}

func TestSpellCheck_MumAndMummyNotJustCommonwealth(t *testing.T) {
	linttest.AssertLintCount(t, "Mum's the word about that Egyptian mummy.", NewSpellCheck(dict.Curated(), dict.American), 0)
}

func TestSpellCheck_AustralianVerandah(t *testing.T) {
	linttest.AssertLintCount(t, "Our house has a verandah.", NewSpellCheck(dict.Curated(), dict.Australian), 0)
}

func TestSpellCheck_AustralianVerandahInAmericanDialect(t *testing.T) {
	linttest.AssertLintCount(t, "Our house has a verandah.", NewSpellCheck(dict.Curated(), dict.American), 1)
}

func TestSpellCheck_AustralianVerandahInBritishDialect(t *testing.T) {
	linttest.AssertLintCount(t, "Our house has a verandah.", NewSpellCheck(dict.Curated(), dict.British), 1)
}

func TestSpellCheck_AustralianVerandahInCanadianDialect(t *testing.T) {
	linttest.AssertLintCount(t, "Our house has a verandah.", NewSpellCheck(dict.Curated(), dict.Canadian), 1)
}

func TestSpellCheck_MixingAustralianAndCanadianDialects(t *testing.T) {
	linttest.AssertLintCount(t, "In summer we sit on the verandah and eat yogourt.", NewSpellCheck(dict.Curated(), dict.Australian), 1)
}

func TestSpellCheck_MixingCanadianAndAustralianDialects(t *testing.T) {
	linttest.AssertLintCount(t, "In summer we sit on the verandah and eat yogourt.", NewSpellCheck(dict.Curated(), dict.Canadian), 1)
}

func TestSpellCheck_AustralianAndCanadianSpellingsThatAreNotAmerican(t *testing.T) {
	linttest.AssertLintCount(t, "In summer we sit on the verandah and eat yogourt.", NewSpellCheck(dict.Curated(), dict.American), 2)
}

func TestSpellCheck_AustralianAndCanadianSpellingsThatAreNotBritish(t *testing.T) {
	linttest.AssertLintCount(t, "In summer we sit on the verandah and eat yogourt.", NewSpellCheck(dict.Curated(), dict.British), 2)
}

func TestSpellCheck_AustralianLabourVsLabor(t *testing.T) {
	linttest.AssertLintCount(t, "In Australia we write 'labour' but the political party is the 'Labor Party'.", NewSpellCheck(dict.Curated(), dict.Australian), 0)
}

func TestSpellCheck_AustralianWordsFlaggedForAmericanEnglish(t *testing.T) {
	linttest.AssertLintCount(t, "There's an esky full of beers in the back of the ute.", NewSpellCheck(dict.Curated(), dict.American), 2)
}

func TestSpellCheck_AmericanWordsNotFlaggedForAustralianEnglish(t *testing.T) {
	linttest.AssertLintCount(t, "In general, utes have unibody construction while pickups have frames.", NewSpellCheck(dict.Curated(), dict.Australian), 0)
}

func TestSpellCheck_AbandonwareCorrection(t *testing.T) {
	linttest.AssertSuggestionResult(t, "abanonedware", NewSpellCheck(dict.Curated(), dict.Australian), "abandonware")
}

func TestSpellCheck_CorrectsAbandonedware11311166(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Abandonedware is abandoned. Do not bother submitting issues about the empty page bug. Author moved to greener pastures", NewSpellCheck(dict.Curated(), dict.American), "Abandonware is abandoned. Do not bother submitting issues about the empty page bug. Author moved to greener pastures")
}

func TestSpellCheck_AfterwardsNotUs(t *testing.T) {
	linttest.AssertLintCount(t, "afterwards", NewSpellCheck(dict.Curated(), dict.American), 1)
}

func TestSpellCheck_AfterwardIsUs(t *testing.T) {
	linttest.AssertLintCount(t, "afterward", NewSpellCheck(dict.Curated(), dict.American), 0)
}

func TestSpellCheck_AfterwardNotAu(t *testing.T) {
	linttest.AssertLintCount(t, "afterward", NewSpellCheck(dict.Curated(), dict.Australian), 1)
}

func TestSpellCheck_AfterwardsIsAu(t *testing.T) {
	linttest.AssertLintCount(t, "afterwards", NewSpellCheck(dict.Curated(), dict.Australian), 0)
}

func TestSpellCheck_AfterwardNotCa(t *testing.T) {
	linttest.AssertLintCount(t, "afterward", NewSpellCheck(dict.Curated(), dict.Canadian), 1)
}

func TestSpellCheck_AfterwardsIsCa(t *testing.T) {
	linttest.AssertLintCount(t, "afterwards", NewSpellCheck(dict.Curated(), dict.Canadian), 0)
}

func TestSpellCheck_AfterwardNotUk(t *testing.T) {
	linttest.AssertLintCount(t, "afterward", NewSpellCheck(dict.Curated(), dict.British), 1)
}

func TestSpellCheck_AfterwardsIsUk(t *testing.T) {
	linttest.AssertLintCount(t, "afterwards", NewSpellCheck(dict.Curated(), dict.British), 0)
}

func TestSpellCheck_CorrectsHes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "hes", NewSpellCheck(dict.Curated(), dict.British), "he's")
}

func TestSpellCheck_CorrectsShes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "shes", NewSpellCheck(dict.Curated(), dict.British), "she's")
}

func TestSpellCheck_Issue1876(t *testing.T) {
	user_dialect := dict.American
	user_dict := dict.NewMutableDictionary()
	user_dict.AppendWordStr("Calibre", dict.DictWordMetadata{Dialects: dict.DialectFlagsFromDialect(user_dialect)})
	merged_dict := dict.NewMergedDictionary()
	merged_dict.AddDictionary(dict.Curated())
	merged_dict.AddDictionary(user_dict)
	if !merged_dict.ContainsWordStr("Calibre") {
		t.Fatal("merged dictionary does not contain Calibre")
	}
	if got := len(NewSpellCheck(merged_dict, user_dialect).Lint(doc.NewPlainEnglish("I like to use the software Calibre.", merged_dict))); got != 0 {
		t.Fatalf("Calibre is not part of the user's dialect! got %d lints", got)
	}
	if got := len(NewSpellCheck(merged_dict, user_dialect).Lint(doc.NewPlainEnglish("I like to use the spelling colour.", merged_dict))); got != 1 {
		t.Fatalf("got %d lints, want 1", got)
	}
}

func TestSpellCheck_MattIsAllowed(t *testing.T) {
	for _, dialect := range []dict.Dialect{dict.American, dict.Canadian, dict.Australian, dict.British, dict.Indian} {
		linttest.AssertNoLints(t, "Matt is a great name.", NewSpellCheck(dict.Curated(), dialect))
	}
}

func TestSpellCheck_Issue2026(t *testing.T) {
	linttest.AssertSuggestionResult(t, "'Tere' is supposed to be 'There'", NewSpellCheck(dict.Curated(), dict.British), "'There' is supposed to be 'There'")
	linttest.AssertSuggestionResult(t, "'fll' is supposed to be 'fill'", NewSpellCheck(dict.Curated(), dict.British), "'fill' is supposed to be 'fill'")
}

func TestSpellCheck_Issue2261(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Generaly", NewSpellCheck(dict.Curated(), dict.British), "Generally")
}

func TestSpellCheck_FlagPreponeInNonIndianEnglish(t *testing.T) {
	linttest.AssertLintCount(t, "We had to prepone the meeting", NewSpellCheck(dict.Curated(), dict.American), 1)
}

func TestSpellCheck_DontFlagPreponeInIndianEnglish(t *testing.T) {
	linttest.AssertNoLints(t, "We had to prepone the meeting", NewSpellCheck(dict.Curated(), dict.Indian))
}

func TestSpellCheck_DontFlagCertainEntriesWithMultipleCaseVariantsInDict(t *testing.T) {
	linttest.AssertNoLints(t, "PR", NewSpellCheck(dict.Curated(), dict.American))
	linttest.AssertNoLints(t, "MB", NewSpellCheck(dict.Curated(), dict.American))
	linttest.AssertNoLints(t, "OS", NewSpellCheck(dict.Curated(), dict.American))
}

func TestSpellCheck_AllowsMultipleParentheticalPluralSMarkers(t *testing.T) {
	linttest.AssertNoLints(t, "Review the person(s) and document(s).", NewSpellCheck(dict.Curated(), dict.American))
}

func TestSpellCheck_AllowsUppercaseParentheticalPluralS(t *testing.T) {
	linttest.AssertNoLints(t, "CONTACT THE PERSON(S).", NewSpellCheck(dict.Curated(), dict.American))
}

func TestSpellCheck_StillFlagsMisspelledWordBeforeParentheticalPluralS(t *testing.T) {
	linttest.AssertLintCount(t, "Please ask each persson(s) to sign.", NewSpellCheck(dict.Curated(), dict.American), 1)
}

func TestSpellCheck_StillFlagsParentheticalSWithoutPrecedingWord(t *testing.T) {
	linttest.AssertLintCount(t, "Please mark (s) on the form.", NewSpellCheck(dict.Curated(), dict.American), 1)
}

func TestSpellCheck_StillFlagsSpacedParentheticalS(t *testing.T) {
	linttest.AssertLintCount(t, "Please ask each person (s) to sign.", NewSpellCheck(dict.Curated(), dict.American), 1)
}

func TestSpellCheck_StillFlagsLongerParentheticalMarker(t *testing.T) {
	linttest.AssertLintCount(t, "Please ask each person(ss) to sign.", NewSpellCheck(dict.Curated(), dict.American), 1)
}

func TestSpellCheck_SuggestColorForColourLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "colour", NewSpellCheck(dict.Curated(), dict.American), "color")
}

func TestSpellCheck_SuggestColourForColorLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "color", NewSpellCheck(dict.Curated(), dict.British), "colour")
}

func TestSpellCheck_SuggestColorForColourTitlecase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Colour", NewSpellCheck(dict.Curated(), dict.American), "Color")
}

func TestSpellCheck_SuggestColourForColorTitlecase(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "Color", NewSpellCheck(dict.Curated(), dict.British), "Colour")
}

func TestSpellCheck_SuggestColorForColourAllCaps(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "COLOUR", NewSpellCheck(dict.Curated(), dict.American), "COLOR")
}

func TestSpellCheck_SuggestColourForColorAllCaps(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "COLOR", NewSpellCheck(dict.Curated(), dict.British), "COLOUR")
}

func TestSpellCheck_SuggestRealiseForRealize(t *testing.T) {
	t.Skip("both spellings are acceptable in UK, AU, and IN despite popular opinion")
	linttest.AssertSuggestionResult(t, "realize", NewSpellCheck(dict.Curated(), dict.British), "realise")
}

func TestSpellCheck_SuggestRealizeForRealise(t *testing.T) {
	linttest.AssertSuggestionResult(t, "realise", NewSpellCheck(dict.Curated(), dict.American), "realize")
}

func TestSpellCheck_SuggestRealiseForRealizeTitlecase(t *testing.T) {
	t.Skip("both spellings are acceptable in UK, AU, and IN despite popular opinion")
	linttest.AssertSuggestionResult(t, "Realize", NewSpellCheck(dict.Curated(), dict.British), "Realise")
}

func TestSpellCheck_SuggestRealizeForRealiseTitlecase(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "Realise", NewSpellCheck(dict.Curated(), dict.American), "Realize")
}

func TestSpellCheck_SuggestRealiseForRealizeAllCaps(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "REALIZE", NewSpellCheck(dict.Curated(), dict.British), "REALISE")
}

func TestSpellCheck_SuggestRealizeForRealiseAllCaps(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "REALISE", NewSpellCheck(dict.Curated(), dict.American), "REALIZE")
}

func TestSpellCheck_SuggestDefenceForDefense(t *testing.T) {
	linttest.AssertSuggestionResult(t, "defense", NewSpellCheck(dict.Curated(), dict.British), "defence")
}

func TestSpellCheck_SuggestDefenseForDefence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "defence", NewSpellCheck(dict.Curated(), dict.American), "defense")
}

func TestSpellCheck_SuggestDefenseForDefenceTitlecase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Defense", NewSpellCheck(dict.Curated(), dict.British), "Defence")
}

func TestSpellCheck_SuggestDefenceForDefenseTitlecase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Defence", NewSpellCheck(dict.Curated(), dict.American), "Defense")
}

func TestSpellCheck_SuggestDefenseForDefenceAllCaps(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "DEFENSE", NewSpellCheck(dict.Curated(), dict.British), "DEFENCE")
}

func TestSpellCheck_SuggestDefenceForDefenseAllCaps(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "DEFENCE", NewSpellCheck(dict.Curated(), dict.American), "DEFENSE")
}

func TestSpellCheck_SuggestScepticForSkeptic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "skeptic", NewSpellCheck(dict.Curated(), dict.British), "sceptic")
}

func TestSpellCheck_SuggestSkepticForSceptic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "sceptic", NewSpellCheck(dict.Curated(), dict.American), "skeptic")
}

func TestSpellCheck_SuggestScepticForSkepticTitlecase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Skeptic", NewSpellCheck(dict.Curated(), dict.British), "Sceptic")
}

func TestSpellCheck_SuggestSkepticForScepticTitlecase(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "Sceptic", NewSpellCheck(dict.Curated(), dict.American), "Skeptic")
}

func TestSpellCheck_SuggestSkepticForScepticAllCaps(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "SKEPTIC", NewSpellCheck(dict.Curated(), dict.British), "SCEPTIC")
}

func TestSpellCheck_SuggestScepticForSkepticAllCaps(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "SCEPTIC", NewSpellCheck(dict.Curated(), dict.American), "SKEPTIC")
}

func TestSpellCheck_SuggestCentimeterForCentimetre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "centimetre", NewSpellCheck(dict.Curated(), dict.American), "centimeter")
}

func TestSpellCheck_SuggestCentimetreForCentimeter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "centimeter", NewSpellCheck(dict.Curated(), dict.British), "centimetre")
}

func TestSpellCheck_SuggestCentimeterForCentimetreTitlecase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Centimetre", NewSpellCheck(dict.Curated(), dict.American), "Centimeter")
}

func TestSpellCheck_SuggestCentimetreForCentimeterTitlecase(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "Centimeter", NewSpellCheck(dict.Curated(), dict.British), "Centimetre")
}

func TestSpellCheck_SuggestCentimeterForCentimetreAllCaps(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "CENTIMETRE", NewSpellCheck(dict.Curated(), dict.American), "CENTIMETER")
}

func TestSpellCheck_SuggestCentimetreForCentimeterAllCaps(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "CENTIMETER", NewSpellCheck(dict.Curated(), dict.British), "CENTIMETRE")
}

func TestSpellCheck_SuggestTravelerForTraveller(t *testing.T) {
	linttest.AssertSuggestionResult(t, "traveller", NewSpellCheck(dict.Curated(), dict.American), "traveler")
}

func TestSpellCheck_SuggestTravellerForTraveler(t *testing.T) {
	linttest.AssertSuggestionResult(t, "traveler", NewSpellCheck(dict.Curated(), dict.British), "traveller")
}

func TestSpellCheck_SuggestTravelerForTravellerTitlecase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Traveller", NewSpellCheck(dict.Curated(), dict.American), "Traveler")
}

func TestSpellCheck_SuggestTravellerForTravelerTitlecase(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "Traveler", NewSpellCheck(dict.Curated(), dict.British), "Traveller")
}

func TestSpellCheck_SuggestTravelerForTravellerAllCaps(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "TRAVELLER", NewSpellCheck(dict.Curated(), dict.American), "TRAVELER")
}

func TestSpellCheck_SuggestTravellerForTravelerAllCaps(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "TRAVELER", NewSpellCheck(dict.Curated(), dict.British), "TRAVELLER")
}

func TestSpellCheck_SuggestGreyForGrayInNonAmerican(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've got a gray cat.", NewSpellCheck(dict.Curated(), dict.British), "I've got a grey cat.")
}

func TestSpellCheck_SuggestGrayForGreyInAmerican(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's a greyscale photo.", NewSpellCheck(dict.Curated(), dict.American), "It's a grayscale photo.")
}

func TestSpellCheck_SuggestGreyForGrayInNonAmericanTitlecase(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "I've Got a Gray Cat.", NewSpellCheck(dict.Curated(), dict.British), "I've Got a Grey Cat.")
}

func TestSpellCheck_SuggestGrayForGreyInAmericanTitlecase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's a Greyscale Photo.", NewSpellCheck(dict.Curated(), dict.American), "It's a Grayscale Photo.")
}

func TestSpellCheck_SuggestGreyForGrayInNonAmericanAllCaps(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "GRAY", NewSpellCheck(dict.Curated(), dict.British), "GREY")
}

func TestSpellCheck_SuggestGrayForGreyInAmericanAllCaps(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "GREY", NewSpellCheck(dict.Curated(), dict.American), "GRAY")
}

func TestSpellCheck_FixCheifAndRecieved(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The cheif recieved a letter.", NewSpellCheck(dict.Curated(), dict.British), "The chief received a letter.")
}

func TestSpellCheck_FixCheifAndRecievedTitlecase(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "The Cheif Recieved a Letter.", NewSpellCheck(dict.Curated(), dict.British), "The Chief Received a Letter.")
}

func TestSpellCheck_FixCheifAndRecievedAllCaps(t *testing.T) {
	t.Skip("known failure due to bug")
	linttest.AssertSuggestionResult(t, "THE CHEIF RECIEVED A LETTER.", NewSpellCheck(dict.Curated(), dict.British), "THE CHEIF RECEIVED A LETTER.")
}

func TestSpellCheck_FixVsApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "v's", NewSpellCheck(dict.Curated(), dict.British), "vs")
}

func TestSpellCheck_FixVsTypographicalApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "v’s", NewSpellCheck(dict.Curated(), dict.British), "vs")
}

func TestSpellCheck_FixChildrensMissingApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "childrens", NewSpellCheck(dict.Curated(), dict.British), "children's")
}

func TestSpellCheck_AllowsInformalLaughter(t *testing.T) {
	for _, source := range []string{"hahah", "hahaha", "hahahah", "Hahahah", "HAHAHA"} {
		linttest.AssertNoLints(t, source, NewSpellCheck(dict.Curated(), dict.American))
	}
}

func TestSpellCheck_AllowsSurveilledAndSurveilling(t *testing.T) {
	linttest.AssertNoLints(t, "The suspect was closely surveilled by the agents.", NewSpellCheck(dict.Curated(), dict.American))
	linttest.AssertNoLints(t, "They were surveilling the premises.", NewSpellCheck(dict.Curated(), dict.American))
	linttest.AssertNoLints(t, "The suspect was closely surveilled by the agents.", NewSpellCheck(dict.Curated(), dict.British))
	linttest.AssertNoLints(t, "They were surveilling the premises.", NewSpellCheck(dict.Curated(), dict.British))
}
