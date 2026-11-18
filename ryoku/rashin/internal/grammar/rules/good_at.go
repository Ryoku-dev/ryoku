package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type GoodAt struct{ expression expr.Expr }

func NewGoodAt() *GoodAt {
	a := expr.NewSequenceExprAnyOf([]expr.Expr{expr.AsExpr(expr.NewInflectionOfBe()), expr.AsExpr(expr.NewWordSet([]string{"I'm", "we're", "you're", "he's", "she's", "it's", "they're", "Im", "were", "youre", "your", "hes", "shes", "its", "theyre"}))}).TWs().ThenOptional(expr.NewSequenceExprAco("not").TWs()).ThenOptional(expr.NewSequenceExpr().ThenFrequencyAdverb().TWs()).ThenOptional(expr.NewSequenceExpr().ThenDegreeAdverb().TWs()).ThenWordSet([]string{"good", "bad", "great", "okay", "OK"}).TWs().TAco("in").TWs().ThenAnyWord()
	b := expr.NewSequenceExprWordSet([]string{"good", "bad", "great", "okay", "OK"}).TWs().TAco("in").TWs().ThenWordSet([]string{"biology", "chemistry", "math", "mathematics", "physics", "programming", "coding", "c", "debugging", "go", "java", "javascript", "laravel", "python", "ruby", "english", "chinese", "french", "german", "japanese", "spanish"})
	return &GoodAt{expression: expr.NewFirstMatchOf([]expr.Expr{a, b})}
}
func (r *GoodAt) Expr() expr.Expr { return r.expression }
func (r *GoodAt) Description() string {
	return "Checks for `good in` used instead of `good at` to describe proficiency with a skill."
}
func (r *GoodAt) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *GoodAt) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	tok, ok := doc.GetRel(t, -3)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: tok.Span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("at", tok.GetCh(s))}, Message: "Use 'good at' to describe proficiency with a skill.", Priority: 127}
}
