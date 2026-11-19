package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type LittleToNo struct{ expression expr.Expr }

func NewLittleToNo() *LittleToNo {
	return &LittleToNo{expression: expr.NewSequenceExprAco("no").TWsH().TAco("to").TWsH().TAco("little")}
}
func (r *LittleToNo) Expr() expr.Expr { return r.expression }
func (r *LittleToNo) Description() string {
	return "Converts the clunky or unidiomatic phrase `no to little` into the standard English idiom `little to no`."
}
func (r *LittleToNo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *LittleToNo) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 5 {
		return nil
	}
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	var correction []rune
	for _, i := range []int{4, 1, 2, 3, 0} {
		correction = append(correction, toks[i].GetCh(src)...)
	}
	suggestions := []lint.Suggestion{lint.ReplaceWithMatchCase(correction, src[span.Start:span.End])}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Message = "The phrase `little to no` is more idiomatic than `no to little`."
	result.Suggestions = suggestions
	return &result
}
