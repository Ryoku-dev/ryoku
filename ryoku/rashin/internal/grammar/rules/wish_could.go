package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type WishCould struct{ expression expr.Expr }

func NewWishCould() *WishCould {
	return &WishCould{expression: expr.NewSequenceExprWordSet([]string{"wish", "wished", "wishes", "wishing"}).TWs().ThenAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenSubjectPronoun(), expr.NewSequenceExprWordSet([]string{"anybody", "anyone", "everybody", "everyone", "nobody", "noone", "somebody", "someone", "these", "this", "those"})}).TWs().TAco("can")}
}
func (r *WishCould) Expr() expr.Expr                  { return r.expression }
func (r *WishCould) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *WishCould) Description() string {
	return "Checks for `can` being used after `wish` when it should be `could`."
}
func (r *WishCould) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	tok := t[len(t)-1]
	result := lint.NewLint()
	result.Span = tok.Span
	result.LintKind = lint.Grammar
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("could", tok.GetCh(s))}
	result.Message = "Use 'could' instead of 'can' after 'wish'."
	return &result
}
