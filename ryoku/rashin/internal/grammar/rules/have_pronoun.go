package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type HavePronoun struct{ expression expr.Expr }

func NewHavePronoun() *HavePronoun {
	return &HavePronoun{expression: expr.NewSequenceExprWith(expr.NewAnchorStart()).TAco("has").TWs().ThenKindEither(doc.TokenKind.IsFirstPersonSingularPronoun, doc.TokenKind.IsPluralPronoun)}
}
func (r *HavePronoun) Expr() expr.Expr { return r.expression }
func (r *HavePronoun) Description() string {
	return "Flags questions that begin with `has` followed by a pronoun that requires `have`, such as `Has we …` or `Has I …`, and suggests the correct auxiliary."
}
func (r *HavePronoun) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *HavePronoun) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Agreement
	result.Priority = 31
	for _, tok := range toks {
		if tok.Kind.IsWord() {
			result.Span = tok.Span
			result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("have", tok.GetCh(src))}
			break
		}
	}
	result.Message = "Use `have` with first-person singular or plural pronouns."
	return &result
}
