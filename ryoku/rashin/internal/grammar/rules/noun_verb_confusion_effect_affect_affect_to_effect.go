package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
	"ryoku-rashin/internal/grammar/text"
	"strings"
)

type AffectToEffect struct {
	expression expr.Expr
	mapping    *expr.ExprMap[int]
}

func NewAffectToEffect() *AffectToEffect {
	mapping := expr.NewExprMap[int]()
	adj := expr.NewSequenceExprWith(func(tok doc.Token, src []rune) bool {
		return nounVerbConfusionEffectAffectAffectToEffectMatchesPrecedingContext(tok, src, false, true)
	}).TWs().Then(nounVerbConfusionEffectAffectAffectToEffectIsAffectWord).TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.ADJ})).TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.NOUN}))
	mapping.Insert(adj, 2)
	word := expr.NewSequenceExprWith(func(tok doc.Token, src []rune) bool {
		return nounVerbConfusionEffectAffectAffectToEffectMatchesPrecedingContext(tok, src, true, true)
	}).TWs().Then(nounVerbConfusionEffectAffectAffectToEffectIsAffectWord).TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.PROPN, pos.INTJ, pos.ADP, pos.SCONJ}))
	mapping.Insert(word, 2)
	verb := expr.NewSequenceExprWith(func(tok doc.Token, src []rune) bool {
		return nounVerbConfusionEffectAffectAffectToEffectMatchesPrecedingContext(tok, src, true, false)
	}).TWs().Then(nounVerbConfusionEffectAffectAffectToEffectIsAffectWord).TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.AUX, pos.VERB}))
	mapping.Insert(verb, 2)
	punctuation := expr.NewSequenceExprWith(func(tok doc.Token, src []rune) bool {
		return nounVerbConfusionEffectAffectAffectToEffectMatchesPrecedingContext(tok, src, true, true)
	}).TWs().Then(nounVerbConfusionEffectAffectAffectToEffectIsAffectWord).ThenKind(doc.TokenKind.IsPunctuation)
	mapping.Insert(punctuation, 2)
	mapping.Insert(expr.NewSequenceExprAco("great").TWs().Then(nounVerbConfusionEffectAffectAffectToEffectIsAffectWord), 2)
	return &AffectToEffect{expression: mapping, mapping: mapping}
}
func (r *AffectToEffect) Expr() expr.Expr { return r.expression }
func (r *AffectToEffect) Description() string {
	return "Corrects `affect` to `effect` when the context shows the noun meaning `result`."
}
func (r *AffectToEffect) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AffectToEffect) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	index, ok := r.mapping.Lookup(0, tokens, source)
	if !ok || index >= len(tokens) {
		return nil
	}
	target := tokens[index]
	for i := index - 1; i >= 0; i-- {
		if tokens[i].Kind.IsWhitespace() {
			continue
		}
		prev := tokens[i]
		if (prev.Kind.IsPronoun() || prev.Kind.IsUpos(pos.PRON)) && !prev.Kind.IsPossessivePronoun() {
			return nil
		}
		break
	}
	replacement := ""
	switch strings.ToLower(target.GetStr(source)) {
	case "affect":
		replacement = "effect"
	case "affects":
		replacement = "effects"
	default:
		return nil
	}
	return &lint.Lint{Span: target.Span, LintKind: lint.WordChoice, Priority: 63, Message: "`affect` is usually a verb; use `effect` here for the result or outcome.", Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, target.GetCh(source))}}
}

func nounVerbConfusionEffectAffectAffectToEffectIsAffectWord(token doc.Token, source []rune) bool {
	return token.Kind.IsWord() && (text.EqStr(token.GetCh(source), "affect") || text.EqStr(token.GetCh(source), "affects"))
}
func nounVerbConfusionEffectAffectAffectToEffectIsTakeForm(chars []rune) bool {
	return text.EqAnyIgnoreASCIICaseStr(chars, []string{"take", "takes", "taking", "took", "taken"})
}
func nounVerbConfusionEffectAffectAffectToEffectIsModalLike(token doc.Token, source []rune, chars []rune) bool {
	if _, ok := expr.NewModalVerb().Matches([]doc.Token{token}, source); ok {
		return true
	}
	return text.EqAnyIgnoreASCIICaseStr(chars, []string{"do", "does", "did", "don't", "dont", "doesn't", "doesnt", "didn't", "didnt"})
}
func nounVerbConfusionEffectAffectAffectToEffectMatchesPrecedingContext(token doc.Token, source []rune, allowNounLike, allowVerbLike bool) bool {
	kind := token.Kind
	if kind.IsPossessiveNominal() || kind.IsUpos(pos.ADV) {
		return false
	}
	if !(kind.IsPunctuation() || kind.IsPreposition() || kind.IsConjunction() || kind.IsProperNoun() || kind.IsVerb() || kind.IsAdjective() || kind.IsDeterminer() || kind.IsNoun()) {
		return false
	}
	chars := token.GetCh(source)
	take := nounVerbConfusionEffectAffectAffectToEffectIsTakeForm(chars)
	if (kind.IsUpos(pos.AUX) || kind.IsAuxiliaryVerb() || nounVerbConfusionEffectAffectAffectToEffectIsModalLike(token, source, chars)) && !take {
		return false
	}
	if !allowVerbLike && kind.IsUpos(pos.VERB) && !take {
		return false
	}
	if !allowNounLike && (kind.IsNoun() || kind.IsProperNoun()) && !take {
		return false
	}
	return true
}
