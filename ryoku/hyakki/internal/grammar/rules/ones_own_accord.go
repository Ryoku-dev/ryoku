package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type OnesOwnAccord struct{ expression expr.Expr }

func NewOnesOwnAccord() *OnesOwnAccord {
	return &OnesOwnAccord{expression: expr.NewSequenceExprAco("on").TWs().TSet([]string{"my", "our", "your", "his", "her", "its", "their", "one's", "it's", "ones", "there", "they're", "theyre"}).TWs().TAco("own").TWs().TSet([]string{"accord", "accords"})}
}
func (r *OnesOwnAccord) Expr() expr.Expr { return r.expression }
func (r *OnesOwnAccord) Description() string {
	return "Detects incorrect usage of `on one's own accord` and suggests `of one's own accord`."
}
func (r *OnesOwnAccord) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *OnesOwnAccord) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	return &lint.Lint{Span: t[0].Span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("of", t[0].GetCh(s))}, Message: "The correct preposition is `of`.", Priority: 127}
}
