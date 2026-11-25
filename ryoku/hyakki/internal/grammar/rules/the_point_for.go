package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ThePointFor struct{ expression expr.Expr }

func NewThePointFor() *ThePointFor {
	return &ThePointFor{expression: expr.NewSequenceExprAnyOf([]expr.Expr{expr.AsExpr(expr.NewWordSet([]string{"is", "was", "what's", "whats"}))}).TWs().TAco("the").TWs().TAco("point").TWs().TAco("for")}
}
func (r *ThePointFor) Expr() expr.Expr                  { return r.expression }
func (r *ThePointFor) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ThePointFor) Description() string              { return "Corrects `the point for` to `the point of`" }
func (r *ThePointFor) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *ThePointFor) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	if c != nil && len(c.After) >= 2 && c.After[0].Kind.IsWhitespace() && text.EqStr(c.After[1].GetCh(s), "which") {
		return nil
	}
	if len(t) == 0 {
		return nil
	}
	tok := t[len(t)-1]
	result := lint.NewLint()
	result.Span = tok.Span
	result.LintKind = lint.Usage
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("of", tok.GetCh(s))}
	result.Message = "Did you mean `the point of`?"
	return &result
}
