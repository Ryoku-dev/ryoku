package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type Misspell struct{ expression expr.Expr }

func NewMisspell() *Misspell {
	return &Misspell{expression: expr.NewSequenceExprWordSet([]string{"miss"}).TWsH().ThenWordSet([]string{"spell", "spelled", "spelling", "spells", "spellings"})}
}
func (r *Misspell) Expr() expr.Expr { return r.expression }
func (r *Misspell) Description() string {
	return "Ensures `misspell` and its inflected forms are written as a single word."
}
func (r *Misspell) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Misspell) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	variant := strings.ToLower(toks[len(toks)-1].GetStr(src))
	var replacement string
	switch variant {
	case "spell", "spelled", "spelling", "spells", "spellings":
		replacement = "mis" + variant
	default:
		return nil
	}
	suggestions := []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, src[span.Start:span.End])}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.BoundaryError
	result.Priority = 63
	result.Message = "Write `misspell` and its inflections as a single word."
	result.Suggestions = suggestions
	return &result
}
