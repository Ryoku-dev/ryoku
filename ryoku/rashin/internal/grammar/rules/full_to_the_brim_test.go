package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestFullToTheBrim_FlagIncorrectIdiomOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "my mug is full to the brim of coffee", NewFullToTheBrim(), "my mug is full to the brim with coffee")
}

func TestFullToTheBrim_DontFlagCorrectIdiomWith(t *testing.T) {
	linttest.AssertNoLints(t, "my brain is full to the brim with good ideas", NewFullToTheBrim())
}

func TestFullToTheBrim_DontFlagCorrectIdiomOfThe(t *testing.T) {
	linttest.AssertNoLints(t, "full to the brim of the cup", NewFullToTheBrim())
}

func TestFullToTheBrim_FixVitaminB(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I feel full to the brim of Vitamin B", NewFullToTheBrim(), "I feel full to the brim with Vitamin B")
}

func TestFullToTheBrim_FixBloodAndGuts(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But at the same time it is full to the brim of blood and guts", NewFullToTheBrim(), "But at the same time it is full to the brim with blood and guts")
}

func TestFullToTheBrim_FixBadBackSpirits(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Once the axe was full to the brim of bad back spirits it would be banged on the ...", NewFullToTheBrim(), "Once the axe was full to the brim with bad back spirits it would be banged on the ...")
}

func TestFullToTheBrim_FixGlassFullOfAlcohol(t *testing.T) {
	linttest.AssertSuggestionResult(t, "glass full to the brim of alcohol", NewFullToTheBrim(), "glass full to the brim with alcohol")
}

func TestFullToTheBrim_FixHeartFullOfSunBeams(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Heart full to the brim of sun beams. 🥰", NewFullToTheBrim(), "Heart full to the brim with sun beams. 🥰")
}

func TestFullToTheBrim_FixGlassFullOfWater(t *testing.T) {
	linttest.AssertSuggestionResult(t, "glass full to the brim of fresh crystal clear transparent water with a frozen sheen on", NewFullToTheBrim(), "glass full to the brim with fresh crystal clear transparent water with a frozen sheen on")
}

func TestFullToTheBrim_FixFullOfNewAndRadStock(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Full to the brim of new and rad stock!!! Pop past and see for yourself! #treatyoself.", NewFullToTheBrim(), "Full to the brim with new and rad stock!!! Pop past and see for yourself! #treatyoself.")
}

func TestFullToTheBrim_FixStartupFilledWithFaangRejects(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If you join a startup - it is frequently filled full to the brim of FAANG rejects, young people who don't realize what FAANG offers, and people from different ...", NewFullToTheBrim(), "If you join a startup - it is frequently filled full to the brim with FAANG rejects, young people who don't realize what FAANG offers, and people from different ...")
}

func TestFullToTheBrim_FixParsedJsonFilesFullOfUtf8(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I parsed JSON files full to the brim of UTF-8, but it wasn't a problem.", NewFullToTheBrim(), "I parsed JSON files full to the brim with UTF-8, but it wasn't a problem.")
}

func TestFullToTheBrim_FixPutInEmptyCapsAndBottleFullOfWater(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Put in a few empty caps, and a bottle full to the brim of water also for comparison.", NewFullToTheBrim(), "Put in a few empty caps, and a bottle full to the brim with water also for comparison.")
}

func TestFullToTheBrim_FixUngoliantFullOfLight(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ungoliant is full to the brim of that light—or would be, if she hadn't kept on growing as she drank, making the concept of “brim” a little hazy!", NewFullToTheBrim(), "Ungoliant is full to the brim with that light—or would be, if she hadn't kept on growing as she drank, making the concept of “brim” a little hazy!")
}

func TestFullToTheBrim_FixOpenInternetFullOfBadActors(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I don't consider this a tacit ad hominem per se, because we must acknowledge the open internet is full to the brim of bad actors and bots and we cannot ...", NewFullToTheBrim(), "I don't consider this a tacit ad hominem per se, because we must acknowledge the open internet is full to the brim with bad actors and bots and we cannot ...")
}

func TestFullToTheBrim_FixInternetFullOfPages(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Internet is full to the brim of pages explaining it better than I can.", NewFullToTheBrim(), "The Internet is full to the brim with pages explaining it better than I can.")
}

func TestFullToTheBrim_FixPouredHeartsIntoJavadocs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "we've poured our hearts into making these javadocs full to the brim of examples", NewFullToTheBrim(), "we've poured our hearts into making these javadocs full to the brim with examples")
}

func TestFullToTheBrim_FixSpreadsheetsFilledWithUnoptimizedFormulas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Imagine 100+ spreadsheets with 1 to 3 sheets on average, each filled to the brim of unoptimized formulas wrote by accountants that have absolutely ZERO ...", NewFullToTheBrim(), "Imagine 100+ spreadsheets with 1 to 3 sheets on average, each filled to the brim with unoptimized formulas wrote by accountants that have absolutely ZERO ...")
}

func TestFullToTheBrim_FixMagazinesFullOfIntelligentPeople(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Magazines, journals, and libraries are full to the brim of intelligent people writing intelligent things.", NewFullToTheBrim(), "Magazines, journals, and libraries are full to the brim with intelligent people writing intelligent things.")
}

