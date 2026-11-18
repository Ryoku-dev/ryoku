package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ProgressiveNeedsBe struct{ expression expr.Expr }

func NewProgressiveNeedsBe() *ProgressiveNeedsBe {
	contracted := expr.NewSequenceExprWordSet([]string{"I've", "We've", "You've", "They've", "Ive", "Weve", "Youve", "Theyve"}).TWs().ThenVerbProgressiveForm()
	non := expr.NewSequenceExprWordSet([]string{"I", "We", "You", "They"}).TWs().ThenAnyCapitalizationOf("have").TWs().ThenVerbProgressiveForm()
	e := expr.NewSequenceExprAnyOf([]expr.Expr{contracted, non})
	return &ProgressiveNeedsBe{expression: e}
}
func (r *ProgressiveNeedsBe) Expr() expr.Expr { return r.expression }
func (r *ProgressiveNeedsBe) Description() string {
	return "Detects the ungrammatical patterns `<pronoun> have …ing` (e.g., `I have …ing`) and `<pronoun>'ve …ing` (e.g., `I've …ing`) and suggests either the present progressive (e.g., `I'm/We're/You're/They're …`) or the present perfect progressive (e.g., `I/We/You/They have been …` or `I've/We've/You've/They've been …`)."
}
func (r *ProgressiveNeedsBe) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ProgressiveNeedsBe) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	words := doc.IterWords(t)
	if len(words) == 0 {
		return nil
	}
	first := words[0]
	span := first.Span
	for _, tok := range words {
		if text.EqStr(tok.GetCh(s), "have") {
			span.End = tok.Span.End
			break
		}
	}
	pronoun := first.GetCh(s)
	replacement := "I'm"
	if text.StartsWithIgnoreASCIICaseStr(pronoun, "we") {
		replacement = "We're"
	} else if text.StartsWithIgnoreASCIICaseStr(pronoun, "you") {
		replacement = "You're"
	} else if text.StartsWithIgnoreASCIICaseStr(pronoun, "they") {
		replacement = "They're"
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.WordChoice
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, span.GetContent(s)), lint.InsertAfter([]rune(" been"))}
	l.Message = "Use present progressive (`…'re/…'m …`) or present perfect progressive (`… have been …`/`…'ve been …`) instead of `… have …ing` or `…'ve …ing`."
	l.Priority = 31
	return &l
}
