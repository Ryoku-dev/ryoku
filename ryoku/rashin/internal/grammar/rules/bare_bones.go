package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type BareBones struct{ expression expr.Expr }

func NewBareBones() *BareBones {
	return &BareBones{expression: expr.NewSequenceExprAco("bare").TWsH().TAco("bone")}
}
func (r *BareBones) Expr() expr.Expr                  { return r.expression }
func (r *BareBones) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*BareBones) Description() string                { return "Corrects `bare-bone` to `bare-bones`." }
func (r *BareBones) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 3 {
		return nil
	}
	span := toks[len(toks)-1].Span
	sep := " "
	if toks[1].Kind.IsHyphen() {
		sep = "-"
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("bones", span.GetContent(src))}
	l.Message = "If this is the idiom meaning `minimal`, it should be plural `bare" + sep + "bones`"
	return &l
}
