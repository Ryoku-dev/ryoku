package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestBrandBrandish_CorrectBrandishATraitor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Unretire Gretzky's sweater . Brandish him a traitor.", NewBrandBrandish(), "Unretire Gretzky's sweater . Brand him a traitor.")
}

func TestBrandBrandish_CorrectBrandishACriminal(t *testing.T) {
	linttest.AssertSuggestionResult(t, "lied to stop kuma's ideology from taking root and to brandish him a criminal that they could arrest", NewBrandBrandish(), "lied to stop kuma's ideology from taking root and to brand him a criminal that they could arrest")
}

func TestBrandBrandish_CorrectBrandishAsA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "he was so afraid his thoughts could brandish him as a paedophile", NewBrandBrandish(), "he was so afraid his thoughts could brand him as a paedophile")
}

func TestBrandBrandish_CorrectBrandishAnOffender(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Chanel Oberlin's reason for purposely leading on Pete Martinez in order to humiliate him and brandish him a registered sex offender", NewBrandBrandish(), "Chanel Oberlin's reason for purposely leading on Pete Martinez in order to humiliate him and brand him a registered sex offender")
}

func TestBrandBrandish_CorrectBrandishWithNicknames(t *testing.T) {
	linttest.AssertSuggestionResult(t, "?? spoke out over the move by Kenyans to continuously brandish him with nicknames even after ...", NewBrandBrandish(), "?? spoke out over the move by Kenyans to continuously brand him with nicknames even after ...")
}

func TestBrandBrandish_CorrectBrandishAsAAymbol(t *testing.T) {
	linttest.AssertSuggestionResult(t, "brandish him as an acclaimed symbol of humility, integrity and incorruptibility in the face of today's corrupt economic and political elite1", NewBrandBrandish(), "brand him as an acclaimed symbol of humility, integrity and incorruptibility in the face of today's corrupt economic and political elite1")
}

func TestBrandBrandish_CorrectBrandishAsIllegal(t *testing.T) {
	linttest.AssertSuggestionResult(t, "To attempt to brandish him as an “illegal immigrant” is absolutely ridiculous and warrants an immediate retraction and apology.", NewBrandBrandish(), "To attempt to brand him as an “illegal immigrant” is absolutely ridiculous and warrants an immediate retraction and apology.")
}

func TestBrandBrandish_CorrectBrandishWithNickname(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The small minded townsfolk brandish him with the nickname \"Genepool\" due to his physical and cognitive shortcomings.", NewBrandBrandish(), "The small minded townsfolk brand him with the nickname \"Genepool\" due to his physical and cognitive shortcomings.")
}

func TestBrandBrandish_CorrectBrandishWithLabel(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One such reason that critics brandish him with this label is due to Peterson's opposition to Canada's Bill C-16", NewBrandBrandish(), "One such reason that critics brand him with this label is due to Peterson's opposition to Canada's Bill C-16")
}

func TestBrandBrandish_CorrectBrandishedUs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The mark they brandished us with will fade to dust when we finally meet our end.", NewBrandBrandish(), "The mark they branded us with will fade to dust when we finally meet our end.")
}

func TestBrandBrandish_CorrectBrandishingHim(t *testing.T) {
	linttest.AssertSuggestionResult(t, "he said some words trying to hit back at the center for brandishing him as a Pakistani at an NRC rally", NewBrandBrandish(), "he said some words trying to hit back at the center for branding him as a Pakistani at an NRC rally")
}

func TestBrandBrandish_CorrectBrandishUs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Our resolute determination for the ultimate quality and all-inclusive directory of food commodities brandish us as a flawless associate in B2B", NewBrandBrandish(), "Our resolute determination for the ultimate quality and all-inclusive directory of food commodities brand us as a flawless associate in B2B")
}

func TestBrandBrandish_CorrectBrandishedHim(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Frank discovers Myra brandished him with the letter 'R', for rapist.", NewBrandBrandish(), "Frank discovers Myra branded him with the letter 'R', for rapist.")
}

func TestBrandBrandish_CorrectBrandishesHim(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Whether one turns a blind eye to Tim's wrongs or brandishes him a traitor will plant audiences in their own personal line in the sand.", NewBrandBrandish(), "Whether one turns a blind eye to Tim's wrongs or brands him a traitor will plant audiences in their own personal line in the sand.")
}
