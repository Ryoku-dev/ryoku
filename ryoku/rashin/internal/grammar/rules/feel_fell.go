package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type FeelFell struct{ expression expr.Expr }

func NewFeelFell() *FeelFell {
	return &FeelFell{expression: expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewSequenceExprWordSet([]string{"didn't", "doesn't"}).TWs().TAco("fell"), expr.NewSequenceExprAco("fell").TWs().TSet([]string{"comfortable", "free", "good", "I", "I'm", "it", "it's", "like", "that", "we", "you"})})}
}
func (r *FeelFell) Expr() expr.Expr                  { return r.expression }
func (r *FeelFell) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*FeelFell) Description() string {
	return "Corrects some expressions using `fell` where `feel` is correct."
}
func (r *FeelFell) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	tok := lint.FindTheOnlyTokenMatching(toks, src, func(tok *doc.Token, src []rune) bool { return text.EqStr(tok.GetCh(src), "fell") })
	if tok == nil {
		return nil
	}
	if len(toks) == 0 {
		return nil
	}
	span := tok.Span
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Typo
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("feel", span.GetContent(src))}
	result.Message = "It looks like this is a typo, did you mean `feel`?"
	result.Priority = 127
	return &result
}
