package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type HyphenateNumberDay struct{ expression expr.Expr }

func NewHyphenateNumberDay() *HyphenateNumberDay {
	return &HyphenateNumberDay{expression: expr.NewSequenceExprNumber().ThenWhitespace().TAco("day").ThenLongestOf([]expr.Expr{expr.NewSequenceExprWhitespace().Then(expr.NewNominalPhrase()), expr.NewSequenceExpr().ThenHyphen().ThenAdjective().ThenWhitespace().Then(expr.NewNominalPhrase())})}
}
func (r *HyphenateNumberDay) Expr() expr.Expr                  { return r.expression }
func (r *HyphenateNumberDay) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*HyphenateNumberDay) Description() string {
	return "Ensures a hyphen is used in `X-day` when it is part of a compound adjective, such as `4-day work week`."
}
func (r *HyphenateNumberDay) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 2 {
		return nil
	}
	number, ok := toks[0].Kind.AsNumber()
	if !ok {
		return nil
	}
	return &lint.Lint{Span: toks[1].Span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune("-"))}, Message: "Use a hyphen in `" + number.String() + "-day` when forming an adjectival compound.", Priority: 31}
}
