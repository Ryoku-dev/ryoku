package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type IfWouldve struct{ expression expr.Expr }

func NewIfWouldve() *IfWouldve {
	return &IfWouldve{expression: expr.NewSequenceExprAco("if").TWs().Then(expr.NewNominalPhrase()).TWs().ThenAnyOf([]expr.Expr{expr.NewSequenceExprWordSet([]string{"would", "had"}).TWs().ThenWordSet([]string{"have", "of"}), expr.AsExpr(expr.NewWordSet([]string{"would've", "wouldve", "had've", "hadve"}))}).TWs().ThenVerbPastParticipleForm()}
}
func (r *IfWouldve) Expr() expr.Expr                  { return r.expression }
func (r *IfWouldve) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *IfWouldve) Description() string {
	return "Corrects `if I would've done` etc. to `if I had done` etc."
}

// MatchToLint identifies and corrects incorrect conditional phrases like "would've",
// "would have", and "would of" to use the correct "had" construction.
func (r *IfWouldve) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	for i := len(toks) - 3; i >= 2; i -= 2 {
		prev, curr := toks[i-2].GetCh(src), toks[i].GetCh(src)
		start := -1
		if text.EndsWithIgnoreASCIICaseStr(curr, "ve") {
			if text.StartsWithAnyIgnoreASCIICaseStr(curr, []string{"would", "had"}) {
				start = i
			} else if text.StartsWithAnyIgnoreASCIICaseStr(prev, []string{"would", "had"}) {
				start = i - 2
			}
		} else if text.EndsWithIgnoreASCIICaseStr(curr, "of") && text.StartsWithAnyIgnoreASCIICaseStr(prev, []string{"would", "had"}) {
			start = i - 2
		}
		if start < 0 {
			continue
		}
		span, ok := doc.Span(toks[start : i+1])
		if !ok {
			return nil
		}
		result := lint.NewLint()
		result.Span = span
		result.LintKind = lint.Nonstandard
		result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("had", span.GetContent(src))}
		result.Message = "If this is counterfactual or hypothetical, use `had` after `if` rather than `would have` or `had have`."
		return &result
	}
	return nil
}
