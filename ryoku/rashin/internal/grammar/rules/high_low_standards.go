package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type HighLowStandards struct{ expression expr.Expr }

func NewHighLowStandards() *HighLowStandards {
	return &HighLowStandards{expression: expr.NewSequenceExprWordSet([]string{"got", "had", "has", "have", "having"}).ThenOptional(expr.NewSequenceExprWhitespace().ThenDegreeAdverb()).TWs().TSet([]string{"high", "low"}).TWs().TAco("standard").ThenAnyOf([]expr.Expr{expr.NewAnchorEnd(), expr.NewSequenceExprWhitespace().Then(func(t doc.Token, s []rune) bool {
		return (t.Kind.IsPreposition() && !text.EqStr(t.GetCh(s), "of")) || text.EqAnyIgnoreASCIICaseStr(t.GetCh(s), []string{"and", "even", "when"})
	})})}
}
func (r *HighLowStandards) Expr() expr.Expr                  { return r.expression }
func (r *HighLowStandards) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *HighLowStandards) Description() string {
	return "Corrects `high/low standard` to `high/low standards` when `standard` is a noun."
}
func (r *HighLowStandards) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	tok := lint.FindTheOnlyTokenMatching(toks, src, func(t *doc.Token, s []rune) bool { return text.EqStr(t.GetCh(s), "standard") })
	if tok == nil {
		return nil
	}
	span := tok.Span
	return &lint.Lint{Span: span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("standards", span.GetContent(src))}, Message: "This idiom uses plural `standards`.", Priority: 127}
}
