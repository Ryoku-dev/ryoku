package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type LittleKnown struct{ expression expr.Expr }

func NewLittleKnown() *LittleKnown {
	return &LittleKnown{expression: expr.NewSequenceExprWordSet([]string{"it's", "its"}).TWs().ThenOptional(expr.NewSequenceExprAco("a").TWs()).TAco("little").TWsH().TAco("known").TWs().TSet([]string{"fact", "that"})}
}
func (r *LittleKnown) Expr() expr.Expr                  { return r.expression }
func (r *LittleKnown) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *LittleKnown) Description() string              { return "Fixes nonstandard uses of `little known`." }
func (r *LittleKnown) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	last := toks[len(toks)-1].GetCh(src)
	if len(last) == 0 {
		return nil
	}
	var span text.Span
	var suggestion lint.Suggestion
	var action string
	switch {
	case len(toks) == 7 && (last[0] == 'f' || last[0] == 'F'):
		span = toks[0].Span
		suggestion = lint.InsertAfter([]rune(" a"))
		action = "Insert"
	case len(toks) == 9 && (last[0] == 't' || last[0] == 'T'):
		span, _ = doc.Span(toks[1:3])
		suggestion = lint.Remove()
		action = "Remove"
	default:
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{suggestion}, Message: action + " the indefinnite article `a`.", Priority: 127}
}
