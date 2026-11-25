package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type SomewhatSomething struct{ expression expr.Expr }

func NewSomewhatSomething() *SomewhatSomething {
	return &SomewhatSomething{expression: expr.NewSequenceExprWordSeq([]string{"somewhat", "of", "a"})}
}
func (r *SomewhatSomething) Expr() expr.Expr                  { return r.expression }
func (r *SomewhatSomething) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *SomewhatSomething) Description() string {
	return "Flags the phrase `somewhat of a` in favor of `something of a`, which can be considered more traditional."
}
func (r *SomewhatSomething) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	return &lint.Lint{Span: toks[0].Span, LintKind: lint.Style, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("something", toks[0].GetCh(src))}, Message: "Consider using `something of a` in more formal writing.", Priority: 63}
}
