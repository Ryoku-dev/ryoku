package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type FirstAidKit struct{ expression expr.Expr }

func NewFirstAidKit() *FirstAidKit {
	return &FirstAidKit{expression: expr.NewSequenceExprWordSet([]string{"aid", "starter", "travel", "tool"}).TWs().TAco("kid")}
}
func (r *FirstAidKit) Expr() expr.Expr                  { return r.expression }
func (r *FirstAidKit) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *FirstAidKit) Description() string {
	return "Detects when “kid” after “aid”, “starter”, “travel”, or “tool” should be “kit” (a set of supplies)."
}
func (r *FirstAidKit) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	tok := toks[len(toks)-1]
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.WordChoice
	l.Priority = 31
	l.Message = "Did you mean `kit` (a set of items) instead of “kid”?"
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("kit", tok.GetCh(src))}
	return &l
}
