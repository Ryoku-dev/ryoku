package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type Likewise struct{ expression expr.Expr }

func NewLikewise() *Likewise {
	e := expr.NewAll()
	e.Add(expr.NewSequenceExprWordSeq([]string{"like", "wise"}))
	e.Add(expr.NewSequenceExprUnless(expr.NewSequenceExprAnything().ThenWhitespace().ThenAnything().ThenWhitespace().ThenNoun()))
	return &Likewise{expression: e}
}
func (r *Likewise) Expr() expr.Expr                  { return r.expression }
func (r *Likewise) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Likewise) Description() string {
	return "Looks for incorrect spacing inside the closed compound `likewise`."
}
func (r *Likewise) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.WordChoice
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("likewise", span.GetContent(src))}
	l.Message = "Did you mean the closed compound `likewise`?"
	return &l
}
