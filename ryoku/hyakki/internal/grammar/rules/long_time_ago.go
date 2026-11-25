package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type LongTimeAgo struct{ expression expr.Expr }

func NewLongTimeAgo() *LongTimeAgo {
	return &LongTimeAgo{expression: expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewAnchorStart(), expr.NewSequenceExpr().ThenWordExcept([]string{"a"}).TWs()}).TAco("long").TWs().TAco("time").TWs().TAco("ago")}
}
func (r *LongTimeAgo) Expr() expr.Expr { return r.expression }
func (r *LongTimeAgo) Description() string {
	return "Corrects the missing article `a` in the phrase `long time ago`."
}
func (r *LongTimeAgo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *LongTimeAgo) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 7 {
		t = t[2:]
	} else if len(t) != 5 {
		return nil
	}
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("a long time ago", span.GetContent(s))}, Message: "The correct phrase is `a long time ago`.", Priority: 127}
}
