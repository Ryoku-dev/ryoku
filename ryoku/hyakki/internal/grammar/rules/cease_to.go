package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type CeaseTo struct{ expression expr.Expr }

func NewCeaseTo() *CeaseTo {
	return &CeaseTo{expression: expr.NewSequenceExprWordSet([]string{"seize", "seized", "seizes", "seizing", "sieze", "siezed", "siezes", "siezing"}).TWs().TAco("to").TWs().ThenVerbLemma()}
}
func (r *CeaseTo) Expr() expr.Expr { return r.expression }
func (r *CeaseTo) Description() string {
	return "Detects when `seize to` is likely a typo for `cease to`."
}
func (r *CeaseTo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *CeaseTo) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	ch := t[0].GetCh(s)
	if len(ch) < 4 {
		return nil
	}
	fix := append([]rune("ceas"), ch[4:]...)
	return &lint.Lint{Span: t[0].Span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCase(fix, ch)}, Message: "Did you mean `cease to` (stop)? If you meant `seize to` (e.g., assets seized to cover a debt), you can ignore this.", Priority: 127}
}
