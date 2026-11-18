package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ForSameReason struct{ expression expr.Expr }

func NewForSameReason() *ForSameReason {
	return &ForSameReason{expression: expr.NewSequenceExprAco("for").TWs().ThenWordSet([]string{"financial", "following", "obvious", "other", "same", "security", "similar", "special", "such", "various"}).TWs().TAco("reason")}
}
func (r *ForSameReason) Expr() expr.Expr                  { return r.expression }
func (r *ForSameReason) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ForSameReason) Description() string {
	return "A linter for detecting incorrect use of `reason` vs `reasons` in certain contexts."
}
func (r *ForSameReason) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 5 {
		return nil
	}
	tok := toks[4]
	replacement := "reasons"
	message := "In this context, use the plural `reasons`."
	mod := toks[2].GetCh(src)
	if text.EqStr(mod, "following") || text.EqStr(mod, "same") {
		tok = toks[0]
		replacement = "for the"
		message = "In this context, insert `the` before `reason`."
	}
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Grammar
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, tok.GetCh(src))}
	l.Message = message
	return &l
}
