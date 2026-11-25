package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type LessWorse struct{ expression expr.Expr }

func NewLessWorse() *LessWorse {
	return &LessWorse{expression: expr.NewSequenceExprWordSet([]string{"less", "least"}).TWsH().ThenWordSet([]string{"worse", "worst"})}
}
func (r *LessWorse) Expr() expr.Expr                  { return r.expression }
func (r *LessWorse) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*LessWorse) Description() string {
	return "Suggests alternatives to `less/least worse/worst` for more standard, clearer comparisons."
}
func (r *LessWorse) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) != 3 {
		return nil
	}
	span, _ := doc.Span(toks)
	little, bad := string(text.ToLower(toks[0].GetCh(src))), string(text.ToLower(toks[2].GetCh(src)))
	var values []string
	var message string
	switch {
	case little == "least" && bad == "worst" && !toks[1].Kind.IsHyphen():
		values = []string{"least bad"}
		message = "Though `least worst` is a common idiom, `least bad` is the standard way to compare bad options."
	case little == "least" && bad == "worst" && toks[1].Kind.IsHyphen():
		values = []string{"least worst", "least bad"}
		message = "`Least worst` (without the hyphen) is a common idiom, but `least bad` is the standard way to compare bad options."
	case little == "less" && bad == "worse":
		values = []string{"less bad"}
		message = "The standard way to compare bad options is `less bad`."
	case little == "less" && bad == "worst":
		values = []string{"less bad", "least bad"}
		message = "These words conflict with each other. Choose `less bad` or `least bad`."
	case little == "least" && bad == "worse":
		values = []string{"least worst", "least bad", "less bad"}
		message = "These words conflict with each other. Choose `less bad` or `least bad` for more standard English, or `least worst` for more idiomatic English."
	default:
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: lint.ToReplaceSuggestions(values, span.GetContent(src)), Message: message, Priority: 126}
}
