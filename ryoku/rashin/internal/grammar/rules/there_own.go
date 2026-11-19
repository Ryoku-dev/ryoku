package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

// ThereOwn flags "there own", "they're own", and "theyre own" and suggests "their own".
type ThereOwn struct{ expression expr.Expr }

func NewThereOwn() *ThereOwn {
	return &ThereOwn{expr.NewSequenceExprWordSet([]string{"there", "they're", "theyre"}).TWs().TAco("own")}
}
func (r *ThereOwn) Expr() expr.Expr                  { return r.expression }
func (r *ThereOwn) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*ThereOwn) Description() string {
	return "Corrects `there own`, `they're own`, and `theyre own` to `their own`."
}
func (r *ThereOwn) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *ThereOwn) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	w := t[0]
	if text.EqStr(w.GetCh(s), "there") && lint.PrecededByWord(c, func(p *doc.Token) bool { return p.Kind.IsNominal() }) {
		return nil
	}
	return &lint.Lint{Span: w.Span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("their", w.GetCh(s))}, Message: "Did you mean the possessive `their`?", Priority: 31}
}
