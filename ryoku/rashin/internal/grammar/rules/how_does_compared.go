package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type HowDoesCompared struct{ expression expr.Expr }

func NewHowDoesCompared() *HowDoesCompared {
	return &HowDoesCompared{expression: expr.NewSequenceExprAco("how").TWs().ThenAnyOf([]expr.Expr{expr.NewSequenceExprAco("do").TWs().TSet([]string{"these", "they", "those"}), expr.NewSequenceExprAco("does").TWs().TSet([]string{"it", "that", "this"}), expr.NewSequenceExprAco("did").TWs().TSet([]string{"it", "that", "these", "they", "this", "those"})}).TWs().TSet([]string{"compared", "compares"})}
}
func (r *HowDoesCompared) Expr() expr.Expr                  { return r.expression }
func (r *HowDoesCompared) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *HowDoesCompared) Description() string {
	return "Corrects `how do/does/did X compared/compares to Y` to use `compare`."
}
func (r *HowDoesCompared) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	tok := toks[len(toks)-1]
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Grammar
	l.Message = "Use the base form of the verb `compare`."
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("compare", tok.GetCh(src))}
	return &l
}
