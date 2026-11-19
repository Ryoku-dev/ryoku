package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type JohnsHopkins struct{ expression expr.Expr }

func NewJohnsHopkins() *JohnsHopkins {
	return &JohnsHopkins{expression: expr.NewSequenceExprWith(func(tok doc.Token, src []rune) bool {
		return tok.Kind.IsProperNoun() && text.EqStr(tok.GetCh(src), "john")
	}).TWs().Then(func(tok doc.Token, src []rune) bool {
		return tok.Kind.IsProperNoun() && text.EqStr(tok.GetCh(src), "hopkins")
	})}
}
func (r *JohnsHopkins) Expr() expr.Expr                  { return r.expression }
func (r *JohnsHopkins) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *JohnsHopkins) Description() string              { return "Recommends the proper spelling `Johns Hopkins`." }
func (r *JohnsHopkins) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	tok := toks[0]
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.WordChoice
	l.Priority = 31
	l.Message = "Use `Johns Hopkins` for this name."
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("Johns", tok.GetCh(src))}
	return &l
}
