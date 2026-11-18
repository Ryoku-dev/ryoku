package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type SomethingIs struct{ expression expr.Expr }

func NewSomethingIs() *SomethingIs {
	return &SomethingIs{expression: expr.NewSequenceExprWith(expr.NewWordSet([]string{"somethings", "anythings", "everythings", "nothings"})).TWs().ThenOptional(expr.NewSequenceExpr().ThenOneOrMoreAdverbs().TWs()).ThenKindAny([]expr.KindPredicate{doc.TokenKind.IsVerbProgressiveForm})}
}
func (r *SomethingIs) Expr() expr.Expr { return r.expression }
func (r *SomethingIs) Description() string {
	return "Flags forms like `somethings` before progressive verbs and suggests using `something's` or `something is`."
}
func (r *SomethingIs) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *SomethingIs) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	original := t[0].GetCh(s)
	if len(original) == 0 {
		return nil
	}
	stem := string(original[:len(original)-1])
	return &lint.Lint{Span: t[0].Span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(stem+"'s", original), lint.ReplaceWithMatchCaseStr(stem+" is", original)}, Message: "Prefer the contraction or full `is` rather than pluralizing this pronoun.", Priority: 31}
}
