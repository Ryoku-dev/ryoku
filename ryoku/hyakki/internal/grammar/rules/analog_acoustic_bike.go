package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type AnalogAcousticBike struct {
	expression expr.Expr
	dialect    dict.Dialect
}

func NewAnalogAcousticBike(dialect dict.Dialect) *AnalogAcousticBike {
	e := expr.NewSequenceExprOptional(expr.NewSequenceExprWith(expr.NewIndefiniteArticle()).TWs()).TSet([]string{"acoustic", "analog", "analogue"}).TWs().TSet([]string{"bike", "bikes", "bicycle", "bicycles"})
	return &AnalogAcousticBike{expression: e, dialect: dialect}
}
func (r *AnalogAcousticBike) Expr() expr.Expr { return r.expression }
func (r *AnalogAcousticBike) Description() string {
	return "Suggests more standard terms for `analog/analogue bike` and `acoustic bike`."
}
func (r *AnalogAcousticBike) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AnalogAcousticBike) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	t = t[len(t)-3:]
	span, _ := doc.Span(t)
	wsCycle, _ := doc.GetStr(t[1:3], s)
	cycle := t[2].GetStr(s)
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.WordChoice
	if r.dialect == dict.British || r.dialect == dict.Australian {
		ending := wsCycle
		if strings.ContainsRune(cycle, 'k') {
			ending = cycle
		}
		l.Suggestions = append(l.Suggestions, lint.ReplaceWithMatchCaseStr("push"+ending, span.GetContent(s)))
	}
	l.Suggestions = append(l.Suggestions, lint.ReplaceWithMatchCaseStr("pedal"+wsCycle, span.GetContent(s)))
	l.Message = "Consider a more standard term."
	return &l
}
