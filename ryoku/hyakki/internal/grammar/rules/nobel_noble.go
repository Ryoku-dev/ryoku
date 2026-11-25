package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type NobelNoble struct{ expression expr.Expr }

func NewNobelNoble() *NobelNoble {
	return &NobelNoble{expression: expr.NewFirstMatchOf([]expr.Expr{expr.ButNot(expr.NewSequenceExprWordSet([]string{"nobel", "noble"}).TWs().TSet([]string{"peace", "piece"}).TWs().TSet([]string{"price", "prize", "prise", "prices", "prizes", "prises"}), expr.NewSequenceExprWordSeq([]string{"nobel", "peace"}).TWs().TSet([]string{"prize", "prizes"})), expr.NewSequenceExprAco("noble").TWs().TSet([]string{"foundation", "laureate", "laureates", "prize", "prizes"})})}
}
func (r *NobelNoble) Expr() expr.Expr                  { return r.expression }
func (r *NobelNoble) Description() string              { return "Corrects misspellings of `Nobel` Peace Prize." }
func (r *NobelNoble) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *NobelNoble) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	replacement := "Nobel"
	switch len(toks) {
	case 5:
		replacement = "Nobel Peace Prize"
		content := src[span.Start:span.End]
		if len(content) > 0 && (content[len(content)-1] == 's' || content[len(content)-1] == 'S') {
			replacement += "s"
		}
	case 3:
		span = toks[0].Span
	default:
		return nil
	}
	suggestions := []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, src[span.Start:span.End])}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Spelling
	result.Message = "Corrects `noble` to `Nobel` when referring to the Nobel Peace Prize"
	result.Suggestions = suggestions
	return &result
}
