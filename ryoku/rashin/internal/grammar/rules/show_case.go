package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ShowCase struct{ expression expr.Expr }

func NewShowCase() *ShowCase {
	exclusion := expr.NewSequenceExprAnything().TAny().TAny().ThenAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenHyphen(), expr.NewSequenceExprWhitespace().TSet([]string{"folded", "folding", "history", "histories", "number", "numbers", "sensitive", "insensitive", "study", "studies"})})
	return &ShowCase{expression: expr.ButNot(expr.NewSequenceExprAco("show").TWsH().TAco("case"), exclusion)}
}
func (r *ShowCase) Expr() expr.Expr                  { return r.expression }
func (r *ShowCase) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ShowCase) Description() string              { return "Corrects `show case` to `showcase`." }
func (r *ShowCase) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Miscellaneous
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("showcase", span.GetContent(src))}
	l.Message = "Did you mean `showcase` (highlight the best qualities of something)?"
	return &l
}
