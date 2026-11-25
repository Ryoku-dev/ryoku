package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMissingSpace_Issue2191(t *testing.T) {
	linttest.AssertSuggestionResult(t, "people that can help us.So I feel like there", NewMissingSpace(), "people that can help us. So I feel like there")
}

func TestMissingSpace_CoffeeTable(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The coffee cooled on the table.The room stayed quiet.", NewMissingSpace(), "The coffee cooled on the table. The room stayed quiet.")
}

func TestMissingSpace_OpenWindow(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A small breeze moved through the open window.The curtains lifted and fell in slow waves.", NewMissingSpace(), "A small breeze moved through the open window. The curtains lifted and fell in slow waves.")
}

func TestMissingSpace_HallwayCat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The cat watched the hallway.Its tail twitched with steady focus.", NewMissingSpace(), "The cat watched the hallway. Its tail twitched with steady focus.")
}

func TestMissingSpace_RainGlass(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Rain tapped against the glass.The sound made the afternoon feel longer.", NewMissingSpace(), "Rain tapped against the glass. The sound made the afternoon feel longer.")
}

func TestMissingSpace_CyclistHouse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A cyclist passed by the house.The wheels hummed softly on the road.", NewMissingSpace(), "A cyclist passed by the house. The wheels hummed softly on the road.")
}

func TestMissingSpace_KettleStove(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The kettle hissed on the stove.A thin ribbon of steam curled toward the ceiling.", NewMissingSpace(), "The kettle hissed on the stove. A thin ribbon of steam curled toward the ceiling.")
}

func TestMissingSpace_SparrowFence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A sparrow landed on the fence.Its wings fluttered once before it settled.", NewMissingSpace(), "A sparrow landed on the fence. Its wings fluttered once before it settled.")
}

func TestMissingSpace_StreetlampDusk(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The streetlamp flickered at dusk.A pale glow spread across the sidewalk.", NewMissingSpace(), "The streetlamp flickered at dusk. A pale glow spread across the sidewalk.")
}

func TestMissingSpace_DistantLaughter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Someone laughed in the distance.The echo drifted between the buildings.", NewMissingSpace(), "Someone laughed in the distance. The echo drifted between the buildings.")
}

func TestMissingSpace_NotebookDesk(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A notebook lay open on the desk.Its blank pages waited for a pen.", NewMissingSpace(), "A notebook lay open on the desk. Its blank pages waited for a pen.")
}

func TestMissingSpace_QuestionMarkMidSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Where are you?I looked around the room.", NewMissingSpace(), "Where are you? I looked around the room.")
}

func TestMissingSpace_QuestionMarkBeforeName(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Are you coming?Elijah is already waiting.", NewMissingSpace(), "Are you coming? Elijah is already waiting.")
}

func TestMissingSpace_ExclamationMidSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The door slammed shut!Everyone in the hall jumped.", NewMissingSpace(), "The door slammed shut! Everyone in the hall jumped.")
}

func TestMissingSpace_ExclamationBeforeClause(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You actually solved it!That changes everything.", NewMissingSpace(), "You actually solved it! That changes everything.")
}

func TestMissingSpace_SemicolonBeforeAdverb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He wanted to leave;however, he stayed until the end.", NewMissingSpace(), "He wanted to leave; however, he stayed until the end.")
}

func TestMissingSpace_SemicolonConnectingClauses(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The night was cold;stars glittered above the dark field.", NewMissingSpace(), "The night was cold; stars glittered above the dark field.")
}
