package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRegionalisms_UsToUkZipCode(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What is your zip code?", NewRegionalisms(dict.British), "What is your postcode?")
}

func TestRegionalisms_UkToUsPostcode(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What is your postcode?", NewRegionalisms(dict.American), "What is your zip code?")
}

func TestRegionalisms_UsToCaZipCode(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What is your zip code?", NewRegionalisms(dict.Canadian), "What is your postal code?")
}

func TestRegionalisms_DontFlagPostalCode(t *testing.T) {
	linttest.AssertLintCount(t, "What is your postal code?", NewRegionalisms(dict.British), 0)
}

func TestRegionalisms_DontFlagZipCodeInAmerican(t *testing.T) {
	linttest.AssertLintCount(t, "What is your zip code?", NewRegionalisms(dict.American), 0)
}

func TestRegionalisms_UkToUsFood(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I can't eat aubergine or coriander, so I'll just have a bag of crisps.", NewRegionalisms(dict.American), "I can't eat eggplant or cilantro, so I'll just have a bag of chips.")
}

func TestRegionalisms_AuToUsPhone(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I dropped my mobile phone in the esky and now it's covered in tomato sauce.", NewRegionalisms(dict.American), "I dropped my cellphone in the cooler and now it's covered in tomato sauce.")
}

func TestRegionalisms_AuToUkCars(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Drive the station wagon onto the footpath and hand me that spanner.", NewRegionalisms(dict.British), "Drive the estate onto the pavement and hand me that spanner.")
}

func TestRegionalisms_AuToUsCars(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Drive the station wagon onto the footpath and hand me that spanner.", NewRegionalisms(dict.American), "Drive the station wagon onto the sidewalk and hand me that wrench.")
}

func TestRegionalisms_UsToAuBaby(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Wash the pacifier under the faucet.", NewRegionalisms(dict.Australian), "Wash the dummy under the tap.")
}

func TestRegionalisms_UsToUkFuel(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I needed more gasoline to drive the truck to the soccer match.", NewRegionalisms(dict.British), "I needed more petrol to drive the truck to the football match.")
}

func TestRegionalisms_AuToUkLight(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Can you sell me a light globe for this torch?", NewRegionalisms(dict.British), "Can you sell me a light bulb for this torch?")
}

func TestRegionalisms_UsToAuOops(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I spilled ketchup on my clean sweater.", NewRegionalisms(dict.Australian), "I spilled tomato sauce on my clean jumper.")
}

func TestRegionalisms_CaravanDoesntAlwaysMeanTrailer(t *testing.T) {
	linttest.AssertLintCount(t, "A caravan (from Persian کاروان kârvân) is a group of people traveling together, often on a trade expedition. Caravans were used mainly in desert areas.", NewRegionalisms(dict.British), 0)
}

func TestRegionalisms_UkToUsWindscreen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Detect raindrops on vehicle windscreen by combining various region proposal algorithm with Convolutional Neural Network.", NewRegionalisms(dict.American), "Detect raindrops on vehicle windshield by combining various region proposal algorithm with Convolutional Neural Network.")
}

func TestRegionalisms_AuToUkBloodNose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Oh no! I got a blood nose.", NewRegionalisms(dict.British), "Oh no! I got a nosebleed.")
}

func TestRegionalisms_InToNonInUpdation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Add apps to queue for updation or installation and resize it.", NewRegionalisms(dict.American), "Add apps to queue for update or installation and resize it.")
}

func TestRegionalisms_DontFlagUpdateOrUpdationForIndian(t *testing.T) {
	linttest.AssertLintCount(t, "Hey, the colab notebook which you have provided, required lot of updations, Can you pls update it.", NewRegionalisms(dict.Indian), 0)
}

func TestRegionalisms_FlagCroreAndLakhForNonIndian(t *testing.T) {
	linttest.AssertLintCount(t, "There are 100 lakhs in one crore.", NewRegionalisms(dict.American), 2)
}

func TestRegionalisms_DontFlagLakhOrCroreForIndian(t *testing.T) {
	linttest.AssertLintCount(t, "There are 100 lakhs in one crore.", NewRegionalisms(dict.Indian), 0)
}

func TestRegionalisms_ABrinjalIsAnAubergine(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is brinjal used in curries or chutneys?", NewRegionalisms(dict.British), "Is aubergine used in curries or chutneys?")
}

func TestRegionalisms_ABrinjalIsAnEggplant(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is brinjal used in curries or chutneys?", NewRegionalisms(dict.Australian), "Is eggplant used in curries or chutneys?")
}

func TestRegionalisms_AmericansDontSayNought(t *testing.T) {
	linttest.AssertSuggestionResult(t, "nought", NewRegionalisms(dict.American), "zero")
}

func TestRegionalisms_GodownIsntUsedOutsideAsia(t *testing.T) {
	linttest.AssertSuggestionResult(t, "godown", NewRegionalisms(dict.American), "warehouse")
}
