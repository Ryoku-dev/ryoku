package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type BehindTheScenes struct{ expression expr.Expr }

func NewBehindTheScenes() *BehindTheScenes {
	return &BehindTheScenes{expr.NewSequenceExprAco("behind").TWsH().TAco("the").TWsH().TAco("scene")}
}
func (r *BehindTheScenes) Expr() expr.Expr                  { return r.expression }
func (r *BehindTheScenes) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*BehindTheScenes) Description() string {
	return "Corrects `behind the scene` to `behind the scenes`."
}
func (r *BehindTheScenes) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *BehindTheScenes) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	if c != nil && len(c.Before) > 0 && c.Before[len(c.Before)-1].Kind.IsHyphen() {
		return nil
	}
	if len(t) == 0 {
		return nil
	}
	w := t[len(t)-1]
	return &lint.Lint{Span: w.Span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("scenes", w.GetCh(s))}, Message: "This idiom uses the plural `scenes`.", Priority: 127}
}
