package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestVerbToAdjective_FlagsIssue2855AndSimilarGerundNounPhrases(t *testing.T) {
	linttest.AssertEachLints(t, []string{
		"By scheduling time to do a fully accounting of where your CPU cycles are going, you can preemptively save yourself (and your contributors) a lot of time.",
		"We need a fully understanding of the risk before launch.",
		"The mockup offered a fully rendering of the scene.",
		"An entirely recording of the hearing was never released.",
		"The appendix contained a completely listing of every exception.",
		"The report ended with a highly framing of the debate.",
		"A fully mapping of the cave is still useful to explorers.",
		"The board requested a truly accounting of the losses.",
	}, NewVerbToAdjective())
}

func TestVerbToAdjective_MessagePointsToTheAdverbAndNoun(t *testing.T) {
	linttest.AssertLintMessage(t, "We need a fully understanding of the risk before launch.", NewVerbToAdjective(), "`fully` is an adverb. Before the noun `understanding`, this phrase more likely needs an adjective.")
}

func TestVerbToAdjective_AllowsTheIssue2855Regressions(t *testing.T) {
	linttest.AssertEachNoLints(t, []string{
		"South Korea Set To Get a Fully Functioning Google Maps integration.",
		"This allows our clients to embrace a truly data-driven approach.",
		"Before he could press the transmit button, the sphere emitted a high-pitched whine.",
		"The target of coordinated, algorithmic pursuit kept moving.",
		"A fully functioning prototype shipped yesterday.",
		"A truly remarkable outcome followed.",
		"A completely different problem remains.",
		"A fully rendered image loaded instantly.",
	}, NewVerbToAdjective())
}

func TestVerbToAdjective_AllowsCorrectAdjectivePlusGerundNounOfPhrases(t *testing.T) {
	linttest.AssertEachNoLints(t, []string{
		"We need a full understanding of the risk before launch.",
		"The audit delivered a full accounting of the losses.",
		"The appendix included a complete listing of the requirements.",
		"The demo provided a detailed rendering of the scene.",
		"They archived a proper recording of the interview.",
		"The memo offered a clear framing of the issue.",
		"The atlas provides a partial mapping of the cave.",
		"The exhibit included the recording of the speech.",
	}, NewVerbToAdjective())
}

func TestVerbToAdjective_AllowsNonmatchingDeterminerPhrasesAndPreviousRegressions(t *testing.T) {
	linttest.AssertEachNoLints(t, []string{
		"I really like my new car.",
		"I want you to write a new sentence for me.",
		"Ensure the correct term is used for individuals residing abroad.",
		"It is something that causes amazement.",
		"Can you suggest a correct auxiliary?",
		"This is the email address that will receive the submitted form data.",
		"Not the unexplored territories, ripe for discovery, but the areas actively erased, obscured, or simply deemed unworthy of representation?",
		"A local mapping service is sufficient here.",
		"The transmit queue drained normally.",
		"A mostly accurate summary is still useful.",
	}, NewVerbToAdjective())
}
