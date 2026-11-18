package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOverPlus_TestSkeleton(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "We looked through over 200 plus photos", NewOverPlus(), []string{
		"We looked through over 200 photos",
		"We looked through 200 plus photos",
	}, []string{})
}

func TestOverPlus_Test100Million(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "$ 19 Billion dollars so far, with over 100 million plus views!", NewOverPlus(), []string{
		"$ 19 Billion dollars so far, with over 100 million views!",
		"$ 19 Billion dollars so far, with 100 million plus views!",
	}, []string{})
}

func TestOverPlus_Test5000Plus(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Over 5000 plus participants of the Global Conference", NewOverPlus(), []string{
		"Over 5000 participants of the Global Conference",
	}, []string{})
}

func TestOverPlus_Test18(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "over 18 plus only censored", NewOverPlus(), []string{"over 18 only censored", "18 plus only censored"}, []string{})
}

func TestOverPlus_Test130(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "100% Halal certified international brand with over 130 plus locations", NewOverPlus(), []string{
		"100% Halal certified international brand with over 130 locations",
		"100% Halal certified international brand with 130 plus locations",
	}, []string{})
}

func TestOverPlus_DontFlagPlusHyphen(t *testing.T) {
	linttest.AssertNoLints(t, "Currently, there are over one million Plus-linked ATMs in 170 countries worldwide", NewOverPlus())
}

func TestOverPlus_DontFlagPlusSize(t *testing.T) {
	linttest.AssertNoLints(t, "Over 50 plus size outfits", NewOverPlus())
}

func TestOverPlus_DontFlagHitlessInnings(t *testing.T) {
	linttest.AssertNoLints(t, "like the Cardinals, who went hitless over five-plus innings", NewOverPlus())
}

func TestOverPlus_DontFlagOnePlusPhone(t *testing.T) {
	linttest.AssertNoLints(t, "OnePlus 9 Series Wifi keep disconnecting over one plus 9 Every time", NewOverPlus())
}

func TestOverPlus_Over160Plus(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "reached approximately 25,00,000 Indian learners with over 160 plus unique online courses", NewOverPlus(), []string{
		"reached approximately 25,00,000 Indian learners with over 160 unique online courses",
		"reached approximately 25,00,000 Indian learners with 160 plus unique online courses",
	}, []string{})
}

func TestOverPlus_Over20PlusYears(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Marketing professional with over 20 plus years of domestic and international marketing experience", NewOverPlus(), []string{
		"Marketing professional with over 20 years of domestic and international marketing experience",
		"Marketing professional with 20 plus years of domestic and international marketing experience",
	}, []string{})
}

func TestOverPlus_DontFlagScorelessInnings(t *testing.T) {
	linttest.AssertNoLints(t, "Jacob Latz K's four over six-plus scoreless innings", NewOverPlus())
}

func TestOverPlus_DontFlagPlusAllCaps(t *testing.T) {
	linttest.AssertNoLints(t, "over 5,000 PLUS personnel will be on duty to assist highway users", NewOverPlus())
}

func TestOverPlus_FixPlusSign(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "It took me over 1000+ blocks to make \"Livin' on a Prayer\" in Stardew Valley", NewOverPlus(), []string{
		"It took me over 1000 blocks to make \"Livin' on a Prayer\" in Stardew Valley",
		"It took me 1000+ blocks to make \"Livin' on a Prayer\" in Stardew Valley",
	}, []string{})
}
