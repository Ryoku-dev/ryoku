package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type SplitWords struct {
	expression expr.Expr
	dictionary dict.Dictionary
}

func NewSplitWords() *SplitWords {
	return &SplitWords{expression: expr.Predicate(func(t doc.Token, _ []rune) bool { return t.Kind.IsWord() }), dictionary: dict.Curated()}
}
func (r *SplitWords) Expr() expr.Expr                  { return r.expression }
func (r *SplitWords) Description() string              { return "Finds missing spaces in improper compound words." }
func (r *SplitWords) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *SplitWords) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *SplitWords) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	word := t[0]
	if word.Kind.Metadata != nil {
		return nil
	}
	chars := word.GetCh(s)
	if informalLaughterIsInformalLaughter(chars) {
		return nil
	}
	candidates := r.dictionary.FindWordsWithCommonPrefix(chars)
	length := len(chars)
	valid := make(map[int]bool, len(candidates))
	for _, candidate := range candidates {
		if len(candidate) < length {
			valid[len(candidate)] = true
		}
	}
	mid := length / 2
	positions := make([]int, 0, length)
	positions = append(positions, mid)
	for offset := 1; offset < length; offset++ {
		if mid >= offset {
			positions = append(positions, mid-offset)
		}
		if mid+offset < length {
			positions = append(positions, mid+offset)
		}
	}
	var suggestions []lint.Suggestion
	hasAnchor := false
	message := ""
	for _, p := range positions {
		if p == 0 || p >= length || !valid[p] {
			continue
		}
		candidate, remainder := chars[:p], chars[p:]
		cm := r.dictionary.GetWordMetadataExact(candidate)
		if cm == nil || !cm.Common {
			continue
		}
		rm := r.dictionary.GetWordMetadataExact(remainder)
		if rm == nil || !rm.Common {
			continue
		}
		if splitWordsIsAnchorSplit(cm) || splitWordsIsAnchorSplit(rm) {
			hasAnchor = true
		}
		replacement := make([]rune, 0, length+1)
		replacement = append(replacement, candidate...)
		replacement = append(replacement, ' ')
		replacement = append(replacement, remainder...)
		suggestions = append(suggestions, lint.ReplaceWith(replacement))
		if len(suggestions) == 1 {
			certainty := "probably"
			if len(candidate) == 1 || len(remainder) == 1 {
				certainty = "possibly"
			}
			message = fmt.Sprintf("`%s` should %s be written as `%s %s`.", string(chars), certainty, string(candidate), string(remainder))
		}
	}
	if len(suggestions) == 0 {
		return nil
	}
	if splitWordsShouldDeferToSpellcheck(r.dictionary, chars, hasAnchor, c) {
		return nil
	}
	if len(suggestions) != 1 {
		message = fmt.Sprintf("`%s` has a missing space between words.", string(chars))
	}
	return &lint.Lint{Span: word.Span, LintKind: lint.Typo, Suggestions: suggestions, Message: message, Priority: 31}
}

// Only tagged function words anchor splits; short non-function words like
// "ha" must not block spelling corrections such as `havent` → `haven't`.
func splitWordsIsAnchorSplit(m *dict.DictWordMetadata) bool {
	return m.Preposition || m.IsDeterminer() || m.IsConjunction() || m.IsPronoun() || m.IsAdverb()
}
func splitWordsShouldDeferToSpellcheck(d dict.Dictionary, chars []rune, anchor bool, c *lint.MatchContext) bool {
	if anchor {
		return false
	}
	nounish := c != nil && (lint.AtStartOfSentence(c) || lint.PrecededByWord(c, func(t *doc.Token) bool {
		return t.Kind.IsDeterminer() || t.Kind.IsPronoun() || t.Kind.IsAdjective() || t.Kind.IsPossessiveDeterminer() || t.Kind.IsPreposition()
	}))
	if !nounish {
		return false
	}
	matches := d.FuzzyMatch(chars, 1, 1)
	return len(matches) > 0 && matches[0].EditDistance == 1
}
