package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ThereIsAgreement struct {
	expression expr.Expr
	dictionary dict.Dictionary
}

func NewThereIsAgreement(dictionary dict.Dictionary) *ThereIsAgreement {
	plural := expr.AsExpr(func(t doc.Token, _ []rune) bool { return t.Kind.IsPluralNoun() })
	singular := expr.ButNot(expr.NewSequenceExpr().ThenSingularNoun(), expr.NewFirstMatchOf([]expr.Expr{
		expr.AsExpr(func(t doc.Token, s []rune) bool {
			return t.Kind.IsAdjective() || text.EqAnyIgnoreASCIICaseStr(t.GetCh(s), []string{"no", "not"})
		}), expr.NewSpelledNumberExpr(),
	}))
	var choices []expr.Expr
	for _, pluralBe := range []bool{false, true} {
		for _, question := range []bool{false, true} {
			for _, past := range []bool{false, true} {
				be := "is"
				if pluralBe {
					be = "are"
				}
				if past {
					if pluralBe {
						be = "were"
					} else {
						be = "was"
					}
				}
				first, second := "there", be
				if question {
					first, second = be, "there"
				}
				seq := expr.NewSequenceExprAco(first).TWs().TAco(second).TWs()
				if pluralBe {
					seq.Then(singular)
				} else {
					seq.Then(plural)
				}
				choices = append(choices, seq)
			}
		}
	}
	choices = append(choices, expr.NewSequenceExprFixedPhrase("there's ").Then(plural))
	return &ThereIsAgreement{expression: expr.NewFirstMatchOf(choices), dictionary: dictionary}
}
func (r *ThereIsAgreement) Expr() expr.Expr                  { return r.expression }
func (r *ThereIsAgreement) Unit() lint.DocumentIterator      { return lint.Sentence{} }
func (r *ThereIsAgreement) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*ThereIsAgreement) Description() string {
	return "Checks for `is there` and its variants agreeing with singular vs plural subjects"
}
func (r *ThereIsAgreement) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *ThereIsAgreement) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if len(toks) < 3 {
		return nil
	}
	first := toks[0].GetCh(src)
	if !text.StartsWithIgnoreASCIICaseStr(first, "there") && text.EqStr(toks[2].GetCh(src), "there") {
		return r.handleQuestion(toks, src, ctx)
	}
	if text.StartsWithIgnoreASCIICaseStr(first, "there") {
		if len(first) > 0 && first[len(first)-1] == 's' {
			return r.handleTheres(toks, src)
		}
		return r.handleStatement(toks, src, ctx)
	}
	return nil
}
func thereIsAgreementNewBe(original []rune) []rune {
	if len(original) < 2 {
		return original
	}
	switch string(text.ToLower(original[:2])) {
	case "is":
		return []rune("are")
	case "ar":
		return []rune("is")
	case "wa":
		return []rune("were")
	case "we":
		return []rune("was")
	}
	return original
}

type thereIsAgreementNoun struct {
	noun    []rune
	article string
}

func (r *ThereIsAgreement) newNouns(original []rune, pluralBe bool) []thereIsAgreementNoun {
	var nouns [][]rune
	irregular := dict.CuratedIrregularNouns()
	if pluralBe {
		if n, ok := irregular.GetPluralForSingularChars(original); ok {
			nouns = append(nouns, []rune(n))
		}
		nouns = append(nouns, dict.GetPlurals(r.dictionary, original)...)
	} else {
		if n, ok := irregular.GetSingularForPluralChars(original); ok {
			nouns = append(nouns, []rune(n))
		}
		nouns = append(nouns, dict.GetSingulars(r.dictionary, original)...)
	}
	result := make([]thereIsAgreementNoun, 0, len(nouns))
	for _, n := range nouns {
		article := "a"
		if sound, ok := dict.StartsWithVowel(n, dict.American); ok && sound == dict.Vowel {
			article = "an"
		}
		result = append(result, thereIsAgreementNoun{noun: n, article: article})
	}
	return result
}
func thereIsAgreementArticle(original []rune) string {
	if sound, ok := dict.StartsWithVowel(original, dict.American); ok {
		if sound == dict.Vowel {
			return "an"
		}
		if sound == dict.Consonant {
			return "a"
		}
	}
	return "🅰️🅰️"
}
func (r *ThereIsAgreement) handleStatement(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if len(toks) < 5 || lint.FollowedByHyphen(ctx) {
		return nil
	}
	if lint.FollowedByWord(ctx, func(t *doc.Token) bool {
		return (t.Kind.IsNoun() && !t.Kind.IsVerbProgressiveForm() && !text.EqAnyIgnoreASCIICaseStr(t.GetCh(src), []string{"but", "in", "like"})) || text.EqAnyIgnoreASCIICaseStr(t.GetCh(src), []string{"and", "or"})
	}) {
		return nil
	}
	span, ok := doc.Span(toks[2:5])
	if !ok {
		return nil
	}
	template := span.GetContent(src)
	be, noun := toks[2].GetCh(src), toks[4].GetCh(src)
	pluralBe := text.EqAnyIgnoreASCIICaseStr(be, []string{"are", "were"})
	replacement := string(thereIsAgreementNewBe(be))
	if pluralBe {
		replacement += " " + thereIsAgreementArticle(noun)
	}
	replacement += " " + string(noun)
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Agreement
	l.Message = "There is disagreement in number between the verb and the noun."
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, template)}
	for _, n := range r.newNouns(noun, pluralBe) {
		replacement := string(be)
		if !pluralBe {
			replacement += " " + n.article
		}
		replacement += " " + string(n.noun)
		l.Suggestions = append(l.Suggestions, lint.ReplaceWithMatchCaseStr(replacement, template))
	}
	return &l
}
func (r *ThereIsAgreement) handleTheres(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks[:3])
	if !ok {
		return nil
	}
	template := span.GetContent(src)
	be, noun := toks[0].GetCh(src), toks[2].GetCh(src)
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Agreement
	l.Message = "`There's` means `there is`, which requires a singular noun."
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("there are "+string(noun), template)}
	for _, n := range r.newNouns(noun, false) {
		l.Suggestions = append(l.Suggestions, lint.ReplaceWithMatchCaseStr(string(be)+" "+n.article+" "+string(n.noun), template))
	}
	return &l
}
func (r *ThereIsAgreement) handleQuestion(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if len(toks) < 5 {
		return nil
	}
	if lint.FollowedByWord(ctx, func(t *doc.Token) bool {
		return t.Kind.IsNoun() && !t.Kind.IsVerbProgressiveForm() && !text.EqAnyIgnoreASCIICaseStr(t.GetCh(src), []string{"in", "or"})
	}) {
		return nil
	}
	span, ok := doc.Span(toks[:5])
	if !ok {
		return nil
	}
	template := span.GetContent(src)
	be, noun := toks[0].GetCh(src), toks[4].GetCh(src)
	pluralBe := text.EqAnyIgnoreASCIICaseStr(be, []string{"are", "were"})
	replacement := string(thereIsAgreementNewBe(be)) + " there"
	if pluralBe {
		replacement += " " + thereIsAgreementArticle(noun)
	}
	replacement += " " + string(noun)
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Agreement
	l.Message = "There is disagreement in number between the verb and the noun."
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, template)}
	for _, n := range r.newNouns(noun, pluralBe) {
		replacement := string(be) + " there"
		if !pluralBe {
			replacement += " " + n.article
		}
		replacement += " " + string(n.noun)
		l.Suggestions = append(l.Suggestions, lint.ReplaceWithMatchCaseStr(replacement, template))
	}
	return &l
}
