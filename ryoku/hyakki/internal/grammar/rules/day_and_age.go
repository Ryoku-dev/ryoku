package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type DayAndAge struct{ expression expr.Expr }

func NewDayAndAge() *DayAndAge {
	return &DayAndAge{expression: expr.NewSequenceExprWordSet([]string{"this", "these"}).TWs().ThenWordSet([]string{"day", "days"}).TWs().ThenWordSet([]string{"and", "in", "an", "on", "of"}).TWs().ThenWordSet([]string{"age", "ages"})}
}
func (r *DayAndAge) Expr() expr.Expr                  { return r.expression }
func (r *DayAndAge) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *DayAndAge) Description() string {
	return "Fixes wrong variants of the idiom `in this day and age`."
}
func (r *DayAndAge) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *DayAndAge) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if ctx == nil {
		return nil
	}
	before := ctx.Before
	var prep *doc.Token
	if len(before) >= 2 {
		a, b := &before[len(before)-2], &before[len(before)-1]
		if b.Kind.IsWhitespace() && (a.Kind.IsPreposition() || text.EqAnyIgnoreASCIICaseStr(a.GetCh(src), []string{"is", "it"})) {
			prep = a
		}
	}
	mainSpan, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	good := []string{"this", "day", "and", "age"}
	goodMain := true
	for i, w := range good {
		if !text.EqStr(toks[i*2].GetCh(src), w) {
			goodMain = false
		}
	}
	span := mainSpan
	replacement := "in this day and age"
	if prep != nil && !text.EqStr(prep.GetCh(src), "since") {
		if goodMain {
			if text.EqAnyIgnoreASCIICaseStr(prep.GetCh(src), []string{"in", "for"}) {
				return nil
			}
			span = prep.Span
			replacement = "in"
		} else {
			span = text.Span{Start: prep.Span.Start, End: mainSpan.End}
		}
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, span.GetContent(src))}
	result.Message = "The correct idiom is `in this day and age`."
	return &result
}
