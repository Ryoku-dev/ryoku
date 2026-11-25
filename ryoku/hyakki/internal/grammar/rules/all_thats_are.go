package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type AllThatsAre struct{ expression expr.Expr }

func NewAllThatsAre() *AllThatsAre {
	return &AllThatsAre{expression: expr.NewSequenceExprAco("all").TWs().ThenAnyOf([]expr.Expr{expr.NewWord("that's"), expr.NewSequenceExprWordSeq([]string{"that", "is"})}).TWs().ThenAdjective().TWs().TAco("are")}
}
func (r *AllThatsAre) Expr() expr.Expr                  { return r.expression }
func (r *AllThatsAre) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AllThatsAre) Description() string {
	return "Looks for agreement errors after `all that is` or `all that's`."
}
func (r *AllThatsAre) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) != 7 && len(toks) != 9 {
		return nil
	}
	span := toks[len(toks)-1].Span
	replacement := append([]rune(nil), span.GetContent(src)...)
	if len(replacement) != 3 {
		return nil
	}
	replacement[0] += 8
	replacement[1]++
	replacement = replacement[:2]
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Agreement
	result.Suggestions = []lint.Suggestion{lint.ReplaceWith(replacement)}
	result.Message = "`Are` must be singular to agree with `" + toks[len(toks)-5].GetStr(src) + "`."
	return &result
}
