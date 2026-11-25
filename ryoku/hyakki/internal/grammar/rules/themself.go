package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type Themself struct{ expression expr.Expr }

func NewThemself() *Themself {
	condition := expr.NewSequenceExprAnything().TAny().TAny().ThenAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenHyphen(), expr.NewSequenceExprWhitespace().Then(func(t doc.Token, _ []rune) bool {
		return t.Kind.IsNoun() || t.Kind.IsVerbPastForm() || t.Kind.IsVerbPastParticipleForm()
	})})
	return &Themself{expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprAco("their").TWs().TAco("selves"), expr.NewAll([]expr.Expr{expr.NewSequenceExprAco("their").TWs().TAco("self"), expr.AsExpr(expr.NewUnlessStep(condition, func(_ doc.Token, _ []rune) bool { return true }))})})}
}
func (r *Themself) Expr() expr.Expr                  { return r.expression }
func (r *Themself) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*Themself) Description() string {
	return "Corrects `their self` and `their selves` to `themself` and `themselves`."
}
func (r *Themself) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 3 {
		return nil
	}
	replacement := "themselves"
	if len(t[2].GetCh(s)) == 4 {
		replacement = "themself"
	}
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, span.GetContent(s))}, Message: "This should be one word, not two.", Priority: 127}
}
