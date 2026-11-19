package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ForNoun struct{ expression expr.Expr }

func NewForNoun() *ForNoun {
	return &ForNoun{expression: expr.NewSequenceExprAco("fro").ThenWhitespace().Then(expr.Or(expr.NewNominalPhrase(), expr.NewWord("sure")))}
}
func (r *ForNoun) Expr() expr.Expr { return r.expression }
func (r *ForNoun) Description() string {
	return "Corrects the archaic or mistaken `fro` to `for` when followed by a noun."
}
func (r *ForNoun) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ForNoun) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.WordChoice
	result.Priority = 31
	result.Span = toks[0].Span
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("for", toks[0].GetCh(src))}
	result.Message = "`For` is more common in this context."
	return &result
}
