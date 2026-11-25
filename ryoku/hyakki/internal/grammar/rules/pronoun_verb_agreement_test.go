package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPronounVerbAgreement_Issue2331(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I likes this place.", NewPronounVerbAgreement(dict.Curated()), "I like this place.")
}

func TestPronounVerbAgreement_Issue2332(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I sits under the AC.", NewPronounVerbAgreement(dict.Curated()), "I sit under the AC.")
}

func TestPronounVerbAgreement_Issue2331Reverse(t *testing.T) {
	t.Skip("because 'like' is an adjective as well as a verb.")
	linttest.AssertSuggestionResult(t, "He like this place.", NewPronounVerbAgreement(dict.Curated()), "He likes this place.")
}

func TestPronounVerbAgreement_WhyWeCantFlagLikeYet(t *testing.T) {
	linttest.AssertNoLints(t, "What is he like?", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_Issue2332Reverse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She sit under the AC.", NewPronounVerbAgreement(dict.Curated()), "She sits under the AC.")
}

func TestPronounVerbAgreement_DontFlagCorrectAgreement(t *testing.T) {
	linttest.AssertNoLints(t, "He likes this place. I sit under the AC.", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_Issue3925DontFlagSimplePastForms(t *testing.T) {
	linttest.AssertNoLints(t, "He fought tenaciously. He wrought tenaciously.", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_FixesI(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I wakes up.", NewPronounVerbAgreement(dict.Curated()), "I wake up.")
}

func TestPronounVerbAgreement_FixesWe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We gets dressed.", NewPronounVerbAgreement(dict.Curated()), "We get dressed.")
}

func TestPronounVerbAgreement_FixesYou(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You drops off the kids.", NewPronounVerbAgreement(dict.Curated()), "You drop off the kids.")
}

func TestPronounVerbAgreement_FixesHe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He work hard.", NewPronounVerbAgreement(dict.Curated()), "He works hard.")
}

func TestPronounVerbAgreement_FixesShe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She study hard.", NewPronounVerbAgreement(dict.Curated()), "She studies hard.")
}

func TestPronounVerbAgreement_WeCantFixItYet(t *testing.T) {
	t.Skip("Becasue 'it' is also object case. Eg. 'watch it break down'")
	linttest.AssertSuggestionResult(t, "It break down.", NewPronounVerbAgreement(dict.Curated()), "It breaks down.")
}

func TestPronounVerbAgreement_WhyWeCantFixItYet(t *testing.T) {
	linttest.AssertNoLints(t, "I heard it break down.", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_FixesThey(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They repairs it.", NewPronounVerbAgreement(dict.Curated()), "They repair it.")
}

func TestPronounVerbAgreement_DontFlagI(t *testing.T) {
	linttest.AssertNoLints(t, "I eat", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_DontFlagWe(t *testing.T) {
	linttest.AssertNoLints(t, "We drink", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_DontFlagYou(t *testing.T) {
	linttest.AssertNoLints(t, "You walk", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_DontFlagHe(t *testing.T) {
	linttest.AssertNoLints(t, "He runs", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_DontFlagShe(t *testing.T) {
	linttest.AssertNoLints(t, "She swims", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_DontFlagIt(t *testing.T) {
	linttest.AssertNoLints(t, "It works!", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_DontFlagThey(t *testing.T) {
	linttest.AssertNoLints(t, "They finish", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_FixFlies(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I flies", NewPronounVerbAgreement(dict.Curated()), "I fly")
}

func TestPronounVerbAgreement_FixCry(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He cry", NewPronounVerbAgreement(dict.Curated()), "He cries")
}

func TestPronounVerbAgreement_FixGo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She go", NewPronounVerbAgreement(dict.Curated()), "She goes")
}

func TestPronounVerbAgreement_FixGoes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They goes", NewPronounVerbAgreement(dict.Curated()), "They go")
}

func TestPronounVerbAgreement_FixHas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You has", NewPronounVerbAgreement(dict.Curated()), "You have")
}

func TestPronounVerbAgreement_FixHave(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She have", NewPronounVerbAgreement(dict.Curated()), "She has")
}

func TestPronounVerbAgreement_FixHasnt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You hasn't", NewPronounVerbAgreement(dict.Curated()), "You haven't")
}

func TestPronounVerbAgreement_FixHavent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He haven't", NewPronounVerbAgreement(dict.Curated()), "He hasn't")
}

func TestPronounVerbAgreement_FixBox(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He box", NewPronounVerbAgreement(dict.Curated()), "He boxes")
}

func TestPronounVerbAgreement_FixBoxes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You boxes", NewPronounVerbAgreement(dict.Curated()), "You box")
}

func TestPronounVerbAgreement_FixDoesnt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We doesn't", NewPronounVerbAgreement(dict.Curated()), "We don't")
}

func TestPronounVerbAgreement_FixDont(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It don't", NewPronounVerbAgreement(dict.Curated()), "It doesn't")
}

func TestPronounVerbAgreement_FixDo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He do", NewPronounVerbAgreement(dict.Curated()), "He does")
}

func TestPronounVerbAgreement_FixDoes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You does", NewPronounVerbAgreement(dict.Curated()), "You do")
}

func TestPronounVerbAgreement_FalsePositiveSheConsider(t *testing.T) {
	linttest.AssertNoLints(t, "On April 10th, I suggested she consider a smaller, more intimate gathering.", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_FalsePositiveSheSell(t *testing.T) {
	linttest.AssertNoLints(t, "I suggested she sell it and use the proceeds to help with her relocation expenses, or perhaps rent a similar camera while in Barcelona.", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_FalsePositiveSheRent(t *testing.T) {
	linttest.AssertNoLints(t, "I suggested she sell it and use the proceeds to help with her relocation expenses, or perhaps rent a similar camera while in Barcelona.", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_FalsePositiveHeDonned(t *testing.T) {
	linttest.AssertNoLints(t, "He donned his heavy oilskins and descended the winding staircase, his boots echoing in the hollow tower.", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_FalsePositiveHeCannot(t *testing.T) {
	linttest.AssertNoLints(t, "Surely, he cannot offer the same sum as the developers.", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_FalsePositiveInsistingSheReturn(t *testing.T) {
	linttest.AssertNoLints(t, "Am I the asshole for insisting she return the dress?", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_FalsePositivePrideInYouIs(t *testing.T) {
	linttest.AssertNoLints(t, "It’s also important to recognize that your family's pride in you is a genuine reflection of your value.", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_FalsePositiveSheSought(t *testing.T) {
	linttest.AssertNoLints(t, "She sought out Mrs. Hawthorne, the village’s oldest resident, a woman known for her vast knowledge of local history and her unsettlingly accurate intuition.", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_FalsePositiveLoseYouPoints(t *testing.T) {
	linttest.AssertNoLints(t, "I admire your dedication to consistently drafting players who are actively trying to lose you points.", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_FalsePositiveSheHungUp(t *testing.T) {
	linttest.AssertNoLints(t, "When I reiterated the conditions I'd previously set, she hung up on me.", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_DontFlag4388(t *testing.T) {
	linttest.AssertNoLints(t, "We humans are naturally curious.", NewPronounVerbAgreement(dict.Curated()))
}

func TestPronounVerbAgreement_DontFlagSheFurtherExplained(t *testing.T) {
	linttest.AssertNoLints(t, "She further explained the plan.", NewPronounVerbAgreement(dict.Curated()))
}
