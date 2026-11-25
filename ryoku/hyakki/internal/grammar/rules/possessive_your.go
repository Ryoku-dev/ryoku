package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type PossessiveYour struct{ expression expr.Expr }

func NewPossessiveYour() *PossessiveYour {
	return &PossessiveYour{expression: expr.NewSequenceExprAco("you").ThenWhitespace().ThenKindIsButIsNotExcept(doc.TokenKind.IsNominal, doc.TokenKind.IsLikelyHomograph, []string{"guys", "what's"})}
}
func (r *PossessiveYour) Expr() expr.Expr { return r.expression }
func (r *PossessiveYour) Description() string {
	return "The possessive form of `you` is more likely before nouns."
}
func (r *PossessiveYour) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *PossessiveYour) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *PossessiveYour) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	if lint.PrecededByWord(c, func(w *doc.Token) bool { return w.Kind.IsVerb() }) {
		return nil
	}
	if len(t) == 0 {
		return nil
	}
	span := t[0].Span
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.WordChoice
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("your", span.GetContent(s)), lint.ReplaceWithMatchCaseStr("you're a", span.GetContent(s)), lint.ReplaceWithMatchCaseStr("you're an", span.GetContent(s))}
	result.Message = "The possessive version of this word is more common in this context."
	result.Priority = 127
	return &result
}
