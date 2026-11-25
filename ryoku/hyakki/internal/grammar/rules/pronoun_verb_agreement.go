package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"strings"
)

var pronounVerbAgreementNonModalAux = []string{"do", "don't", "does", "doesn't", "have", "has", "haven't", "hasn't", "dont", "doesnt", "havent", "hasnt"}
var pronounVerbAgreementIrregular = [][2]string{{"don't", "doesn't"}, {"have", "has"}, {"haven't", "hasn't"}}
var pronounVerbAgreementSubjunctive = []string{"that", "demanded", "demanding", "insisted", "insisting", "recommended", "recommending", "requested", "requesting", "suggested", "suggesting"}
var pronounVerbAgreementDitransitive = []string{"give", "gave", "given", "gives", "giving", "lose", "lost", "loses", "losing"}

type PronounVerbAgreement struct {
	expression expr.Expr
	dictionary dict.Dictionary
}

func NewPronounVerbAgreement(dictionary dict.Dictionary) *PronounVerbAgreement {
	nonThird := expr.NewSequenceExpr().ThenKindBothButNot([2]expr.KindPredicate{doc.TokenKind.IsPersonalPronoun, doc.TokenKind.IsSubjectPronoun}, doc.TokenKind.IsThirdPersonSingularPronoun).TWs().ThenVerbThirdPersonSingularPresentForm()
	third := func(t doc.Token, _ []rune) bool {
		k := t.Kind
		return k.IsSubjectPronoun() && !k.IsObjectPronoun() && k.IsPersonalPronoun() && k.IsThirdPersonSingularPronoun() && !k.IsPluralPronoun()
	}
	verb := func(t doc.Token, src []rune) bool {
		k := t.Kind
		return k.IsVerbLemma() && !k.IsVerbThirdPersonSingularPresentForm() && !k.IsVerbSimplePastForm() && !k.IsAdverb() && !k.IsConjunction() && (!k.IsAuxiliaryVerb() || text.EqAnyIgnoreASCIICaseStr(t.GetCh(src), pronounVerbAgreementNonModalAux))
	}
	return &PronounVerbAgreement{dictionary: dictionary, expression: expr.NewFirstMatchOf([]expr.Expr{nonThird, expr.NewSequenceExprWith(third).TWs().Then(verb), expr.NewSequenceExprWordSeq([]string{"it", "don't"})})}
}
func (r *PronounVerbAgreement) Expr() expr.Expr { return r.expression }
func (r *PronounVerbAgreement) Description() string {
	return "Ensures pronouns agree with their verbs."
}
func (r *PronounVerbAgreement) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *PronounVerbAgreement) thirdPersonSingularPresentToLemma(form []rune) [][]rune {
	var words [][]rune
	lower := strings.ToLower(string(form))
	if strings.HasSuffix(lower, "s") {
		words = append(words, form[:len(form)-1])
		if strings.HasSuffix(lower, "es") {
			words = append(words, form[:len(form)-2])
			if strings.HasSuffix(lower, "ies") {
				words = append(words, []rune(string(form[:len(form)-3])+"y"))
			}
		}
	}
	for _, pair := range pronounVerbAgreementIrregular {
		if text.EqStr(form, pair[1]) {
			words = append(words, []rune(pair[0]))
			break
		}
	}
	out := words[:0]
	for _, w := range words {
		if md := r.dictionary.GetWordMetadata(w); md != nil && md.IsVerbLemma() {
			out = append(out, w)
		}
	}
	return out
}
func (r *PronounVerbAgreement) lemmaToThirdPersonSingularPresent(input string) [][]rune {
	words := [][]rune{[]rune(input + "s"), []rune(input + "es")}
	if strings.HasSuffix(input, "y") {
		words = append(words, []rune(strings.TrimSuffix(input, "y")+"ies"))
	}
	for _, pair := range pronounVerbAgreementIrregular {
		if strings.EqualFold(input, pair[0]) {
			words = append(words, []rune(pair[1]))
			break
		}
	}
	out := words[:0]
	for _, w := range words {
		if md := r.dictionary.GetWordMetadata(w); md != nil && md.IsVerbThirdPersonSingularPresentForm() {
			out = append(out, w)
		}
	}
	return out
}
func (r *PronounVerbAgreement) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *PronounVerbAgreement) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	pron := toks[0]
	third := pron.Kind.IsThirdPersonSingularPronoun()
	if pron.Span.End < len(src) && src[pron.Span.End] == '-' {
		return nil
	}
	verb := toks[len(toks)-1]
	if ctx != nil && len(ctx.Before) >= 2 {
		before := ctx.Before
		prev := before[len(before)-2]
		if before[len(before)-1].Kind.IsWhitespace() {
			exempt := false
			if third {
				exempt = prev.Kind.IsAuxiliaryVerb() || text.EqAnyIgnoreASCIICaseStr(prev.GetCh(src), pronounVerbAgreementSubjunctive)
			} else if pron.Kind.IsSubjectPronoun() {
				exempt = (prev.Kind.IsPreposition() && !text.EqStr(prev.GetCh(src), "up")) || (text.EqAnyIgnoreASCIICaseStr(prev.GetCh(src), pronounVerbAgreementDitransitive) && verb.Kind.IsNoun())
			}
			if exempt {
				return nil
			}
		}
	}
	var forms [][]rune
	if third {
		forms = r.lemmaToThirdPersonSingularPresent(verb.GetStr(src))
	} else {
		forms = r.thirdPersonSingularPresentToLemma(verb.GetCh(src))
	}
	result := lint.NewLint()
	result.Span = verb.Span
	result.LintKind = lint.Agreement
	result.Message = "The form of the verb must agree in grammatical number with the pronoun."
	for _, form := range forms {
		result.Suggestions = append(result.Suggestions, lint.ReplaceWithMatchCase(form, verb.GetCh(src)))
	}
	return &result
}
