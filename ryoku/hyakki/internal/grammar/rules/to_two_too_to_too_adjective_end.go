package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
	"ryoku-rashin/internal/grammar/text"
)

type ToTooAdjectiveEnd struct{ expression expr.Expr }

func NewToTooAdjectiveEnd() *ToTooAdjectiveEnd {
	return &ToTooAdjectiveEnd{expression: expr.NewSequenceExprOptional(expr.NewSequenceExprAnyWord().TWs()).TAco("to").TWs().ThenKindIsButIsNotExcept(doc.TokenKind.IsAdjective, doc.TokenKind.IsVerb, []string{"standard", "only"}).ThenOptionalWhitespace().ThenOptional(expr.NewSequenceExprAnyWord()).ThenOptionalWhitespace().ThenOptional(expr.NewSequenceExpr().ThenPunctuation())}
}
func (r *ToTooAdjectiveEnd) Expr() expr.Expr { return r.expression }
func (r *ToTooAdjectiveEnd) Description() string {
	return "Detects `to` before an adjective when no word follows (end or punct)."
}
func (r *ToTooAdjectiveEnd) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ToTooAdjectiveEnd) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
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
	j := idx + 1
	for j < len(tokens) && tokens[j].Kind.IsWhitespace() {
		j++
	}
	if j < len(tokens) {
		if !tokens[j].Kind.IsPunctuation() {
			return nil
		}
		switch tokens[j].GetStr(source) {
		case "`", "\"", "'", "“", "”", "‘", "’", "-", "–", "—":
			return nil
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
