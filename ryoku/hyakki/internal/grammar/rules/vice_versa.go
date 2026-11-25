package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"unicode"
)

type ViceVersa struct{ expression expr.Expr }

func NewViceVersa() *ViceVersa {
	return &ViceVersa{expression: expr.NewSequenceExprWordSet([]string{"vice", "vise"}).ThenHyphen().ThenOptional(expr.NewSequenceExprAco("a").ThenHyphen()).TAco("versa")}
}
func (r *ViceVersa) Expr() expr.Expr { return r.expression }
func (r *ViceVersa) Description() string {
	return "Recommends writing ‘vice versa’ without hyphens."
}
func (r *ViceVersa) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ViceVersa) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Punctuation, Suggestions: []lint.Suggestion{lint.ReplaceWith(viceVersaReplacementFor(span.GetContent(s)))}, Message: "The expression \"vice versa\" is spelled without hyphens.", Priority: 60}
}

func viceVersaReplacementFor(template []rune) []rune {
	replacement := []rune("vice versa")
	hasUpper, hasLower, seen, firstUpper := false, false, false, false
	for _, ch := range template {
		if !unicode.IsLetter(ch) {
			continue
		}
		hasUpper = hasUpper || unicode.IsUpper(ch)
		hasLower = hasLower || unicode.IsLower(ch)
		if !seen {
			seen = true
			firstUpper = unicode.IsUpper(ch)
		}
	}
	if hasUpper && !hasLower {
		for i, ch := range replacement {
			if ch >= 'a' && ch <= 'z' {
				replacement[i] = ch - ('a' - 'A')
			}
		}
		return replacement
	}
	if !hasUpper {
		return replacement
	}
	if firstUpper {
		replacement[0] = 'V'
	}
	return replacement
}
