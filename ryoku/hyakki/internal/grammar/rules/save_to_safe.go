package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type SaveToSafe struct{ expression expr.Expr }

func NewSaveToSafe() *SaveToSafe {
	return &SaveToSafe{expression: expr.NewSequenceExprWith(expr.Or(expr.NewInflectionOfBe(), expr.NewWord("it"))).TWs().TAco("save").TWs().TAco("to").TWs().ThenVerb()}
}
func (r *SaveToSafe) Expr() expr.Expr                  { return r.expression }
func (r *SaveToSafe) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *SaveToSafe) Description() string {
	return "Corrects `save to <verb>` to `safe to <verb>` after a form of `be`."
}
func (r *SaveToSafe) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 5 {
		return nil
	}
	span := toks[2].Span
	verb := strings.ToLower(toks[4].GetStr(src))
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune("safe"))}, Message: fmt.Sprintf("Did you mean `safe to %s`?", verb), Priority: 57}
}
