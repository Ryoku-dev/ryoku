package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type LetToDo struct{ expression expr.Expr }

func NewLetToDo() *LetToDo {
	return &LetToDo{expression: expr.NewSequenceExprWordSet([]string{"let", "lets", "let's"}).TWs().ThenAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenObjectPronoun(), expr.NewSequenceExprWordSet([]string{"anybody", "anyone", "everybody", "everyone", "nobody", "noone", "somebody", "someone"}), expr.NewSequenceExprWordSet([]string{"any", "every", "no", "some"}).TWs().TSet([]string{"body", "one"})}).TWs().TAco("to")}
}
func (r *LetToDo) Expr() expr.Expr                  { return r.expression }
func (r *LetToDo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *LetToDo) Description() string              { return "Corrects extraneous `to` after `let`." }
func (r *LetToDo) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 2 {
		return nil
	}
	span, _ := doc.Span(toks[len(toks)-2:])
	return &lint.Lint{Span: span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.Remove()}, Message: "The word `to` should not be used with `let` in this way.", Priority: 127}
}
