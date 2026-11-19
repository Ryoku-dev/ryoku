package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type PayForPrice struct{ expression expr.Expr }

func NewPayForPrice() *PayForPrice {
	return &PayForPrice{expression: expr.NewSequenceExprWordSet([]string{"pay", "paid", "pays", "paying"}).TWs().TAco("for").TWs().ThenOptional(expr.NewSequenceExpr().ThenDeterminer().TWs()).Then(expr.ButNot(expr.NewSequenceExprWordSet([]string{"bill", "bills", "check", "checks", "cheque", "cheques", "charge", "charges", "cost", "costs", "fee", "fees", "price", "prices"}), expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewSequenceExprAco("check").TWsH().TSet([]string{"in", "out"}), expr.NewSequenceExprAco("price").TWsH().TAco("increase")})))}
}
func (r *PayForPrice) Expr() expr.Expr                  { return r.expression }
func (r *PayForPrice) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*PayForPrice) Description() string {
	return "Corrects extraneous `for` when used of charges, fees, prices, etc."
}
func (r *PayForPrice) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t[2:4])
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{lint.Remove()}
	l.Message = "You `pay for` things or services, but just `pay prices/fees`, etc with no preposition."
	return &l
}
