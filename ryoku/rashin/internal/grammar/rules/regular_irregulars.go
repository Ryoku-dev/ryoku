package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"sort"
	"strings"
)

type RegularIrregulars struct {
	expression expr.Expr
	dictionary dict.Dictionary
}

func NewRegularIrregulars(dictionary dict.Dictionary) *RegularIrregulars {
	return &RegularIrregulars{expression: expr.AsExpr(func(t doc.Token, s []rune) bool {
		return t.Kind.IsOov() && text.EndsWithAnyIgnoreASCIICaseChars(t.GetCh(s), [][]rune{[]rune("s"), []rune("ed"), []rune("er"), []rune("est")})
	}), dictionary: dictionary}
}
func (r *RegularIrregulars) Expr() expr.Expr { return r.expression }
func (r *RegularIrregulars) Description() string {
	return "Replaces wrong regular inflections of words with their correct irregular forms."
}
func (r *RegularIrregulars) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *RegularIrregulars) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) != 1 {
		return nil
	}
	ch := t[0].GetCh(s)
	suggs := map[string]bool{}
	regularIrregularsHandlePluralNouns(r.dictionary, suggs, ch)
	regularIrregularsHandlePastVerbs(r.dictionary, suggs, ch)
	regularIrregularsHandleAdjectives(suggs, ch)
	if len(suggs) == 0 {
		return nil
	}
	values := make([]string, 0, len(suggs))
	for v := range suggs {
		values = append(values, v)
	}
	sort.Strings(values)
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = "'" + v + "'"
	}
	result := lint.NewLint()
	result.Span = t[0].Span
	result.LintKind = lint.Grammar
	result.Suggestions = lint.ToReplaceSuggestions(values, ch)
	result.Message = "Use the irregular form " + strings.Join(quoted, " or ") + " instead of '" + string(ch) + "'"
	result.Priority = 62
	return &result
}

func regularIrregularsHandleAdjectives(suggs map[string]bool, ch []rune) {
	if text.EndsWithIgnoreASCIICaseStr(ch, "er") {
		for _, v := range regularIrregularsGetCompar(string(ch[:len(ch)-2])) {
			suggs[v] = true
		}
	}
	if text.EndsWithIgnoreASCIICaseStr(ch, "est") {
		for _, v := range regularIrregularsGetSuper(string(ch[:len(ch)-2])) {
			suggs[v] = true
		}
	}
}
func regularIrregularsGetCompar(w string) []string {
	switch w {
	case "good", "well":
		return []string{"better"}
	case "far":
		return []string{"farther", "further"}
	case "bad":
		return []string{"worse"}
	}
	return nil
}
func regularIrregularsGetSuper(w string) []string {
	switch w {
	case "good", "well":
		return []string{"best"}
	case "far":
		return []string{"farthest", "furthest"}
	case "bad":
		return []string{"worst"}
	}
	return nil
}
func regularIrregularsHandlePluralNouns(dictionary dict.Dictionary, suggs map[string]bool, ch []rune) {
	if len(ch) == 0 || ch[len(ch)-1] != 's' {
		return
	}
	candidates := [][]rune{ch[:len(ch)-1]}
	if len(ch) >= 2 && ch[len(ch)-2] == 'e' {
		candidates = append(candidates, ch[:len(ch)-2])
	}
	for _, sg := range candidates {
		md := dictionary.GetWordMetadata(sg)
		if md == nil || !md.IsSingularNoun() {
			continue
		}
		if pl, ok := dict.CuratedIrregularNouns().GetPluralForSingular(string(sg)); ok {
			suggs[pl] = true
		}
		variants := []string{}
		w := string(sg)
		if strings.HasSuffix(w, "y") {
			variants = append(variants, strings.TrimSuffix(w, "y")+"ies")
		}
		if strings.HasSuffix(w, "fe") {
			variants = append(variants, strings.TrimSuffix(w, "fe")+"ves")
		}
		if strings.HasSuffix(w, "f") {
			variants = append(variants, strings.TrimSuffix(w, "f")+"ves")
		}
		if text.EndsWithIgnoreASCIICaseStr(sg, "o") {
			variants = append(variants, w+"es")
		}
		for _, v := range variants {
			m := dictionary.GetWordMetadataStr(v)
			if m != nil && m.IsPluralNoun() {
				suggs[v] = true
			}
		}
	}
}
func regularIrregularsHandlePastVerbs(dictionary dict.Dictionary, suggs map[string]bool, ch []rune) {
	if !text.EndsWithIgnoreASCIICaseStr(ch, "ed") {
		return
	}
	candidates := [][]rune{ch[:len(ch)-2], ch[:len(ch)-1]}
	if len(ch) >= 5 && ch[len(ch)-3] == ch[len(ch)-4] {
		candidates = append(candidates, ch[:len(ch)-3])
	}
	for _, lem := range candidates {
		md := dictionary.GetWordMetadata(lem)
		if md == nil || !md.IsVerbLemma() {
			continue
		}
		if pasts, ok := dict.CuratedIrregularVerbs().GetPastsForLemma(string(lem)); ok {
			for _, p := range pasts {
				suggs[p] = true
			}
		}
	}
}
