package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestInStock_FixAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "don't want to redirect to product page if not enough products are on stock", NewInStock(), "don't want to redirect to product page if not enough products are in stock")
}

func TestInStock_FixAt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Python script to check if a product is back on stock at flipkart.com - shyamjos/flipkart_checker.", NewInStock(), "Python script to check if a product is back in stock at flipkart.com - shyamjos/flipkart_checker.")
}

func TestInStock_FixBack(t *testing.T) {
	linttest.AssertSuggestionResult(t, "notify customer when an article is back on stock", NewInStock(), "notify customer when an article is back in stock")
}

func TestInStock_FixBe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Could there be some complete stock goods listing regardless how many products should be on stock?", NewInStock(), "Could there be some complete stock goods listing regardless how many products should be in stock?")
}

func TestInStock_FixHas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I think it would be best if the BOM and CPL files could be updated with a component that JLCPBC has on stock.", NewInStock(), "I think it would be best if the BOM and CPL files could be updated with a component that JLCPBC has in stock.")
}

func TestInStock_FixHasBut(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Can't Notify customer if product has inventory on stock but not on particular source", NewInStock(), "Can't Notify customer if product has inventory in stock but not on particular source")
}

func TestInStock_FixHave(t *testing.T) {
	linttest.AssertSuggestionResult(t, "classic product that we don't have on stock or we may still have few", NewInStock(), "classic product that we don't have in stock or we may still have few")
}

func TestInStock_FixHaveThem(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but we also may have them on stock for x reasons", NewInStock(), "but we also may have them in stock for x reasons")
}

func TestInStock_FixIs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "why not immidiatly show the 2000 as that is what is on stock.", NewInStock(), "why not immidiatly show the 2000 as that is what is in stock.")
}

func TestInStock_DontFlagIsOnStockSamsungRoms(t *testing.T) {
	linttest.AssertNoLints(t, "So i am pretty sure issue is on stock samsung roms", NewInStock())
}

func TestInStock_DontFlagOnStockSambaLens(t *testing.T) {
	linttest.AssertNoLints(t, "Samba lens is missing on Stock Lenses page", NewInStock())
}

func TestInStock_DontFlagOnStockOs(t *testing.T) {
	linttest.AssertNoLints(t, "I'm actually not sure if that prop is really doing anything even on stock OS.", NewInStock())
}

func TestInStock_DontFlagOnStockPixel(t *testing.T) {
	linttest.AssertNoLints(t, "Storage Manager link is invisible on stock Pixel OS too.", NewInStock())
}
