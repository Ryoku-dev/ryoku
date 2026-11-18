package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type Whereas struct{ expression expr.Expr }

func NewWhereas() *Whereas {
	return &Whereas{expr.NewSequenceExprAco("where").ThenWhitespace().TAco("as")}
}
func (r *Whereas) Expr() expr.Expr                  { return r.expression }
func (r *Whereas) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*Whereas) Description() string {
	return "The Whereas rule is designed to identify instances where the phrase `where as` is used in text and suggests replacing it with the single word `whereas`."
}
func (r *Whereas) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("whereas", span.GetContent(s))}, Message: "`Whereas` is commonly mistaken for `where as`.", Priority: 127}
}
