package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type CallItQuits struct{ expression expr.Expr }

func NewCallItQuits() *CallItQuits {
	return &CallItQuits{expression: expr.NewSequenceExprWordSet([]string{"call", "calls", "called", "calling"}).TWs().TAco("it").TWs().ThenExactWord("quit")}
}
func (r *CallItQuits) Expr() expr.Expr                  { return r.expression }
func (r *CallItQuits) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *CallItQuits) Description() string {
	return "Corrects wrong variants of the idiom 'call it quits'."
}
func (r *CallItQuits) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *CallItQuits) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if lint.FollowedByToken(ctx, func(t *doc.Token) bool { return t.Kind.IsOpenRound() }) {
		return nil
	}
	if len(toks) == 0 {
		return nil
	}
	l := lint.NewLint()
	l.Span = toks[len(toks)-1].Span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{lint.ReplaceWith([]rune("quits"))}
	l.Message = "This idiom uses the plural 'quits' to mean 'gives up' or 'stops'."
	return &l
}
