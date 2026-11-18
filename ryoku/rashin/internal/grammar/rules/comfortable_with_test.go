package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestComfortableWith_FixAmComfortableOfDoing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It is not something that I am comfortable of doing at the moment until more work is done on the MediaWiki parser migration to Parsoid.", NewComfortableWith(), "It is not something that I am comfortable with doing at the moment until more work is done on the MediaWiki parser migration to Parsoid.")
}

func TestComfortableWith_FixImComfortableOfDeclaring(t *testing.T) {
	linttest.AssertSuggestionResult(t, "identify any bugs and pain points before I'm comfortable of declaring this ready for production use", NewComfortableWith(), "identify any bugs and pain points before I'm comfortable with declaring this ready for production use")
}

func TestComfortableWith_FixReallyComfortableOfConverting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So if you are very comfortable in being in that messy middle and really comfortable of converting chaos into clarity for you and those around you", NewComfortableWith(), "So if you are very comfortable in being in that messy middle and really comfortable with converting chaos into clarity for you and those around you")
}

func TestComfortableWith_FixNotComfortableOfCompiling(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Most people are not comfortable of compiling their own binary, or are technically capable of doing it.", NewComfortableWith(), "Most people are not comfortable with compiling their own binary, or are technically capable of doing it.")
}

func TestComfortableWith_DontFlagComfortableOfCourse(t *testing.T) {
	linttest.AssertNoLints(t, "Folks are more then welcome to discuss Carbon wherever they feel most comfortable of course, but I think we can't really expand the official chat spaces", NewComfortableWith())
}

func TestComfortableWith_DontFlagComfortableCommaOfCourse(t *testing.T) {
	linttest.AssertNoLints(t, "Folks are more then welcome to discuss Carbon wherever they feel most comfortable of course, but I think we can't really expand the official chat spaces", NewComfortableWith())
}

func TestComfortableWith_FixAreComfortableOfThe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "While we are comfortable of the core design, there is still plenty of activity being done and refinements made", NewComfortableWith(), "While we are comfortable with the core design, there is still plenty of activity being done and refinements made")
}

func TestComfortableWith_DontFlagMostComfortableOfTheThree(t *testing.T) {
	linttest.AssertNoLints(t, "It is the most comfortable of the three to sit in front of for a long stretch.", NewComfortableWith())
}

func TestComfortableWith_DontFlagAreComfortableOfThe3(t *testing.T) {
	linttest.AssertNoLints(t, "at 0.8 convergence there is the least amount of red hues and usually that means this is the most comfortable of the 3", NewComfortableWith())
}

func TestComfortableWith_DontFlagTheMostComfortableOfAll(t *testing.T) {
	linttest.AssertNoLints(t, "The seven league boots were, Mr Giant admits, the most comfortable of all the magic boots he's worn.", NewComfortableWith())
}

func TestComfortableWith_DontFlagTheLeastComfortableOfThose(t *testing.T) {
	linttest.AssertNoLints(t, "I have big hands, so 1-3-5 feels the least comfortable of those, although it may be the 'automatic' choice.", NewComfortableWith())
}

func TestComfortableWith_DontFlagTheMostComfortableOfThose(t *testing.T) {
	linttest.AssertNoLints(t, "On the other hand, TiDB is the most comfortable of those", NewComfortableWith())
}
