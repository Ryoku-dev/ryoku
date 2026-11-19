package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type ShouldContract struct{ expression expr.Expr }

func NewShouldContract() *ShouldContract {
	cap := expr.NewSequenceExprWordSet([]string{"your", "were"}).TWs().ThenNonQuantifierDeterminer().TWs().Then(expr.Or(expr.NewSequenceExpr().ThenAdjective(), expr.NewSequenceExprWordSet([]string{"man", "boss"})))
	start := expr.NewSequenceExprWith(expr.NewAnchorStart()).Then(cap)
	mid := expr.NewSequenceExprUnless(expr.NewWordSet([]string{"what"})).TWs().Then(cap)
	return &ShouldContract{expression: expr.Or(start, mid)}
}
func (r *ShouldContract) Expr() expr.Expr { return r.expression }
func (r *ShouldContract) Description() string {
	return "Neglecting the apostrophe when contracting pronouns with \"are\" (like \"your\" and \"you are\") is a fatal, but extremely common mistake to make."
}
func (r *ShouldContract) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ShouldContract) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	var target *doc.Token
	var replacements []string
	for i := 0; i < 2 && i < len(tokens); i++ {
		switch strings.ToLower(tokens[i].GetStr(source)) {
		case "your":
			target = &tokens[i]
			replacements = []string{"you're", "you are"}
		case "were":
			target = &tokens[i]
			replacements = []string{"we're", "we are"}
		}
	}
	if target == nil {
		return nil
	}
	result := lint.NewLint()
	result.Span = target.Span
	result.LintKind = lint.WordChoice
	result.Priority = 31
	result.Suggestions = lint.ToReplaceSuggestions(replacements, target.GetCh(source))
	result.Message = "Use the contraction or separate the words instead."
	return &result
}
