package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type Ajar struct{ expression expr.Expr }

func NewAjar() *Ajar {
	return &Ajar{expression: expr.ButNot(expr.NewSequenceExprWordSet([]string{"door", "doors", "gate", "gates", "mouth", "slightly"}).TWsH().TAco("a").TWsH().TAco("jar"), expr.NewSequenceExprAnything().TAny().TAny().TAny().TAny().TWs().TSet([]string{"containing", "contains", "full", "of"}))}
}
func (r *Ajar) Expr() expr.Expr                  { return r.expression }
func (r *Ajar) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Ajar) Description() string              { return "Corrects `a jar` to `ajar`." }
func (r *Ajar) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 5 {
		return nil
	}
	span, _ := doc.Span(toks[2:5])
	fix := append([]rune(nil), toks[2].GetCh(src)...)
	fix = append(fix, toks[4].GetCh(src)...)
	return &lint.Lint{Span: span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWith(fix)}, Message: "Did you mean `ajar` (slightly open)?", Priority: 127}
}
