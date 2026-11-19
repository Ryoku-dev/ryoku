package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type WillNonLemma struct {
	expression expr.Expr
	dictionary dict.Dictionary
}

func NewWillNonLemma(dictionary dict.Dictionary) *WillNonLemma {
	return &WillNonLemma{expression: expr.NewSequenceExprWordSet([]string{"will", "shall"}).TWs().ThenKindWhere(func(k doc.TokenKind) bool {
		return k.IsVerb() && !k.IsVerbLemma() && (!k.IsNoun() || k.IsVerbProgressiveForm()) && !(k.IsLinkingVerb() && k.IsVerbSimplePastForm())
	}), dictionary: dictionary}
}
func (r *WillNonLemma) Expr() expr.Expr                  { return r.expression }
func (r *WillNonLemma) Description() string              { return "Flags wrong verb forms after `will` or `shall`" }
func (r *WillNonLemma) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *WillNonLemma) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *WillNonLemma) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Grammar

	future, verb := toks[0], toks[len(toks)-1]
	word := verb.GetStr(src)
	template := src[span.Start:span.End]
	if strings.ToLower(future.GetStr(src)) == "will" && lint.PrecededByWord(ctx, func(t *doc.Token) bool { return t.Kind.IsDeterminer() }) {
		return nil
	}
	var previous *doc.Token
	if ctx != nil && len(ctx.Before) >= 2 {
		n := len(ctx.Before)
		if ctx.Before[n-1].Kind.IsWhitespace() && ctx.Before[n-2].Kind.IsWord() {
			previous = &ctx.Before[n-2]
		}
	}
	isLemma := func(s string) bool { m := r.dictionary.GetWordMetadataStr(s); return m != nil && m.IsVerbLemma() }
	add := func(s string) {
		result.Suggestions = append(result.Suggestions, lint.ReplaceWithMatchCaseStr(s, template))
	}
	if verb.Kind.IsVerbSimplePastForm() {
		if lemma, ok := dict.CuratedIrregularVerbs().GetLemmaForPreterite(word); ok && isLemma(lemma) {
			add("will " + lemma)
			add(word)
		}
	}
	if verb.Kind.IsVerbThirdPersonSingularPresentForm() {
		chars := []rune(word)
		candidate := string(chars[:len(chars)-1])
		if isLemma(candidate) {
			add("will " + candidate)
			add(word)
			if previous != nil && previous.Kind.IsPluralNominal() {
				add(candidate)
			}
		}
	}
	if verb.Kind.IsVerbProgressiveForm() {
		if stem, ok := strings.CutSuffix(word, "ing"); ok {
			if isLemma(stem) {
				add("will " + stem)
			}
			if isLemma(stem + "e") {
				add("will " + stem + "e")
			}
		}
		add(word)
		add("will be " + word)
	}
	result.Message = "`Will` and `shall` should be followed by a verb in its base form."
	return &result
}
