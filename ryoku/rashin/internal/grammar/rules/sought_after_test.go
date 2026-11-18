package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSoughtAfter_FixAbitSortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Blue mountain buffalo no damage abit sort after,$120 ono.", NewSoughtAfter(), "Blue mountain buffalo no damage abit sought after,$120 ono.")
}

func TestSoughtAfter_DontFlagAlwaysSortAfter(t *testing.T) {
	linttest.AssertLintCount(t, "Always sort after converting your set into list objects", NewSoughtAfter(), 0)
}

func TestSoughtAfter_FixAreSortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ux engineers are sort after, but it requires experience.", NewSoughtAfter(), "Ux engineers are sought after, but it requires experience.")
}

func TestSoughtAfter_FixAreSortAfterHyphenated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "optimistic people are sort-after for their life", NewSoughtAfter(), "optimistic people are sought-after for their life")
}

func TestSoughtAfter_FixBitSortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's the early enduro model getting a bit sort after now", NewSoughtAfter(), "It's the early enduro model getting a bit sought after now")
}

func TestSoughtAfter_FixExtremelySortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "3 extremely sort after Pokémon trading cards.", NewSoughtAfter(), "3 extremely sought after Pokémon trading cards.")
}

func TestSoughtAfter_FixFairlySortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The ability for editors to add tables to pages is a fairly sort after piece of functionality", NewSoughtAfter(), "The ability for editors to add tables to pages is a fairly sought after piece of functionality")
}

func TestSoughtAfter_FixHighlySortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Wrestlemania 2K adds three highly sort after features", NewSoughtAfter(), "Wrestlemania 2K adds three highly sought after features")
}

func TestSoughtAfter_FixHugelySortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Currently the hugely sort after and most highly prized variety is the electric neon blue Paraiba Tourmaline.", NewSoughtAfter(), "Currently the hugely sought after and most highly prized variety is the electric neon blue Paraiba Tourmaline.")
}

func TestSoughtAfter_FixIncrediblySortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is no surprise as it's been an incredibly sort after and top choice amongst outdoorists from all walks of life.", NewSoughtAfter(), "This is no surprise as it's been an incredibly sought after and top choice amongst outdoorists from all walks of life.")
}

func TestSoughtAfter_FixIsSortAfter(t *testing.T) {
	t.Skip("'Is' is a bit more subtle to handle correctly")
	linttest.AssertSuggestionResult(t, "White bait is sort after by many fisherman because they  are a delicacy.", NewSoughtAfter(), "White bait is sought after by many fisherman because they  are a delicacy.")
}

func TestSoughtAfter_DontFlagIsSortAfter(t *testing.T) {
	linttest.AssertLintCount(t, "What I would do is sort after the union or join.", NewSoughtAfter(), 0)
}

func TestSoughtAfter_FixKindaSortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If so is the US bond still kinda sort after as it's tied to USD (that's how I took OPs post).", NewSoughtAfter(), "If so is the US bond still kinda sought after as it's tied to USD (that's how I took OPs post).")
}

func TestSoughtAfter_DontFlagMaybeSortAfter(t *testing.T) {
	linttest.AssertLintCount(t, "Or maybe sort after adding the index?", NewSoughtAfter(), 0)
}

func TestSoughtAfter_FixMostSortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This has got to be one of the most sort after solutions.", NewSoughtAfter(), "This has got to be one of the most sought after solutions.")
}

func TestSoughtAfter_FixMostlySortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What color and size is mostly sort after In ladies footwear", NewSoughtAfter(), "What color and size is mostly sought after In ladies footwear")
}

func TestSoughtAfter_FixMuchSortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Sending audio files is a much sort after feature in chat apps.", NewSoughtAfter(), "Sending audio files is a much sought after feature in chat apps.")
}

func TestSoughtAfter_DontFlagNotSortAfter(t *testing.T) {
	linttest.AssertLintCount(t, "My issue is that it does not sort after the startyear", NewSoughtAfter(), 0)
}

func TestSoughtAfter_FixOccasionallySortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This part is occasionally sort after and if they were easily available I reckon you'd sell a few easily enough.", NewSoughtAfter(), "This part is occasionally sought after and if they were easily available I reckon you'd sell a few easily enough.")
}

func TestSoughtAfter_FixOfSortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A couple of sort after casserole pots .", NewSoughtAfter(), "A couple of sought after casserole pots .")
}

func TestSoughtAfter_FixOftenSortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "North American countries (ok, there are only two) often sort after the total amount of medals.", NewSoughtAfter(), "North American countries (ok, there are only two) often sought after the total amount of medals.")
}

func TestSoughtAfter_FixPerhapsSortAfter(t *testing.T) {
	t.Skip("'Perhaps' is a bit more subtle to handle correctly")
	linttest.AssertSuggestionResult(t, "Perhaps sort after guitar teachers could do a similar teaching tour to a few major cities", NewSoughtAfter(), "Perhaps sought after guitar teachers could do a similar teaching tour to a few major cities")
}

func TestSoughtAfter_DontFlatPerhapsSortAfter(t *testing.T) {
	linttest.AssertLintCount(t, "min_Vround: perhaps sort after min_breadth.", NewSoughtAfter(), 0)
}

func TestSoughtAfter_FlagPrettySortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But just like jin and V, he is also pretty sort after", NewSoughtAfter(), "But just like jin and V, he is also pretty sought after")
}

func TestSoughtAfter_FixQuietSortAfterSic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "MBA in Christ (deemed university) is quiet sort after in South India", NewSoughtAfter(), "MBA in Christ (deemed university) is quiet sought after in South India")
}

func TestSoughtAfter_FixQuiteSortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The university that i studied my MBBS from offers the course as well and it is quite sort after for the above said course.", NewSoughtAfter(), "The university that i studied my MBBS from offers the course as well and it is quite sought after for the above said course.")
}

func TestSoughtAfter_FixRatherSortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In a bid to satisfy an innate inquisitive hunger for a rather sort after phenomenon that only a few could precisely speak", NewSoughtAfter(), "In a bid to satisfy an innate inquisitive hunger for a rather sought after phenomenon that only a few could precisely speak")
}

func TestSoughtAfter_FixReallySortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Creators - especially women in their 30s, 40s, 50s and 60s are really sort after.", NewSoughtAfter(), "Creators - especially women in their 30s, 40s, 50s and 60s are really sought after.")
}

func TestSoughtAfter_FixSometimesSortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the N15 1.6L gearbox is sometimes sort after for the micra", NewSoughtAfter(), "the N15 1.6L gearbox is sometimes sought after for the micra")
}

func TestSoughtAfter_FixSomewhatSortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I know tri res boots used to be somewhat sort after, but not sure now!", NewSoughtAfter(), "I know tri res boots used to be somewhat sought after, but not sure now!")
}

func TestSoughtAfter_FixStronglySortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This eventually leads to the growth that is so strongly sort after.", NewSoughtAfter(), "This eventually leads to the growth that is so strongly sought after.")
}

func TestSoughtAfter_FixVastlySortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hardie stuff no longer vastly sort after as it was years ago , hasn't been for decades.", NewSoughtAfter(), "Hardie stuff no longer vastly sought after as it was years ago , hasn't been for decades.")
}

func TestSoughtAfter_FixVerySortAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I could imagine, this functionality very sort after.", NewSoughtAfter(), "I could imagine, this functionality very sought after.")
}
