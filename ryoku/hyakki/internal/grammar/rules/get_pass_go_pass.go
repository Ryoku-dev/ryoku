package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type GetPassGoPass struct{ expression expr.Expr }

func NewGetPassGoPass() *GetPassGoPass {
	return &GetPassGoPass{expression: expr.NewSequenceExprWordSet([]string{"get", "gets", "getting", "got", "gotten", "go", "goes", "going", "gone", "went"}).TWs().TAco("pass")}
}
func (r *GetPassGoPass) Expr() expr.Expr { return r.expression }
func (r *GetPassGoPass) Description() string {
	return "Corrects `pass` to `past` after `get` and `go`."
}
func (r *GetPassGoPass) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *GetPassGoPass) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[len(toks)-1].Span
	suggestions := []lint.Suggestion{lint.ReplaceWithMatchCaseStr("past", src[span.Start:span.End])}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Grammar
	result.Message = "Use the preposition `past` here and not the verb `pass`."
	result.Suggestions = suggestions
	return &result
}
