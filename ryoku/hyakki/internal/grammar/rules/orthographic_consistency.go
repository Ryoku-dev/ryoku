package rules

import (
	"fmt"
	"math/bits"
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"unicode"
)

type OrthographicConsistency struct {
	dictionary dict.Dictionary
	expression expr.Expr
}

func NewOrthographicConsistency() *OrthographicConsistency {
	return &OrthographicConsistency{dict.Curated(), expr.NewSequenceExpr().ThenAnyWord()}
}
func (r *OrthographicConsistency) Expr() expr.Expr                  { return r.expression }
func (r *OrthographicConsistency) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*OrthographicConsistency) Description() string {
	return "Ensures word casing matches the dictionary's canonical orthography."
}
func (r *OrthographicConsistency) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *OrthographicConsistency) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	if c != nil && ((len(c.Before) > 0 && c.Before[len(c.Before)-1].Kind.IsHyphen()) || (len(c.After) > 0 && c.After[0].Kind.IsHyphen())) {
		return nil
	}
	if len(t) == 0 {
		return nil
	}
	w := t[0]
	metadata, ok := w.Kind.AsWord()
	if !ok || metadata == nil {
		return nil
	}
	chars := w.GetCh(s)
	if r.dictionary.ContainsExactWord(chars) {
		return nil
	}
	current := dict.OrthFlagsFromLetters(chars)
	canonicalFlags := metadata.OrthInfo
	makeLint := func(value []rune, message string, priority uint8) *lint.Lint {
		return &lint.Lint{Span: w.Span, LintKind: lint.Capitalization, Suggestions: []lint.Suggestion{lint.ReplaceWith(value)}, Message: message, Priority: priority}
	}
	if metadata.IsAllcaps() && !current.Contains(dict.ALLCAPS) && bits.OnesCount64(uint64(metadata.OrthInfo.CaseFlags().Bits())) == 1 {
		value := append([]rune(nil), chars...)
		for i, ch := range value {
			if ch >= 'a' && ch <= 'z' {
				value[i] = ch - 'a' + 'A'
			}
		}
		p := uint8(127)
		if len(chars) > 2 {
			p = 31
		}
		return makeLint(value, "This word's canonical spelling is all-caps.", p)
	}
	canonical, hasCanonical := r.dictionary.GetCorrectCapitalizationOf(chars)
	if len(chars) >= 3 && chars[0] >= 'A' && chars[0] <= 'Z' && chars[1] >= 'A' && chars[1] <= 'Z' {
		lower := true
		for _, ch := range chars[2:] {
			if ch < 'a' || ch > 'z' {
				lower = false
				break
			}
		}
		if lower && hasCanonical && dict.OrthFlagsFromLetters(canonical).Intersects(dict.LOWERCASE|dict.TITLECASE) {
			corrected := append([]rune(nil), chars...)
			corrected[1] += 'a' - 'A'
			return makeLint(corrected, fmt.Sprintf("Did you mean `%s`? Only the first letter should be capitalized.", string(corrected)), 31)
		}
	}
	mismatch := false
	for _, flag := range []dict.OrthFlags{dict.LOWER_CAMEL, dict.UPPER_CAMEL} {
		if canonicalFlags.Contains(flag) != current.Contains(flag) && !canonicalFlags.Contains(dict.LOWERCASE) && !canonicalFlags.Contains(dict.TITLECASE) {
			mismatch = true
		}
	}
	if mismatch && hasCanonical && orthographicConsistencyAlphabeticDiffers(canonical, chars) {
		return makeLint(append([]rune(nil), canonical...), fmt.Sprintf("The canonical dictionary spelling is `%s`.", string(canonical)), 31)
	}
	if metadata.IsTitlecase() && current.Contains(dict.LOWERCASE) && hasCanonical && orthographicConsistencyAlphabeticDiffers(canonical, chars) {
		return makeLint(append([]rune(nil), canonical...), fmt.Sprintf("The canonical dictionary spelling is title case: `%s`.", string(canonical)), 127)
	}
	return nil
}

// Checks whether corresponding alphabetic characters differ, ignoring non-alphabetic characters.
func orthographicConsistencyAlphabeticDiffers(a, b []rune) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if unicode.IsLetter(a[i]) && unicode.IsLetter(b[i]) && a[i] != b[i] {
			return true
		}
	}
	return false
}
