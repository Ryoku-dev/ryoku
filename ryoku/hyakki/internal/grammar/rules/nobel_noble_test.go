package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestNobelNoble_NoblePeacePrice(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He accepted the Noble Peace Price at the ceremony.", NewNobelNoble(), "He accepted the Nobel Peace Prize at the ceremony.")
}

func TestNobelNoble_NoblePeacePriceTimeline(t *testing.T) {
	t.Skip("'replace_with_match_case' issue")
	linttest.AssertSuggestionResult(t, "Our timeline mentions the Noble peace price until 1950.", NewNobelNoble(), "Our timeline mentions the Nobel Peace Prize until 1950.")
}

func TestNobelNoble_NoblePeacePriceFacts(t *testing.T) {
	t.Skip("'replace_with_match_case' issue")
	linttest.AssertSuggestionResult(t, "noble peace price facts are easy to recap.", NewNobelNoble(), "Nobel Peace Prize facts are easy to recap.")
}

func TestNobelNoble_NoblePiecePriceCommittees(t *testing.T) {
	t.Skip("'replace_with_match_case' issue")
	linttest.AssertSuggestionResult(t, "Nobel piece price committees meet tonight.", NewNobelNoble(), "Nobel Peace Prize committees meet tonight.")
}

func TestNobelNoble_NoblePiecePrizeWinners(t *testing.T) {
	t.Skip("'replace_with_match_case' issue")
	linttest.AssertSuggestionResult(t, "nobel piece prize winners shared stories.", NewNobelNoble(), "Nobel Peace Prize winners shared stories.")
}

func TestNobelNoble_NoblePiecePriseCeremonies(t *testing.T) {
	t.Skip("'replace_with_match_case' issue")
	linttest.AssertSuggestionResult(t, "Nobel piece prise ceremonies look ready.", NewNobelNoble(), "Nobel Peace Prize ceremonies look ready.")
}

func TestNobelNoble_NoblePiecePriceStandsOut(t *testing.T) {
	t.Skip("'replace_with_match_case' issue")
	linttest.AssertSuggestionResult(t, "NOBLE PIECE PRICE stands out in the article.", NewNobelNoble(), "Nobel Peace Prize stands out in the article.")
}

func TestNobelNoble_NoblePiecePriceMemo(t *testing.T) {
	t.Skip("'replace_with_match_case' issue")
	linttest.AssertSuggestionResult(t, "She referenced the noble piece price memo.", NewNobelNoble(), "She referenced the Nobel Peace Prize memo.")
}

func TestNobelNoble_NoblePeacePriseReferences(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Noble Peace Prise references appear frequently.", NewNobelNoble(), "Nobel Peace Prize references appear frequently.")
}

func TestNobelNoble_NoblePeacePriseNews(t *testing.T) {
	t.Skip("'replace_with_match_case' issue")
	linttest.AssertSuggestionResult(t, "nobel peace prise news alerted the team.", NewNobelNoble(), "Nobel Peace Prize news alerted the team.")
}

func TestNobelNoble_NoblePiecePriceReports(t *testing.T) {
	t.Skip("'replace_with_match_case' issue")
	linttest.AssertSuggestionResult(t, "nobel piece price reports track the conference.", NewNobelNoble(), "Nobel Peace Prize reports track the conference.")
}

func TestNobelNoble_NoblePiecePriseHistorian(t *testing.T) {
	t.Skip("'replace_with_match_case' issue")
	linttest.AssertSuggestionResult(t, "The noble piece prise historian wrote a book.", NewNobelNoble(), "The Nobel Peace Prize historian wrote a book.")
}

func TestNobelNoble_NobelPeacePrizeCeremony(t *testing.T) {
	linttest.AssertNoLints(t, "The Nobel Peace Prize ceremony is broadcast worldwide.", NewNobelNoble())
}

func TestNobelNoble_NobelPrizeForPeace(t *testing.T) {
	linttest.AssertNoLints(t, "Nobel Prize for Peace winners remain inspiring.", NewNobelNoble())
}

func TestNobelNoble_PeacePrizeWinners(t *testing.T) {
	linttest.AssertNoLints(t, "Peace Prize winners celebrate after the award.", NewNobelNoble())
}

func TestNobelNoble_NoblePeacePrizesAcrossYears(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Stories mention the Noble Peace Prizes across years.", NewNobelNoble(), "Stories mention the Nobel Peace Prizes across years.")
}

func TestNobelNoble_NobleFoundation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The data used in this project is taken from The Noble Foundation which contains data about the Noble Prize from 1901 to 2016.", NewNobelNoble(), "The data used in this project is taken from The Nobel Foundation which contains data about the Nobel Prize from 1901 to 2016.")
}
