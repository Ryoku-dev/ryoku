package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type BrandBrandish struct{ expression expr.Expr }

func NewBrandBrandish() *BrandBrandish {
	return &BrandBrandish{expression: expr.NewSequenceExprWordSet([]string{"brandish", "brandished", "brandishes", "brandishing"}).TWs().ThenKindExcept(doc.TokenKind.IsObjectPronoun, []string{"her", "it", "them"})}
}
func (r *BrandBrandish) Expr() expr.Expr                  { return r.expression }
func (r *BrandBrandish) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*BrandBrandish) Description() string {
	return "Looks for `brandish` wrongly used when `brand` is intended."
}
func (r *BrandBrandish) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span := t[0].Span
	chars := t[0].GetCh(s)
	var replacement string
	switch chars[len(chars)-1] {
	case 'h', 'H':
		replacement = "brand"
	case 'd', 'D':
		replacement = "branded"
	case 's', 'S':
		replacement = "brands"
	case 'g', 'G':
		replacement = "branding"
	default:
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Malapropism
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, chars)}
	l.Message = "`Brandish` means to wield a weapon. You probably mean `brand`."
	return &l
}
