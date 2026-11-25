package rules

import (
	"sync"
	"testing"

	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/linttest"
	"ryoku-rashin/internal/grammar/weir"
)

var curatedTestPool = sync.OnceValue(func() *lint.PooledLinter[*lint.LintGroup] {
	return lint.NewPooledLinter(func() *lint.LintGroup { return NewCurated(dict.Curated(), dict.American) })
})

func TestGroupCleanInterjection(t *testing.T) {
	linttest.AssertNoLints(t, "Although I only saw the need to interject once, I still saw it.", curatedTestPool())
}
func TestGroupCleanConsensus(t *testing.T) {
	linttest.AssertNoLints(t, "But there is less consensus on this.", curatedTestPool())
}
func TestGroupIveCorrectsToSingleWord(t *testing.T) {
	linttest.AssertSuggestionResult(t, "ive never seen that before", curatedTestPool(), "I've never seen that before")
}
func TestGroupWorthcheckingIsSplit(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It is worthchecking", curatedTestPool(), "It is worth checking")
}
func TestGroupItsNotPerfectKeepsApostrophe(t *testing.T) {
	linttest.AssertNoLints(t, "It's not perfect", curatedTestPool())
}
func TestGroupCorrectsExtention(t *testing.T) {
	curatedTestPool().RunWithInner(func(g *lint.LintGroup) {
		organized := g.OrganizedLints(doc.NewPlainEnglishCurated("I love this extention!"))
		ls := organized["SpellCheck"]
		if len(ls) != 1 {
			t.Fatal(ls)
		}
		found := false
		for _, s := range ls[0].Suggestions {
			found = found || s.String() == "Replace with: “extension”"
		}
		if !found {
			t.Fatal(ls)
		}
		if len(organized["SplitWords"]) != 0 {
			t.Fatal(organized["SplitWords"])
		}
	})
}
func TestGroupOkBecomesOkay(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is ok.", curatedTestPool(), "This is okay.")
}
func TestGroupWeirLinterUsesConfiguredSentenceScope(t *testing.T) {
	l, err := weir.NewWeirLinter(`expr main one**two
let message "Use three."
let description "Test sentence-scoped Weir."
let kind "Miscellaneous"
let becomes "three"
let strategy "Exact"
let scope "Sentence"`)
	if err != nil {
		t.Fatal(err)
	}
	sentence, ok := l.IntoSentenceLinter()
	if !ok {
		t.Fatal("not sentence")
	}
	g := lint.NewLintGroup()
	g.AddSentenceExprLinter("TestSentenceWeir", sentence)
	g.Config.SetRuleEnabled("TestSentenceWeir", true)
	linttest.AssertSuggestionResult(t, "one, two.", g, "three.")
}
func TestGroupCanGetAllDescriptions(t *testing.T) {
	g := NewCurated(dict.NewMutableDictionary(), dict.American)
	g.AllDescriptions()
}
func TestGroupDontFlagLowHangingFruitMsg(t *testing.T) {
	linttest.AssertNoLints(t, "The standard form is low-hanging fruit with a hyphen and singular form.", curatedTestPool())
}
func TestGroupDontFlagLowHangingFruitDesc(t *testing.T) {
	linttest.AssertNoLints(t, "Corrects nonstandard variants of low-hanging fruit.", curatedTestPool())
}
func TestGroupFixStoped(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He stoped to think.", curatedTestPool(), "He stopped to think.")
}
func TestGroupAllowPlaythrough(t *testing.T) {
	linttest.AssertNoLints(t, "I recently did a playthrough.", curatedTestPool())
}
func TestGroupFixesPoliticans(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I guess politicans like to complain.", curatedTestPool(), "I guess politicians like to complain.")
}
func TestGroupFixesBenefitting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Who is really benefitting from it?", curatedTestPool(), "Who is really benefiting from it?")
}
func TestGroupFixesExtentionCorrectly(t *testing.T) {
	linttest.AssertSuggestionResult(t, "There is no extention quite like this one.", curatedTestPool(), "There is no extention quite like this one.")
}
func TestGroupFixesEasir(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It makes it easir to select it.", curatedTestPool(), "It makes it easier to select it.")
}
func TestGroupFixesBreakfest(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ice cream for breakfest?", curatedTestPool(), "Ice cream for breakfast?")
}
func TestGroupFixesBuget(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It was in their buget range.", curatedTestPool(), "It was in their budget range.")
}
func TestGroupAllowsChestCompressions(t *testing.T) {
	linttest.AssertNoLints(t, "Please continue chest compressions.", curatedTestPool())
}
func TestGroupNoLinterNamesClash(t *testing.T) {
	g := NewCurated(dict.NewMutableDictionary(), dict.American)
	if len(g.ClashingLinterNames) != 0 {
		t.Fatal(g.ClashingLinterNames)
	}
}
func TestGroupSpellcheckBeatsSplitWordsForAdvices4346(t *testing.T) {
	linttest.AssertSuggestionResult(t, "IMO these kind of advices never matters.", curatedTestPool(), "IMO these kind of advice never matters.")
}
func TestCuratedDefaultConfigListsEveryRegisteredRule(t *testing.T) {
	g := NewCurated(dict.NewMutableDictionary(), dict.American)
	registered := map[string]bool{}
	for _, key := range g.IterKeys() {
		registered[key] = true
		if !g.Config.HasRule(key) {
			t.Errorf("registered rule missing default: %s", key)
		}
	}
	for key := range lint.NewCuratedFlatConfig() {
		if key != "BoringWords" && !registered[key] {
			t.Errorf("default rule not registered: %s", key)
		}
	}
}
