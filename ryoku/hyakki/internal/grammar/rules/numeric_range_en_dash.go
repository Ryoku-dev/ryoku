package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type NumericRangeEnDash struct{ expression expr.Expr }

func NewNumericRangeEnDash() *NumericRangeEnDash {
	return &NumericRangeEnDash{expression: expr.NewSequenceExprNumber().Then(func(t doc.Token, _ []rune) bool {
		p, ok := t.Kind.AsPunctuation()
		return ok && (p.IsHyphen() || p.IsEmDash())
	}).ThenNumber()}
}
func (r *NumericRangeEnDash) Expr() expr.Expr                  { return r.expression }
func (r *NumericRangeEnDash) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *NumericRangeEnDash) Description() string {
	return "Replaces hyphens and em dashes with en dashes in isolated numeric ranges such as `12–14`."
}
func (r *NumericRangeEnDash) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func numericRangeEnDashIsRange(t doc.Token) bool {
	p, ok := t.Kind.AsPunctuation()
	return ok && (p.IsHyphen() || p.IsEnDash() || p.IsEmDash())
}
func (r *NumericRangeEnDash) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if ctx != nil {
		b, a := ctx.Before, ctx.After
		if len(b) >= 2 && b[len(b)-2].Kind.IsNumber() && numericRangeEnDashIsRange(b[len(b)-1]) {
			return nil
		}
		if len(a) >= 2 && numericRangeEnDashIsRange(a[0]) && a[1].Kind.IsNumber() {
			return nil
		}
		if len(b) >= 2 && b[len(b)-1].Kind.IsHyphen() && b[len(b)-2].Kind.IsWord() && text.EqStr(b[len(b)-2].GetCh(src), "cve") {
			return nil
		}
	}
	if len(toks) < 2 {
		return nil
	}
	l := lint.NewLint()
	l.Span = toks[1].Span
	l.LintKind = lint.Formatting
	l.Priority = 63
	l.Message = "Use an en dash (–) in ranges of numbers. Ignore this if it is math."
	l.Suggestions = []lint.Suggestion{lint.ReplaceWith([]rune{'–'})}
	return &l
}
