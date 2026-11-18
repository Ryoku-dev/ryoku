package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type OutOfDate struct{ expression expr.Expr }

func NewOutOfDate() *OutOfDate {
	return &OutOfDate{expression: expr.NewFirstMatchOf([]expr.Expr{expr.NewFixedPhraseFromPhrase("out of date"), expr.NewFixedPhraseFromPhrase("out-of date"), expr.NewFixedPhraseFromPhrase("out of-date")})}
}
func (r *OutOfDate) Expr() expr.Expr                  { return r.expression }
func (r *OutOfDate) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*OutOfDate) Description() string {
	return "Ensures that the phrase `out of date` is written with a hyphen as `out-of-date` when used as a compound adjective."
}
func (r *OutOfDate) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("out-of-date", span.GetContent(src))}, Message: "Did you mean the compound adjective?", Priority: 31}
}
