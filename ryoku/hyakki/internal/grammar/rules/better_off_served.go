package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type BetterOffServed struct{ expression expr.Expr }

func NewBetterOffServed() *BetterOffServed {
	e := expr.NewSequenceExprWordSeq([]string{"better", "off", "served"}).TWs().ThenAnyWord()
	return &BetterOffServed{expression: e}
}
func (r *BetterOffServed) Expr() expr.Expr { return r.expression }
func (r *BetterOffServed) Description() string {
	return "Corrects `better off served` to `better off` or `better served`."
}
func (r *BetterOffServed) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *BetterOffServed) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *BetterOffServed) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	next := t[len(t)-1]
	ch := next.GetCh(s)
	keepOff := false
	if text.EqStr(ch, "by") {
	} else if next.Kind.IsVerbProgressiveForm() || text.EqStr(ch, "if") {
		keepOff = true
	} else if text.EqAnyIgnoreASCIICaseStr(ch, []string{"with", "as", "over", "in"}) {
	} else if text.EqAnyIgnoreASCIICaseStr(ch, []string{"not", "just"}) {
		keepOff = lint.FollowedByWord(c, func(t *doc.Token) bool { return t.Kind.IsVerbProgressiveForm() })
	} else {
		return nil
	}
	idx := 1
	if keepOff {
		idx = 3
	}
	span, _ := doc.Span(t[idx : idx+2])
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{lint.Remove()}
	l.Message = "Are you confusing `better off` and `better served`?"
	return &l
}
