package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type Confident struct{ expression expr.Expr }

func NewConfident() *Confident {
	return &Confident{expression: expr.NewSequenceExprWith(expr.Or(expr.Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsVerb() || t.Kind.IsDeterminer() }), expr.NewWord("very"))).ThenWhitespace().TAco("confidant")}
}
func (r *Confident) Expr() expr.Expr { return r.expression }
func (r *Confident) Description() string {
	return "This linter detects instances where the noun `confidant` is incorrectly used in place of the adjective `confident`. `Confidant` refers to a trusted person, whereas `confident` describes certainty or self-assurance. The rule suggests replacing `confidant` with `confident` when used in an adjectival context."
}
func (r *Confident) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Confident) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	return &lint.Lint{Span: t[len(t)-1].Span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune("confident"))}, Message: "Use the adjective.", Priority: 127}
}
