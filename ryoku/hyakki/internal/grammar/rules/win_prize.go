package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type WinPrize struct{ expression expr.Expr }

func NewWinPrize() *WinPrize {
	return &WinPrize{expression: expr.OrLongest(expr.NewSequenceExprWith(expr.NewWordSet([]string{"win", "wins", "won", "winning"})).ThenWhitespace().ThenDeterminer().ThenWhitespace().Then(expr.NewWordSet([]string{"price", "prices", "prise", "prises"})), expr.NewSequenceExprWith(expr.NewWordSet([]string{"win", "wins", "won", "winning"})).ThenWhitespace().Then(expr.NewWordSet([]string{"price", "prices", "prise", "prises"})))}
}
func (r *WinPrize) Expr() expr.Expr                  { return r.expression }
func (r *WinPrize) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*WinPrize) Description() string {
	return "Catches the mix-up between `price`/`prise` and `prize` after the verb `win`."
}
func (r *WinPrize) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	candidate := toks[len(toks)-1]
	repl := ""
	switch strings.ToLower(candidate.GetStr(src)) {
	case "price", "prise":
		repl = "prize"
	case "prices", "prises":
		repl = "prizes"
	default:
		return nil
	}
	return &lint.Lint{Span: candidate.Span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune(repl))}, Message: "Perhaps you meant `" + repl + "`, the word for an award.", Priority: 50}
}
