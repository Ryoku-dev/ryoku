package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"strings"
	"unicode"
)

type SingleBe struct{ expression expr.Expr }

func NewSingleBe() *SingleBe {
	return &SingleBe{expression: expr.NewSequenceExprWith(expr.NewSequenceExprAnyOf([]expr.Expr{singleBeLikeExpr(), expr.AsExpr(singleBeLooksLikeBeContraction)})).TWs().Then(singleBeLikeExpr())}
}
func (r *SingleBe) Expr() expr.Expr                  { return r.expression }
func (r *SingleBe) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*SingleBe) Description() string {
	return "Removes adjacent duplicate inflections of `be`, including contracted forms followed by another `be` verb."
}
func (r *SingleBe) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 3 {
		return nil
	}
	first, second := toks[0], toks[len(toks)-1]
	chars := first.GetCh(src)
	if first.Kind.IsPossessiveNominal() && (first.Kind.IsProperNoun() || (len(chars) > 0 && unicode.IsUpper(chars[0]))) {
		return nil
	}
	for _, t := range []doc.Token{first, second} {
		if forms, ok := singleBeForms(t); ok && forms.Intersects(dict.PROGRESSIVE|dict.PAST_PARTICIPLE) {
			return nil
		}
	}
	firstPast, secondPast := first.Kind.IsVerbPastForm(), second.Kind.IsVerbPastForm()
	if f, ok := singleBeForms(first); ok {
		firstPast = f.Intersects(dict.PAST | dict.PRETERITE)
	}
	if f, ok := singleBeForms(second); ok {
		secondPast = f.Intersects(dict.PAST | dict.PRETERITE)
	}
	if idx := strings.LastIndexAny(string(chars), "'’"); idx >= 0 {
		base := []rune(string(chars)[:idx])
		common := text.EqAnyIgnoreASCIICaseStr(base, []string{"i", "we", "they", "you", "he", "she", "it", "that", "there"})
		if len(base) > 0 && unicode.IsUpper(base[0]) && !first.Kind.IsPronoun() && !common {
			return nil
		}
	}
	if firstPast && secondPast {
		return nil
	}
	return &lint.Lint{Span: text.Span{Start: toks[1].Span.Start, End: second.Span.End}, LintKind: lint.Grammar, Suggestions: []lint.Suggestion{lint.Remove()}, Message: "Drop the repeated verb form so only one instance of `be` remains.", Priority: 31}
}

func singleBeForms(t doc.Token) (dict.VerbFormFlags, bool) {
	m, ok := t.Kind.AsWord()
	if !ok || m == nil || m.Verb == nil || m.Verb.VerbForms == nil {
		return 0, false
	}
	return *m.Verb.VerbForms, true
}
func singleBeLikeExpr() expr.Expr {
	return expr.NewSequenceExprAnyOf([]expr.Expr{expr.AsExpr(expr.NewInflectionOfBe()), expr.AsExpr(expr.NewDerivedFromStr("be"))})
}
func singleBeLooksLikeBeContraction(t doc.Token, src []rune) bool {
	if !t.Kind.IsWord() || (t.Kind.IsPossessiveNominal() && t.Kind.IsProperNoun()) {
		return false
	}
	content := t.GetCh(src)
	idx := -1
	for i, c := range content {
		if c == '\'' || c == '’' {
			idx = i
		}
	}
	if idx < 0 {
		return false
	}
	base := content[:idx]
	if len(base) > 0 && unicode.IsUpper(base[0]) && t.Kind.IsNominal() && !t.Kind.IsPronoun() {
		return false
	}
	if text.EqStr(base, "let") {
		return false
	}
	suffix := content[idx+1:]
	return idx > 0 && text.EqAnyIgnoreASCIICaseStr(suffix, []string{"s", "re", "m"})
}
