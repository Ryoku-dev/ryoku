package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type RedundantFirsts struct{ expression expr.Expr }

func NewRedundantFirsts() *RedundantFirsts {
	return &RedundantFirsts{expression: expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewSequenceExprAco("first").TWs().Then(expr.NewWordSet(redundantFirstsVerbs)), expr.NewSequenceExprWith(expr.NewWordSet(redundantFirstsVerbs)).ThenOptional(expr.NewSequenceExprWhitespace().ThenKindEither(doc.TokenKind.IsNoun, doc.TokenKind.IsOov)).TWs().ThenWordSeq([]string{"for", "the", "first", "time"})})}
}
func (r *RedundantFirsts) Expr() expr.Expr                  { return r.expression }
func (r *RedundantFirsts) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*RedundantFirsts) Description() string {
	return "Looks for redundant use of `first` with verbs that already imply order."
}
func (r *RedundantFirsts) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	verb, start, end := 0, 0, 0
	switch len(toks) {
	case 3:
		verb, start, end = 2, 0, 1
	case 9:
		verb, start, end = 0, 1, 8
	case 11:
		verb, start, end = 0, 3, 10
	default:
		return nil
	}
	span, ok := doc.Span(toks[start : end+1])
	if !ok {
		return nil
	}
	v := toks[verb].GetCh(src)
	if !text.EqAnyIgnoreASCIICaseStr(v, redundantFirstsVerbs) {
		return nil
	}
	message := "This is probably redundant as this verb already implies the first time."
	if text.EqAnyIgnoreASCIICaseStr(v, []string{"introduced", "released"}) {
		message = "Did this occur multiple times? If not, this wording may be redundant."
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Redundancy
	l.Suggestions = []lint.Suggestion{lint.Remove()}
	l.Message = message
	return &l
}

var redundantFirstsVerbs = []string{"coined", "discovered", "founded", "introduced", "invented", "originated", "released"}
