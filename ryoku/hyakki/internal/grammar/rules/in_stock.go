package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type InStock struct{ expression expr.Expr }

func NewInStock() *InStock {
	return &InStock{expression: expr.NewSequenceExprAnyOf([]expr.Expr{expr.AsExpr(expr.NewInflectionOfBe()), expr.NewSequenceExprWordSet([]string{"have", "had", "has", "having"}).ThenOptional(expr.NewSequenceExprWhitespace().TSet([]string{"it", "one", "some", "them", "inventory"})), expr.AsExpr(expr.NewWord("back"))}).TWs().ThenWordSeq([]string{"on", "stock"}).ThenAnyOf([]expr.Expr{expr.AsExpr(expr.NewAnchorEnd()), expr.NewSequenceExprWhitespace().ThenKindEither(doc.TokenKind.IsConjunction, doc.TokenKind.IsPreposition)})}
}
func (r *InStock) Expr() expr.Expr                  { return r.expression }
func (r *InStock) Description() string              { return "Corrects `on stock` to `in stock`." }
func (r *InStock) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *InStock) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	tok := lint.FindTheOnlyTokenMatching(t, s, func(t *doc.Token, _ []rune) bool { return text.EqStr(t.GetCh(s), "on") })
	if tok == nil {
		return nil
	}
	return &lint.Lint{Span: tok.Span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("in", tok.GetCh(s))}, Message: "The correct usage is `in stock` not `on stock`.", Priority: 127}
}
