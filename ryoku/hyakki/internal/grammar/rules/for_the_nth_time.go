package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ForTheNthTime struct{ expression expr.Expr }

func NewForTheNthTime() *ForTheNthTime {
	return &ForTheNthTime{expression: expr.NewSequenceExprAco("for").TWs().TSet([]string{"first", "second", "third", "fourth", "fifth", "sixth", "seventh", "eighth", "ninth", "tenth", "eleventh", "twelfth", "thirteenth", "fourteenth", "fifteenth", "sixteenth", "seventeenth", "eighteenth", "nineteenth", "twentieth", "thirtieth", "fortieth", "fiftieth", "sixtieth", "seventieth", "eightieth", "ninetieth", "hundredth", "thousandth", "millionth", "billionth", "first", "nth", "umpteenth", "zeroth", "zeroeth"}).TWs().TAco("time").ThenAnyOf([]expr.Expr{expr.NewAnchorEnd(), expr.NewSequenceExprWhitespace().TSet([]string{"and", "but"})})}
}
func (r *ForTheNthTime) Expr() expr.Expr                  { return r.expression }
func (r *ForTheNthTime) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ForTheNthTime) Description() string {
	return "Corrects missing `the` for occasions like `on third time` -> `on the third time`."
}
func (r *ForTheNthTime) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	result := lint.NewLint()
	result.Span = t[0].Span
	result.LintKind = lint.Usage
	result.Suggestions = []lint.Suggestion{lint.InsertAfter([]rune(" the"))}
	result.Message = "The standard form of this construction requires the word 'the' to precede the ordinal."
	return &result
}
