package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ComplainAsNoun struct{ expression expr.Expr }

func NewComplainAsNoun() *ComplainAsNoun {
	e := expr.ButNot(expr.NewSequenceExpr().ThenDeterminer().TWs().TSet([]string{"complain", "complains"}), expr.NewSequenceExprAnything().TAny().TAny().TWs().ThenNoun())
	return &ComplainAsNoun{expression: e}
}
func (r *ComplainAsNoun) Expr() expr.Expr                  { return r.expression }
func (r *ComplainAsNoun) Description() string              { return "Corrects the use of `complain` as a noun." }
func (r *ComplainAsNoun) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ComplainAsNoun) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) != 3 {
		return nil
	}
	noun := "complaint"
	if text.EndsWithIgnoreASCIICaseStr(t[2].GetCh(s), "s") {
		noun += "s"
	}
	l := lint.NewLint()
	l.Span = t[2].Span
	l.LintKind = lint.Grammar
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(noun, t[2].GetCh(s))}
	l.Message = "The noun form is `complaint`."
	return &l
}