func TestFullToTheBrim_FixContinuingToUseAmazon(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Continuing to use Amazon when you know how full to the brim of scams it is, just seems to me like rewarding them for bad behaviour.", NewFullToTheBrim(), "Continuing to use Amazon when you know how full to the brim with scams it is, just seems to me like rewarding them for bad behaviour.")
}

func TestFullToTheBrim_FixIfStatementHugeAndFullOfImports(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The if statement was huge and full to the brim of imports that did nothing, and code that just didn't look right.", NewFullToTheBrim(), "The if statement was huge and full to the brim with imports that did nothing, and code that just didn't look right.")
}

func TestFullToTheBrim_FixHeFlungHimselfOverToTheEdge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He flung himself over to the edge of the rock and plunged the goblet into the lake, bringing it up full to the brim of icy water that did not ...", NewFullToTheBrim(), "He flung himself over to the edge of the rock and plunged the goblet into the lake, bringing it up full to the brim with icy water that did not ...")
}

func TestFullToTheBrim_FixOofThisIsFullToTheBrimOfBadCPractices(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Oof, this is full to the brim of bad C practices.", NewFullToTheBrim(), "Oof, this is full to the brim with bad C practices.")
}

func TestFullToTheBrim_FixCppSyntaxIsFullToTheBrimOfUglyCorners(t *testing.T) {
	linttest.AssertSuggestionResult(t, "C++'s syntax is full to the brim of such ugly corners, the standard doesn't even bother to specify a binding BNF on the implementations!", NewFullToTheBrim(), "C++'s syntax is full to the brim with such ugly corners, the standard doesn't even bother to specify a binding BNF on the implementations!")
}

func TestFullToTheBrim_FixMysqlIsFullToTheBrimOfFootGuns(t *testing.T) {
	linttest.AssertSuggestionResult(t, "MySQL is full to the brim of foot-guns", NewFullToTheBrim(), "MySQL is full to the brim with foot-guns")
}

func TestFullToTheBrim_FixCIsSadlyFilledToTheBrimOfFootguns(t *testing.T) {
	linttest.AssertSuggestionResult(t, "C is sadly filled to the brim of footguns, and C++ inherited them all and added some more of their own.", NewFullToTheBrim(), "C is sadly filled to the brim with footguns, and C++ inherited them all and added some more of their own.")
}

func TestFullToTheBrim_FixTheFieldIsMoreFullToTheBrimOfPeoplePresentingOpinionsAsFact(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The field is more full to the brim of people presenting opinions as fact than I would ever have believed.", NewFullToTheBrim(), "The field is more full to the brim with people presenting opinions as fact than I would ever have believed.")
}

func TestFullToTheBrim_DontFlagFullToTheBrimOfTheBowl(t *testing.T) {
	linttest.AssertNoLints(t, "For a little person to pour a kymyz full to the brim of the bowl, so guests know that hosts do not skimp on kymyz", NewFullToTheBrim())
}

func TestFullToTheBrim_DontFlagFilledToTheBrimOfTheScreen(t *testing.T) {
	linttest.AssertNoLints(t, "The tv is filled to the brim of the screen with news of the world, all bad tidings for the erstwhile faint-of-heart.", NewFullToTheBrim())
}

func TestFullToTheBrim_DontFlagDishesFullToTheBrimOfTheSink(t *testing.T) {
	linttest.AssertNoLints(t, "The kitchen is never clean, an open bin, dishes full to the brim of the sink", NewFullToTheBrim())
}

func TestFullToTheBrim_FixFullToTheBrimOfOur(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I decided to make a ribbon world full to the brim of our true enemy.", NewFullToTheBrim(), "I decided to make a ribbon world full to the brim with our true enemy.")
}

func TestFullToTheBrim_FixFullToTheBrimOfThis(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When you're full to the brim of this goodness, you can sop up that extra sauce with rice, cooked crispy on the same skillet.", NewFullToTheBrim(), "When you're full to the brim with this goodness, you can sop up that extra sauce with rice, cooked crispy on the same skillet.")
}

func TestFullToTheBrim_FixFilledToTheBrimOfThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This sub is already filled to the brim of that topic.", NewFullToTheBrim(), "This sub is already filled to the brim with that topic.")
}

func TestFullToTheBrim_FixFilledToTheBrimOfThese(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'll never forget the time an internet friend of mine showed my a 50 quart bin filled to the brim of these lil buggers.", NewFullToTheBrim(), "I'll never forget the time an internet friend of mine showed my a 50 quart bin filled to the brim with these lil buggers.")
}

func TestFullToTheBrim_FixFullToTheBrimOfThose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Romance fantasy is full to the brim of those.", NewFullToTheBrim(), "Romance fantasy is full to the brim with those.")
}

func TestFullToTheBrim_FixFilledToTheBrimOfHisSoul(t *testing.T) {
	t.Skip("Known false negative")
	linttest.AssertSuggestionResult(t, "It is ingrained into his very being to be the kind of kid who is a leader, a solver of problems, and filled to the brim of his soul with love", NewFullToTheBrim(), "It is ingrained into his very being to be the kind of kid who is a leader, a solver of problems, and full to the brim of his soul with love")
}
