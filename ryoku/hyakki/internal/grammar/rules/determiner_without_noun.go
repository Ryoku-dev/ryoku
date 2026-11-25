package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type DeterminerWithoutNoun struct{ expression expr.Expr }

func NewDeterminerWithoutNoun() *DeterminerWithoutNoun {
	return &DeterminerWithoutNoun{expression: expr.NewSequenceExpr().ThenKindWhere(doc.TokenKind.IsDeterminer).TWs().ThenConjunction()}
}
func (r *DeterminerWithoutNoun) Expr() expr.Expr                  { return r.expression }
func (r *DeterminerWithoutNoun) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *DeterminerWithoutNoun) Description() string {
	return "Flags sequences where a determiner (`a`, `an`, `the`, etc.) is directly followed by a coordinating or subordinating conjunction (`and`, `or`, `but`, `because`, etc.), indicating a missing noun."
}
func (r *DeterminerWithoutNoun) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Miscellaneous, Message: "A determiner should not be immediately followed by a conjunction.", Priority: 32}
}
