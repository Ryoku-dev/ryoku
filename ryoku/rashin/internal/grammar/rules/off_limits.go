package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type OffLimits struct{ expression expr.Expr }

func NewOffLimits() *OffLimits {
	return &OffLimits{expression: expr.NewSequenceExprAco("off").TWsH().TAco("limit")}
}
func (r *OffLimits) Expr() expr.Expr                  { return r.expression }
func (r *OffLimits) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *OffLimits) Description() string              { return "Corrects `off-limit` to `off-limits`." }
func (r *OffLimits) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *OffLimits) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if lint.PrecededByWord(ctx, func(t *doc.Token) bool {
		return text.EqAnyIgnoreASCIICaseStr(t.GetCh(src), []string{"back", "backed", "backing", "backs", "switch", "switched", "switching", "switches", "turn", "turned", "turning", "turns"})
	}) {
		return nil
	}
	if len(toks) == 0 {
		return nil
	}
	tok := toks[len(toks)-1]
	ch := tok.GetCh(src)
	replacement := append([]rune(nil), ch...)
	replacement = append(replacement, rune(uint8(ch[len(ch)-1])-1))
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Usage
	l.Message = "This idiom always uses the plural form."
	l.Suggestions = []lint.Suggestion{lint.ReplaceWith(replacement)}
	return &l
}
