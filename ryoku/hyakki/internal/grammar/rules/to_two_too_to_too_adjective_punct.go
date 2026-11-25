package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
	"ryoku-rashin/internal/grammar/text"
)

type ToTooAdjectivePunct struct{ expression expr.Expr }

func NewToTooAdjectivePunct() *ToTooAdjectivePunct {
	return &ToTooAdjectivePunct{expression: expr.NewSequenceExprOptional(expr.NewSequenceExprAnyWord().TWs()).TAco("to").TWs().ThenKindIsButIsNotExcept(doc.TokenKind.IsAdjective, doc.TokenKind.IsVerb, []string{"standard", "only"}).ThenOptionalWhitespace().ThenSentenceTerminator()}
}
func (r *ToTooAdjectivePunct) Expr() expr.Expr { return r.expression }
func (r *ToTooAdjectivePunct) Description() string {
	return "Detects `to` before an adjective when followed by punctuation."
}
func (r *ToTooAdjectivePunct) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ToTooAdjectivePunct) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	toIndex := -1
	for i, tok := range tokens {
		if text.EqStr(tok.GetCh(source), "to") {
			toIndex = i
			break
		}
	}
	if toIndex < 0 {
		return nil
	}
	idx := toIndex + 1
	for idx < len(tokens) && tokens[idx].Kind.IsWhitespace() {
		idx++
	}
	if idx >= len(tokens) {
		return nil
	}
	adjective := tokens[idx]
	if !adjective.Kind.IsAdjective() || !adjective.Kind.IsPositiveAdjective() || !adjective.Kind.IsUpos(pos.ADJ) || adjective.Kind.IsPreposition() {
		return nil
	}
	for i := toIndex - 1; i >= 0; i-- {
		if !tokens[i].Kind.IsWhitespace() {
			if expr.PrepositionalPreceder().MatchesToken(tokens[i], source) {
				return nil
			}
			break
		}
	}
	var target *doc.Token
	for i := range tokens {
		if text.EqStr(tokens[i].GetCh(source), "to") {
			target = &tokens[i]
			break
		}
	}
	if target == nil {
		return nil
	}
	result := lint.NewLint()
	result.Span = target.Span
	result.LintKind = lint.WordChoice
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("too", target.GetCh(source))}
	result.Message = "Use `too` here to mean ‘also’ or an excessive degree."
	return &result
}
