package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type AmInTheMorning struct{ expression expr.Expr }

func NewAmInTheMorning() *AmInTheMorning {
	am := expr.NewWordSet([]string{"am", "a.m."})
	pm := expr.NewWordSet([]string{"pm", "p.m."})
	a := expr.NewLongestMatchOf([]expr.Expr{expr.NewSequenceExprWith(am), expr.NewSequenceExprWhitespace().Then(am)})
	p := expr.NewLongestMatchOf([]expr.Expr{expr.NewSequenceExprWith(pm), expr.NewSequenceExprWhitespace().Then(pm)})
	e := expr.NewSequenceExprAnyOf([]expr.Expr{a, p}).ThenAnyOf([]expr.Expr{expr.NewSequenceExprFixedPhrase(" in the ").ThenWordSet([]string{"morning", "afternoon", "evening", "night"}), expr.NewFixedPhraseFromPhrase(" at night")})
	return &AmInTheMorning{expression: e}
}
func (r *AmInTheMorning) Expr() expr.Expr                  { return r.expression }
func (r *AmInTheMorning) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AmInTheMorning) Description() string {
	return "Finds redundant am/pm indicators used together with time periods such as 'in the morning' or 'at night'."
}
func (r *AmInTheMorning) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	idx := 0
	if toks[0].Kind.IsWhitespace() {
		idx = 1
	}
	a := text.Span{Start: toks[0].Span.Start, End: toks[idx].Span.End}
	b := text.Span{Start: toks[idx+1].Span.Start, End: span.End}
	return &lint.Lint{Span: span, LintKind: lint.Redundancy, Suggestions: []lint.Suggestion{lint.ReplaceWith(a.GetContent(src)), lint.ReplaceWith(b.GetContent(src))}, Message: "The time period is redundant because it repeats what's already specified by the AM/PM indicator.", Priority: 50}
}
