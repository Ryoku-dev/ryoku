package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ReasonForDoing struct{ expression expr.Expr }

func NewReasonForDoing() *ReasonForDoing {
	return &ReasonForDoing{expression: expr.NewSequenceExprWordSet([]string{"reason", "reasons"}).TWs().TAco("of").TWs().ThenVerbProgressiveForm()}
}
func (r *ReasonForDoing) Expr() expr.Expr                  { return r.expression }
func (r *ReasonForDoing) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ReasonForDoing) Description() string {
	return "Corrects `reason of doing` to `reason for doing` etc."
}
func (r *ReasonForDoing) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if len(toks) != 5 {
		return nil
	}
	ch := toks[0].GetCh(src)
	if len(ch) == 0 {
		return nil
	}
	if ch[len(ch)-1] == 's' && lint.PrecededByWord(ctx, func(t *doc.Token) bool { return text.EqStr(t.GetCh(src), "for") }) {
		return nil
	}
	return &lint.Lint{Span: toks[2].Span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("for", toks[2].GetCh(src))}, Message: "Use 'for' instead of 'of' with 'reason' and progressive verbs.", Priority: 127}
}
func (r *ReasonForDoing) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
