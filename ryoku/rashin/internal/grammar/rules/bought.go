package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type Bought struct{ expression expr.Expr }

func NewBought() *Bought {
	return &Bought{expression: expr.NewSequenceExprWith(boughtIsSubjectPronounLike).TWs().ThenOptional(expr.NewSequenceExpr().ThenAdverb().TWs()).ThenOptional(expr.NewSequenceExpr().ThenAuxiliaryVerb().TWs()).ThenOptional(expr.NewSequenceExpr().ThenAdverb().TWs()).ThenAnyCapitalizationOf("bough")}
}
func (r *Bought) Expr() expr.Expr                  { return r.expression }
func (r *Bought) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Bought) Description() string {
	return "Replaces the incorrect past-tense spelling `bough` with `bought` after subject pronouns."
}
func (r *Bought) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	tok := t[len(t)-1]
	return &lint.Lint{Span: tok.Span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("bought", tok.GetCh(s))}, Message: "Prefer the past-tense form `bought` here.", Priority: 31}
}
func boughtIsSubjectPronounLike(t doc.Token, s []rune) bool {
	if t.Kind.IsSubjectPronoun() {
		return true
	}
	if !t.Kind.IsWord() || !t.Kind.IsApostrophized() {
		return false
	}
	stem, suffix, ok := strings.Cut(strings.ToLower(t.GetStr(s)), "'")
	if !ok {
		return false
	}
	switch stem {
	case "i", "you", "we", "they", "he", "she", "it":
		return suffix == "d" || suffix == "ve"
	}
	return false
}
