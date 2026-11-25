package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type GoSoFarAsTo struct{ expression expr.Expr }

func NewGoSoFarAsTo() *GoSoFarAsTo {
	return &GoSoFarAsTo{expression: expr.NewSequenceExprWordSet([]string{"go", "goes", "going", "gone", "went"}).ThenFixedPhrase(" so far to ").ThenOptional(expr.NewSequenceExpr().ThenAdverb().TWs()).ThenAnyWord()}
}
func (r *GoSoFarAsTo) Expr() expr.Expr { return r.expression }
func (r *GoSoFarAsTo) Description() string {
	return "Flags 'go so far to' when it should be 'go so far as to' to express going beyond expectations"
}
func (r *GoSoFarAsTo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *GoSoFarAsTo) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Nonstandard

	n := len(toks)
	if n != 9 && n != 11 {
		return nil
	}
	if !toks[n-1].Kind.IsVerbLemma() || (n == 11 && !toks[n-3].Kind.IsAdverb()) {
		return nil
	}
	result.Span, _ = doc.Span(toks[:7])
	prefix, _ := doc.GetStr(toks[:5], src)
	replacement := prefix + " as " + toks[6].GetStr(src)
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, src[result.Span.Start:result.Span.End])}
	result.Message = "If this is intended to express going beyond what's expected, the standard idiom is `go so far as to`"
	return &result
}
