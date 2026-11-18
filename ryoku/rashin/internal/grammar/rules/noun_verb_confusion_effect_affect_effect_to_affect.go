package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
	"ryoku-rashin/internal/grammar/text"
	"strings"
)

type EffectToAffect struct {
	expression expr.Expr
	mapping    *expr.ExprMap[int]
}

func NewEffectToAffect() *EffectToAffect {
	mapping := expr.NewExprMap[int]()
	mapping.Insert(expr.NewSequenceExprWith(nounVerbConfusionEffectAffectEffectToAffectMatchesPrecedingContext).TWs().Then(nounVerbConfusionEffectAffectEffectToAffectIsEffectWord).TWs().Then(nounVerbConfusionEffectAffectEffectToAffectMatchesFollowingContext).ThenOptionalWhitespace().ThenOptional(nounVerbConfusionEffectAffectEffectToAffectMatchesOptionalFollowing).ThenOptionalWhitespace(), 2)
	return &EffectToAffect{expression: mapping, mapping: mapping}
}
func (r *EffectToAffect) Expr() expr.Expr { return r.expression }
func (r *EffectToAffect) Description() string {
	return "Corrects `effect` to `affect` when the context shows the verb meaning `influence`."
}
func (r *EffectToAffect) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *EffectToAffect) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	index, ok := r.mapping.Lookup(0, tokens, source)
	if !ok || index >= len(tokens) {
		return nil
	}
	target := tokens[index]
	var preceding *doc.Token
	for i := index - 1; i >= 0; i-- {
		if !tokens[i].Kind.IsWhitespace() {
			preceding = &tokens[i]
			break
		}
	}
	var first, second *doc.Token
	for i := index + 1; i < len(tokens); i++ {
		if !tokens[i].Kind.IsWhitespace() {
			if first == nil {
				first = &tokens[i]
			} else {
				second = &tokens[i]
				break
			}
		}
	}
	if first == nil {
		return nil
	}
	if preceding != nil {
		switch strings.ToLower(preceding.GetStr(source)) {
		case "take", "takes", "taking", "took", "taken", "side":
			return nil
		}
	}
	if first.Kind.IsUpos(pos.AUX) || first.Kind.IsLinkingVerb() {
		return nil
	}
	switch strings.ToLower(first.GetStr(source)) {
	case "is", "are", "was", "were", "be", "been", "being":
		return nil
	}
	if preceding != nil && text.EqStr(preceding.GetCh(source), "to") && nounVerbConfusionEffectAffectEffectToAffectIsChangeLike(*first, source) {
		return nil
	}
	if first.Kind.IsUpos(pos.VERB) && preceding != nil {
		kind := preceding.Kind
		if kind.IsUpos(pos.NOUN) || kind.IsUpos(pos.DET) || kind.IsUpos(pos.ADJ) || (kind.IsNoun() && !kind.IsUpos(pos.VERB) && !kind.IsUpos(pos.AUX)) {
			return nil
		}
	}
	if preceding != nil && (preceding.Kind.IsUpos(pos.DET) || preceding.Kind.IsUpos(pos.ADJ)) {
		return nil
	}
	if second != nil && second.Kind.IsNoun() && nounVerbConfusionEffectAffectEffectToAffectIsChangeLike(*second, source) {
		return nil
	}
	lower := strings.ToLower(target.GetStr(source))
	if lower == "effects" && preceding != nil && preceding.Kind.IsUpos(pos.VERB) {
		return nil
	}
	replacement := ""
	switch lower {
	case "effect":
		replacement = "affect"
	case "effects":
		replacement = "affects"
	default:
		return nil
	}
	return &lint.Lint{Span: target.Span, LintKind: lint.WordChoice, Priority: 63, Message: "Use `affect` for the verb meaning to influence; `effect` usually names the result.", Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, target.GetCh(source))}}
}

func nounVerbConfusionEffectAffectEffectToAffectIsEffectWord(token doc.Token, source []rune) bool {
	return token.Kind.IsWord() && (text.EqStr(token.GetCh(source), "effect") || text.EqStr(token.GetCh(source), "effects"))
}
func nounVerbConfusionEffectAffectEffectToAffectIsChangeLike(token doc.Token, source []rune) bool {
	if !token.Kind.IsWord() {
		return false
	}
	switch strings.ToLower(token.GetStr(source)) {
	case "change", "changes", "substitution", "substitutions":
		return true
	}
	return false
}
func nounVerbConfusionEffectAffectEffectToAffectMatchesPrecedingContext(token doc.Token, _ []rune) bool {
	return nounVerbConfusionEffectAffectEffectToAffectTagMatchesAny(token, []pos.UPOS{pos.PART, pos.NOUN, pos.PRON, pos.PROPN, pos.ADV, pos.AUX, pos.VERB, pos.ADJ})
}
func nounVerbConfusionEffectAffectEffectToAffectMatchesFollowingContext(token doc.Token, _ []rune) bool {
	return nounVerbConfusionEffectAffectEffectToAffectTagMatchesAny(token, []pos.UPOS{pos.ADV, pos.AUX, pos.PRON, pos.PROPN, pos.VERB, pos.NUM, pos.NOUN, pos.INTJ, pos.SCONJ, pos.DET, pos.ADJ})
}
func nounVerbConfusionEffectAffectEffectToAffectMatchesOptionalFollowing(token doc.Token, _ []rune) bool {
	return token.Kind.IsPunctuation() || nounVerbConfusionEffectAffectEffectToAffectTagMatchesAny(token, []pos.UPOS{pos.NOUN})
}
func nounVerbConfusionEffectAffectEffectToAffectTagMatchesAny(token doc.Token, allowed []pos.UPOS) bool {
	meta, ok := token.Kind.AsWord()
	if !ok {
		return false
	}
	if meta == nil || meta.PosTag == nil {
		return true
	}
	for _, tag := range allowed {
		if *meta.PosTag == tag {
			return true
		}
	}
	return false
}
