package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type OnceOrTwice struct{ expression expr.Expr }

func NewOnceOrTwice() *OnceOrTwice {
	return &OnceOrTwice{expression: expr.NewSequenceExprWordSeq([]string{"once", "a", "twice"})}
}
func (r *OnceOrTwice) Expr() expr.Expr                  { return r.expression }
func (r *OnceOrTwice) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *OnceOrTwice) Description() string {
	return "Detects the mistaken phrase `once a twice` and suggests `once or twice`."
}
func (r *OnceOrTwice) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	for _, tok := range t {
		if tok.Kind.IsWord() && text.EqStr(tok.GetCh(s), "a") {
			return &lint.Lint{Span: tok.Span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("or", tok.GetCh(s))}, Message: "Did you mean “or”?", Priority: 31}
		}
	}
	return nil
}
