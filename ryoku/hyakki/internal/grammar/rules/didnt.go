package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type Didnt struct{ expression expr.Expr }

func NewDidnt() *Didnt                            { return &Didnt{expr.NewSequenceExpr().ThenSubjectPronoun().TWs().TAco("dint")} }
func (r *Didnt) Expr() expr.Expr                  { return r.expression }
func (r *Didnt) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*Didnt) Description() string                { return "Corrects `dint` to `didn't` after subject pronouns." }
func (r *Didnt) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	w := t[len(t)-1]
	return &lint.Lint{Span: w.Span, LintKind: lint.Typo, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("didn't", w.GetCh(s))}, Message: "Consider using `didn't` here.", Priority: 63}
}
