package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type CureFor struct{ expression expr.Expr }

func NewCureFor() *CureFor {
	return &CureFor{expression: expr.NewSequenceExprWith(expr.NewDerivedFromStr("cure")).TWs().TAco("against")}
}
func (r *CureFor) Expr() expr.Expr { return r.expression }
func (r *CureFor) Description() string {
	return "Flags `cure against` and prefers the standard `cure for` pairing."
}
func (r *CureFor) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *CureFor) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Priority = 31
	result.Span = toks[len(toks)-1].Span
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("for", toks[len(toks)-1].GetCh(src))}
	result.Message = "Prefer `cure for` when describing a treatment target."
	return &result
}
