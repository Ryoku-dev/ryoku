package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ExpandPeople struct{ expression expr.Expr }

func NewExpandPeople() *ExpandPeople {
	return &ExpandPeople{expression: expr.AsExpr(expr.NewWordExact("ppl"))}
}
func (r *ExpandPeople) Expr() expr.Expr                  { return r.expression }
func (r *ExpandPeople) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*ExpandPeople) Description() string {
	return "Expands the abbreviation `ppl` to the full word `people` for clarity."
}
func (r *ExpandPeople) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := text.Span{Start: toks[0].Span.Start, End: toks[len(toks)-1].Span.End}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Style
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("people", span.GetContent(src))}
	result.Message = "Use `people` instead of `ppl`."
	result.Priority = 127
	return &result
}
