package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type WasAloud struct{ expression expr.Expr }

func NewWasAloud() *WasAloud {
	return &WasAloud{expression: expr.NewSequenceExprWordSet([]string{"was", "were", "be", "been"}).TWs().ThenExactWord("aloud")}
}
func (r *WasAloud) Expr() expr.Expr                  { return r.expression }
func (r *WasAloud) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *WasAloud) Description() string {
	return "Ensures `was aloud` and `were aloud` are corrected to `was allowed` or `were allowed` when referring to permission."
}
func (r *WasAloud) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span, _ := doc.Span(toks)
	verb := toks[0].GetStr(src)
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(verb+" allowed", toks[0].GetCh(src))}, Message: fmt.Sprintf("Did you mean `%s allowed`?", verb), Priority: 31}
}
