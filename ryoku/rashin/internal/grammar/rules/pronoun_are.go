package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"unicode"
)

type PronounAre struct{ expression expr.Expr }

func NewPronounAre() *PronounAre {
	return &PronounAre{expression: expr.NewSequenceExpr().ThenKindWhere(func(k doc.TokenKind) bool {
		return k.IsPronoun() && k.IsSubjectPronoun() && (k.IsSecondPersonPronoun() || k.IsFirstPersonPluralPronoun() || k.IsThirdPersonPluralPronoun())
	}).TWs().TAco("r")}
}
func (r *PronounAre) Expr() expr.Expr { return r.expression }
func (r *PronounAre) Description() string {
	return "Spots the letter `r` used in place of `are` or `you're` after plural first- or second-person pronouns."
}
func (r *PronounAre) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *PronounAre) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.WordChoice
	result.Priority = 40
	pronoun, gap, letter := toks[0].GetCh(src), toks[1].GetCh(src), toks[2].GetCh(src)
	allUpper := true
	for _, c := range pronoun {
		if unicode.IsLetter(c) && !unicode.IsUpper(c) {
			allUpper = false
		}
	}
	upper := allUpper
	for _, c := range letter {
		if unicode.IsUpper(c) {
			upper = true
		}
	}
	are, re := "are", "re"
	if upper {
		are, re = "ARE", "RE"
	}
	result.Suggestions = []lint.Suggestion{lint.ReplaceWith([]rune(string(pronoun) + string(gap) + are)), lint.ReplaceWith([]rune(string(pronoun) + "'" + re))}
	result.Message = "Use the full verb or the contraction after this pronoun."
	return &result
}
