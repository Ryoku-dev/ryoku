package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ItAreTheOne struct{ expression expr.Expr }

func NewItAreTheOne() *ItAreTheOne {
	return &ItAreTheOne{expression: expr.NewSequenceExpr().ThenWordExcept([]string{"you", "they"}).TWs().TSet([]string{"are", "were"}).TWs().TAco("the").TWs().ThenOptional(expr.NewSequenceExprAco("only").TWs()).TAco("one").TWs().TSet([]string{"which", "who", "whose", "that", "whom"})}
}
func (r *ItAreTheOne) Expr() expr.Expr                  { return r.expression }
func (r *ItAreTheOne) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ItAreTheOne) Description() string {
	return "A linter skeleton for contributors to copy into `harper_core/src/linting/` and rename."
}
func (r *ItAreTheOne) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 7 {
		return nil
	}
	i := 6
	if text.EqStr(toks[6].GetCh(src), "only") {
		i = 8
	}
	if i >= len(toks) {
		return nil
	}
	span := toks[i].Span
	one := append([]rune(nil), span.GetContent(src)...)
	if len(one) == 0 {
		return nil
	}
	one = append(one, rune(byte(one[len(one)-1])+byte('s'-'e')))
	return &lint.Lint{Span: span, LintKind: lint.Agreement, Suggestions: []lint.Suggestion{lint.ReplaceWith(one)}, Message: "Since `are` is plural, `ones` also needs to be plural.", Priority: 127}
}
