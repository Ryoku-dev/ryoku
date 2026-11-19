package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ExpandTimeShorthands struct{ expression expr.Expr }

func NewExpandTimeShorthands() *ExpandTimeShorthands {
	words := expr.NewWordSet([]string{"hr", "hrs", "min", "mins", "sec", "secs", "ms", "msec", "msecs"})
	return &ExpandTimeShorthands{expression: expr.NewSequenceExprWith(expr.NewImpliesQuantity()).ThenLongestOf([]expr.Expr{expr.NewSequenceExprWith(words), expr.NewSequenceExpr().TWsH().Then(words)})}
}
func (r *ExpandTimeShorthands) Expr() expr.Expr                  { return r.expression }
func (r *ExpandTimeShorthands) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ExpandTimeShorthands) Description() string {
	return "Expands time-related abbreviations (`hr`, `hrs`, `min`, `mins`, `sec`, `secs`, `ms`, `msec`, `msecs`) to their full forms (`hour`, `hours`, `minute`, `minutes`, `second`, `seconds`, `millisecond`, `milliseconds`)."
}
func (r *ExpandTimeShorthands) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	tok := toks[len(toks)-1]
	plural, known := expr.ImpliesPlurality(toks[0], src)
	abbr := tok.GetStr(src)
	if !known {
		plural = abbr == "hrs" || abbr == "mins" || abbr == "secs" || abbr == "msecs"
	}
	var replacement string
	switch abbr {
	case "hr", "hrs":
		replacement = "hour"
	case "min", "mins":
		replacement = "minute"
	case "sec", "secs":
		replacement = "second"
	case "ms", "msec", "msecs":
		replacement = "millisecond"
	default:
		return nil
	}
	if plural {
		replacement += "s"
	}
	chars := replacement
	if len(toks) == 2 {
		chars = " " + chars
	}
	if chars == abbr {
		return nil
	}
	return &lint.Lint{Span: tok.Span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune(chars))}, Message: "Did you mean `" + replacement + "`?", Priority: 31}
}
