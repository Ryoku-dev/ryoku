package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type SuchShame struct{ expression expr.Expr }

func NewSuchShame() *SuchShame {
	return &SuchShame{expression: expr.NewSequenceExprWordSet([]string{"is", "it's", "its", "that's", "thats", "was"}).TWs().ThenWordSeq([]string{"such", "shame"})}
}
func (r *SuchShame) Expr() expr.Expr                  { return r.expression }
func (r *SuchShame) Description() string              { return "Corrects `such shame` to `such a shame`." }
func (r *SuchShame) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *SuchShame) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *SuchShame) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage

	first := strings.ToLower(toks[0].GetStr(src))
	if (first == "is" || first == "was") && lint.PrecededByWord(ctx, func(t *doc.Token) bool { w := strings.ToLower(t.GetStr(src)); return w == "there" || w == "their" }) {
		return nil
	}
	c := toks[2].GetCh(src)
	if len(c) < 3 {
		return nil
	}
	replacement := append([]rune{rune(byte(c[2]) - 2)}, toks[3].GetCh(src)...)
	result.Span = toks[3].Span
	result.Suggestions = []lint.Suggestion{lint.InsertAfter(replacement)}
	result.Message = "This expression requires `a shame`."
	return &result
}
